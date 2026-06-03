// Package spec parses the Debi OpenAPI specification that is embedded into the
// binary. It exposes just enough of the document (paths, operations,
// parameters and request bodies) to build CLI commands dynamically.
package spec

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/tucuota/debi-cli/pkg/config"
)

// SourceURL is the canonical home of the Debi OpenAPI specification.
const SourceURL = "https://raw.githubusercontent.com/debipro/openapi/refs/heads/main/openapi/spec1.yaml"

// raw is the canonical spec embedded at build time. It is used as an offline
// fallback when no refreshed copy has been cached locally.
//
//go:embed openapi.yaml
var raw []byte

// Spec is a minimal view of an OpenAPI 3.1 document.
type Spec struct {
	OpenAPI string               `yaml:"openapi"`
	Info    Info                 `yaml:"info"`
	Paths   map[string]*PathItem `yaml:"paths"`
}

// Info holds document metadata.
type Info struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

// PathItem groups the operations available on a single path.
type PathItem struct {
	Get    *Operation `yaml:"get"`
	Post   *Operation `yaml:"post"`
	Put    *Operation `yaml:"put"`
	Patch  *Operation `yaml:"patch"`
	Delete *Operation `yaml:"delete"`
}

// MethodOperation pairs an HTTP method with its operation.
type MethodOperation struct {
	Method    string
	Operation *Operation
}

// Operations returns the path's operations in a stable method order.
func (p *PathItem) Operations() []MethodOperation {
	var ops []MethodOperation
	add := func(method string, op *Operation) {
		if op != nil {
			ops = append(ops, MethodOperation{Method: method, Operation: op})
		}
	}
	add("GET", p.Get)
	add("POST", p.Post)
	add("PUT", p.Put)
	add("PATCH", p.Patch)
	add("DELETE", p.Delete)
	return ops
}

// Operation describes a single API operation.
type Operation struct {
	OperationID string       `yaml:"operationId"`
	Summary     string       `yaml:"summary"`
	Description string       `yaml:"description"`
	Parameters  []*Parameter `yaml:"parameters"`
	RequestBody *RequestBody `yaml:"requestBody"`
}

// Parameter describes a path, query or header parameter.
type Parameter struct {
	Name        string  `yaml:"name"`
	In          string  `yaml:"in"`
	Required    bool    `yaml:"required"`
	Description string  `yaml:"description"`
	Schema      *Schema `yaml:"schema"`
}

// RequestBody describes a request payload.
type RequestBody struct {
	Required bool                  `yaml:"required"`
	Content  map[string]*MediaType `yaml:"content"`
}

// JSONSchema returns the schema for application/json content, if present.
func (r *RequestBody) JSONSchema() *Schema {
	if r == nil {
		return nil
	}
	if mt, ok := r.Content["application/json"]; ok {
		return mt.Schema
	}
	for _, mt := range r.Content {
		return mt.Schema
	}
	return nil
}

// MediaType wraps a schema for a particular content type.
type MediaType struct {
	Schema *Schema `yaml:"schema"`
}

// Schema is a trimmed-down JSON Schema. Type is normalized to handle the
// OpenAPI 3.1 form where "type" may be a string or a list (e.g. ["string",
// "null"]).
type Schema struct {
	Type        TypeField          `yaml:"type"`
	Description string             `yaml:"description"`
	Properties  map[string]*Schema `yaml:"properties"`
	Required    []string           `yaml:"required"`
	Items       *Schema            `yaml:"items"`
	Enum        []interface{}      `yaml:"enum"`
}

// SortedProperties returns property names in a stable order.
func (s *Schema) SortedProperties() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// TypeField holds one or more JSON Schema types.
type TypeField struct {
	Types []string
}

// UnmarshalYAML accepts either a scalar or a sequence of type names.
func (t *TypeField) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		t.Types = []string{value.Value}
	case yaml.SequenceNode:
		for _, n := range value.Content {
			t.Types = append(t.Types, n.Value)
		}
	}
	return nil
}

// Primary returns the first non-null type, which is the meaningful one for
// generating flags.
func (t TypeField) Primary() string {
	for _, x := range t.Types {
		if x != "null" {
			return x
		}
	}
	return ""
}

// CachePath returns the location of the locally cached specification.
func CachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openapi.yaml"), nil
}

// Source reports where the active specification was loaded from: the cached
// file path, or "embedded" when the built-in copy is used.
func Source() string {
	path, err := CachePath()
	if err != nil {
		return "embedded"
	}
	if data, err := os.ReadFile(path); err == nil && parses(data) {
		return path
	}
	return "embedded"
}

// Load parses the active specification, preferring a valid locally-cached copy
// (refreshed via Update) and falling back to the embedded canonical copy.
func Load() (*Spec, error) {
	data := raw
	if path, err := CachePath(); err == nil {
		if cached, rerr := os.ReadFile(path); rerr == nil && parses(cached) {
			data = cached
		}
	}

	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}
	return &s, nil
}

// Update downloads the canonical specification and writes it to the cache,
// validating that it parses first. It returns the cache path on success.
func Update(ctx context.Context) (string, error) {
	data, err := fetch(ctx, SourceURL)
	if err != nil {
		return "", err
	}
	if !parses(data) {
		return "", fmt.Errorf("downloaded spec is not valid YAML/OpenAPI")
	}

	path, err := CachePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching spec: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func parses(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	var s Spec
	return yaml.Unmarshal(data, &s) == nil && len(s.Paths) > 0
}

// Raw returns the embedded specification bytes.
func Raw() []byte {
	return raw
}
