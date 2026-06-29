package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
)

type Model struct {
	workspaces []workspace.LocatedDefinition
	reports    []doctor.Report
	actions    Actions
	selected   int
	status     string
	quitting   bool
}

type Actions interface {
	Status(workspace.LocatedDefinition) (string, error)
	Build(workspace.LocatedDefinition) (string, error)
	Up(workspace.LocatedDefinition) (string, error)
	Down(workspace.LocatedDefinition) (string, error)
	Attach(workspace.LocatedDefinition) (string, error)
	Prepare(workspace.LocatedDefinition) (string, error)
	Refresh(workspace.LocatedDefinition) (doctor.Report, error)
}

func NewModel(workspaces []workspace.LocatedDefinition) Model {
	return newModel(workspaces, nil, nil)
}

func NewModelWithActions(workspaces []workspace.LocatedDefinition, actions Actions) Model {
	return newModel(workspaces, nil, actions)
}

func NewModelWithReports(reports []doctor.Report, actions Actions) Model {
	return newModel(nil, reports, actions)
}

func newModel(workspaces []workspace.LocatedDefinition, reports []doctor.Report, actions Actions) Model {
	return Model{
		workspaces: workspaces,
		reports:    reports,
		actions:    actions,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected+1 < len(m.workspaces) {
				m.selected++
			}
		case "home", "g":
			m.selected = 0
		case "end", "G":
			if len(m.workspaces) > 0 {
				m.selected = len(m.workspaces) - 1
			}
		case "s":
			m = m.runAction("status", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Status(item)
			})
		case "b":
			m = m.runAction("build", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Build(item)
			})
		case "u":
			m = m.runAction("up", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Up(item)
			})
		case "d":
			m = m.runAction("down", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Down(item)
			})
		case "a":
			m = m.runAction("attach", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Attach(item)
			})
		case "p":
			m = m.runAction("prepare", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Prepare(item)
			})
		case "r":
			m = m.refreshSelectedReport()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	lines := []string{"AGW Workspaces"}
	if len(m.workspaces) == 0 {
		lines = append(lines, "No workspaces")
	} else {
		for i, item := range m.workspaces {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			report := m.reportAt(i)
			lines = append(lines, fmt.Sprintf("%s%s %s %s", prefix, item.Definition.ID, report.State, firstFailingCheck(report)))
		}
	}

	if report, ok := m.selectedReport(); ok {
		lines = append(lines,
			"",
			"Details",
			fmt.Sprintf("  Workspace: %s", report.WorkspaceID),
			fmt.Sprintf("  State: %s", report.State),
			fmt.Sprintf("  Next: %s", report.NextAction),
			"",
			"Checks",
		)
		if len(report.Checks) == 0 {
			lines = append(lines, "  none")
		} else {
			for _, check := range report.Checks {
				lines = append(lines, fmt.Sprintf("  %s %s: %s", checkStatusSymbol(check.Status), check.Name, check.Detail))
			}
		}
	}
	lines = append(lines, "", "Log")
	if m.status != "" {
		lines = append(lines, "  "+m.status)
	} else {
		lines = append(lines, "  idle")
	}
	lines = append(lines, "", "↑/↓ move cursor • s status • r refresh • b build • u up • d down • a attach • p prepare • q quit")
	return strings.Join(lines, "\n")
}

func (m Model) runAction(name string, run func(workspace.LocatedDefinition) (string, error)) Model {
	if m.actions == nil {
		m.status = name + " failed: no actions configured"
		return m
	}
	if len(m.workspaces) == 0 {
		m.status = name + " failed: no workspace selected"
		return m
	}
	item := m.workspaces[m.selected]
	result, err := run(item)
	if err != nil {
		m.status = name + " failed: " + err.Error()
		return m
	}
	if result == "" {
		result = item.Definition.ID
	}
	m.status = name + " ok: " + result
	return m
}

func (m Model) refreshSelectedReport() Model {
	if m.actions == nil {
		m.status = "refresh failed: no actions configured"
		return m
	}
	if len(m.workspaces) == 0 {
		m.status = "refresh failed: workspace path unavailable"
		return m
	}
	if m.selected >= len(m.workspaces) {
		m.status = "refresh failed: no workspace selected"
		return m
	}

	item := m.workspaces[m.selected]
	report, err := m.actions.Refresh(item)
	if err != nil {
		m.status = "refresh failed: " + err.Error()
		return m
	}
	if report.WorkspaceID == "" {
		report.WorkspaceID = item.Definition.ID
	}
	if m.selected < len(m.reports) {
		m.reports[m.selected] = report
	} else {
		for len(m.reports) < m.selected {
			m.reports = append(m.reports, doctor.Report{})
		}
		m.reports = append(m.reports, report)
	}
	m.status = "refresh ok: " + report.WorkspaceID
	return m
}

func (m Model) selectedReport() (doctor.Report, bool) {
	if m.selected < len(m.reports) {
		return m.reports[m.selected], true
	}
	return doctor.Report{}, false
}

func (m Model) reportAt(index int) doctor.Report {
	if index < len(m.reports) {
		return m.reports[index]
	}
	if index < len(m.workspaces) {
		return doctor.Report{WorkspaceID: m.workspaces[index].Definition.ID}
	}
	return doctor.Report{}
}

func firstFailingCheck(report doctor.Report) string {
	for _, check := range report.Checks {
		if check.Status == doctor.CheckFail {
			return check.Name
		}
	}
	return "-"
}

func checkStatusSymbol(status doctor.CheckStatus) string {
	switch status {
	case doctor.CheckPass:
		return "✓"
	case doctor.CheckFail:
		return "✗"
	case doctor.CheckWarn:
		return "!"
	case doctor.CheckSkip:
		return "-"
	default:
		return "-"
	}
}

func Run(workspaces []workspace.LocatedDefinition) error {
	_, err := tea.NewProgram(NewModel(workspaces)).Run()
	return err
}

func RunWithActions(workspaces []workspace.LocatedDefinition, actions Actions) error {
	_, err := tea.NewProgram(NewModelWithActions(workspaces, actions)).Run()
	return err
}

func RunWithReports(workspaces []workspace.LocatedDefinition, reports []doctor.Report, actions Actions) error {
	_, err := tea.NewProgram(Model{
		workspaces: workspaces,
		reports:    reports,
		actions:    actions,
	}).Run()
	return err
}
