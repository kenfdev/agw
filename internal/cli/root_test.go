package cli

import (
	"bytes"
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
			ID:      "agw",
			Storage: workspace.Storage{Path: workspaceDir},
			Container: workspace.Container{
				Service:       "dev",
				WorkspaceRoot: "/workspace",
			},
			Projects: []workspace.Project{{Name: "agw", Path: projectDir, MountPath: "/workspace"}},
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
