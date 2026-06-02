package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeServices struct {
	started     bool
	status      []core.ServiceActionResult
	statusCalls int
}

func (f *fakeServices) Start() ([]core.ServiceActionResult, error) {
	f.started = true
	return []core.ServiceActionResult{{Service: "litellm", Action: "start", State: core.ServiceStateLoaded}}, nil
}

func (f *fakeServices) Stop() ([]core.ServiceActionResult, error) {
	return []core.ServiceActionResult{{Service: "litellm", Action: "stop", State: core.ServiceStateStopped}}, nil
}

func (f *fakeServices) Restart() ([]core.ServiceActionResult, error) {
	return []core.ServiceActionResult{{Service: "litellm", Action: "restart", State: core.ServiceStateLoaded}}, nil
}

func (f *fakeServices) Status() ([]core.ServiceActionResult, error) {
	f.statusCalls++
	if f.status != nil {
		return f.status, nil
	}
	return []core.ServiceActionResult{{Service: "litellm", Action: "status", State: core.ServiceStateRunning}}, nil
}

func TestTUIViewNavigatesSetupCompanionTabs(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			return core.DoctorReport{Checks: []core.Check{{Name: "config.yaml", OK: false, Detail: "create a managed profile with ezyl3 setup"}}}
		},
		services: &fakeServices{},
	})

	if !strings.Contains(m.View(), "Overview") || !strings.Contains(m.View(), "Run ezyl3 setup") {
		t.Fatalf("overview missing setup guidance:\n%s", m.View())
	}
	updated, _ := m.Update(key("tab"))
	m = updated.(model)
	if m.activeTab != cursorTab || !strings.Contains(m.View(), "Cursor") {
		t.Fatalf("tab navigation failed: active=%d view=\n%s", m.activeTab, m.View())
	}
}

func TestTUIRendersCommandCenterSidebarAndHelp(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: &fakeServices{},
	})

	view := m.View()
	for _, want := range []string{"ezyl3", "Navigation", "> Overview", "Profile", "Services", "Models", "Doctor", "Logs", "Keys", "tab next", "r refresh", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command-center view missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewShowsGuidedSetupChecklistWhenIncomplete(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			return core.DoctorReport{Checks: []core.Check{
				{Name: "config.yaml", OK: false, Detail: "missing config.yaml; run ezyl3 setup"},
				{Name: "LITELLM_MASTER_KEY", OK: false, Detail: "empty"},
			}}
		},
		services: &fakeServices{status: []core.ServiceActionResult{
			{Service: "litellm", State: core.ServiceStateMissing, Detail: "missing LaunchAgent"},
			{Service: "ngrok", State: core.ServiceStateNotConfigured, Detail: "not configured"},
		}},
	})

	view := m.View()
	for _, want := range []string{"Setup Guide", "0%", "Profile", "Config", "Secrets", "Services", "Cursor", "missing", "not configured", "Current Step", "Why it matters", "How to fix", "Next Action", "Run ezyl3 setup"} {
		if !strings.Contains(view, want) {
			t.Fatalf("guided setup view missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewFocusesServiceStepWhenSetupFilesAreReady(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	writeTestProfile(t, runtime, core.ProfileModeManaged)
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			return core.DoctorReport{Checks: []core.Check{
				{Name: "config.yaml", OK: true, Detail: "found"},
				{Name: "LITELLM_MASTER_KEY", OK: true, Detail: "set"},
			}}
		},
		services: &fakeServices{status: []core.ServiceActionResult{{Service: "litellm", State: core.ServiceStateStopped, Detail: "stopped"}}},
	})

	view := m.View()
	for _, want := range []string{"Setup Guide", "60%", "3/5 complete", "Current Step: Services", "Press s to start LiteLLM", "LiteLLM must be running"} {
		if !strings.Contains(view, want) {
			t.Fatalf("service setup step missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewFocusesCursorStepAfterBridgeIsReady(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	writeTestProfile(t, runtime, core.ProfileModeManaged)
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			return core.DoctorReport{Checks: []core.Check{
				{Name: "config.yaml", OK: true, Detail: "found"},
				{Name: "LITELLM_MASTER_KEY", OK: true, Detail: "set"},
				{Name: "LiteLLM liveliness", OK: true, Detail: "200 OK"},
			}}
		},
		services: &fakeServices{status: []core.ServiceActionResult{{Service: "litellm", State: core.ServiceStateRunning}}},
	})

	view := m.View()
	for _, want := range []string{"Setup Guide", "80%", "4/5 complete", "Current Step: Cursor", "Review Cursor settings", "Run ezyl3 cursor settings"} {
		if !strings.Contains(view, want) {
			t.Fatalf("cursor setup step missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewShowsReadySummaryWhenHealthy(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	writeTestProfile(t, runtime, core.ProfileModeManaged)
	if err := os.WriteFile(filepath.Join(runtime.Path, "config.yaml"), []byte("model_list: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			return core.DoctorReport{Checks: []core.Check{
				{Name: "config.yaml", OK: true, Detail: "found"},
				{Name: "LITELLM_MASTER_KEY", OK: true, Detail: "set"},
				{Name: "LiteLLM liveliness", OK: true, Detail: "200 OK"},
				{Name: "Cursor settings", OK: true, Detail: "copied"},
			}}
		},
		services: &fakeServices{status: []core.ServiceActionResult{{Service: "litellm", State: core.ServiceStateRunning}}},
	})

	view := m.View()
	for _, want := range []string{"Bridge Status", "ready", "Setup Guide", "100%", "5/5", "Current Step: Complete", "Next Action", "Cursor settings ready"} {
		if !strings.Contains(view, want) {
			t.Fatalf("ready overview missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewShowsUsageSummary(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: &fakeServices{},
		usage: func(core.Runtime) (core.UsageSummary, error) {
			return core.UsageSummary{
				Requests:     2,
				TotalTokens:  50,
				CostUSD:      0.005,
				TopModels:    []core.UsageBreakdown{{Name: "litellm-simple", Requests: 2, TotalTokens: 50}},
				TopProviders: []core.UsageBreakdown{{Name: "huggingface", Requests: 2, TotalTokens: 50}},
			}, nil
		},
	})

	view := m.View()
	for _, want := range []string{"Usage Today", "Requests: 2", "Tokens: 50", "Estimated spend: $0.005000", "Top model: litellm-simple"} {
		if !strings.Contains(view, want) {
			t.Fatalf("usage view missing %q:\n%s", want, view)
		}
	}
}

