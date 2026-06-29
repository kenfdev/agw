package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestDiagnoseNeedsPrepareWhenPromptMissing(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	mustMkdir(t, project)
	located := locatedDefinition(root, workspace.Definition{
		ID:        "agw",
		Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
		Projects:  []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
	})

	report := Diagnose(located, fakeRunner{})

	if report.State != StateNeedsPrepare {
		t.Fatalf("State = %q, want %q", report.State, StateNeedsPrepare)
	}
	if !strings.Contains(report.NextAction, "agw workspace prepare agw") {
		t.Fatalf("NextAction = %q", report.NextAction)
	}
	assertCheck(t, report, "project path", CheckPass)
	assertCheck(t, report, "prompt", CheckFail)
}

func TestDiagnoseBrokenWhenProjectPathMissing(t *testing.T) {
	root := t.TempDir()
	located := locatedDefinition(root, workspace.Definition{
		ID:        "agw",
		Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
		Projects:  []workspace.Project{{Name: "agw", Path: filepath.Join(root, "missing"), MountPath: "/workspace"}},
	})

	report := Diagnose(located, fakeRunner{})

	if report.State != StateBroken {
		t.Fatalf("State = %q, want %q", report.State, StateBroken)
	}
	assertCheck(t, report, "project path", CheckFail)
}

func TestDiagnoseNeedsApplyWhenComposeMissing(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	mustMkdir(t, project)
	ws := filepath.Join(root, "ws")
	mustMkdir(t, ws)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{ID: "agw", Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}}},
		Path:       filepath.Join(ws, "agw.yaml"),
	}

	report := Diagnose(located, fakeRunner{})

	if report.State != StateNeedsApply {
		t.Fatalf("State = %q, want %q", report.State, StateNeedsApply)
	}
	assertCheck(t, report, "compose.yaml", CheckFail)
}

func TestDiagnoseBrokenWhenSelectedNetworkMissing(t *testing.T) {
	_, project, ws := validWorkspaceDirs(t)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(ws, "compose.yaml"), fmt.Sprintf("services:\n  dev:\n    build: .\n    volumes:\n      - %s:/workspace\n    networks:\n      - target\nnetworks:\n  target:\n    external: true\n    name: target_default\n", project))
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID: "agw", Container: workspace.Container{Service: "dev"},
			Projects: []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
			Networks: &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "target_default"}}},
		},
		Path: filepath.Join(ws, "agw.yaml"),
	}

	report := Diagnose(located, fakeRunner{networks: map[string]bool{"target_default": false}})

	if report.State != StateBroken {
		t.Fatalf("State = %q, want %q", report.State, StateBroken)
	}
	assertCheck(t, report, "external network", CheckFail)
}

func TestDiagnoseBrokenWhenServiceMissing(t *testing.T) {
	_, project, ws := validWorkspaceDirs(t)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(ws, "compose.yaml"), fmt.Sprintf(
		"services:\n  other:\n    build: .\n    volumes:\n      - %s:/workspace\n",
		project,
	))
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev"},
			Projects:  []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
		},
		Path: filepath.Join(ws, "agw.yaml"),
	}

	report := Diagnose(located, fakeRunner{})

	if report.State != StateBroken {
		t.Fatalf("State = %q, want %q", report.State, StateBroken)
	}
	assertCheckDetail(t, report, "service", CheckFail, "service dev not found in compose.yaml")
}

func TestDiagnoseBrokenWhenMountMissing(t *testing.T) {
	_, project, ws := validWorkspaceDirs(t)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(ws, "compose.yaml"), fmt.Sprintf(
		"services:\n  dev:\n    build: .\n    volumes:\n      - %s:/missing\n",
		project,
	))
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev"},
			Projects:  []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
		},
		Path: filepath.Join(ws, "agw.yaml"),
	}

	report := Diagnose(located, fakeRunner{})

	if report.State != StateBroken {
		t.Fatalf("State = %q, want %q", report.State, StateBroken)
	}
	assertCheckDetail(t, report, "project mount", CheckFail, fmt.Sprintf("missing volume %s:/workspace for project agw", project))
}

