package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

	// Start probes status first (print) to stay idempotent, then bootstraps the
	// not-yet-loaded litellm service.
	bootstrap := lastCallWithVerb(runner.calls, "bootstrap")
	if bootstrap == nil || bootstrap[2] != paths.LaunchAgentPath("litellm") {
		t.Fatalf("launchctl calls = %#v", runner.calls)
	}
	if findServiceResult(results, "ngrok").State != ServiceStateNotConfigured {
		t.Fatalf("ngrok result = %#v", findServiceResult(results, "ngrok"))
	}
}

func TestServiceStartTreatsAlreadyLoadedServiceAsRunningAndContinues(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writeServiceFile(t, paths.LaunchAgentPath("litellm"))
	writeServiceFile(t, paths.LaunchAgentPath("ngrok"))
	// litellm reports running; ngrok is not yet loaded (empty print output).
	runner := &fakeServiceRunner{out: map[string]string{
		"print gui/501/com.ezyl3.default.litellm": "state = running\n",
	}}
	manager := ServiceManager{Paths: paths, Runner: runner, UID: 501}

	results, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if findServiceResult(results, "litellm").State != ServiceStateAlreadyRunning {
		t.Fatalf("litellm result = %#v, want already running", findServiceResult(results, "litellm"))
	}
	// litellm must NOT be bootstrapped again, but ngrok must be.
	for _, call := range runner.calls {
		if call[0] == "bootstrap" && call[2] == paths.LaunchAgentPath("litellm") {
			t.Fatalf("litellm should not be re-bootstrapped: %#v", runner.calls)
		}
	}
	bootstrap := lastCallWithVerb(runner.calls, "bootstrap")
	if bootstrap == nil || bootstrap[2] != paths.LaunchAgentPath("ngrok") {
		t.Fatalf("ngrok should be bootstrapped even though litellm was already running: %#v", runner.calls)
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

func TestServiceManagerStopServicesStopsOnlyRequestedServices(t *testing.T) {
	paths := setupTestPaths(t, "default")
	writeServiceFile(t, paths.LaunchAgentPath("ngrok"))
	runner := &fakeServiceRunner{}
	manager := ServiceManager{Paths: paths, Runner: runner, UID: 501}

	results, err := manager.StopServices([]string{"ngrok"})
	if err != nil {
		t.Fatalf("StopServices returned error: %v", err)
	}
	if len(results) != 1 || results[0].Service != "ngrok" {
		t.Fatalf("results = %#v, want only ngrok", results)
	}
	want := []string{"bootout", "gui/501", paths.LaunchAgentPath("ngrok")}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("launchctl calls = %#v, want %#v", runner.calls, [][]string{want})
	}
}

func TestServiceManagerStopServicesReportsMissingRequestedService(t *testing.T) {
	paths := setupTestPaths(t, "default")
	manager := ServiceManager{Paths: paths, Runner: &fakeServiceRunner{}, UID: 501}

	results, err := manager.StopServices([]string{"ngrok"})
	if err == nil || !strings.Contains(err.Error(), "stop ngrok") || !strings.Contains(err.Error(), "missing LaunchAgent") {
		t.Fatalf("StopServices error = %v", err)
	}
	if findServiceResult(results, "ngrok").State != ServiceStateMissing {
		t.Fatalf("ngrok result = %#v", findServiceResult(results, "ngrok"))
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

func lastCallWithVerb(calls [][]string, verb string) []string {
	for i := len(calls) - 1; i >= 0; i-- {
		if len(calls[i]) > 0 && calls[i][0] == verb {
			return calls[i]
		}
	}
	return nil
}

func findServiceResult(results []ServiceActionResult, service string) ServiceActionResult {
	for _, result := range results {
		if result.Service == service {
			return result
		}
	}
	return ServiceActionResult{}
}
