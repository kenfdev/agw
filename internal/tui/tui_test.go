package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/base"
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

func TestModelConfirmsBuildActionForSelectedWorkspace(t *testing.T) {
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
	if actions.buildWorkspace != "" {
		t.Fatalf("build ran before confirmation for workspace %q", actions.buildWorkspace)
	}
	if !strings.Contains(model.View(), "Build workspace second?") {
		t.Fatalf("view missing build confirmation:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)

	if actions.buildWorkspace != "second" {
		t.Fatalf("build workspace = %q", actions.buildWorkspace)
	}
	if !strings.Contains(model.View(), "build ok: built second") {
		t.Fatalf("view missing action status:\n%s", model.View())
	}
}

func TestModelShowsBaseImageInTopBar(t *testing.T) {
	model := NewModelWithReportsAndBaseStatus(
		[]doctor.Report{{WorkspaceID: "agw", State: doctor.StateRunning}},
		baseStatus("agw-base:latest", base.StatusAvailable, "3h"),
		nil,
	)

	view := model.View()
	for _, want := range []string{"Base: agw-base:latest available 3h", "focus:workspaces"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestModelTabFocusesBaseWhenConfigured(t *testing.T) {
	model := NewModelWithReportsAndBaseStatus(
		[]doctor.Report{{WorkspaceID: "agw", State: doctor.StateRunning}},
		baseStatus("agw-base:latest", base.StatusAvailable, "3h"),
		nil,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	view := model.View()
	for _, want := range []string{"[Base: agw-base:latest available 3h]", "focus:base", "Base image:  agw-base:latest", "Context:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestModelTabSkipsBaseWhenNotConfigured(t *testing.T) {
	model := NewModelWithReports([]doctor.Report{{WorkspaceID: "agw", State: doctor.StateRunning}}, nil)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	view := model.View()
	if strings.Contains(view, "focus:base") || strings.Contains(view, "Base:") {
		t.Fatalf("tab should not focus missing base:\n%s", view)
	}
}

func TestModelConfirmsBaseBuildWhenBaseFocused(t *testing.T) {
	actions := &fakeActions{buildBaseResult: "built base"}
	model := NewModelWithReportsAndBaseStatus(
		[]doctor.Report{{WorkspaceID: "agw", State: doctor.StateRunning}},
		baseStatus("agw-base:latest", base.StatusMissing, ""),
		actions,
	)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)
	if actions.buildBaseCalls != 0 {
		t.Fatalf("base build ran before confirmation")
	}
	if !strings.Contains(model.View(), "Build base image agw-base:latest?") {
		t.Fatalf("view missing base build confirmation:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)
	if actions.buildBaseCalls != 1 {
		t.Fatalf("base build calls = %d", actions.buildBaseCalls)
	}
	if !strings.Contains(model.View(), "base build ok: built base") {
		t.Fatalf("view missing base build result:\n%s", model.View())
	}
}

func baseStatus(image string, status base.ImageStatus, age string) *base.Status {
	created := time.Date(2026, 7, 2, 10, 15, 0, 0, time.UTC)
	return &base.Status{
		Config: base.Config{
			Image:      image,
			ContextDir: "/agw/base",
			Dockerfile: "/agw/base/Dockerfile",
		},
		Status:    status,
		CreatedAt: &created,
		Age:       age,
	}
}

func TestModelStartsSelectedWorkspace(t *testing.T) {
	items := []workspace.LocatedDefinition{
		{Definition: workspace.Definition{ID: "first"}},
		{Definition: workspace.Definition{ID: "second"}},
	}
	actions := &fakeActions{startResult: "started second"}
	model := NewModelWithActions(items, actions)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model = updated.(Model)

	if actions.startWorkspace != "second" {
		t.Fatalf("start workspace = %q", actions.startWorkspace)
	}
	if !strings.Contains(model.View(), "start ok: started second") {
		t.Fatalf("view missing start status:\n%s", model.View())
	}
}

func TestModelMovesSelectionWithVimKeys(t *testing.T) {
	items := []workspace.LocatedDefinition{
		{Definition: workspace.Definition{ID: "first"}},
		{Definition: workspace.Definition{ID: "second"}},
	}
	actions := &fakeActions{buildResult: "built second"}
	model := NewModelWithActions(items, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)

	if actions.buildWorkspace != "second" {
		t.Fatalf("build workspace after j = %q", actions.buildWorkspace)
	}

	actions.buildWorkspace = ""
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)

	if actions.buildWorkspace != "first" {
		t.Fatalf("build workspace after k = %q", actions.buildWorkspace)
	}
}

func TestModelShowsShellActionError(t *testing.T) {
	actions := &fakeActions{shellErr: errors.New("docker unavailable")}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(Model)

	if actions.shellCalls != 0 {
		t.Fatalf("shell calls = %d, want 0 before TUI exits", actions.shellCalls)
	}
	if !model.RequestedShell() {
		t.Fatal("shell was not requested")
	}
	if !model.quitting {
		t.Fatal("model did not quit for shell handoff")
	}
}

func TestModelInitialViewUsesK9sStyleChrome(t *testing.T) {
	model := NewModelWithReports([]doctor.Report{{WorkspaceID: "agw", State: doctor.StateRunning}}, nil)
	view := model.View()
	for _, want := range []string{"AGW / Workspaces", "WORKSPACE", "STATE", "Logs", "Keys", "j/k"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestModelUsesWindowSizeForFullScreenBorderedLayout(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{
		ID:        "agw",
		Container: workspace.Container{Service: "dev"},
	}}})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model)
	view := model.View()

	for _, want := range []string{"╭", "╰", "AGW / Workspaces", "Details", "Logs"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 24 {
		t.Fatalf("line count = %d, want 24\n%s", len(lines), view)
	}
}

func TestModelViewerUsesFullScreenBorderedLayout(t *testing.T) {
	actions := &fakeActions{logsResult: "ready"}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	view := updated.(Model).View()

	for _, want := range []string{"╭", "Logs", "ready", "esc back"} {
		if !strings.Contains(view, want) {
			t.Fatalf("viewer missing %q:\n%s", want, view)
		}
	}
	if lines := strings.Split(view, "\n"); len(lines) != 20 {
		t.Fatalf("line count = %d, want 20\n%s", len(lines), view)
	}
}

func TestModelShowsActionResultMessage(t *testing.T) {
	actions := &fakeActions{prepareResult: "rendered preparation prompt for agw"}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	model = updated.(Model)

	if !strings.Contains(model.View(), "prepare ok: rendered preparation prompt for agw") {
		t.Fatalf("view missing prepare result:\n%s", model.View())
	}
}

type fakeActions struct {
	buildWorkspace  string
	buildResult     string
	startWorkspace  string
	startResult     string
	prepareResult   string
	logsResult      string
	refreshReport   doctor.Report
	refreshCalls    int
	lastRefresh     workspace.LocatedDefinition
	statusErr       error
	shellErr        error
	shellCalls      int
	openPath        string
	copiedText      string
	buildBaseCalls  int
	buildBaseResult string
	baseStatus      *base.Status
	baseStatusCalls int
}

func (a *fakeActions) Status(workspace.LocatedDefinition) (string, error) {
	return "", a.statusErr
}
func (a *fakeActions) Build(item workspace.LocatedDefinition) (string, error) {
	a.buildWorkspace = item.Definition.ID
	return a.buildResult, nil
}
func (a *fakeActions) BaseStatus() (*base.Status, error) {
	a.baseStatusCalls++
	return a.baseStatus, nil
}
func (a *fakeActions) BuildBase() (string, error) {
	a.buildBaseCalls++
	return a.buildBaseResult, nil
}
func (a *fakeActions) Start(item workspace.LocatedDefinition) (string, error) {
	a.startWorkspace = item.Definition.ID
	return a.startResult, nil
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
func (a *fakeActions) Logs(workspace.LocatedDefinition) (string, error) {
	return a.logsResult, nil
}
func (a *fakeActions) OpenShell(workspace.LocatedDefinition) (string, error) {
	a.shellCalls++
	return "", a.shellErr
}
func (a *fakeActions) OpenPath(path string) (string, error) {
	a.openPath = path
	return path, nil
}
func (a *fakeActions) EditPath(string) (string, error) { return "", nil }
func (a *fakeActions) CopyText(text string) (string, error) {
	a.copiedText = text
	return text, nil
}

func TestModelOpensReadOnlyWorkspaceFiles(t *testing.T) {
	item := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: "workspaces/agw"},
		},
		Path: "/tmp/agw/agw.yaml",
	}
	model := NewModelWithActions([]workspace.LocatedDefinition{item}, &fakeActions{})

	for _, tc := range []struct {
		key  string
		want string
	}{
		{key: "d", want: "agw.yaml"},
		{key: "c", want: "compose.yaml"},
		{key: "f", want: "Dockerfile"},
	} {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		got := updated.(Model).View()
		if !strings.Contains(got, tc.want) || !strings.Contains(got, "Viewer") {
			t.Fatalf("key %q view missing viewer title %q:\n%s", tc.key, tc.want, got)
		}
	}
}

func TestModelOpensLogsViewer(t *testing.T) {
	actions := &fakeActions{logsResult: "web-1 ready"}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	view := updated.(Model).View()
	if !strings.Contains(view, "Logs") || !strings.Contains(view, "web-1 ready") {
		t.Fatalf("view missing logs:\n%s", view)
	}
}

func TestModelFiltersWorkspaces(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{
		{Definition: workspace.Definition{ID: "api"}},
		{Definition: workspace.Definition{ID: "web"}},
	})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("we")})
	model = updated.(Model)

	view := model.View()
	if strings.Contains(view, "api") || !strings.Contains(view, "web") || !strings.Contains(view, "Filter: we") {
		t.Fatalf("filtered view mismatch:\n%s", view)
	}
}

