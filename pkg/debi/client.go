// Package debi provides an HTTP client for the Debi REST API, handling
// authentication, environment selection, idempotency, request IDs and
// structured error parsing.
package debi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/debipro/cli/pkg/version"
)

const (
	// LiveBaseURL is the production API endpoint.
	LiveBaseURL = "https://api.debi.pro"
	// TestBaseURL is the sandbox API endpoint.
	TestBaseURL = "https://api.debi-test.pro"
)

// BaseURLForMode returns the API base URL for the given mode ("live"/"test").
func BaseURLForMode(mode string) string {
	if mode == "live" {
		return LiveBaseURL
	}
	return TestBaseURL
}

// ModeForKey infers the environment from a secret key prefix. Returns an empty
// string when it cannot be determined.
func ModeForKey(key string) string {
	switch {
	case strings.HasPrefix(key, "sk_live_"), strings.HasPrefix(key, "pk_live_"):
		return "live"
	case strings.HasPrefix(key, "sk_test_"), strings.HasPrefix(key, "pk_test_"):
		return "test"
	default:
		return ""
	}
}

// Client talks to the Debi API.
type Client struct {
	APIKey     string
	BaseURL    string
	APIVersion string
	HTTPClient *http.Client
}

// NewClient builds a Client for the given key and base URL.
func NewClient(apiKey, baseURL, apiVersion string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIVersion: apiVersion,
		HTTPClient: &http.Client{Timeout: 80 * time.Second},
	}
}

// Request describes a single API call.
type Request struct {
	Method         string
	Path           string
	Query          url.Values
	Body           interface{}
	IdempotencyKey string
}

// Response is a successful (2xx) API response.
type Response struct {
	StatusCode int
	RequestID  string
	Body       []byte
}

// Do performs the request, building the URL from Path + Query.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	u := c.BaseURL + req.Path
	if len(req.Query) > 0 {
		u += "?" + req.Query.Encode()
	}
	return c.do(ctx, req.Method, u, req.Body, req.IdempotencyKey)
}

// DoURL performs a request against an absolute URL (used to follow pagination
// links returned by the API).
func (c *Client) DoURL(ctx context.Context, method, absoluteURL string) (*Response, error) {
	return c.do(ctx, method, absoluteURL, nil, "")
}

func (c *Client) do(ctx context.Context, method, fullURL string, body interface{}, idempotencyKey string) (*Response, error) {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "debi-cli/"+version.Version)
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if c.APIVersion != "" {
		httpReq.Header.Set("Api-Version", c.APIVersion)
	}
	if idempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	requestID := resp.Header.Get("Request-Id")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &Response{StatusCode: resp.StatusCode, RequestID: requestID, Body: data}, nil
	}
	return nil, parseAPIError(resp.StatusCode, requestID, data)
}
