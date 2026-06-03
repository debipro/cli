package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestSmokeHelpAndSpec(t *testing.T) {
	t.Setenv("DEBI_CONFIG_DIR", t.TempDir())

	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		t.Fatalf("rootCmd: %v", err)
	}

	var help bytes.Buffer
	root.SetOut(&help)
	root.SetErr(&help)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	out := help.String()
	for _, want := range []string{"customers", "login", "completion", "listen"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q", want)
		}
	}

	help.Reset()
	app2 := &App{}
	root2, err := app2.rootCmd()
	if err != nil {
		t.Fatalf("rootCmd: %v", err)
	}
	root2.SetOut(&help)
	root2.SetErr(&help)
	root2.SetArgs([]string{"spec", "info"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("spec info: %v", err)
	}
	if !strings.Contains(help.String(), "Paths:") {
		t.Fatalf("spec info output: %q", help.String())
	}
}
