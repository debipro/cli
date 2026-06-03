package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidSpecCacheFallsBackToEmbedded(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(bad, []byte("not: valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEBI_CONFIG_DIR", dir)

	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		t.Fatalf("expected embedded fallback, got error: %v", err)
	}
	if findChild(root, "customers") == nil {
		t.Fatal("expected customers command from embedded spec")
	}
}
