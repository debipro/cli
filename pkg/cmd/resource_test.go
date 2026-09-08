package cmd

import (
	"strings"
	"testing"
)

func TestAnalyzePath(t *testing.T) {
	cases := []struct {
		path      string
		method    string
		singleton bool
		namespace string
		leaf      string
		params    int
	}{
		{"/v1/customers", "GET", false, "customers", "list", 0},
		{"/v1/customers", "POST", false, "customers", "create", 0},
		{"/v1/customers/{id}", "GET", true, "customers", "retrieve", 1},
		{"/v1/customers/{id}", "PUT", true, "customers", "update", 1},
		{"/v1/customers/{id}/actions/archive", "POST", true, "customers", "archive", 1},
		{"/v1/customers/search", "GET", false, "customers", "search", 0},
		{"/v1/customers/{id}/payment_methods", "GET", false, "customers", "payment_methods", 1},
		{"/v1/billing_portal/configurations", "GET", false, "billing_portal configurations", "list", 0},
		{"/v1/billing_portal/configurations/{id}", "PUT", true, "billing_portal configurations", "update", 1},
		{"/v1/gateways/{id}/actions/disable", "POST", true, "gateways", "disable", 1},
		{"/v1/links/{id}/actions/sendToCustomers", "POST", true, "links", "send_to_customers", 1},
		{"/v1/payment_methods/{id}/attach", "POST", true, "payment_methods", "attach", 1},
		{"/v1/payments/{id}/actions/stop_auto_retrying", "POST", true, "payments", "stop_auto_retrying", 1},
		// Singleton collection paths read as "retrieve" rather than "list".
		{"/v1/account", "GET", true, "account", "retrieve", 0},
		{"/v1/account/payment_methods", "GET", true, "account payment_methods", "retrieve", 0},
		{"/v1/gateways", "GET", false, "gateways", "list", 0},
	}

	for _, c := range cases {
		got := analyzePath(c.path, c.method, c.singleton)
		ns := strings.Join(got.Namespace, " ")
		if ns != c.namespace || got.Leaf != c.leaf || len(got.PathParams) != c.params {
			t.Errorf("analyzePath(%q, %q, singleton=%t) = ns=%q leaf=%q params=%d; want ns=%q leaf=%q params=%d",
				c.path, c.method, c.singleton, ns, got.Leaf, len(got.PathParams), c.namespace, c.leaf, c.params)
		}
	}
}

// TestEventsResendIsServerSide guards the naming split between the local
// replay helper and the API's own resend operation: `events replay` forwards a
// stored event to a local endpoint, while `events resend` is generated from
// POST /v1/events/{id}/actions/resend and re-delivers to registered webhooks.
func TestEventsResendIsServerSide(t *testing.T) {
	t.Setenv("DEBI_CONFIG_DIR", t.TempDir())

	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		t.Fatal(err)
	}

	events := findChild(root, "events")
	if events == nil {
		t.Fatal("expected an events command")
	}

	replay := findChild(events, "replay")
	if replay == nil {
		t.Fatal("expected `events replay` for local forwarding")
	}
	if replay.Flags().Lookup("forward-to") == nil {
		t.Error("expected `events replay` to keep the --forward-to flag")
	}

	resend := findChild(events, "resend")
	if resend == nil {
		t.Fatal("expected `events resend` generated from the spec")
	}
	if resend.Flags().Lookup("forward-to") != nil {
		t.Error("`events resend` should be the server-side operation, not the local forwarder")
	}
	if findChild(events, "resend-post") != nil {
		t.Error("`events resend-post` should no longer exist once the collision is resolved")
	}
}

// TestCommandTreeBuilds ensures the full command tree is constructed from the
// embedded spec without duplicate-command panics.
func TestCommandTreeBuilds(t *testing.T) {
	t.Setenv("DEBI_CONFIG_DIR", t.TempDir())

	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		t.Fatal(err)
	}

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