func TestDiagnoseNeedsApplyWhenDockerfileMissing(t *testing.T) {
	_, project, ws := validWorkspaceDirs(t)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	mustWrite(t, filepath.Join(ws, "compose.yaml"), fmt.Sprintf(
		"services:\n  dev:\n    build: .\n    volumes:\n      - %s:/workspace\n",
		project,
	))
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev"},
			Projects:  []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
		},
		Path: filepath.Join(ws, "agw.yaml"),
	}

	report := Diagnose(located, fakeRunner{})

	if report.State != StateNeedsApply {
		t.Fatalf("State = %q, want %q", report.State, StateNeedsApply)
	}
	assertCheckContains(t, report, "Dockerfile", CheckFail, "no such file")
}

func TestDiagnoseBrokenWhenComposeConfigFails(t *testing.T) {
	_, project, ws := validWorkspaceDirs(t)
	mustWrite(t, filepath.Join(ws, "prompt.md"), "prompt")
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(ws, "compose.yaml"), fmt.Sprintf(
		"services:\n  dev:\n    build: .\n    volumes:\n      - %s:/workspace\n",
		project,
	))
	located := workspace.LocatedDefinition{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev"},
			Projects:  []workspace.Project{{Name: "agw", Path: project, MountPath: "/workspace"}},
		},
		Path: filepath.Join(ws, "agw.yaml"),
	}
	runner := fakeRunner{composeErr: errors.New("compose config failed")}

	report := Diagnose(located, runner)

	if report.State != StateBroken {
		t.Fatalf("State = %q, want %q", report.State, StateBroken)
	}
	assertCheckDetail(t, report, "compose config", CheckFail, "compose config failed")
}

func validWorkspaceDirs(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	ws := filepath.Join(root, "ws")
	mustMkdir(t, project)
	mustMkdir(t, ws)
	return root, project, ws
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeRunner struct {
	composeErr error
	networks   map[string]bool
	running    bool
	runtimeErr error
}

func (r fakeRunner) ComposeConfig(string) error { return r.composeErr }
func (r fakeRunner) NetworkExists(name string) (bool, error) {
	if r.networks == nil {
		return true, nil
	}
	return r.networks[name], nil
}
func (r fakeRunner) ServiceRunning(string, string) (bool, error) {
	return r.running, r.runtimeErr
}

func locatedDefinition(root string, def workspace.Definition) workspace.LocatedDefinition {
	dir := filepath.Join(root, "ws")
	mustMkdirForTest(dir)
	return workspace.LocatedDefinition{Definition: def, Root: root, Path: filepath.Join(dir, "agw.yaml")}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirForTest(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		panic(err)
	}
}

func assertCheck(t *testing.T, report Report, name string, status CheckStatus) {
	t.Helper()
	check := findCheck(t, report, name)
	if check.Status != status {
		t.Fatalf("check %q status = %q, want %q", name, check.Status, status)
	}
}

func assertCheckDetail(t *testing.T, report Report, name string, status CheckStatus, detail string) {
	t.Helper()
	check := findCheck(t, report, name)
	if check.Status != status {
		t.Fatalf("check %q status = %q, want %q", name, check.Status, status)
	}
	if check.Detail != detail {
		t.Fatalf("check %q detail = %q, want %q", name, check.Detail, detail)
	}
}

func assertCheckContains(t *testing.T, report Report, name string, status CheckStatus, detail string) {
	t.Helper()
	check := findCheck(t, report, name)
	if check.Status != status {
		t.Fatalf("check %q status = %q, want %q", name, check.Status, status)
	}
	if !strings.Contains(check.Detail, detail) {
		t.Fatalf("check %q detail = %q, want to contain %q", name, check.Detail, detail)
	}
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q in %#v", name, report.Checks)
	panic("unreachable")
}
