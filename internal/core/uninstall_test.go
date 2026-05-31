package core

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlistFixture(t *testing.T, paths ProfilePaths, service string) {
	t.Helper()
	plist := paths.LaunchAgentPath(service)
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestPlanUninstallManagedProfileRemovesEverythingEzyl3Created(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePlistFixture(t, paths, "litellm")
	writePlistFixture(t, paths, "ngrok")

	plan := PlanUninstall(paths, Profile{Name: paths.Profile, Mode: ProfileModeManaged})

	if plan.Mode != ProfileModeManaged {
		t.Fatalf("plan mode = %q, want managed", plan.Mode)
	}
	for _, want := range []string{paths.ProfileDir, paths.LogsDir, paths.LaunchAgentPath("litellm"), paths.LaunchAgentPath("ngrok")} {
		if !contains(plan.RemovePaths, want) {
			t.Fatalf("plan.RemovePaths missing %q: %#v", want, plan.RemovePaths)
		}
	}
	if !contains(plan.Services, "litellm") || !contains(plan.Services, "ngrok") {
		t.Fatalf("plan.Services = %#v, want litellm and ngrok", plan.Services)
	}
	if contains(plan.RemovePaths, paths.CacheDir) {
		t.Fatalf("shared cache dir should not be removed: %#v", plan.RemovePaths)
	}
}

func TestPlanUninstallLocalOnlyProfileSkipsMissingNgrok(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePlistFixture(t, paths, "litellm")

	plan := PlanUninstall(paths, Profile{Name: paths.Profile, Mode: ProfileModeManaged})

	if contains(plan.Services, "ngrok") {
		t.Fatalf("ngrok should not be planned when its plist is absent: %#v", plan.Services)
	}
	if contains(plan.RemovePaths, paths.LaunchAgentPath("ngrok")) {
		t.Fatalf("absent ngrok plist should not be in RemovePaths: %#v", plan.RemovePaths)
	}
	if !contains(plan.RemovePaths, paths.LaunchAgentPath("litellm")) {
		t.Fatalf("litellm plist should be in RemovePaths: %#v", plan.RemovePaths)
	}
}

func TestPlanUninstallExternalProfilePreservesImportedRuntime(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePlistFixture(t, paths, "litellm")
	importedRuntime := t.TempDir()

	plan := PlanUninstall(paths, Profile{Name: paths.Profile, Mode: ProfileModeExternal, RuntimeDir: importedRuntime})

	if plan.Mode != ProfileModeExternal {
		t.Fatalf("plan mode = %q, want external", plan.Mode)
	}
	if plan.ExternalRuntimeDir != importedRuntime {
		t.Fatalf("plan.ExternalRuntimeDir = %q, want %q", plan.ExternalRuntimeDir, importedRuntime)
	}
	if contains(plan.RemovePaths, importedRuntime) {
		t.Fatalf("imported runtime must never be in RemovePaths: %#v", plan.RemovePaths)
	}
	if !contains(plan.RemovePaths, paths.ProfileDir) {
		t.Fatalf("ezyl3 metadata dir should be removed: %#v", plan.RemovePaths)
	}
}

func TestPlanUninstallDefaultsBlankModeToManaged(t *testing.T) {
	paths := setupTestPaths(t, "default")
	if err := os.MkdirAll(paths.ProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := PlanUninstall(paths, Profile{Name: paths.Profile})

	if plan.Mode != ProfileModeManaged {
		t.Fatalf("blank mode should default to managed, got %q", plan.Mode)
	}
}
