package setupwizard

import (
	"fmt"
	"strings"

	"ezyl3/internal/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Options struct {
	Paths          core.ProfilePaths
	Domain         string
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
	err      error
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
	finalModel, err := tea.NewProgram(newModel(opts, core.RunSetup), tea.WithAltScreen()).Run()
	if err != nil {
		return core.SetupResult{}, err
	}
	m, ok := finalModel.(model)
	if !ok {
		return core.SetupResult{}, fmt.Errorf("setup wizard returned unexpected model")
	}
	return core.SetupResult{}, m.err
}

func newModel(opts Options, runner setupRunner) model {
	if runner == nil {
		runner = core.RunSetup
	}
	helpView := help.New()
	helpView.ShortSeparator = "  "
	return model{
		opts: opts,
		progress: progress.New(
			progress.WithWidth(42),
			progress.WithSolidFill("#5FD7AF"),
			progress.WithFillCharacters('=', '-'),
		),
		help:   helpView,
		keys:   newKeyMap(),
		runner: runner,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.next):
			if m.step < stepReview {
				m.step++
			}
		case key.Matches(msg, m.keys.back):
			if m.step > stepProfile {
				m.step--
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("ezyl3 setup"))
	b.WriteString("\n")
	b.WriteString(m.progress.ViewAs(float64(m.step+1) / float64(len(stepNames))))
	b.WriteString("\n\n")
	b.WriteString(m.renderSteps())
	b.WriteString("\n\n")
	b.WriteString(m.renderStep())
	b.WriteString("\n\n")
	b.WriteString(m.help.View(m.keys))
	return b.String()
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
		return "Choose local-only setup or enter an ngrok domain."
	case stepSecrets:
		return "Enter optional provider secrets. Secret values are masked."
	case stepReview:
		return "Review setup choices, then press enter to create the profile."
	default:
		return ""
	}
}
