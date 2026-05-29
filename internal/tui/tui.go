package tui

import (
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

var (
	appStyle       = lipgloss.NewStyle().Padding(1, 2)
	brandStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	activeTabStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	panelStyle     = lipgloss.NewStyle().Padding(0, 1).MarginBottom(1)
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	missingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

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
	main := m.renderMain()
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), main)
	footer := m.renderHelp()
	return appStyle.Render(body + "\n\n" + footer + "\n")
}

func (m model) renderMain() string {
	body := titleStyle.Render(tabTitle(m.activeTab)) + "\n" + mutedStyle.Render(m.statusSummary()) + "\n\n"
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
	return lipgloss.NewStyle().Width(82).Render(body)
}

func (m *model) refresh() {
	m.report = m.deps.doctor(m.runtime)
	m.profile = loadProfile(m.runtime.Path)
	m.serviceResults, _ = m.deps.services.Status()
	m.models, m.modelErr = loadModels(m.runtime)
	m.logPreview = loadLogPreview(m.runtime)
}

func (m model) renderSidebar() string {
	lines := []string{brandStyle.Render("ezyl3"), mutedStyle.Render("LiteLLM bridge"), "", "Navigation"}
	for i, tab := range tabs {
		if i == m.activeTab {
			lines = append(lines, activeTabStyle.Render("> "+tab))
			continue
		}
		lines = append(lines, mutedStyle.Render("  "+tab))
	}
	return lipgloss.NewStyle().Width(20).MarginRight(3).Render(strings.Join(lines, "\n"))
}

func (m model) renderHelp() string {
	help := "Keys: tab next, r refresh, q quit"
	if m.activeTab == servicesTab {
		help += ", s start, x stop, k restart"
	}
	return mutedStyle.Render(help)
}

func (m model) renderOverview() string {
	mode := "not set up"
	if m.profile != nil {
		mode = m.profile.Mode
	}
	progress := m.setupProgress()
	return section("Bridge Status",
		fmt.Sprintf("Status: %s\nRuntime: %s\nProfile mode: %s\nTunnel: %s", progress.overallStatus(), m.runtime.Path, mode, statusText(tunnelState(m.serviceResults))),
	) + section("Setup Progress",
		progress.render(),
	) + section("Next Action",
		progress.nextAction,
	)
}

func (m model) renderProfile() string {
	if m.profile == nil {
		return section("Profile", fmt.Sprintf("No profile.json found at %s\nRun ezyl3 setup or ezyl3 import.", filepath.Join(m.runtime.Path, core.ProfileFileName)))
	}
	mutability := "managed by ezyl3"
	if m.profile.Mode == core.ProfileModeExternal {
		mutability = "external profile, install actions are read-only"
	}
	tunnel := "local-only"
	if m.profile.Domain != "" {
		tunnel = m.profile.TunnelProvider + ": " + m.profile.Domain
	}
	return section("Profile Identity",
		fmt.Sprintf("Name: %s\nMode: %s\nRuntime: %s\nPort: %d\nTunnel: %s\nNotice: %s", m.profile.Name, m.profile.Mode, m.profile.RuntimeDir, m.profile.Port, tunnel, mutability),
	)
}

func (m model) renderServices() string {
	var b strings.Builder
	if len(m.serviceResults) == 0 {
		b.WriteString("No service status available.\n")
		return section("Services", b.String()) + section("Actions", "s start  x stop  k restart")
	}
	for _, result := range m.serviceResults {
		fmt.Fprintf(&b, "%-8s %-16s %s\n", result.Service, statusText(result.State), result.Detail)
	}
	return section("Services", b.String()) + section("Actions", "s start  x stop  k restart")
}

func (m model) renderModels() string {
	if m.modelErr != nil {
		return section("Model Validation", statusText("warning")+" "+m.modelErr.Error())
	}
	var b strings.Builder
	for _, entry := range m.models {
		fmt.Fprintf(&b, "%-24s %v\n", entry.ModelName, entry.LiteLLMParams["model"])
	}
	if b.Len() == 0 {
		b.WriteString("No model tiers found.")
	}
	return section("Model Tiers", b.String())
}

func (m model) renderDoctor() string {
	var b strings.Builder
	for _, check := range m.report.Checks {
		fmt.Fprintf(&b, "%-22s %-12s %s\n", check.Name, checkStatus(check), check.Detail)
	}
	if b.Len() == 0 {
		b.WriteString("No doctor checks available.")
	}
	return section("Doctor Checks", b.String())
}

func (m model) renderLogs() string {
	if m.logPreview == "" {
		return section("Recent LiteLLM Logs", "No LiteLLM log output yet.")
	}
	return section("Recent LiteLLM Logs", m.logPreview)
}

func runServiceAction(action func() ([]core.ServiceActionResult, error)) ([]core.ServiceActionResult, string) {
	results, err := action()
	if err != nil {
		return results, err.Error()
	}
	return results, "service action complete"
}

