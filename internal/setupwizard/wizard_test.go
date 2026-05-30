package setupwizard

import (
	"path/filepath"
	"strings"
	"testing"

	"ezyl3/internal/core"
)

func TestWizardInitialViewShowsGuidedSetupFrame(t *testing.T) {
	m := newModel(testOptions(t), nil)

	view := m.View()
	for _, want := range []string{"ezyl3 setup", "Profile", "Tunnel", "Secrets", "Review", "enter next"} {
		if !strings.Contains(view, want) {
			t.Fatalf("initial view missing %q:\n%s", want, view)
		}
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		Paths: core.ProfilePaths{
			Profile:    "default",
			ProfileDir: filepath.Join(root, "profile"),
			LogsDir:    filepath.Join(root, "logs"),
			HomeDir:    root,
		},
		SkipPythonDeps: true,
	}
}
