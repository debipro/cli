package debi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientPaginationIntegration(t *testing.T) {
	page := 0
	var nextURL string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			nextURL = srv.URL + "/page2"
			_, _ = w.Write([]byte(`{"data":[{"id":"a"}],"links":{"next":"` + nextURL + `"}}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":[{"id":"b"}],"links":{}}`))
		default:
			t.Fatalf("unexpected page %d", page)
		}
	}))
	defer srv.Close()

	client := NewClient("sk_test_x", srv.URL, "")
	resp, err := client.Do(context.Background(), Request{Method: "GET", Path: "/items"})
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("page1 data len = %d", len(env.Data))
	}

	resp, err = client.DoURL(context.Background(), "GET", nextURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("page2 data len = %d", len(env.Data))
	}
}

func TestClientDeviceHeader(t *testing.T) {
	var gotDevice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDevice = r.Header.Get("X-Debi-CLI-Device")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient("sk_test_x", srv.URL, "")
	client.DeviceName = "ci-runner"
	if _, err := client.Do(context.Background(), Request{Method: "GET", Path: "/ping"}); err != nil {
		t.Fatal(err)
	}
	if gotDevice != "ci-runner" {
		t.Fatalf("device header = %q", gotDevice)
	}
}

func TestClientExtraHeaders(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient("sk_test_x", srv.URL, "")
	h := make(http.Header)
	h.Set("X-Custom", "value")
	if _, err := client.Do(context.Background(), Request{Method: "GET", Path: "/", Headers: h}); err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("X-Custom = %q", got)
	}
}
