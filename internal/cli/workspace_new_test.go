package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
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

func TestWorkspaceNewFromProjectUsesConfigRootAndStandaloneDefaults(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "src", "my-app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new", "--from", project, "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "workspaces", "my-app", "agw.yaml")
	loaded, err := workspace.LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "my-app" {
		t.Fatalf("ID = %q, want my-app", loaded.ID)
	}
	if loaded.Name != "my-app" {
		t.Fatalf("Name = %q, want my-app", loaded.Name)
	}
	if loaded.Container.Service != "dev" {
		t.Fatalf("Service = %q, want dev", loaded.Container.Service)
	}
	if loaded.Container.WorkspaceRoot != "/workspace" {
		t.Fatalf("WorkspaceRoot = %q, want /workspace", loaded.Container.WorkspaceRoot)
	}
	if len(loaded.Projects) != 1 {
		t.Fatalf("Projects = %#v", loaded.Projects)
	}
	projectDef := loaded.Projects[0]
	if projectDef.Name != "my-app" || projectDef.Path != project || projectDef.MountPath != "/workspace" {
		t.Fatalf("Project = %#v", projectDef)
	}
	if loaded.Networks != nil {
		t.Fatalf("Networks = %#v, want nil for standalone default", loaded.Networks)
	}
}

func TestWorkspaceNewFromProjectUsesPathMappingForStorage(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "ghq")
	project := filepath.Join(sourceRoot, "github.com", "kenfdev", "agw")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoots: []string{root},
		PathMappings:   []config.PathMapping{{SourceRoot: sourceRoot, WorkspacePrefix: "workspaces"}},
	}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"workspace", "new", "--from", project, "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "workspaces", "github.com", "kenfdev", "agw", "agw.yaml")
	loaded, err := workspace.LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Storage.Path != filepath.Join("workspaces", "github.com", "kenfdev", "agw") {
		t.Fatalf("Storage.Path = %q", loaded.Storage.Path)
	}
}

func TestWorkspaceNewFromProjectPrintsContainerSetupHints(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "src", "api")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"workspace", "new", "--from", project, "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{"Created standalone workspace api", "Found possible container setup", "compose.yaml", "Dockerfile", "External networks are optional"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
