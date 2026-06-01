package setupwizard

import (
	"bytes"
	"fmt"
	"strings"

	"ezyl3/internal/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Options struct {
	Paths          core.ProfilePaths
	Domain         string
	ExecutablePath string
	Secrets        core.Secrets
	Force          bool
	SkipPythonDeps bool
	Dependencies   core.SetupDependencies
}

type setupRunner func(core.SetupOptions, core.SetupDependencies) (core.SetupResult, error)

type step int

const (
	stepProfile step = iota
	stepTunnel
	stepSecrets
	stepReview
	stepRunning
	stepDone
)

var stepNames = []string{"Profile", "Tunnel", "Secrets", "Review"}

type model struct {
	opts     Options
	step     step
	progress progress.Model
	help     help.Model
	keys     keyMap
	runner   setupRunner
	spinner  spinner.Model
	result   core.SetupResult
	err      error

	domainInput  textinput.Model
	secretInputs []textinput.Model
	secretFocus  int
}

type keyMap struct {
	next key.Binding
	back key.Binding
	quit key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		next: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next")),
		back: key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b/esc", "back")),
		quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.next, k.back, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.next, k.back, k.quit}}
}

func Run(opts Options) (core.SetupResult, error) {
	// Python dependency installation shells out to uv/pip, which stream verbose
	// output. Inside the Bubble Tea alt-screen that output corrupts the rendered
	// UI, so capture it into a buffer instead of letting it reach the terminal.
	// The buffer is surfaced only if setup fails.
	var installLog bytes.Buffer
	if opts.Dependencies.Installer == nil {
		opts.Dependencies.Installer = core.UVToolchain{Stdout: &installLog, Stderr: &installLog}
	}

	finalModel, err := tea.NewProgram(newModel(opts, core.RunSetup), tea.WithAltScreen()).Run()
	if err != nil {
		return core.SetupResult{}, err
	}
	m, ok := finalModel.(model)
	if !ok {
		return core.SetupResult{}, fmt.Errorf("setup wizard returned unexpected model")
	}
	if m.err != nil && installLog.Len() > 0 {
		return m.result, fmt.Errorf("%w\n\n--- dependency install output ---\n%s", m.err, installLog.String())
	}
	return m.result, m.err
}

func newModel(opts Options, runner setupRunner) model {
	if runner == nil {
		runner = core.RunSetup
	}
	helpView := help.New()
	helpView.ShortSeparator = "  "
	spin := spinner.New(spinner.WithSpinner(spinner.Line))
	return model{
		opts: opts,
		progress: progress.New(
			progress.WithWidth(42),
			progress.WithSolidFill("#5FD7AF"),
			progress.WithFillCharacters('=', '-'),
		),
		help:    helpView,
		keys:    newKeyMap(),
		runner:  runner,
		spinner: spin,

		domainInput:  newInput("name.ngrok-free.dev", opts.Domain, false),
		secretInputs: newSecretInputs(opts.Secrets),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.syncFocus()
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case setupDoneMsg:
		m.result = msg.result
		m.err = msg.err
		m.step = stepDone
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.next):
			if m.step == stepSecrets && m.secretFocus < len(m.secretInputs)-1 {
				m.secretFocus++
			} else if m.step == stepReview {
				m.step = stepRunning
				return m, m.runSetup()
			} else if m.step < stepReview {
				m.step++
			}
		case key.Matches(msg, m.keys.back):
			if m.step == stepSecrets && m.secretFocus > 0 {
				m.secretFocus--
			} else if m.step > stepProfile {
				m.step--
			}
		default:
			return m.updateInput(msg)
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("ezyl3 setup"))
	b.WriteString("\n")
	b.WriteString(m.progress.ViewAs(m.progressPercent()))
	b.WriteString("\n\n")
	b.WriteString(m.renderSteps())
	b.WriteString("\n\n")
	b.WriteString(m.renderStep())
	b.WriteString("\n\n")
	b.WriteString(m.help.View(m.keys))
	return b.String()
}

