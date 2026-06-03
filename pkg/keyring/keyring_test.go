package keyring

import "testing"

func TestKeyringFileBackendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEBI_CONFIG_DIR", dir)
	t.Setenv("DEBI_KEYRING_PASSWORD", "test-passphrase")

	if err := Set("testprofile", "sk_test_roundtrip"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get("testprofile")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk_test_roundtrip" {
		t.Fatalf("Get = %q", got)
	}
	if err := Delete("testprofile"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Get("testprofile"); err == nil {
		t.Fatal("expected ErrNotFound after Delete")
	}
}
