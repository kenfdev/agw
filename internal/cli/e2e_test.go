package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/docker"
)

func TestEndToEndWorkspaceWorkflow(t *testing.T) {
	root := t.TempDir()
	projectPath := "/tmp/agw-e2e-project"
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(projectPath)
	})

	if err := os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte("module github.com/kenfdev/agw\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(root, "config.yaml")
	storage := filepath.Join("workspaces", "github.com", "kenfdev", "agw")
	workspacePrompt := filepath.Join(root, "prompt.md")

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file path")
	}
	generatedDir := filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", "testdata", "simple-generated"))

	cmd := NewRootCommand("test")

	cmd.SetArgs([]string{
		"config", "init",
		"--config", configPath,
		"--root", root,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v", err)
	}

	cmd.SetArgs([]string{
		"workspace", "new",
		"--root", root,
		"--id", "agw",
		"--name", "AGW",
		"--storage", storage,
		"--project", fmt.Sprintf("agw=%s:/workspace", projectPath),
		"--service", "dev",
		"--workspace-root", "/workspace",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace new failed: %v", err)
	}

	defPath := filepath.Join(root, storage, "agw.yaml")
	if _, err := os.Stat(defPath); err != nil {
		t.Fatalf("expected agw.yaml to exist: %v", err)
	}

	cmd.SetArgs([]string{
		"workspace", "prepare",
		"agw",
		"--config", configPath,
		"--output", workspacePrompt,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace prepare failed: %v", err)
	}
	if _, err := os.Stat(workspacePrompt); err != nil {
		t.Fatalf("expected prompt.md to exist: %v", err)
	}

	var composeDir string
	oldRunner := newDockerRunner
	newDockerRunner = func() docker.Runner {
		return cliFakeRunner{composeDir: &composeDir}
	}
	defer func() {
		newDockerRunner = oldRunner
	}()

	cmd.SetArgs([]string{
		"workspace", "apply",
		"agw",
		generatedDir,
		"--config", configPath,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace apply failed: %v", err)
	}
	workspaceCompose := filepath.Join(root, storage, "compose.yaml")
	if _, err := os.Stat(workspaceCompose); err != nil {
		t.Fatalf("expected compose.yaml copy at %s: %v", workspaceCompose, err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"list",
		"--config", configPath,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out.String(), "agw\t"+storage) {
		t.Fatalf("list output missing workspace: %s", out.String())
	}
}
