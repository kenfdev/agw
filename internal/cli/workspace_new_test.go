package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestWorkspaceNewWritesDefinition(t *testing.T) {
	root := t.TempDir()
	storage := "workspaces/github.com/kenfdev/agw"
	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new",
		"--root", root,
		"--id", "agw",
		"--name", "AGW",
		"--storage", storage,
		"--project", "agw=/src/agw:/workspace",
		"--service", "dev",
		"--workspace-root", "/workspace",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, storage, "agw.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := workspace.LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "agw" {
		t.Fatalf("ID = %q", loaded.ID)
	}
	if loaded.Name != "AGW" {
		t.Fatalf("Name = %q", loaded.Name)
	}
	if loaded.Container.WorkspaceRoot != "/workspace" {
		t.Fatalf("WorkspaceRoot = %q", loaded.Container.WorkspaceRoot)
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].Name != "agw" || loaded.Projects[0].Path != "/src/agw" || loaded.Projects[0].MountPath != "/workspace" {
		t.Fatalf("Projects = %#v", loaded.Projects)
	}
	if loaded.Storage.Path != storage {
		t.Fatalf("Storage.Path = %q", loaded.Storage.Path)
	}
}

func TestWorkspaceNewRejectsAbsoluteStorage(t *testing.T) {
	root := t.TempDir()
	absStorage := filepath.Join(root, "outside")
	dest := filepath.Join(absStorage, "agw.yaml")
	if err := os.RemoveAll(absStorage); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new",
		"--root", root,
		"--id", "agw",
		"--name", "AGW",
		"--storage", absStorage,
		"--project", "agw=/src/agw:/workspace",
		"--service", "dev",
		"--workspace-root", "/workspace",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for absolute storage path")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected destination not created: %v", err)
	}
}

func TestWorkspaceNewRejectsEscapingStorage(t *testing.T) {
	root := t.TempDir()
	storage := "../outside"
	dest := filepath.Join(filepath.Clean(filepath.Join(root, storage)), "agw.yaml")
	if err := os.RemoveAll(filepath.Dir(dest)); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new",
		"--root", root,
		"--id", "agw",
		"--name", "AGW",
		"--storage", storage,
		"--project", "agw=/src/agw:/workspace",
		"--service", "dev",
		"--workspace-root", "/workspace",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for escaping storage path")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected escaped destination not created: %v", err)
	}
}
