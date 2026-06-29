package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/workspace"
)

type Model struct {
	workspaces []workspace.LocatedDefinition
	actions    Actions
	selected   int
	status     string
	quitting   bool
}

type Actions interface {
	Status(workspace.LocatedDefinition) error
	Build(workspace.LocatedDefinition) error
	Up(workspace.LocatedDefinition) error
	Down(workspace.LocatedDefinition) error
	Attach(workspace.LocatedDefinition) error
	Prepare(workspace.LocatedDefinition) error
}

func NewModel(workspaces []workspace.LocatedDefinition) Model {
	return Model{workspaces: workspaces}
}

func NewModelWithActions(workspaces []workspace.LocatedDefinition, actions Actions) Model {
	return Model{workspaces: workspaces, actions: actions}
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
			m = m.runAction("status", func(item workspace.LocatedDefinition) error {
				return m.actions.Status(item)
			})
		case "b":
			m = m.runAction("build", func(item workspace.LocatedDefinition) error {
				return m.actions.Build(item)
			})
		case "u":
			m = m.runAction("up", func(item workspace.LocatedDefinition) error {
				return m.actions.Up(item)
			})
		case "d":
			m = m.runAction("down", func(item workspace.LocatedDefinition) error {
				return m.actions.Down(item)
			})
		case "a":
			m = m.runAction("attach", func(item workspace.LocatedDefinition) error {
				return m.actions.Attach(item)
			})
		case "p":
			m = m.runAction("prepare", func(item workspace.LocatedDefinition) error {
				return m.actions.Prepare(item)
			})
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	lines := make([]string, 0, len(m.workspaces)+2)
	if len(m.workspaces) == 0 {
		lines = append(lines, "No workspaces")
	} else {
		for i, item := range m.workspaces {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			lines = append(lines, fmt.Sprintf("%s%s", prefix, item.Definition.ID))
		}
	}
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	lines = append(lines, "", "↑/↓ move cursor • s status • b build • u up • d down • a attach • p prepare • q quit")
	return strings.Join(lines, "\n")
}

func (m Model) runAction(name string, run func(workspace.LocatedDefinition) error) Model {
	if m.actions == nil {
		m.status = name + " failed: no actions configured"
		return m
	}
	if len(m.workspaces) == 0 {
		m.status = name + " failed: no workspace selected"
		return m
	}
	item := m.workspaces[m.selected]
	if err := run(item); err != nil {
		m.status = name + " failed: " + err.Error()
		return m
	}
	m.status = name + " ok: " + item.Definition.ID
	return m
}

func Run(workspaces []workspace.LocatedDefinition) error {
	_, err := tea.NewProgram(NewModel(workspaces)).Run()
	return err
}

func RunWithActions(workspaces []workspace.LocatedDefinition, actions Actions) error {
	_, err := tea.NewProgram(NewModelWithActions(workspaces, actions)).Run()
	return err
}
