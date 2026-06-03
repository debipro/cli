package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSONRaw(t *testing.T) {
	var b bytes.Buffer
	if err := PrintJSON(&b, []byte(`{"a":1,"b":2}`), true, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(b.String()); got != `{"a":1,"b":2}` {
		t.Errorf("raw output = %q", got)
	}
}

func TestPrintJSONIndented(t *testing.T) {
	var b bytes.Buffer
	if err := PrintJSON(&b, []byte(`{"a":1,"b":[1,2]}`), false, false); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "\n  \"a\": 1") {
		t.Errorf("expected indented output, got:\n%s", out)
	}
}

func TestPrintJSONInvalidPassthrough(t *testing.T) {
	var b bytes.Buffer
	if err := PrintJSON(&b, []byte("not json"), false, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(b.String()); got != "not json" {
		t.Errorf("passthrough = %q", got)
	}
}

func TestPrintJSONEmpty(t *testing.T) {
	var b bytes.Buffer
	if err := PrintJSON(&b, []byte("   "), false, false); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 0 {
		t.Errorf("expected no output for empty body, got %q", b.String())
	}
}

func TestColorizePreservesKeysAndValues(t *testing.T) {
	colored := colorize("{\n  \"id\": \"abc\",\n  \"n\": 5,\n  \"ok\": true\n}")
	for _, want := range []string{"id", "abc", "5", "true"} {
		if !strings.Contains(colored, want) {
			t.Errorf("colorized output missing %q: %s", want, colored)
		}
	}
}
