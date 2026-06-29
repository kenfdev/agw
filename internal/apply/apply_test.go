package apply

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestApplyCopiesAndValidatesCompose(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Path: "/src/agw", MountPath: "/workspace"}}}
	if err := Apply(ws, def, gen, fakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyBacksUpExistingRegularFiles(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "old compose\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Path: "/src/agw", MountPath: "/workspace"}}}

	if err := Apply(ws, def, gen, fakeRunner{}); err != nil {
		t.Fatal(err)
	}

	backupsRoot := filepath.Join(ws, ".agw", "backups")
	entries, err := os.ReadDir(backupsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("backup directories = %d", len(entries))
	}
	b, err := os.ReadFile(filepath.Join(backupsRoot, entries[0].Name(), "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old compose\n" {
		t.Fatalf("backup content = %q", string(b))
	}
}

func TestApplyRejectsMissingGeneratedCompose(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "services:\n  web:\n    build: .\n")
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	def := workspace.Definition{Container: workspace.Container{Service: "web"}}

	err := Apply(ws, def, gen, fakeRunner{})
	if err == nil {
		t.Fatal("expected missing generated compose.yaml to fail")
	}
	if !strings.Contains(err.Error(), "generated compose.yaml not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsMissingService(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  web:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "service dev not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsMissingProjectMount(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /other:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Path: "/src/agw", MountPath: "/workspace"}}}

	err := Apply(ws, def, gen, fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "missing volume /src/agw:/workspace") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyAcceptsVolumeOptions(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace:cached\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Path: "/src/agw", MountPath: "/workspace"}}}

	if err := Apply(ws, def, gen, fakeRunner{}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsMissingExternalNetwork(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{Path: "/src/agw", MountPath: "/workspace"}}}

	err := Apply(ws, def, gen, fakeRunner{missingNetworks: map[string]bool{"acme_default": true}})
	if err == nil || !strings.Contains(err.Error(), "external network acme_default not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyReturnsComposeConfigError(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, fakeRunner{composeErr: errors.New("bad compose")})
	if err == nil || !strings.Contains(err.Error(), "docker compose config") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeRunner struct {
	composeErr      error
	missingNetworks map[string]bool
}

func (r fakeRunner) ComposeConfig(string) error { return r.composeErr }
func (r fakeRunner) NetworkExists(name string) (bool, error) {
	return !r.missingNetworks[name], nil
}
