package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestModelInitialViewContainsWorkspace(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}})
	view := model.View()
	if !strings.Contains(view, "agw") {
		t.Fatalf("view = %q", view)
	}
}

func TestModelInvokesActionForSelectedWorkspace(t *testing.T) {
	items := []workspace.LocatedDefinition{
		{Definition: workspace.Definition{ID: "first"}},
		{Definition: workspace.Definition{ID: "second"}},
	}
	actions := &fakeActions{}
	model := NewModelWithActions(items, actions)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)

	if actions.buildWorkspace != "second" {
		t.Fatalf("build workspace = %q", actions.buildWorkspace)
	}
	if !strings.Contains(model.View(), "build ok") {
		t.Fatalf("view missing action status:\n%s", model.View())
	}
}

func TestModelShowsActionError(t *testing.T) {
	actions := &fakeActions{statusErr: errors.New("docker unavailable")}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(Model)

	if !strings.Contains(model.View(), "status failed: docker unavailable") {
		t.Fatalf("view missing action error:\n%s", model.View())
	}
}

type fakeActions struct {
	buildWorkspace string
	statusErr      error
}

func (a *fakeActions) Status(workspace.LocatedDefinition) error { return a.statusErr }
func (a *fakeActions) Build(item workspace.LocatedDefinition) error {
	a.buildWorkspace = item.Definition.ID
	return nil
}
func (a *fakeActions) Up(workspace.LocatedDefinition) error      { return nil }
func (a *fakeActions) Down(workspace.LocatedDefinition) error    { return nil }
func (a *fakeActions) Attach(workspace.LocatedDefinition) error  { return nil }
func (a *fakeActions) Prepare(workspace.LocatedDefinition) error { return nil }
