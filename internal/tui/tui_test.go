package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestModelInitialViewContainsWorkspace(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}})
	view := model.View()
	if !strings.Contains(view, "agw") {
		t.Fatalf("view = %q", view)
	}
}

func TestModelInitialViewShowsWorkspaceStateAndDetails(t *testing.T) {
	reports := []doctor.Report{{
		WorkspaceID: "agw",
		State:       doctor.StateNeedsApply,
		Checks: []doctor.Check{
			{Name: "project path", Status: doctor.CheckPass, Detail: "/repo"},
			{Name: "compose.yaml", Status: doctor.CheckFail, Detail: "missing"},
		},
		NextAction: "agw workspace apply agw <generated-dir>",
	}}
	model := NewModelWithReports(reports, nil)
	view := model.View()
	for _, want := range []string{"agw", "needs-apply", "compose.yaml", "Next:", "agw workspace apply agw"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestModelRefreshUpdatesSelectedReport(t *testing.T) {
	actions := &fakeActions{
		refreshReport: doctor.Report{WorkspaceID: "agw", State: doctor.StateRunning},
	}
	model := NewModelWithActions([]workspace.LocatedDefinition{{
		Definition: workspace.Definition{ID: "agw"},
		Path:       "/repo/agw.yaml",
	}}, actions)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model = updated.(Model)
	if actions.refreshCalls != 1 {
		t.Fatalf("refresh calls = %d", actions.refreshCalls)
	}
	if actions.lastRefresh.Path != "/repo/agw.yaml" {
		t.Fatalf("refresh path = %q", actions.lastRefresh.Path)
	}
	if !strings.Contains(model.View(), "running") {
		t.Fatalf("view did not refresh:\n%s", model.View())
	}
}

func TestModelReportOnlyRefreshFailsWithoutCallingActions(t *testing.T) {
	actions := &fakeActions{refreshReport: doctor.Report{WorkspaceID: "agw", State: doctor.StateRunning}}
	model := NewModelWithReports([]doctor.Report{{WorkspaceID: "agw", State: doctor.StateNeedsApply}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model = updated.(Model)

	if actions.refreshCalls != 0 {
		t.Fatalf("refresh calls = %d", actions.refreshCalls)
	}
	if !strings.Contains(model.View(), "refresh failed: workspace path unavailable") {
		t.Fatalf("view missing refresh failure:\n%s", model.View())
	}
}

func TestModelInvokesActionForSelectedWorkspace(t *testing.T) {
	items := []workspace.LocatedDefinition{
		{Definition: workspace.Definition{ID: "first"}},
		{Definition: workspace.Definition{ID: "second"}},
	}
	actions := &fakeActions{buildResult: "built second"}
	model := NewModelWithActions(items, actions)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)

	if actions.buildWorkspace != "second" {
		t.Fatalf("build workspace = %q", actions.buildWorkspace)
	}
	if !strings.Contains(model.View(), "build ok: built second") {
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

func TestModelShowsActionResultMessage(t *testing.T) {
	actions := &fakeActions{prepareResult: "wrote /tmp/agw/prompt.md"}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	model = updated.(Model)

	if !strings.Contains(model.View(), "prepare ok: wrote /tmp/agw/prompt.md") {
		t.Fatalf("view missing prepare result:\n%s", model.View())
	}
}

type fakeActions struct {
	buildWorkspace string
	buildResult    string
	prepareResult  string
	refreshReport  doctor.Report
	refreshCalls   int
	lastRefresh    workspace.LocatedDefinition
	statusErr      error
}

func (a *fakeActions) Status(workspace.LocatedDefinition) (string, error) {
	return "", a.statusErr
}
func (a *fakeActions) Build(item workspace.LocatedDefinition) (string, error) {
	a.buildWorkspace = item.Definition.ID
	return a.buildResult, nil
}
func (a *fakeActions) Up(workspace.LocatedDefinition) (string, error)     { return "", nil }
func (a *fakeActions) Down(workspace.LocatedDefinition) (string, error)   { return "", nil }
func (a *fakeActions) Attach(workspace.LocatedDefinition) (string, error) { return "", nil }
func (a *fakeActions) Prepare(workspace.LocatedDefinition) (string, error) {
	return a.prepareResult, nil
}
func (a *fakeActions) Refresh(item workspace.LocatedDefinition) (doctor.Report, error) {
	a.refreshCalls++
	a.lastRefresh = item
	return a.refreshReport, nil
}