func (m model) progressPercent() float64 {
	if m.step >= stepRunning {
		return 1
	}
	return float64(m.step+1) / float64(len(stepNames))
}

func (m model) renderSteps() string {
	var lines []string
	for i, name := range stepNames {
		marker := "[ ]"
		if i < int(m.step) {
			marker = "[x]"
		}
		if i == int(m.step) {
			marker = ">> "
		}
		lines = append(lines, fmt.Sprintf("%s %s", marker, name))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderStep() string {
	switch m.step {
	case stepProfile:
		return fmt.Sprintf("Profile: %s\nRuntime: %s", m.opts.Paths.Profile, m.opts.Paths.ProfileDir)
	case stepTunnel:
		mode := "local-only setup"
		if strings.TrimSpace(m.domainInput.Value()) != "" {
			mode = "ngrok tunnel"
		}
		return "Tunnel: " + mode + "\n" + m.domainInput.View()
	case stepSecrets:
		var b strings.Builder
		b.WriteString("Enter optional provider secrets. Secret values are masked.\n")
		labels := []string{"Hugging Face token", "Ollama API key", "Hugging Face billing org", "LiteLLM master key"}
		for i, input := range m.secretInputs {
			fmt.Fprintf(&b, "%s: %s\n", labels[i], input.View())
		}
		return strings.TrimRight(b.String(), "\n")
	case stepReview:
		return "Review setup choices, then press enter to create the profile."
	case stepRunning:
		return m.spinner.View() + " Creating managed profile..."
	case stepDone:
		if m.err != nil {
			return "Setup failed: " + m.err.Error()
		}
		return m.result.Summary()
	default:
		return ""
	}
}

type setupDoneMsg struct {
	result core.SetupResult
	err    error
}

func (m model) runSetup() tea.Cmd {
	opts := m.setupOptions()
	deps := m.opts.Dependencies
	return func() tea.Msg {
		result, err := m.runner(opts, deps)
		return setupDoneMsg{result: result, err: err}
	}
}

func (m model) setupOptions() core.SetupOptions {
	return core.SetupOptions{
		Paths:          m.opts.Paths,
		Domain:         strings.TrimSpace(m.domainInput.Value()),
		ExecutablePath: m.opts.ExecutablePath,
		Secrets: core.Secrets{
			HFToken:          strings.TrimSpace(m.secretInputs[0].Value()),
			OllamaAPIKey:     strings.TrimSpace(m.secretInputs[1].Value()),
			HFBillTo:         strings.TrimSpace(m.secretInputs[2].Value()),
			LiteLLMMasterKey: strings.TrimSpace(m.secretInputs[3].Value()),
		},
		Force:          m.opts.Force,
		SkipPythonDeps: m.opts.SkipPythonDeps,
	}
}

func (m *model) syncFocus() {
	m.domainInput.Blur()
	for i := range m.secretInputs {
		m.secretInputs[i].Blur()
	}
	switch m.step {
	case stepTunnel:
		_ = m.domainInput.Focus()
	case stepSecrets:
		if m.secretFocus < 0 || m.secretFocus >= len(m.secretInputs) {
			m.secretFocus = 0
		}
		_ = m.secretInputs[m.secretFocus].Focus()
	}
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepTunnel:
		var cmd tea.Cmd
		m.domainInput, cmd = m.domainInput.Update(msg)
		return m, cmd
	case stepSecrets:
		var cmd tea.Cmd
		m.secretInputs[m.secretFocus], cmd = m.secretInputs[m.secretFocus].Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func newInput(placeholder, value string, secret bool) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Width = 48
	if secret {
		input.EchoMode = textinput.EchoPassword
	}
	input.SetValue(value)
	return input
}

func newSecretInputs(secrets core.Secrets) []textinput.Model {
	return []textinput.Model{
		newInput("optional", secrets.HFToken, true),
		newInput("optional", secrets.OllamaAPIKey, true),
		newInput("optional", secrets.HFBillTo, false),
		newInput("generated if blank", secrets.LiteLLMMasterKey, true),
	}
}
