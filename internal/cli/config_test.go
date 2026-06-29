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
	if len(loaded.WorkspaceRoots) != 1 || loaded.WorkspaceRoots[0] != "/tmp/agw" {
		t.Fatalf("WorkspaceRoots = %#v", loaded.WorkspaceRoots)
	}
}