func TestTUIOverviewShowsEmptyUsageState(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: &fakeServices{},
		usage: func(core.Runtime) (core.UsageSummary, error) {
			return core.UsageSummary{Since: time.Now()}, nil
		},
	})

	if !strings.Contains(m.View(), "No usage recorded yet") {
		t.Fatalf("overview missing empty usage state:\n%s", m.View())
	}
}

func TestTUIRefreshUpdatesDoctorReport(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	calls := 0
	m := newModelWithDeps(runtime, dependencies{
		doctor: func(core.Runtime) core.DoctorReport {
			calls++
			return core.DoctorReport{Checks: []core.Check{{Name: "doctor", OK: calls > 1, Detail: "refreshed"}}}
		},
		services: &fakeServices{},
	})

	updated, _ := m.Update(key("r"))
	m = updated.(model)
	if !m.report.Checks[0].OK || !strings.Contains(m.View(), "refreshed") {
		t.Fatalf("refresh did not update report: %#v\n%s", m.report.Checks, m.View())
	}
}

func TestTUIServiceActionUpdatesStateMessage(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	services := &fakeServices{}
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: services,
	})
	m.activeTab = servicesTab

	updated, _ := m.Update(key("s"))
	m = updated.(model)
	if !services.started {
		t.Fatalf("service start was not called")
	}
	if !strings.Contains(m.View(), "litellm") || !strings.Contains(m.View(), core.ServiceStateRunning) || !strings.Contains(m.View(), "service action complete") {
		t.Fatalf("service view missing refreshed status or action message:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "s start") || !strings.Contains(m.View(), "x stop") || !strings.Contains(m.View(), "k restart") {
		t.Fatalf("service view missing action hints:\n%s", m.View())
	}
}

