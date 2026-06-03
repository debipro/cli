// Package webhook implements Debi webhook signature generation and verification.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const signatureHeader = "Debi-Signature"

// DefaultTolerance is the maximum age of a webhook timestamp accepted by Verify.
const DefaultTolerance = 5 * time.Minute

// Sign builds a Debi-Signature header value for the given payload and secret.
func Sign(secret string, payload []byte, ts time.Time) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	sig := computeSignature(secret, timestamp, payload)
	return fmt.Sprintf("t=%s,v1=%s", timestamp, sig)
}

// Verify checks a Debi-Signature header against the payload and secret.
func Verify(header, secret string, payload []byte, tolerance time.Duration) error {
	if tolerance == 0 {
		tolerance = DefaultTolerance
	}
	timestamp, signatures, err := parseHeader(header)
	if err != nil {
		return err
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook timestamp %q", timestamp)
	}
	age := time.Since(time.Unix(ts, 0))
	if age > tolerance || age < -tolerance {
		return fmt.Errorf("webhook timestamp outside tolerance (%s)", age)
	}
	expected := computeSignature(secret, timestamp, payload)
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return fmt.Errorf("webhook signature mismatch")
}

func computeSignature(secret, timestamp string, payload []byte) string {
	signed := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseHeader(header string) (timestamp string, signatures []string, err error) {
	if header == "" {
		return "", nil, fmt.Errorf("missing %s header", signatureHeader)
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		i := strings.IndexByte(part, '=')
		if i < 0 {
			continue
		}
		prefix, value := part[:i], part[i+1:]
		switch prefix {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return "", nil, fmt.Errorf("malformed %s header", signatureHeader)
	}
	return timestamp, signatures, nil
}

// HeaderName returns the HTTP header used for webhook signatures.
func HeaderName() string {
	return signatureHeader
}
