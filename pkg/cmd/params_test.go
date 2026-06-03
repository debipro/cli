package cmd

import "testing"

func TestParsePair(t *testing.T) {
	k, v, err := parsePair("name=Jane")
	if err != nil || k != "name" || v != "Jane" {
		t.Errorf("string pair = (%q,%v,%v)", k, v, err)
	}

	k, v, err = parsePair("amount:=1600")
	if err != nil || k != "amount" || v != float64(1600) {
		t.Errorf("json pair = (%q,%v,%v)", k, v, err)
	}

	if _, _, err := parsePair("noequals"); err == nil {
		t.Error("expected error for missing separator")
	}

	if _, _, err := parsePair("bad:=not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildQuery(t *testing.T) {
	q, err := buildQuery([]string{"limit=5", "active:=true"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("limit") != "5" {
		t.Errorf("limit = %q", q.Get("limit"))
	}
	if q.Get("active") != "true" {
		t.Errorf("active = %q", q.Get("active"))
	}
}

func TestSnakeCase(t *testing.T) {
	cases := map[string]string{
		"sendToCustomers":    "send_to_customers",
		"stop_auto_retrying": "stop_auto_retrying",
		"disable":            "disable",
	}
	for in, want := range cases {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSubstitutePath(t *testing.T) {
	got := substitutePath("/v1/customers/{id}/payment_methods", []string{"id"}, []string{"CS abc"})
	want := "/v1/customers/CS%20abc/payment_methods"
	if got != want {
		t.Errorf("substitutePath = %q; want %q", got, want)
	}
}

func TestNormalizePath(t *testing.T) {
	if normalizePath("v1/customers") != "/v1/customers" {
		t.Error("should prepend slash")
	}
	if normalizePath("/v1/customers") != "/v1/customers" {
		t.Error("should leave leading slash intact")
	}
}
