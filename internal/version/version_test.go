package version

import "testing"

func TestStringDefaultsToDevPlaceholder(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = ""
	if got := String(); got != "dev" {
		t.Fatalf("String() with empty Version = %q, want dev", got)
	}

	Version = "   "
	if got := String(); got != "dev" {
		t.Fatalf("String() with blank Version = %q, want dev", got)
	}
}

func TestStringReportsInjectedVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "v1.2.3"
	if got := String(); got != "v1.2.3" {
		t.Fatalf("String() = %q, want v1.2.3", got)
	}
}
