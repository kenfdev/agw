package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesAGWConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-config.yaml")
	t.Setenv("AGW_CONFIG", path)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if got != path {
		t.Fatalf("DefaultPath() = %q, want %q", got, path)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	want := Config{
		WorkspaceRoots: []string{"/tmp/agw-root"},
		PathMappings: []PathMapping{{
			SourceRoot:      "/Users/me/ghq",
			WorkspacePrefix: "workspaces",
		}},
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.WorkspaceRoots[0] != want.WorkspaceRoots[0] {
		t.Fatalf("WorkspaceRoots = %#v", got.WorkspaceRoots)
	}
	if got.PathMappings[0].WorkspacePrefix != "workspaces" {
		t.Fatalf("PathMappings = %#v", got.PathMappings)
	}
}
