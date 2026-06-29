package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceNewWritesDefinition(t *testing.T) {
	root := t.TempDir()
	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new",
		"--root", root,
		"--id", "agw",
		"--name", "AGW",
		"--storage", "workspaces/github.com/kenfdev/agw",
		"--project", "agw=/src/agw:/workspace",
		"--service", "dev",
		"--workspace-root", "/workspace",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "workspaces/github.com/kenfdev/agw/agw.yaml")); err != nil {
		t.Fatal(err)
	}
}
