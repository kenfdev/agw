package config

import (
	"os"
	"path/filepath"
	"strings"
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
		WorkspaceRoot: "/tmp/agw-root",
		PathMappings: []PathMapping{{
			SourceRoot:      "/Users/me/ghq",
			WorkspacePrefix: "workspaces",
		}},
		BaseEnvironment: BaseEnvironment{
			GuidancePath: "base-environment.md",
			Image:        "agw-base:latest",
			Build: Build{
				Context:    "base",
				Dockerfile: "Dockerfile",
			},
		},
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.WorkspaceRoot != want.WorkspaceRoot {
		t.Fatalf("WorkspaceRoot = %q", got.WorkspaceRoot)
	}
	if got.PathMappings[0].WorkspacePrefix != "workspaces" {
		t.Fatalf("PathMappings = %#v", got.PathMappings)
	}
	if got.BaseEnvironment.GuidancePath != "base-environment.md" {
		t.Fatalf("BaseEnvironment.GuidancePath = %q", got.BaseEnvironment.GuidancePath)
	}
	if got.BaseEnvironment.Image != "agw-base:latest" {
		t.Fatalf("BaseEnvironment.Image = %q", got.BaseEnvironment.Image)
	}
	if got.BaseEnvironment.Build.Context != "base" || got.BaseEnvironment.Build.Dockerfile != "Dockerfile" {
		t.Fatalf("BaseEnvironment.Build = %#v", got.BaseEnvironment.Build)
	}
}

func TestLoadExpandsWorkspaceRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte(`
workspaceRoot: ~/agw/../other
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := filepath.Join(home, "other")
	if got.WorkspaceRoot != want {
		t.Fatalf("WorkspaceRoot = %q, want %q", got.WorkspaceRoot, want)
	}
}

func TestLoadMigratesSingleLegacyWorkspaceRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte("workspaceRoots:\n  - /tmp/agw\n")
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.WorkspaceRoot != "/tmp/agw" {
		t.Fatalf("WorkspaceRoot = %q", got.WorkspaceRoot)
	}
}

func TestLoadRejectsMultipleLegacyWorkspaceRoots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte("workspaceRoots:\n  - /tmp/one\n  - /tmp/two\n")
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "choose one and set workspaceRoot") {
		t.Fatalf("Load() error = %v", err)
	}
}
