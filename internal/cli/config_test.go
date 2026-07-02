package cli

import (
	"path/filepath"
	"testing"

	"github.com/kenfdev/agw/internal/config"
)

func TestNewConfigCommandInit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cmd := NewConfigCommand()
	cmd.SetArgs([]string{"init", "--root", "/tmp/agw", "--config", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.WorkspaceRoot != "/tmp/agw" {
		t.Fatalf("WorkspaceRoot = %q", loaded.WorkspaceRoot)
	}
}

func TestNewConfigCommandInitUsesAGWConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env-config.yaml")
	t.Setenv("AGW_CONFIG", path)
	cmd := NewConfigCommand()
	cmd.SetArgs([]string{"init", "--root", "/tmp/agw"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.WorkspaceRoot != "/tmp/agw" {
		t.Fatalf("WorkspaceRoot = %q", loaded.WorkspaceRoot)
	}
}
