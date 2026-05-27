package crawlora

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
