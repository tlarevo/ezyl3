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
