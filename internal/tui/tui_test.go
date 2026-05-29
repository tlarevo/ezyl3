package tui

import (
	"strings"
	"testing"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeServices struct {
	started bool
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

	if !strings.Contains(m.View(), "Overview") || !strings.Contains(m.View(), "create a managed profile") {
		t.Fatalf("overview missing setup guidance:\n%s", m.View())
	}
	updated, _ := m.Update(key("tab"))
	m = updated.(model)
	if m.activeTab != 1 || !strings.Contains(m.View(), "Profile") {
		t.Fatalf("tab navigation failed: active=%d view=\n%s", m.activeTab, m.View())
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
	if !strings.Contains(m.View(), "litellm") || !strings.Contains(m.View(), core.ServiceStateLoaded) {
		t.Fatalf("service view missing action result:\n%s", m.View())
	}
}

func key(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
