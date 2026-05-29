package tui

import (
	"fmt"

	"ezyl3/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	runtime core.Runtime
	report  core.DoctorReport
}

func Run(runtime core.Runtime) error {
	_, err := tea.NewProgram(newModel(runtime), tea.WithAltScreen()).Run()
	return err
}

func newModel(runtime core.Runtime) model {
	return model{runtime: runtime, report: core.Doctor(runtime)}
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
		case "r":
			m.report = core.Doctor(m.runtime)
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("ezyl3 LiteLLM Cursor Bridge")
	body := title + "\n\n" + fmt.Sprintf("Runtime: %s\n\n", m.runtime.Path)
	for _, check := range m.report.Checks {
		mark := "x"
		if check.OK {
			mark = "ok"
		}
		body += fmt.Sprintf("%-22s %-4s %s\n", check.Name, mark, check.Detail)
	}
	body += "\nPress r to refresh, q to quit.\n"
	return body
}
