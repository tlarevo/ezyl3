package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	overviewTab = iota
	profileTab
	servicesTab
	modelsTab
	doctorTab
	logsTab
)

var tabs = []string{"Overview", "Profile", "Services", "Models", "Doctor", "Logs"}

type serviceController interface {
	Start() ([]core.ServiceActionResult, error)
	Stop() ([]core.ServiceActionResult, error)
	Restart() ([]core.ServiceActionResult, error)
	Status() ([]core.ServiceActionResult, error)
}

type dependencies struct {
	doctor   func(core.Runtime) core.DoctorReport
	services serviceController
}

type model struct {
	runtime        core.Runtime
	report         core.DoctorReport
	profile        *core.Profile
	serviceResults []core.ServiceActionResult
	models         []core.ModelEntry
	modelErr       error
	logPreview     string
	message        string
	activeTab      int
	deps           dependencies
}

func Run(runtime core.Runtime) error {
	_, err := tea.NewProgram(newModel(runtime), tea.WithAltScreen()).Run()
	return err
}

func newModel(runtime core.Runtime) model {
	return newModelWithDeps(runtime, dependencies{})
}

func newModelWithDeps(runtime core.Runtime, deps dependencies) model {
	if deps.doctor == nil {
		deps.doctor = core.Doctor
	}
	if deps.services == nil {
		deps.services = core.ServiceManager{Paths: pathsForRuntime(runtime)}
	}
	m := model{runtime: runtime, deps: deps}
	m.refresh()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(tabs)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab + len(tabs) - 1) % len(tabs)
		case "r":
			m.refresh()
			m.message = "refreshed"
		case "s":
			if m.activeTab == servicesTab {
				m.serviceResults, m.message = runServiceAction(m.deps.services.Start)
			}
		case "x":
			if m.activeTab == servicesTab {
				m.serviceResults, m.message = runServiceAction(m.deps.services.Stop)
			}
		case "k":
			if m.activeTab == servicesTab {
				m.serviceResults, m.message = runServiceAction(m.deps.services.Restart)
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("ezyl3 LiteLLM Cursor Bridge")
	body := title + "\n" + m.renderTabs() + "\n\n"
	switch m.activeTab {
	case overviewTab:
		body += m.renderOverview()
	case profileTab:
		body += m.renderProfile()
	case servicesTab:
		body += m.renderServices()
	case modelsTab:
		body += m.renderModels()
	case doctorTab:
		body += m.renderDoctor()
	case logsTab:
		body += m.renderLogs()
	}
	if m.message != "" {
		body += "\n" + m.message + "\n"
	}
	body += "\nKeys: tab next, r refresh, q quit"
	if m.activeTab == servicesTab {
		body += ", s start, x stop, k restart"
	}
	body += "\n"
	return body
}

func (m *model) refresh() {
	m.report = m.deps.doctor(m.runtime)
	m.profile = loadProfile(m.runtime.Path)
	m.serviceResults, _ = m.deps.services.Status()
	m.models, m.modelErr = loadModels(m.runtime)
	m.logPreview = loadLogPreview(m.runtime)
}

func (m model) renderTabs() string {
	parts := make([]string, 0, len(tabs))
	for i, tab := range tabs {
		if i == m.activeTab {
			parts = append(parts, "["+tab+"]")
			continue
		}
		parts = append(parts, tab)
	}
	return strings.Join(parts, "  ")
}

func (m model) renderOverview() string {
	mode := "not set up"
	if m.profile != nil {
		mode = m.profile.Mode
	}
	return fmt.Sprintf("Runtime: %s\nProfile mode: %s\nNext check: %s\n", m.runtime.Path, mode, firstDoctorDetail(m.report))
}

func (m model) renderProfile() string {
	if m.profile == nil {
		return fmt.Sprintf("Profile\nNo profile metadata found at %s\nRun ezyl3 setup or ezyl3 import.\n", filepath.Join(m.runtime.Path, "metadata.json"))
	}
	mutability := "managed by ezyl3"
	if m.profile.Mode == core.ProfileModeExternal {
		mutability = "external profile, install actions are read-only"
	}
	tunnel := "local-only"
	if m.profile.Domain != "" {
		tunnel = m.profile.TunnelProvider + ": " + m.profile.Domain
	}
	return fmt.Sprintf("Profile\nName: %s\nMode: %s\nRuntime: %s\nPort: %d\nTunnel: %s\n%s\n", m.profile.Name, m.profile.Mode, m.profile.RuntimeDir, m.profile.Port, tunnel, mutability)
}

func (m model) renderServices() string {
	var b strings.Builder
	b.WriteString("Services\n")
	if len(m.serviceResults) == 0 {
		b.WriteString("No service status available.\n")
		return b.String()
	}
	for _, result := range m.serviceResults {
		fmt.Fprintf(&b, "%-8s %-14s %s\n", result.Service, result.State, result.Detail)
	}
	return b.String()
}

func (m model) renderModels() string {
	if m.modelErr != nil {
		return "Models\n" + m.modelErr.Error() + "\n"
	}
	var b strings.Builder
	b.WriteString("Models\n")
	for _, entry := range m.models {
		fmt.Fprintf(&b, "%-24s %v\n", entry.ModelName, entry.LiteLLMParams["model"])
	}
	return b.String()
}

func (m model) renderDoctor() string {
	var b strings.Builder
	b.WriteString("Doctor\n")
	for _, check := range m.report.Checks {
		mark := "fail"
		if check.OK {
			mark = "ok"
		}
		fmt.Fprintf(&b, "%-22s %-4s %s\n", check.Name, mark, check.Detail)
	}
	return b.String()
}

func (m model) renderLogs() string {
	if m.logPreview == "" {
		return "Logs\nNo LiteLLM log output yet.\n"
	}
	return "Logs\n" + m.logPreview
}

func runServiceAction(action func() ([]core.ServiceActionResult, error)) ([]core.ServiceActionResult, string) {
	results, err := action()
	if err != nil {
		return results, err.Error()
	}
	return results, "service action complete"
}

func loadProfile(runtimePath string) *core.Profile {
	data, err := os.ReadFile(filepath.Join(runtimePath, "metadata.json"))
	if err != nil {
		return nil
	}
	var profile core.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil
	}
	return &profile
}

func loadModels(runtime core.Runtime) ([]core.ModelEntry, error) {
	cfg, err := core.LoadLiteLLMConfig(filepath.Join(runtime.Path, "config.yaml"))
	if err != nil {
		return nil, err
	}
	return cfg.Models(), nil
}

func loadLogPreview(runtime core.Runtime) string {
	data, err := core.FileLogReader{Runtime: runtime}.Read("litellm")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	return strings.Join(lines, "\n")
}

func firstDoctorDetail(report core.DoctorReport) string {
	for _, check := range report.Checks {
		if !check.OK {
			return check.Detail
		}
	}
	if len(report.Checks) > 0 {
		return "all visible checks passed"
	}
	return "run setup to create a profile"
}

func pathsForRuntime(runtime core.Runtime) core.ProfilePaths {
	paths, err := core.ResolvePaths(envMap(), runtime.Profile)
	if err != nil {
		return core.ProfilePaths{Profile: runtime.Profile, ProfileDir: runtime.Path}
	}
	return paths
}

func envMap() map[string]string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}
