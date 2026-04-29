package getmnemo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMemoriesAdd(t *testing.T) {
	var gotPath, gotAuth, gotWS string
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotWS = r.Header.Get("x-workspace-id")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"mem_1","content":"hello","createdAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{APIKey: "key", WorkspaceID: "ws", BaseURL: srv.URL})
	mem, err := c.Memories.Add(context.Background(), AddMemoryInput{Content: "hello"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if mem.ID != "mem_1" {
		t.Errorf("expected id mem_1, got %q", mem.ID)
	}
	if gotPath != "/v1/memories" {
		t.Errorf("expected path /v1/memories, got %q", gotPath)
	}
	if gotAuth != "Bearer key" {
		t.Errorf("expected Bearer key, got %q", gotAuth)
	}
	if gotWS != "ws" {
		t.Errorf("expected workspace ws, got %q", gotWS)
	}
	if gotBody["content"] != "hello" {
		t.Errorf("expected body content=hello, got %v", gotBody)
	}
}

func TestSearchAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"bad key"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{APIKey: "x", WorkspaceID: "ws", BaseURL: srv.URL})
	_, err := c.Search(context.Background(), SearchInput{Query: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", apiErr.StatusCode)
	}
}