func TestTUIServiceActionRefreshesFullServiceStatus(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	services := &fakeServices{status: []core.ServiceActionResult{
		{Service: "litellm", Action: "status", State: core.ServiceStateRunning, Detail: "running"},
		{Service: "ngrok", Action: "status", State: core.ServiceStateNotConfigured, Detail: "not configured"},
	}}
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: services,
	})
	m.activeTab = servicesTab
	initialStatusCalls := services.statusCalls

	updated, _ := m.Update(key("s"))
	m = updated.(model)
	if services.statusCalls <= initialStatusCalls {
		t.Fatalf("service status was not refreshed after action")
	}
	if serviceState(m.serviceResults, "ngrok") != core.ServiceStateNotConfigured {
		t.Fatalf("service action did not preserve full status results: %#v", m.serviceResults)
	}
	if !strings.Contains(m.View(), "service action complete") {
		t.Fatalf("service action message was not preserved:\n%s", m.View())
	}
}

func TestTUILoadsProfileJSON(t *testing.T) {
	runtime := core.NewRuntime(t.TempDir())
	profile := core.Profile{
		Name:           "work",
		Mode:           core.ProfileModeExternal,
		RuntimeDir:     runtime.Path,
		Port:           4400,
		TunnelProvider: "ngrok",
		Domain:         "work.ngrok-free.dev",
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, "profile.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: &fakeServices{},
	})
	m.activeTab = profileTab

	if !strings.Contains(m.View(), "work") || !strings.Contains(m.View(), "external profile") {
		t.Fatalf("profile view did not load profile.json:\n%s", m.View())
	}
}

func writeTestProfile(t *testing.T, runtime core.Runtime, mode string) {
	t.Helper()
	profile := core.Profile{
		Name:           "default",
		Mode:           mode,
		RuntimeDir:     runtime.Path,
		Port:           4400,
		TunnelProvider: "ngrok",
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, "profile.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

// writeCursorFixture creates a runtime with a master key, optionally tunneled.
func writeCursorFixture(t *testing.T, domain string) core.Runtime {
	t.Helper()
	runtime := core.NewRuntime(t.TempDir())
	if err := os.WriteFile(filepath.Join(runtime.Path, ".env"), []byte("LITELLM_MASTER_KEY=\"sk-cursor-abc123\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := core.Profile{Name: "default", Mode: core.ProfileModeManaged}
	if domain != "" {
		profile.Domain = domain
		profile.TunnelProvider = "ngrok"
	}
	if err := core.WriteProfileFile(runtime.Path, profile); err != nil {
		t.Fatal(err)
	}
	return runtime
}

func cursorModel(t *testing.T, runtime core.Runtime) model {
	t.Helper()
	m := newModelWithDeps(runtime, dependencies{
		doctor:   func(core.Runtime) core.DoctorReport { return core.DoctorReport{} },
		services: &fakeServices{},
	})
	m.activeTab = cursorTab
	return m
}

func TestTUICursorTabShowsBaseURLAndModelsRedactedByDefault(t *testing.T) {
	runtime := writeCursorFixture(t, "demo.ngrok-free.dev")
	view := cursorModel(t, runtime).View()

	if !strings.Contains(view, "https://demo.ngrok-free.dev/v1") {
		t.Fatalf("cursor tab missing base URL:\n%s", view)
	}
	for _, name := range []string{"litellm-auto", "litellm-simple", "litellm-medium", "litellm-complex", "litellm-reasoning"} {
		if !strings.Contains(view, name) {
			t.Fatalf("cursor tab missing model %q:\n%s", name, view)
		}
	}
	if strings.Contains(view, "sk-cursor-abc123") {
		t.Fatalf("cursor tab leaked the key while redacted:\n%s", view)
	}
}

func TestTUICursorTabWarnsForLocalOnlyProfile(t *testing.T) {
	local := cursorModel(t, writeCursorFixture(t, "")).View()
	if !strings.Contains(local, "http://127.0.0.1:4400/v1") || !strings.Contains(local, "Cursor cannot use it") {
		t.Fatalf("local-only cursor tab should warn:\n%s", local)
	}

	tunneled := cursorModel(t, writeCursorFixture(t, "demo.ngrok-free.dev")).View()
	if strings.Contains(tunneled, "Cursor cannot use it") {
		t.Fatalf("tunneled cursor tab should not warn:\n%s", tunneled)
	}
}
