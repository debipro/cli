package config

import (
	"path/filepath"
	"testing"
)

func TestConfigGet(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	cfg, err := New(cfgFile, "default")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("mode", "test"); err != nil {
		t.Fatal(err)
	}
	got, err := cfg.Get("mode")
	if err != nil {
		t.Fatal(err)
	}
	if got != "test" {
		t.Fatalf("Get(mode) = %q", got)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEBI_CONFIG_DIR", dir)

	cfg, err := New("", "default")
	if err != nil {
		t.Fatal(err)
	}

	// A fresh profile defaults to test mode.
	if cfg.CurrentProfile().Mode != ModeTest {
		t.Errorf("default mode = %q; want test", cfg.CurrentProfile().Mode)
	}

	if err := cfg.Set("mode", ModeLive); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("api_version", "2025-10-02"); err != nil {
		t.Fatal(err)
	}

	// Reload from disk and confirm persistence.
	reloaded, err := New("", "default")
	if err != nil {
		t.Fatal(err)
	}
	p := reloaded.CurrentProfile()
	if p.Mode != ModeLive {
		t.Errorf("mode = %q; want live", p.Mode)
	}
	if p.APIVersion != "2025-10-02" {
		t.Errorf("api_version = %q", p.APIVersion)
	}

	if got := reloaded.Profiles(); len(got) != 1 || got[0] != "default" {
		t.Errorf("profiles = %v; want [default]", got)
	}
}

func TestConfigUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEBI_CONFIG_DIR", dir)

	cfg, _ := New("", "prod")
	if err := cfg.Set("mode", ModeLive); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Unset("prod"); err != nil {
		t.Fatal(err)
	}

	reloaded, _ := New("", "prod")
	if len(reloaded.Profiles()) != 0 {
		t.Errorf("profiles after unset = %v; want none", reloaded.Profiles())
	}
}

func TestActiveProfileFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEBI_CONFIG_DIR", dir)
	t.Setenv(EnvProfile, "staging")

	cfg, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "staging" {
		t.Errorf("profile = %q; want staging", cfg.Profile)
	}
}
