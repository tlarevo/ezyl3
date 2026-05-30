package setupwizard

import (
	"path/filepath"
	"strings"
	"testing"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
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

func TestWizardCapturesTunnelDomainInput(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel

	updated, _ := m.Update(keyRunes("demo.ngrok-free.dev"))
	m = updated.(model)

	if got := m.domainInput.Value(); got != "demo.ngrok-free.dev" {
		t.Fatalf("domain input = %q", got)
	}
	if !strings.Contains(m.View(), "demo.ngrok-free.dev") {
		t.Fatalf("domain was not rendered:\n%s", m.View())
	}
}

func TestWizardShowsLocalOnlyCopyWhenDomainBlank(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel

	view := m.View()
	if !strings.Contains(view, "local-only") {
		t.Fatalf("blank domain should explain local-only setup:\n%s", view)
	}
}

func TestWizardMasksSecretInputs(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepSecrets

	updated, _ := m.Update(keyRunes("hf_secret"))
	m = updated.(model)

	view := m.View()
	if strings.Contains(view, "hf_secret") {
		t.Fatalf("secret leaked in wizard view:\n%s", view)
	}
	if !strings.Contains(view, "*********") {
		t.Fatalf("masked secret not rendered:\n%s", view)
	}
}

func TestWizardNavigationPreservesInputValues(t *testing.T) {
	m := newModel(testOptions(t), nil)
	m.step = stepTunnel
	updated, _ := m.Update(keyRunes("demo.ngrok-free.dev"))
	m = updated.(model)

	updated, _ = m.Update(keyString("enter"))
	m = updated.(model)
	updated, _ = m.Update(keyString("b"))
	m = updated.(model)

	if m.step != stepTunnel {
		t.Fatalf("step = %v, want tunnel", m.step)
	}
	if got := m.domainInput.Value(); got != "demo.ngrok-free.dev" {
		t.Fatalf("domain input after navigation = %q", got)
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

func keyString(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "b":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
