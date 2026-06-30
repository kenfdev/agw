package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestScanProjectIncludesMajorConfigAndExcludesEnv(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"scripts":{"dev":"vite"}}`)
	mustWrite(t, filepath.Join(dir, ".devcontainer", "devcontainer.json"), `{"name":"dev-container"}`)
	mustWrite(t, filepath.Join(dir, ".env"), "TOKEN=secret")
	mustWrite(t, filepath.Join(dir, ".env.example"), "TOKEN=")
	mustWrite(t, filepath.Join(dir, ".hiddenconfig"), "secret-config=1")

	snap, err := ScanProject(workspace.Project{Name: "web", HostPath: dir, ContainerPath: "/workspace/web"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(snap.Files, "package.json") {
		t.Fatalf("missing package.json: %#v", snap.Files)
	}
	if !hasFile(snap.Files, ".env.example") {
		t.Fatalf("missing .env.example: %#v", snap.Files)
	}
	if !hasFile(snap.Files, ".devcontainer/devcontainer.json") {
		t.Fatalf("missing .devcontainer/devcontainer.json: %#v", snap.Files)
	}
	if hasFile(snap.Files, ".env") {
		t.Fatalf("secret .env was included")
	}
	if hasFile(snap.Files, ".hiddenconfig") {
		t.Fatalf("unrelated hidden file was included: %#v", snap.Files)
	}
}

func TestScanProjectRejectsMissingProjectPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	_, err := ScanProject(workspace.Project{Name: "web", HostPath: missing, ContainerPath: "/workspace/web"})
	if err == nil {
		t.Fatal("expected missing project path to fail")
	}
	if !strings.Contains(err.Error(), "project path must be an existing directory") {
		t.Fatalf("ScanProject() error = %v", err)
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

func hasFile(files []FileSnapshot, name string) bool {
	for _, f := range files {
		if f.Path == name {
			return true
		}
	}
	return false
}
