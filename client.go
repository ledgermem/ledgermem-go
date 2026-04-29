// Package getmnemo is the official Go SDK for the Mnemo API.
package getmnemo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.getmnemo.xyz"
	defaultTimeout    = 30 * time.Second
	userAgent         = "getmnemo-go/0.1.0"
	defaultMaxRetries = 3
	defaultBaseDelay  = 200 * time.Millisecond
	defaultMaxDelay   = 5 * time.Second
)

// Config configures a Client.
type Config struct {
	APIKey      string
	WorkspaceID string
	BaseURL     string
	HTTPClient  *http.Client
	// MaxRetries is the maximum number of retry attempts on 429/5xx responses
	// and transient network errors. Zero uses the default (3). Negative disables retries.
	MaxRetries int
}

// Client is a Mnemo API client.
type Client struct {
	apiKey      string
	workspaceID string
	baseURL     string
	httpClient  *http.Client
	maxRetries  int

	Memories *MemoriesService
}

// NewClient builds a Client from the given config, falling back to env vars.
func NewClient(cfg Config) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("GETMNEMO_API_KEY")
	}
	if cfg.WorkspaceID == "" {
		cfg.WorkspaceID = os.Getenv("GETMNEMO_WORKSPACE_ID")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("GETMNEMO_API_URL")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.HTTPClient == nil {
		// Tune transport with reasonable defaults so callers don't accidentally
		// share a single open connection forever.
		t := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		}
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout, Transport: t}
	}
	retries := defaultMaxRetries
	if cfg.MaxRetries > 0 {
		retries = cfg.MaxRetries
	} else if cfg.MaxRetries < 0 {
		retries = 0
	}
	c := &Client{
		apiKey:      cfg.APIKey,
		workspaceID: cfg.WorkspaceID,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:  cfg.HTTPClient,
		maxRetries:  retries,
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
		return fmt.Sprintf("getmnemo: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("getmnemo: %d", e.StatusCode)
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
	if err := s.client.do(ctx, http.MethodPatch, "/v1/memories/"+url.PathEscape(id), nil, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a memory.
func (s *MemoriesService) Delete(ctx context.Context, id string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/memories/"+url.PathEscape(id), nil, nil, nil)
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
	reqURL := c.baseURL + path
	if len(query) > 0 {
		values := make(url.Values, len(query))
		for k, v := range query {
			values.Set(k, v)
		}
		reqURL += "?" + values.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("getmnemo: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	// Buffer the body so we can resend it on retry.
	var bodyBytes []byte
	if reader != nil {
		var err error
		bodyBytes, err = io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("getmnemo: read body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var attemptReader io.Reader
		if bodyBytes != nil {
			attemptReader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, attemptReader)
		if err != nil {
			return fmt.Errorf("getmnemo: build request: %w", err)
		}
		if bodyBytes != nil {
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
			lastErr = fmt.Errorf("getmnemo: request: %w", err)
			if attempt < c.maxRetries && !isContextErr(err) {
				if waitErr := backoffSleep(ctx, attempt); waitErr != nil {
					return waitErr
				}
				continue
			}
			return lastErr
		}

		if (resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600)) && resp.StatusCode != 501 {
			raw, _ := io.ReadAll(resp.Body)
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			lastErr = &APIError{StatusCode: resp.StatusCode, Body: string(raw), Message: extractMessage(raw)}
			if attempt < c.maxRetries {
				if waitErr := backoffSleepHinted(ctx, attempt, retryAfter); waitErr != nil {
					return waitErr
				}
				continue
			}
			return lastErr
		}
		if resp.StatusCode == 501 {
			// 501 Not Implemented is permanent — fall through to the error path.
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return &APIError{StatusCode: resp.StatusCode, Body: string(raw), Message: extractMessage(raw)}
		}

		// Non-retryable from here.
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(resp.Body)
			return &APIError{StatusCode: resp.StatusCode, Body: string(raw), Message: extractMessage(raw)}
		}
		if resp.StatusCode == http.StatusNoContent || out == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("getmnemo: decode response: %w", err)
		}
		return nil
	}
	return lastErr
}

// backoffSleep waits with exponential backoff plus jitter, but bails out if
// the caller's context is cancelled.
func backoffSleep(ctx context.Context, attempt int) error {
	return backoffSleepHinted(ctx, attempt, 0)
}

// backoffSleepHinted prefers a server-provided Retry-After hint over
// computed backoff when present. The hint is capped at defaultMaxDelay so
// a hostile server cannot stall the client indefinitely.
func backoffSleepHinted(ctx context.Context, attempt int, hint time.Duration) error {
	var d time.Duration
	if hint > 0 {
		d = hint
		if d > defaultMaxDelay {
			d = defaultMaxDelay
		}
	} else {
		shift := attempt
		if shift > 20 {
			shift = 20
		}
		d = defaultBaseDelay << shift
		if d > defaultMaxDelay {
			d = defaultMaxDelay
		}
		// Full jitter: random in [0, d].
		d = time.Duration(rand.Int63n(int64(d) + 1))
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// parseRetryAfter understands both delta-seconds and HTTP-date forms.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
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
