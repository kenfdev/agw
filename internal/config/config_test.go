package config

import (
	"os"
	"path/filepath"
	"reflect"
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
		BaseEnvironment: BaseEnvironment{
			GuidancePath: "base-environment.md",
		},
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
	if got.BaseEnvironment.GuidancePath != "base-environment.md" {
		t.Fatalf("BaseEnvironment.GuidancePath = %q", got.BaseEnvironment.GuidancePath)
	}
}

func TestLoadExpandsWorkspaceRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte(`
workspaceRoots:
  - ~/agw
  - $HOME/agw/../other
  - ${HOME}/third
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{
		filepath.Join(home, "agw"),
		filepath.Join(home, "other"),
		filepath.Join(home, "third"),
	}
	if !reflect.DeepEqual(got.WorkspaceRoots, want) {
		t.Fatalf("WorkspaceRoots = %#v, want %#v", got.WorkspaceRoots, want)
	}
}
