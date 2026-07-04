package crawlora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAPIKeyAuthAndQueryParams(t *testing.T) {
	var gotPath, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotKey = r.Header.Get("x-api-key")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"ok": true}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	out, err := client.Bing.Search(context.Background(), Params{"q": "coffee", "count": 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	body := out.(map[string]any)
	if body["data"].(map[string]any)["ok"] != true {
		t.Fatalf("unexpected response %#v", out)
	}
	if gotPath != "/api/v1/bing/search?count=3&q=coffee" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotKey != "api_test" {
		t.Fatalf("x-api-key = %q", gotKey)
	}
}

func TestJWTAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithJWTToken("jwt_test"))
	if _, err := client.User.Me(context.Background(), nil); err != nil {
		t.Fatalf("me: %v", err)
	}
	if gotAuth != "Token jwt_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestJWTAuthSchemeIsCaseInsensitive(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithJWTToken("bearer jwt_test"))
	if _, err := client.User.Me(context.Background(), nil); err != nil {
		t.Fatalf("me: %v", err)
	}
	if gotAuth != "bearer jwt_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestMissingRequiredParamsFailBeforeRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Bing.Search(context.Background(), Params{}); err == nil || !strings.Contains(err.Error(), "missing required query parameter: q") {
		t.Fatalf("missing query error = %v", err)
	}
	if _, err := client.Google.Search(context.Background(), Params{}); err == nil || !strings.Contains(err.Error(), "missing required body parameter: searchOption") {
		t.Fatalf("missing body error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("network calls = %d", calls)
	}
}

func TestInvalidEnumParamsFailBeforeRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Request(context.Background(), "amazon-product", Params{
		"asin":     "B000000000",
		"language": "fr_FR",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid query parameter language: expected one of en_US") {
		t.Fatalf("enum error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("network calls = %d", calls)
	}
}

func TestValidEnumParamSerializes(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Request(context.Background(), "amazon-product", Params{
		"asin":     "B000000000",
		"language": "en_US",
	}); err != nil {
		t.Fatalf("product: %v", err)
	}
	if !strings.Contains(gotQuery, "language=en_US") {
		t.Fatalf("expected language enum in %q", gotQuery)
	}
}

func TestNegativeRetryOptionsAreNormalized(t *testing.T) {
	client := NewClient(WithRetries(-3), WithRetryDelay(-1))
	if client.Retries != 0 {
		t.Fatalf("Retries = %d", client.Retries)
	}
	if client.RetryDelay != 0 {
		t.Fatalf("RetryDelay = %v", client.RetryDelay)
	}
}

func TestTextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	out, err := client.YouTube.Transcript(context.Background(), Params{"id": "abc123", "format": "text"})
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %#v", out)
	}
}

func TestRequestOptionsHeadersAndUserAgent(t *testing.T) {
	var gotHeader, gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-test")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Bing.Search(context.Background(), Params{"q": "coffee"}, WithRequestHeader("x-test", "yes")); err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotHeader != "yes" {
		t.Fatalf("x-test = %q", gotHeader)
	}
	if gotUA != "crawlora-go-sdk/"+Version {
		t.Fatalf("User-Agent = %q", gotUA)
	}
}

func TestRequestHeadersOverrideDefaultAuthAndContentHeaders(t *testing.T) {
	var gotKey, gotContentType string
	var gotHeader http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotContentType = r.Header.Get("content-type")
		gotHeader = r.Header.Clone()
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_default"))
	_, err := client.Google.Search(context.Background(), Params{"searchOption": Params{"q": "coffee"}},
		WithRequestHeader("X-API-KEY", "api_request"),
		WithRequestHeader("Content-Type", "application/custom+json"),
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotKey != "api_request" {
		t.Fatalf("x-api-key = %q", gotKey)
	}
	if gotContentType != "application/custom+json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if len(gotHeader.Values("x-api-key")) != 1 {
		t.Fatalf("x-api-key values = %#v", gotHeader.Values("x-api-key"))
	}
	if len(gotHeader.Values("content-type")) != 1 {
		t.Fatalf("content-type values = %#v", gotHeader.Values("content-type"))
	}
}

func TestInvalidResponseTypeFailsBeforeRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Bing.Search(context.Background(), Params{"q": "coffee"}, WithResponseType("xml"))
	if err == nil || !strings.Contains(err.Error(), "invalid response type: expected one of auto, json, text") {
		t.Fatalf("response type error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("network calls = %d", calls)
	}
}

func TestQuerySerializationKeepsFalseZero(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Request(context.Background(), "datasets-google-map-businesses-search", Params{
		"q":           "coffee",
		"page":        0,
		"has_website": false,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(gotQuery, "page=0") {
		t.Fatalf("expected page=0 in %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "has_website=false") {
		t.Fatalf("expected has_website=false in %q", gotQuery)
	}
}

func TestQuerySerializationRepeatsArrays(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Request(context.Background(), "tripadvisor-search", Params{
		"q":              "hotel",
		"geo_id":         "293919",
		"type":           "hotel",
		"amenities":      []int{1, 2},
		"online_options": []string{"3", "4"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(gotQuery, "amenities=1") || !strings.Contains(gotQuery, "amenities=2") {
		t.Fatalf("expected repeated amenities in %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "online_options=3") || !strings.Contains(gotQuery, "online_options=4") {
		t.Fatalf("expected repeated online_options in %q", gotQuery)
	}
}

func TestAPIErrorIncludesStatusCodeAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 429, "msg": "rate limited"})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Bing.Search(context.Background(), Params{"q": "coffee"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	if apiErr.Status != http.StatusTooManyRequests || apiErr.Code != 429 || apiErr.Error() != "rate limited" {
		t.Fatalf("unexpected error %#v", apiErr)
	}
	if apiErr.Headers.Get("content-type") != "application/json" {
		t.Fatalf("content-type header = %q", apiErr.Headers.Get("content-type"))
	}
	if apiErr.RawBody == "" {
		t.Fatal("expected raw body")
	}
}

func TestInvalidJSONResponseIsWrapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	_, err := client.Bing.Search(context.Background(), Params{"q": "coffee"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	if apiErr.Status != http.StatusOK || apiErr.Error() != "crawlora JSON parse error" || apiErr.Err == nil {
		t.Fatalf("unexpected parse error %#v", apiErr)
	}
	if apiErr.Headers.Get("content-type") != "application/json" {
		t.Fatalf("content-type header = %q", apiErr.Headers.Get("content-type"))
	}
	if apiErr.RawBody != "{not-json" {
		t.Fatalf("raw body = %q", apiErr.RawBody)
	}
}

func TestRetriesRetryableStatus(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 503, "msg": "try again"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"ok": true}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"), WithRetries(1), WithRetryDelay(0))
	if _, err := client.Bing.Search(context.Background(), Params{"q": "coffee"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestRetryDelayHonorsRetryAfterHeader(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0.001")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 429, "msg": "slow down"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"ok": true}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"), WithRetries(1), WithRetryDelay(time.Hour))
	start := time.Now()
	if _, err := client.Bing.Search(context.Background(), Params{"q": "coffee"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("retry-after delay took too long: %v", elapsed)
	}
}

func TestContextCancellationIsPreserved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not reach server")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"), WithRetries(1))
	_, err := client.Bing.Search(ctx, Params{"q": "coffee"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %T %v", err, err)
	}
}

func TestOperationMetadataCount(t *testing.T) {
	if len(operations) != operationCount {
		t.Fatalf("operations = %d, operationCount = %d", len(operations), operationCount)
	}
	if operationCount != 532 {
		t.Fatalf("operationCount = %d", operationCount)
	}
}

func TestDeprecatedEndpointsAreNotGenerated(t *testing.T) {
	if _, ok := operations["google-lens"]; ok {
		t.Fatal("google-lens should not be generated")
	}
}

func TestDocsCoverOperationsAndRecipes(t *testing.T) {
	operationsDoc, err := os.ReadFile("docs/operations.md")
	if err != nil {
		t.Fatalf("read operations docs: %v", err)
	}
	recipesDoc, err := os.ReadFile("docs/recipes.md")
	if err != nil {
		t.Fatalf("read recipes docs: %v", err)
	}
	operationsText := string(operationsDoc)
	recipesText := string(recipesDoc)
	for _, want := range []string{
		"Total operations: `532`",
		"`bing-search`",
		"`GET /bing/search`",
		"`Bing.Search`",
		"`BingSearchResponse`",
		"`shopify-store`",
		"`GET /shopify/store`",
		"`Shopify.Store`",
	} {
		if !strings.Contains(operationsText, want) {
			t.Fatalf("operations docs missing %q", want)
		}
	}
	if strings.Contains(operationsText, "google-lens") || strings.Contains(strings.ToLower(operationsText), "deprecated") && strings.Contains(operationsText, "`google-lens`") {
		t.Fatal("operations docs should not include deprecated google-lens")
	}
	for _, want := range []string{"RequestTyped", "OperationBingSearch", "WithResponseType", "Crawlora Go SDK Recipes", "Retry-After", "apiErr.Headers"} {
		if !strings.Contains(recipesText, want) {
			t.Fatalf("recipes docs missing %q", want)
		}
	}
}

func TestTypedEndpointQueryParams(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"ok": true}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Bing.SearchTyped(context.Background(), BingSearchParams{
		Q:     "coffee",
		Count: Int(0),
	}); err != nil {
		t.Fatalf("typed search: %v", err)
	}
	if gotPath != "/api/v1/bing/search?count=0&q=coffee" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestTypedEndpointResponseStruct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"msg":  "OK",
			"data": map[string]any{
				"results": []map[string]any{
					{"title": "Coffee result", "url": "https://example.test/coffee"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	out, err := client.Bing.SearchTyped(context.Background(), BingSearchParams{Q: "coffee"})
	if err != nil {
		t.Fatalf("typed search: %v", err)
	}
	if out.Data.Results[0].Title != "Coffee result" {
		t.Fatalf("typed title = %q", out.Data.Results[0].Title)
	}
}

func TestPointerHelpers(t *testing.T) {
	if *String("coffee") != "coffee" {
		t.Fatal("String helper returned wrong value")
	}
	if *Int(3) != 3 {
		t.Fatal("Int helper returned wrong value")
	}
	if *Bool(false) != false {
		t.Fatal("Bool helper returned wrong value")
	}
	if *Float64(1.25) != 1.25 {
		t.Fatal("Float64 helper returned wrong value")
	}
}

func TestRequestTypedHelperAndOperationConstant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"msg":  "OK",
			"data": map[string]any{
				"results": []map[string]any{
					{"title": "Coffee result"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	out, err := RequestTyped[BingSearchResponse](client, context.Background(), OperationBingSearch, Params{"q": "coffee"})
	if err != nil {
		t.Fatalf("request typed: %v", err)
	}
	if out.Data.Results[0].Title != "Coffee result" {
		t.Fatalf("typed title = %q", out.Data.Results[0].Title)
	}
}

func TestErrorClassification(t *testing.T) {
	cases := []struct {
		status   int
		isClient bool
		isServer bool
	}{
		{status: http.StatusNotFound, isClient: true},
		{status: http.StatusInternalServerError, isServer: true},
	}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(tc.status)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.status, "msg": "err"})
		}))
		client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
		_, err := client.Bing.Search(context.Background(), Params{"q": "coffee"})
		server.Close()
		if tc.isClient && !errors.Is(err, ErrClient) {
			t.Fatalf("status %d: expected ErrClient, got %v", tc.status, err)
		}
		if tc.isServer && !errors.Is(err, ErrServer) {
			t.Fatalf("status %d: expected ErrServer, got %v", tc.status, err)
		}
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.IsClientError() != tc.isClient {
			t.Fatalf("status %d: IsClientError = %v", tc.status, apiErr.IsClientError())
		}
	}
}

func TestNetworkErrorClassification(t *testing.T) {
	client := NewClient(WithBaseURL("http://127.0.0.1:1/api/v1"), WithAPIKey("api_test"))
	_, err := client.Bing.Search(context.Background(), Params{"q": "coffee"})
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("expected ErrNetwork, got %v", err)
	}
}

func TestPaginateAdvancesAndStopsOnEmpty(t *testing.T) {
	var seenPages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		seenPages = append(seenPages, page)
		w.Header().Set("content-type", "application/json")
		data := []map[string]any{}
		if page != "3" {
			data = append(data, map[string]any{"page": page})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": data})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	var pages int
	err := client.Paginate(context.Background(), "ebay-seller-feedback", Params{"seller": "acme"}, func(page any) error {
		pages++
		return nil
	})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if pages != 3 {
		t.Fatalf("pages = %d", pages)
	}
	if strings.Join(seenPages, ",") != "1,2,3" {
		t.Fatalf("seen pages = %v", seenPages)
	}
}

func TestPaginateStopError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": []map[string]any{{"x": 1}}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	calls := 0
	err := client.Paginate(context.Background(), "ebay-seller-feedback", Params{"seller": "acme"}, func(page any) error {
		calls++
		return ErrStopPagination
	})
	if err != nil {
		t.Fatalf("paginate stop: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestPaginateRequiresPageParam(t *testing.T) {
	client := NewClient(WithAPIKey("api_test"))
	err := client.Paginate(context.Background(), "user-me", nil, func(page any) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "no page or offset query parameter") {
		t.Fatalf("expected page param error, got %v", err)
	}
}

func TestRetryPredicateAndOnRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 500, "msg": "x"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"ok": true}})
	}))
	defer server.Close()

	var retries []int
	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"), WithRetries(1), WithRetryDelay(0),
		WithRetryPredicate(func(status int, err error) bool { return status == 500 }),
		WithOnRetry(func(attempt int, err error, delay time.Duration) { retries = append(retries, attempt) }),
	)
	if _, err := client.Bing.Search(context.Background(), Params{"q": "c"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if calls != 2 || len(retries) != 1 || retries[0] != 1 {
		t.Fatalf("calls=%d retries=%v", calls, retries)
	}
}

func TestRequestIDGenerated(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-request-id")
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 500, "msg": "x"})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"), WithRequestID(true))
	_, err := client.Bing.Search(context.Background(), Params{"q": "c"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if got == "" || apiErr.RequestID != got {
		t.Fatalf("request id header=%q error=%q", got, apiErr.RequestID)
	}
}

func TestPaginateItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("content-type", "application/json")
		data := []map[string]any{}
		if page != "3" {
			data = append(data, map[string]any{"page": page})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": data})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"))
	var items int
	if err := client.PaginateItems(context.Background(), "ebay-seller-feedback", Params{"seller": "a"}, func(item any) error {
		items++
		return nil
	}); err != nil {
		t.Fatalf("paginate items: %v", err)
	}
	if items != 2 {
		t.Fatalf("items=%d", items)
	}
}

func TestCursorPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := r.URL.Query().Get("cursor")
		next := map[string]string{"": "a", "a": "b", "b": ""}[cur]
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": []any{cur}, "next": next})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"))
	pages := 0
	err := client.Paginate(context.Background(), "producthunt-leaderboard", nil, func(page any) error {
		pages++
		return nil
	}, WithCursorParam("cursor"), WithNextCursor(func(page any) any {
		if m, ok := page.(map[string]any); ok {
			if n, ok := m["next"].(string); ok && n != "" {
				return n
			}
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("cursor paginate: %v", err)
	}
	if pages != 3 {
		t.Fatalf("pages=%d", pages)
	}
}

func TestCursorParamMustBeQueryParam(t *testing.T) {
	client := NewClient(WithAPIKey("k"))
	err := client.Paginate(context.Background(), "producthunt-leaderboard", nil, func(page any) error { return nil },
		WithCursorParam("bogus"), WithNextCursor(func(page any) any { return nil }))
	if err == nil || !strings.Contains(err.Error(), "is not a query parameter") {
		t.Fatalf("expected cursor param error, got %v", err)
	}
}

func TestStreamingResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/octet-stream")
		_, _ = w.Write([]byte("streamed-bytes"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"))
	out, err := client.Bing.Search(context.Background(), Params{"q": "c"}, WithResponseType(ResponseStream))
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	body, ok := out.(io.ReadCloser)
	if !ok {
		t.Fatalf("expected io.ReadCloser, got %T", out)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "streamed-bytes" {
		t.Fatalf("stream data=%q", data)
	}
}

func TestEnvVarConfig(t *testing.T) {
	t.Setenv("CRAWLORA_API_KEY", "env_key")
	t.Setenv("CRAWLORA_BASE_URL", "https://env.example/api/v1")
	client := NewClient()
	if client.APIKey != "env_key" {
		t.Fatalf("api key=%q", client.APIKey)
	}
	if client.BaseURL != "https://env.example/api/v1" {
		t.Fatalf("base url=%q", client.BaseURL)
	}
	explicit := NewClient(WithAPIKey("explicit"))
	if explicit.APIKey != "explicit" {
		t.Fatalf("explicit api key=%q", explicit.APIKey)
	}
}

func TestBeforeRequestAndAfterResponse(t *testing.T) {
	var gotSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("x-sig")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"),
		WithBeforeRequest(func(req *http.Request) error { req.Header.Set("x-sig", "signed"); return nil }),
		WithAfterResponse(func(operationID string, status int, headers http.Header, body any) (any, error) {
			return map[string]any{"op": operationID}, nil
		}),
	)
	out, err := client.Bing.Search(context.Background(), Params{"q": "c"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotSig != "signed" {
		t.Fatalf("x-sig = %q", gotSig)
	}
	if m, ok := out.(map[string]any); !ok || m["op"] != "bing-search" {
		t.Fatalf("after response = %#v", out)
	}
}

func TestIdempotencyKeyStableAcrossRetries(t *testing.T) {
	var keys []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		calls++
		w.Header().Set("content-type", "application/json")
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 503})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"), WithRetries(1), WithRetryDelay(0), WithIdempotencyKeys(true))
	if _, err := client.Google.Search(context.Background(), Params{"searchOption": Params{"q": "c"}}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("idempotency keys = %#v", keys)
	}
}

func TestPerRequestRetriesOverride(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 503})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"), WithRetries(5), WithRetryDelay(0))
	_, err := client.Bing.Search(context.Background(), Params{"q": "c"}, WithRequestRetries(0))
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d (expected 1 with per-request retries=0)", calls)
	}
}

func TestMaxConcurrencyCapsInFlight(t *testing.T) {
	var mu sync.Mutex
	active, maxActive := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(15 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("k"), WithMaxConcurrency(2))
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Bing.Search(context.Background(), Params{"q": "c"})
		}()
	}
	wg.Wait()
	if maxActive > 2 {
		t.Fatalf("max in-flight = %d (cap 2)", maxActive)
	}
}

func TestTypedEndpointJSONBody(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		gotBody = buf.String()
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Google.SearchTyped(context.Background(), GoogleSearchParams{
		SearchOption: ModelGoogleSearchOption{
			Country:  "us",
			Keyword:  "coffee",
			Language: "en",
		},
	}); err != nil {
		t.Fatalf("typed google search: %v", err)
	}
	if gotBody != `{"country":"us","keyword":"coffee","language":"en"}` {
		t.Fatalf("body = %q", gotBody)
	}
}
