// Package ledgermem is the official Go SDK for the LedgerMem API.
package ledgermem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.proofly.dev"
	defaultTimeout = 30 * time.Second
	userAgent      = "ledgermem-go/0.1.0"
)

// Config configures a Client.
type Config struct {
	APIKey      string
	WorkspaceID string
	BaseURL     string
	HTTPClient  *http.Client
}

// Client is a LedgerMem API client.
type Client struct {
	apiKey      string
	workspaceID string
	baseURL     string
	httpClient  *http.Client

	Memories *MemoriesService
}

// NewClient builds a Client from the given config, falling back to env vars.
func NewClient(cfg Config) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("LEDGERMEM_API_KEY")
	}
	if cfg.WorkspaceID == "" {
		cfg.WorkspaceID = os.Getenv("LEDGERMEM_WORKSPACE_ID")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("LEDGERMEM_API_URL")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}
	c := &Client{
		apiKey:      cfg.APIKey,
		workspaceID: cfg.WorkspaceID,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:  cfg.HTTPClient,
	}
	c.Memories = &MemoriesService{client: c}
	return c
}

// APIError is returned for non-2xx responses.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("ledgermem: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("ledgermem: %d", e.StatusCode)
}

// Memory is a single stored memory.
type Memory struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"createdAt,omitempty"`
}

// SearchHit is one entry in a search result.
type SearchHit struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// SearchInput is the body for Search.
type SearchInput struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	ActorID string `json:"actorId,omitempty"`
}

// SearchResult is the response from Search.
type SearchResult struct {
	Hits []SearchHit `json:"hits"`
}

// AddMemoryInput is the body for Memories.Add.
type AddMemoryInput struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	ActorID  string                 `json:"actorId,omitempty"`
}

// UpdateMemoryInput is the body for Memories.Update.
type UpdateMemoryInput struct {
	Content  *string                `json:"content,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ListMemoriesInput is the query for Memories.List.
type ListMemoriesInput struct {
	Limit   int
	Cursor  string
	ActorID string
}

// ListMemoriesResult is the response from Memories.List.
type ListMemoriesResult struct {
	Data       []Memory `json:"data"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// Search performs a semantic search.
func (c *Client) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	out := &SearchResult{}
	if err := c.do(ctx, http.MethodPost, "/v1/search", nil, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

// MemoriesService groups memory operations.
type MemoriesService struct {
	client *Client
}

// Add creates a memory.
func (s *MemoriesService) Add(ctx context.Context, in AddMemoryInput) (*Memory, error) {
	out := &Memory{}
	if err := s.client.do(ctx, http.MethodPost, "/v1/memories", nil, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update partially updates a memory.
func (s *MemoriesService) Update(ctx context.Context, id string, in UpdateMemoryInput) (*Memory, error) {
	out := &Memory{}
	if err := s.client.do(ctx, http.MethodPatch, "/v1/memories/"+id, nil, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a memory.
func (s *MemoriesService) Delete(ctx context.Context, id string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/memories/"+id, nil, nil, nil)
}

// List returns a page of memories.
func (s *MemoriesService) List(ctx context.Context, in ListMemoriesInput) (*ListMemoriesResult, error) {
	q := map[string]string{}
	if in.Limit > 0 {
		q["limit"] = strconv.Itoa(in.Limit)
	}
	if in.Cursor != "" {
		q["cursor"] = in.Cursor
	}
	if in.ActorID != "" {
		q["actorId"] = in.ActorID
	}
	out := &ListMemoriesResult{}
	if err := s.client.do(ctx, http.MethodGet, "/v1/memories", q, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body, out interface{}) error {
	url := c.baseURL + path
	if len(query) > 0 {
		parts := make([]string, 0, len(query))
		for k, v := range query {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ledgermem: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("ledgermem: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.workspaceID != "" {
		req.Header.Set("x-workspace-id", c.workspaceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ledgermem: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(raw), Message: extractMessage(raw)}
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("ledgermem: decode response: %w", err)
	}
	return nil
}

func extractMessage(raw []byte) string {
	var probe struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		if probe.Message != "" {
			return probe.Message
		}
		if probe.Error != "" {
			return probe.Error
		}
	}
	return ""
}
