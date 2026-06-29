package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/workspace"
)

type Model struct {
	workspaces []workspace.LocatedDefinition
	selected   int
	quitting   bool
}

func NewModel(workspaces []workspace.LocatedDefinition) Model {
	return Model{workspaces: workspaces}
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
	lines = append(lines, "", "↑/↓ move cursor • q quit")
	return strings.Join(lines, "\n")
}

func Run(workspaces []workspace.LocatedDefinition) error {
	_, err := tea.NewProgram(NewModel(workspaces)).Run()
	return err
}
