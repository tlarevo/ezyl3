package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeServiceRunner struct {
	calls [][]string
	out   map[string]string
	err   error
}

func (f *fakeServiceRunner) RunLaunchctl(args ...string) (string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if f.err != nil {
		return "", f.err
	}
	return f.out[strings.Join(args, " ")], nil
}

func TestServiceStartBootstrapsLiteLLMAndSkipsUnconfiguredNgrok(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writeServiceFile(t, paths.LaunchAgentPath("litellm"))
	runner := &fakeServiceRunner{}
	manager := ServiceManager{Paths: paths, Runner: runner, UID: 501}

	results, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if len(runner.calls) != 1 || runner.calls[0][0] != "bootstrap" || runner.calls[0][2] != paths.LaunchAgentPath("litellm") {
		t.Fatalf("launchctl calls = %#v", runner.calls)
	}
	if findServiceResult(results, "ngrok").State != ServiceStateNotConfigured {
		t.Fatalf("ngrok result = %#v", findServiceResult(results, "ngrok"))
	}
}

func TestServiceStartReportsMissingLiteLLMLaunchAgent(t *testing.T) {
	paths := setupTestPaths(t, "default")
	manager := ServiceManager{Paths: paths, Runner: &fakeServiceRunner{}, UID: 501}

	results, err := manager.Start()
	if err == nil || !strings.Contains(err.Error(), "litellm") || !strings.Contains(err.Error(), "missing LaunchAgent") {
		t.Fatalf("Start error = %v", err)
	}
	if findServiceResult(results, "litellm").State != ServiceStateMissing {
		t.Fatalf("litellm result = %#v", findServiceResult(results, "litellm"))
	}
}

func TestServiceStatusParsesRunningAndStoppedStates(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writeServiceFile(t, paths.LaunchAgentPath("litellm"))
	writeServiceFile(t, paths.LaunchAgentPath("ngrok"))
	runner := &fakeServiceRunner{out: map[string]string{
		"print gui/501/com.ezyl3.default.litellm": "state = running\n",
		"print gui/501/com.ezyl3.default.ngrok":   "state = waiting\n",
	}}
	manager := ServiceManager{Paths: paths, Runner: runner, UID: 501}

	results, err := manager.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if findServiceResult(results, "litellm").State != ServiceStateRunning {
		t.Fatalf("litellm result = %#v", findServiceResult(results, "litellm"))
	}
	if findServiceResult(results, "ngrok").State != ServiceStateStopped {
		t.Fatalf("ngrok result = %#v", findServiceResult(results, "ngrok"))
	}
}

func TestServiceErrorsIncludeActionAndService(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writeServiceFile(t, paths.LaunchAgentPath("litellm"))
	runner := &fakeServiceRunner{err: errors.New("launchctl failed")}
	manager := ServiceManager{Paths: paths, Runner: runner, UID: 501}

	_, err := manager.Start()
	if err == nil || !strings.Contains(err.Error(), "start litellm") || !strings.Contains(err.Error(), "launchctl failed") {
		t.Fatalf("Start error = %v", err)
	}
}

func writeServiceFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plist"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func findServiceResult(results []ServiceActionResult, service string) ServiceActionResult {
	for _, result := range results {
		if result.Service == service {
			return result
		}
	}
	return ServiceActionResult{}
}
