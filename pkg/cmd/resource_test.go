package cmd

import (
	"strings"
	"testing"
)

func TestAnalyzePath(t *testing.T) {
	cases := []struct {
		path      string
		method    string
		namespace string
		leaf      string
		params    int
	}{
		{"/v1/customers", "GET", "customers", "list", 0},
		{"/v1/customers", "POST", "customers", "create", 0},
		{"/v1/customers/{id}", "GET", "customers", "retrieve", 1},
		{"/v1/customers/{id}", "PUT", "customers", "update", 1},
		{"/v1/customers/{id}/actions/archive", "POST", "customers", "archive", 1},
		{"/v1/customers/search", "GET", "customers", "search", 0},
		{"/v1/customers/{id}/payment_methods", "GET", "customers", "payment_methods", 1},
		{"/v1/billing_portal/configurations", "GET", "billing_portal configurations", "list", 0},
		{"/v1/billing_portal/configurations/{id}", "PUT", "billing_portal configurations", "update", 1},
		{"/v1/gateways/{id}/actions/disable", "POST", "gateways", "disable", 1},
		{"/v1/links/{id}/actions/sendToCustomers", "POST", "links", "send_to_customers", 1},
		{"/v1/payment_methods/{id}/attach", "POST", "payment_methods", "attach", 1},
		{"/v1/payments/{id}/actions/stop_auto_retrying", "POST", "payments", "stop_auto_retrying", 1},
	}

	for _, c := range cases {
		got := analyzePath(c.path, c.method)
		ns := strings.Join(got.Namespace, " ")
		if ns != c.namespace || got.Leaf != c.leaf || len(got.PathParams) != c.params {
			t.Errorf("analyzePath(%q, %q) = ns=%q leaf=%q params=%d; want ns=%q leaf=%q params=%d",
				c.path, c.method, ns, got.Leaf, len(got.PathParams), c.namespace, c.leaf, c.params)
		}
	}
}

// TestCommandTreeBuilds ensures the full command tree is constructed from the
// embedded spec without duplicate-command panics.
func TestCommandTreeBuilds(t *testing.T) {
	app := &App{}
	root := app.rootCmd()

	for _, name := range []string{"customers", "payments", "subscriptions", "events", "billing_portal"} {
		if findChild(root, name) == nil {
			t.Errorf("expected top-level command %q to exist", name)
		}
	}
}

func TestBuildBodyNested(t *testing.T) {
	body, err := buildBody([]string{"name=Jane", "amount:=1600", "metadata.order_id=123"})
	if err != nil {
		t.Fatal(err)
	}
	if body["name"] != "Jane" {
		t.Errorf("name = %v; want Jane", body["name"])
	}
	if body["amount"] != float64(1600) {
		t.Errorf("amount = %v (%T); want 1600 (float64)", body["amount"], body["amount"])
	}
	meta, ok := body["metadata"].(map[string]interface{})
	if !ok || meta["order_id"] != "123" {
		t.Errorf("metadata = %v; want nested order_id=123", body["metadata"])
	}
}
