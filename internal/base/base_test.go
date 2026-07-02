package base

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
)

func TestResolveBaseEnvironmentImage(t *testing.T) {
	root := t.TempDir()
	got, err := Resolve(config.Config{
		WorkspaceRoot: root,
		BaseEnvironment: config.BaseEnvironment{
			Image: "agw-base:latest",
			Build: config.Build{Context: "base", Dockerfile: "Dockerfile"},
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Image != "agw-base:latest" {
		t.Fatalf("Image = %q", got.Image)
	}
	if got.ContextDir != filepath.Join(root, "base") {
		t.Fatalf("ContextDir = %q", got.ContextDir)
	}
	if got.Dockerfile != filepath.Join(root, "base", "Dockerfile") {
		t.Fatalf("Dockerfile = %q", got.Dockerfile)
	}
}

func TestResolveRequiresBuildFieldsWhenImageConfigured(t *testing.T) {
	_, err := Resolve(config.Config{
		WorkspaceRoot:   "/tmp/agw",
		BaseEnvironment: config.BaseEnvironment{Image: "agw-base:latest"},
	})
	if err == nil || !strings.Contains(err.Error(), "build.context") {
		t.Fatalf("Resolve() error = %v", err)
	}
}
