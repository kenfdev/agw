package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestWorkspacePrepareWritesPromptToOutputFile(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	def := workspace.Definition{
		ID:      "agw",
		Storage: workspace.Storage{Path: filepath.Join(root, "storage", "agw")},
		Container: workspace.Container{
			Service:       "dev",
			WorkspaceRoot: "/workspace",
		},
		Projects: []workspace.Project{{Name: "agw", Path: wsDir, MountPath: "/workspace"}},
		Networks: &workspace.Networks{
			Attach: []workspace.NetworkAttachment{{Name: "acme_default"}},
		},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "go.mod"), []byte("module example.com/agw"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wsDir, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".devcontainer", "devcontainer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env"), []byte("TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env.example"), []byte("TOKEN="), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".hiddenconfig"), []byte("TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(root, "prompt.md")
	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"prepare", "agw", "--config", cfgPath, "--output", outPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, want := range []string{"agw", "go.mod", "acme_default"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"#### `.env`", "#### `.hiddenconfig`"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("forbidden file included in output: %q", forbidden)
		}
	}
	for _, allowed := range []string{"#### `.env.example`", "#### `.devcontainer/devcontainer.json`"} {
		if !strings.Contains(out, allowed) {
			t.Fatalf("allowed hidden file missing in output: %q", allowed)
		}
	}
}
