package crawlora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if apiErr.RawBody == "" {
		t.Fatal("expected raw body")
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

func TestOperationMetadataCount(t *testing.T) {
	if len(operations) != operationCount {
		t.Fatalf("operations = %d, operationCount = %d", len(operations), operationCount)
	}
	if operationCount != 303 {
		t.Fatalf("operationCount = %d", operationCount)
	}
}

func TestDeprecatedEndpointsAreNotGenerated(t *testing.T) {
	if _, ok := operations["google-lens"]; ok {
		t.Fatal("google-lens should not be generated")
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

	count := 0
	client := NewClient(WithBaseURL(server.URL+"/api/v1"), WithAPIKey("api_test"))
	if _, err := client.Bing.SearchTyped(context.Background(), BingSearchParams{
		Q:     "coffee",
		Count: &count,
	}); err != nil {
		t.Fatalf("typed search: %v", err)
	}
	if gotPath != "/api/v1/bing/search?count=0&q=coffee" {
		t.Fatalf("path = %q", gotPath)
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
		SearchOption: map[string]any{"q": "coffee"},
	}); err != nil {
		t.Fatalf("typed google search: %v", err)
	}
	if gotBody != `{"q":"coffee"}` {
		t.Fatalf("body = %q", gotBody)
	}
}
