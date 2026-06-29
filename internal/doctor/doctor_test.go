package doctor

import (
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
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("check %q status = %q, want %q", name, check.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing check %q in %#v", name, report.Checks)
}
