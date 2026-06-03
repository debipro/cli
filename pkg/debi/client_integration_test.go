package debi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientPaginationIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/items":
			next := "http://" + r.Host + "/page2"
			_, _ = w.Write([]byte(`{"data":[{"id":"a"}],"links":{"next":"` + next + `"}}`))
		case "/page2":
			_, _ = w.Write([]byte(`{"data":[{"id":"b"}],"links":{}}`))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
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
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("page1 data len = %d", len(env.Data))
	}
	if env.Links.Next == "" {
		t.Fatal("expected next link on page 1")
	}

	resp, err = client.DoURL(context.Background(), "GET", env.Links.Next)
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
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("X-Debi-CLI-Device")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient("sk_test_x", srv.URL, "")
	client.DeviceName = "ci-runner"
	if _, err := client.Do(context.Background(), Request{Method: "GET", Path: "/ping"}); err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "ci-runner" {
		t.Fatalf("device header = %q", v)
	}
}

func TestClientExtraHeaders(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("X-Custom")
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
	if v := <-got; v != "value" {
		t.Fatalf("X-Custom = %q", v)
	}
}
