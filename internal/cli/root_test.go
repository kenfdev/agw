package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func TestTUIActionsPrepareRendersPromptWithoutWritingWorkspacePromptFile(t *testing.T) {
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
	if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
		t.Fatalf("prompt.md should not be written, stat err = %v", err)
	}
	if !strings.Contains(result, "rendered preparation prompt for agw") {
		t.Fatalf("result = %q", result)
	}
	if out.Len() != 0 {
		t.Fatalf("prepare wrote prompt to command output:\n%s", out.String())
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
	oldRun := runWorkspaceTUI
	defer func() { runWorkspaceTUI = oldRun }()
	runWorkspaceTUI = func(cmd *cobra.Command, configPath string) error {
		t.Fatal("help should not run TUI")
		return nil
	}

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
	if bytes.Contains(out.Bytes(), []byte("tui")) {
		t.Fatalf("help output should not expose tui subcommand:\n%s", out.String())
	}
}

func TestRootCommandRunsTUIByDefault(t *testing.T) {
	oldRun := runWorkspaceTUI
	defer func() { runWorkspaceTUI = oldRun }()

	var called bool
	runWorkspaceTUI = func(cmd *cobra.Command, configPath string) error {
		called = true
		if configPath != "" {
			t.Fatalf("config path = %q, want empty", configPath)
		}
		return nil
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("root command did not run TUI")
	}
	if out.Len() != 0 {
		t.Fatalf("root command wrote unexpected output:\n%s", out.String())
	}
}

func TestRootCommandRejectsRemovedTUISubcommand(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want unknown command")
	}
}
