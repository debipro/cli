package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandTreeGolden(t *testing.T) {
	t.Setenv("DEBI_CONFIG_DIR", t.TempDir())

	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Join(collectPaths(root, ""), "\n") + "\n"

	golden := filepath.Join("testdata", "commands.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("updated golden file")
		return
	}

	wantBytes, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 go test ./pkg/cmd -run Golden): %v", err)
	}
	want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
	if want != got {
		t.Fatalf("command tree changed; run UPDATE_GOLDEN=1 go test ./pkg/cmd -run Golden to refresh")
	}
}

func collectPaths(cmd *cobra.Command, prefix string) []string {
	name := cmd.Name()
	path := name
	if prefix != "" {
		path = prefix + " " + name
	}
	var out []string
	children := cmd.Commands()
	if len(children) == 0 {
		out = append(out, path)
	} else {
		for _, child := range children {
			out = append(out, collectPaths(child, path)...)
		}
	}
	sort.Strings(out)
	return out
}
