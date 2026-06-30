package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestTUIActionsPrepareWritesPromptToWorkspacePromptFile(t *testing.T) {
	projectDir := t.TempDir()
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/agw"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := workspace.LocatedDefinition{
		Path: filepath.Join(workspaceDir, "agw.yaml"),
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: workspaceDir},
			Container: workspace.Container{
				Service: "dev",
				Workdir: "/workspace",
			},
			Projects: []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		},
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	actions := &tuiActions{out: &out, err: &errOut}

	result, err := actions.Prepare(item)
	if err != nil {
		t.Fatal(err)
	}

	promptPath := filepath.Join(workspaceDir, "prompt.md")
	b, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "agw") {
		t.Fatalf("prompt content missing workspace id:\n%s", string(b))
	}
	if out.Len() != 0 {
		t.Fatalf("prepare wrote prompt to command output:\n%s", out.String())
	}
	if !strings.Contains(result, promptPath) {
		t.Fatalf("result = %q, want path %q", result, promptPath)
	}
}

func TestTUIActionsRefreshDoesNotWriteLifecycleNoiseToTUIOutput(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	workspaceDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteCLI(t, filepath.Join(workspaceDir, "prompt.md"), "prompt")
	mustWriteCLI(t, filepath.Join(workspaceDir, "Dockerfile"), "FROM alpine\n")
	mustWriteCLI(t, filepath.Join(workspaceDir, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - "+projectDir+":/workspace\n")

	item := workspace.LocatedDefinition{
		Path: filepath.Join(workspaceDir, "agw.yaml"),
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: workspaceDir},
			Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
			Projects:  []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		},
	}

	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return noisyDoctorRunner{out: stdout}
	}
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	actions := &tuiActions{out: &out, err: &out}
	if _, err := actions.Refresh(item); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "compose noise") {
		t.Fatalf("refresh wrote lifecycle output to TUI stdout:\n%s", out.String())
	}
}

func TestRootCommandShowsHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Agent Workspace")) {
		t.Fatalf("help output did not contain product name: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("external Docker CLI")) {
		t.Fatalf("help output did not describe Docker boundary: %s", out.String())
	}
}