func loadProfile(runtimePath string) *core.Profile {
	profile, err := core.LoadProfileFromRuntime(runtimePath)
	if err != nil {
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

type setupProgress struct {
	items      []setupProgressItem
	nextAction string
}

type setupProgressItem struct {
	name   string
	state  string
	detail string
}

func (m model) setupProgress() setupProgress {
	profileOK := m.profile != nil
	configOK := doctorCheckOK(m.report, "config.yaml") || m.modelErr == nil
	secretsOK := doctorCheckOK(m.report, "LITELLM_MASTER_KEY")
	serviceOK := serviceState(m.serviceResults, "litellm") == core.ServiceStateRunning
	cursorOK := profileOK && configOK && secretsOK && serviceOK

	items := []setupProgressItem{
		{name: "Profile", state: boolState(profileOK), detail: profileDetail(m.profile)},
		{name: "Config", state: boolState(configOK), detail: configDetail(m.modelErr)},
		{name: "Secrets", state: boolState(secretsOK), detail: secretDetail(secretsOK)},
		{name: "Services", state: boolState(serviceOK), detail: serviceDetail(m.serviceResults)},
		{name: "Cursor", state: boolState(cursorOK), detail: cursorDetail(cursorOK)},
	}
	next := "Cursor settings ready"
	switch {
	case !profileOK || !configOK || !secretsOK:
		next = "Run ezyl3 setup"
	case !serviceOK:
		next = "Press s to start LiteLLM"
	case !cursorOK:
		next = "Review Cursor settings"
	}
	return setupProgress{items: items, nextAction: next}
}

func (p setupProgress) completeCount() int {
	count := 0
	for _, item := range p.items {
		if item.state == "ok" {
			count++
		}
	}
	return count
}

func (p setupProgress) overallStatus() string {
	if p.completeCount() == len(p.items) {
		return statusText("ready")
	}
	return statusText("warning") + " needs setup"
}

func (p setupProgress) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d complete\n", p.completeCount(), len(p.items))
	for _, item := range p.items {
		fmt.Fprintf(&b, "%-9s %-16s %s\n", item.name, statusText(item.state), item.detail)
	}
	return b.String()
}

func section(title, body string) string {
	return panelStyle.Render(titleStyle.Render(title)+"\n"+strings.TrimRight(body, "\n")) + "\n"
}

func tabTitle(tab int) string {
	if tab >= 0 && tab < len(tabs) {
		return tabs[tab]
	}
	return "Overview"
}

func (m model) statusSummary() string {
	progress := m.setupProgress()
	return "Bridge Status: " + progress.overallStatus() + "  |  Next: " + progress.nextAction
}

func statusText(state string) string {
	switch state {
	case "ok", "ready", core.ServiceStateRunning, core.ServiceStateLoaded:
		return okStyle.Render(state)
	case "warning", core.ServiceStateNotConfigured, core.ServiceStateStopped:
		return warnStyle.Render(state)
	case "missing", "fail":
		return missingStyle.Render(state)
	case core.ServiceStateUnknown:
		return mutedStyle.Render(state)
	default:
		return state
	}
}

func checkStatus(check core.Check) string {
	if check.OK {
		return statusText("ok")
	}
	return statusText("fail")
}

func boolState(ok bool) string {
	if ok {
		return "ok"
	}
	return "missing"
}

func doctorCheckOK(report core.DoctorReport, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.OK
		}
	}
	return false
}

func serviceState(results []core.ServiceActionResult, service string) string {
	for _, result := range results {
		if result.Service == service {
			return result.State
		}
	}
	return core.ServiceStateUnknown
}

func tunnelState(results []core.ServiceActionResult) string {
	state := serviceState(results, "ngrok")
	if state == core.ServiceStateUnknown {
		return core.ServiceStateNotConfigured
	}
	return state
}

func profileDetail(profile *core.Profile) string {
	if profile == nil {
		return "profile.json missing"
	}
	return profile.Name + " / " + profile.Mode
}

func configDetail(err error) string {
	if err != nil {
		return "config needs attention"
	}
	return "model tiers readable"
}

func secretDetail(ok bool) string {
	if ok {
		return "master key set"
	}
	return "master key missing"
}

func serviceDetail(results []core.ServiceActionResult) string {
	state := serviceState(results, "litellm")
	switch state {
	case core.ServiceStateRunning:
		return "LiteLLM running"
	case core.ServiceStateLoaded:
		return "LiteLLM loaded"
	case core.ServiceStateMissing:
		return "LaunchAgent missing"
	case core.ServiceStateUnknown:
		return "status unknown"
	default:
		return "LiteLLM " + state
	}
}

func cursorDetail(ok bool) string {
	if ok {
		return "Cursor settings ready"
	}
	return "waiting on setup"
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
