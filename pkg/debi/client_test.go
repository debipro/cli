package debi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientDoSendsAuthAndHeaders(t *testing.T) {
	var (
		gotAuth    string
		gotVersion string
		gotIdem    string
		gotMethod  string
		gotPath    string
		gotQuery   string
		gotBody    map[string]interface{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Api-Version")
		gotIdem = r.Header.Get("Idempotency-Key")
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("limit")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Request-Id", "req_123")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient("sk_test_abc", srv.URL, "2025-10-02")
	resp, err := c.Do(context.Background(), Request{
		Method:         "POST",
		Path:           "/v1/customers",
		Query:          url.Values{"limit": {"5"}},
		Body:           map[string]interface{}{"name": "Jane"},
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}

	if gotAuth != "Bearer sk_test_abc" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotVersion != "2025-10-02" {
		t.Errorf("Api-Version = %q", gotVersion)
	}
	if gotIdem != "idem-1" {
		t.Errorf("Idempotency-Key = %q", gotIdem)
	}
	if gotMethod != "POST" || gotPath != "/v1/customers" || gotQuery != "5" {
		t.Errorf("method/path/query = %s %s ?limit=%s", gotMethod, gotPath, gotQuery)
	}
	if gotBody["name"] != "Jane" {
		t.Errorf("body name = %v", gotBody["name"])
	}
	if resp.StatusCode != http.StatusCreated || resp.RequestID != "req_123" {
		t.Errorf("status=%d requestID=%q", resp.StatusCode, resp.RequestID)
	}
}

func TestClientDoParsesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Request-Id", "req_err")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The given data was invalid.","errors":{"name":["required"]}}`))
	}))
	defer srv.Close()

	c := NewClient("sk_test_abc", srv.URL, "")
	_, err := c.Do(context.Background(), Request{Method: "POST", Path: "/v1/customers"})
	if err == nil {
		t.Fatal("expected an error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if apiErr.Message != "The given data was invalid." {
		t.Errorf("message = %q", apiErr.Message)
	}
	if got := apiErr.Fields["name"]; len(got) != 1 || got[0] != "required" {
		t.Errorf("fields = %v", apiErr.Fields)
	}
	if apiErr.RequestID != "req_err" {
		t.Errorf("requestID = %q", apiErr.RequestID)
	}
}

func TestModeForKey(t *testing.T) {
	cases := map[string]string{
		"sk_live_x": "live",
		"pk_live_x": "live",
		"sk_test_x": "test",
		"pk_test_x": "test",
		"weird":     "",
	}
	for key, want := range cases {
		if got := ModeForKey(key); got != want {
			t.Errorf("ModeForKey(%q) = %q; want %q", key, got, want)
		}
	}
}

func TestBaseURLForMode(t *testing.T) {
	if BaseURLForMode("live") != LiveBaseURL {
		t.Error("live base url wrong")
	}
	if BaseURLForMode("test") != TestBaseURL {
		t.Error("test base url wrong")
	}
	if BaseURLForMode("anything-else") != TestBaseURL {
		t.Error("default should be test")
	}
}