func TestModelShowsHelpAndCommandPalette(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if view := updated.(Model).View(); !strings.Contains(view, "Help") || !strings.Contains(view, "l logs") || !strings.Contains(view, "ctrl+d stop/down") || strings.Contains(view, "x stop/down") {
		t.Fatalf("help view mismatch:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	if view := updated.(Model).View(); !strings.Contains(view, "Commands") || !strings.Contains(view, "open") {
		t.Fatalf("command view mismatch:\n%s", view)
	}
}

func TestModelConfirmsDownAction(t *testing.T) {
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, &fakeActions{})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	model = updated.(Model)
	view := model.View()
	for _, want := range []string{"AGW / Workspaces", "WORKSPACE", "Confirm", "Stop workspace agw?"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation view missing %q:\n%s", want, view)
		}
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)
	if !strings.Contains(model.View(), "down ok") {
		t.Fatalf("view missing down result:\n%s", model.View())
	}
}

func TestModelCancelsBuildConfirmation(t *testing.T) {
	actions := &fakeActions{}
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = updated.(Model)

	if actions.buildWorkspace != "" {
		t.Fatalf("build ran despite cancellation for workspace %q", actions.buildWorkspace)
	}
	if !strings.Contains(model.View(), "build canceled") {
		t.Fatalf("view missing build cancellation:\n%s", model.View())
	}
}

