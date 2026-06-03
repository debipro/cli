package spec

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadEmbedded(t *testing.T) {
	// Ensure no cached copy interferes with the embedded-spec assertions.
	t.Setenv("DEBI_CONFIG_DIR", t.TempDir())

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Paths) == 0 {
		t.Fatal("expected paths in embedded spec")
	}
	if s.Info.Title == "" {
		t.Error("expected a spec title")
	}

	item, ok := s.Paths["/v1/customers"]
	if !ok || item.Get == nil || item.Post == nil {
		t.Fatal("expected GET and POST on /v1/customers")
	}
	if item.Post.RequestBody.JSONSchema() == nil {
		t.Error("expected a JSON request body schema for customer creation")
	}
}

func TestTypeFieldUnmarshal(t *testing.T) {
	var scalar struct {
		Type TypeField `yaml:"type"`
	}
	if err := yaml.Unmarshal([]byte("type: string"), &scalar); err != nil {
		t.Fatal(err)
	}
	if scalar.Type.Primary() != "string" {
		t.Errorf("scalar primary = %q", scalar.Type.Primary())
	}

	var list struct {
		Type TypeField `yaml:"type"`
	}
	if err := yaml.Unmarshal([]byte("type: [\"null\", string]"), &list); err != nil {
		t.Fatal(err)
	}
	if list.Type.Primary() != "string" {
		t.Errorf("list primary = %q; want string (non-null)", list.Type.Primary())
	}
}

func TestParsesGuard(t *testing.T) {
	if parses([]byte("")) {
		t.Error("empty data should not parse")
	}
	if parses([]byte("not: a spec")) {
		t.Error("spec without paths should not parse")
	}
	if !parses(raw) {
		t.Error("embedded spec should parse")
	}
}

func TestSortedProperties(t *testing.T) {
	s := &Schema{Properties: map[string]*Schema{"b": {}, "a": {}, "c": {}}}
	got := strings.Join(s.SortedProperties(), ",")
	if got != "a,b,c" {
		t.Errorf("SortedProperties = %q; want a,b,c", got)
	}
}
