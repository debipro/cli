package cmd

import "testing"

func TestValidateForwardURL(t *testing.T) {
	cases := []struct {
		url     string
		allowed bool
	}{
		{"http://127.0.0.1:3000/webhooks", true},
		{"http://localhost:4242/", true},
		{"http://[::1]:8080/hook", true},
		{"https://127.0.0.1/hook", false},
		{"http://example.com/hook", false},
		{"http://169.254.169.254/", false},
	}
	for _, c := range cases {
		err := validateForwardURL(c.url)
		ok := err == nil
		if ok != c.allowed {
			t.Errorf("validateForwardURL(%q) allowed=%v err=%v", c.url, ok, err)
		}
	}
}

func TestForwardJSONRejectsRemoteHost(t *testing.T) {
	_, _, err := forwardJSON(t.Context(), "http://example.com/hook", []byte(`{}`), "")
	if err == nil {
		t.Fatal("expected remote host to be rejected")
	}
}
