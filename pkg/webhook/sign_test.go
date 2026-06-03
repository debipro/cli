package webhook

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	secret := "whsec_test_secret"
	payload := []byte(`{"id":"EV123","type":"customer.created"}`)
	now := time.Now()

	header := Sign(secret, payload, now)
	if err := Verify(header, secret, payload, time.Minute); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	payload := []byte(`{"id":"EV123"}`)
	header := Sign("whsec_a", payload, time.Now())
	if err := Verify(header, "whsec_b", payload, DefaultTolerance); err == nil {
		t.Fatal("expected verification to fail with wrong secret")
	}
}

func TestVerifyRejectsStaleTimestamp(t *testing.T) {
	payload := []byte(`{"id":"EV123"}`)
	header := Sign("whsec_a", payload, time.Unix(1, 0))
	if err := Verify(header, "whsec_a", payload, DefaultTolerance); err == nil {
		t.Fatal("expected stale timestamp to fail")
	}
}