func TestModelXDoesNotConfirmDown(t *testing.T) {
	model := NewModelWithActions([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}}, &fakeActions{})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if strings.Contains(model.View(), "Stop workspace agw?") {
		t.Fatalf("x should not open confirmation:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(Model)
	if strings.Contains(model.View(), "down ok") {
		t.Fatalf("unexpected down result after x/y:\n%s", model.View())
	}
}

func TestModelTabDoesNotFocusBaseWhenBaseIsUnavailable(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	view := updated.(Model).View()
	if strings.Contains(view, "focus:base") {
		t.Fatalf("tab should not focus base when unavailable:\n%s", view)
	}
}

func TestModelCopiesOnlyProjectPathWithY(t *testing.T) {
	actions := &fakeActions{}
	model := NewModelWithActions([]workspace.LocatedDefinition{{
		Definition: workspace.Definition{
			ID:       "agw",
			Projects: []workspace.Project{{Name: "app", HostPath: "/src/app"}},
		},
	}}, actions)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	model = updated.(Model)

	if actions.copiedText != "/src/app" {
		t.Fatalf("copied text = %q", actions.copiedText)
	}
	if model.quitting || cmd != nil {
		t.Fatalf("copy should not quit TUI, quitting=%v cmd=%v", model.quitting, cmd)
	}
}

func TestModelShowsProjectSelectorForMultipleProjectsBeforeCopying(t *testing.T) {
	actions := &fakeActions{}
	model := NewModelWithActions([]workspace.LocatedDefinition{{
		Definition: workspace.Definition{
			ID: "agw",
			Projects: []workspace.Project{
				{Name: "api", HostPath: "/src/api"},
				{Name: "web", HostPath: "/src/web"},
			},
		},
	}}, actions)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Project Path") || !strings.Contains(view, "api") || !strings.Contains(view, "web") {
		t.Fatalf("project selector view mismatch:\n%s", view)
	}
	if actions.copiedText != "" {
		t.Fatalf("copied text before selection = %q", actions.copiedText)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if actions.copiedText != "/src/web" {
		t.Fatalf("copied text = %q", actions.copiedText)
	}
	if strings.Contains(model.View(), "Project Path") {
		t.Fatalf("selector did not close:\n%s", model.View())
	}
}

func TestModelJDoesNotJumpOrQuit(t *testing.T) {
	actions := &fakeActions{}
	model := NewModelWithActions([]workspace.LocatedDefinition{{
		Definition: workspace.Definition{
			ID:       "agw",
			Projects: []workspace.Project{{Name: "app", HostPath: "/src/app"}},
		},
	}}, actions)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	model = updated.(Model)

	if actions.openPath != "" || actions.copiedText != "" || model.quitting || cmd != nil {
		t.Fatalf("J should be inert, open=%q copy=%q quitting=%v cmd=%v", actions.openPath, actions.copiedText, model.quitting, cmd)
	}
}
