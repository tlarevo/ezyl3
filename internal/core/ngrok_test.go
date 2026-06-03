package core

import "testing"

func TestNormalizeNgrokDomain(t *testing.T) {
	for _, input := range []string{"example.ngrok-free.dev", "https://example.ngrok-free.app/"} {
		got, err := NormalizeNgrokDomain(input)
		if err != nil {
			t.Fatalf("NormalizeNgrokDomain(%q) error: %v", input, err)
		}
		if got == "" || got[:5] == "https" {
			t.Fatalf("NormalizeNgrokDomain(%q) = %q", input, got)
		}
	}

	for _, input := range []string{"", "http://localhost:4400", "bad domain.ngrok-free.dev", "example.com"} {
		if _, err := NormalizeNgrokDomain(input); err == nil {
			t.Fatalf("NormalizeNgrokDomain(%q) succeeded, want error", input)
		}
	}
}

func TestNormalizePublicURL(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string
	}{
		{"https://llm.example.com", "https://llm.example.com"},
		{"https://llm.example.com/", "https://llm.example.com"},
		{"https://llm.example.com:8443", "https://llm.example.com:8443"},
	} {
		got, err := NormalizePublicURL(tt.in)
		if err != nil {
			t.Fatalf("NormalizePublicURL(%q) error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("NormalizePublicURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	for _, bad := range []string{
		"http://llm.example.com",
		"llm.example.com",
		"https://localhost",
		"https://127.0.0.1:4400",
		"https://[::1]",
		// Origin-only contract: callers append /v1 and /health/liveliness, so a
		// path (or query/fragment) must be rejected to avoid a doubled suffix.
		"https://llm.example.com/v1",
		"https://llm.example.com/api",
		"https://llm.example.com/?x=1",
		"",
	} {
		if _, err := NormalizePublicURL(bad); err == nil {
			t.Fatalf("NormalizePublicURL(%q) should have errored", bad)
		}
	}
}
