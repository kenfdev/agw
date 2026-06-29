package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/docker"
)

func TestEndToEndWorkspaceWorkflow(t *testing.T) {
	root := t.TempDir()
	projectPath := t.TempDir()
	projectMarkerPath := filepath.Join(projectPath, "e2e-target-marker.txt")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Snapshot initial target-project contents so we can prove workspace commands do not mutate it.
	if err := os.WriteFile(projectMarkerPath, []byte("target project marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte("module github.com/kenfdev/agw\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectTreeBefore, err := snapshotDirectoryFiles(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(root, "config.yaml")
	storage := filepath.Join("workspaces", "github.com", "kenfdev", "agw")
	workspacePrompt := filepath.Join(root, "prompt.md")

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file path")
	}
	fixtureDir := filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", "testdata", "simple-generated"))
	generatedDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtureComposePath := filepath.Join(fixtureDir, "compose.yaml")
	fixtureCompose, err := os.ReadFile(fixtureComposePath)
	if err != nil {
		t.Fatal(err)
	}
	fixtureCompose = bytes.ReplaceAll(fixtureCompose, []byte("/tmp/agw-e2e-project"), []byte(projectPath))
	if err := os.WriteFile(filepath.Join(generatedDir, "compose.yaml"), fixtureCompose, 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureDockerfile := filepath.Join(fixtureDir, "Dockerfile")
	dockerfile, err := os.ReadFile(fixtureDockerfile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "Dockerfile"), dockerfile, 0o644); err != nil {
		t.Fatal(err)
	}

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
	if err := assertDirectoryFilesUnchanged(projectPath, projectTreeBefore); err != nil {
		t.Fatalf("target project was modified by workspace prepare: %v", err)
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
	if err := assertDirectoryFilesUnchanged(projectPath, projectTreeBefore); err != nil {
		t.Fatalf("target project was modified by workspace apply: %v", err)
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

func snapshotDirectoryFiles(root string) (map[string]string, error) {
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(relPath)] = string(b)
		return nil
	})
	return entries, err
}

func assertDirectoryFilesUnchanged(root string, expected map[string]string) error {
	actual, err := snapshotDirectoryFiles(root)
	if err != nil {
		return err
	}
	if len(expected) != len(actual) {
		return fmt.Errorf("file count changed: expected %d got %d", len(expected), len(actual))
	}
	expectedPaths := make([]string, 0, len(expected))
	for p := range expected {
		expectedPaths = append(expectedPaths, p)
	}
	actualPaths := make([]string, 0, len(actual))
	for p := range actual {
		actualPaths = append(actualPaths, p)
	}
	sort.Strings(expectedPaths)
	sort.Strings(actualPaths)
	for i := range expectedPaths {
		if expectedPaths[i] != actualPaths[i] {
			return fmt.Errorf("directory layout changed: expected %v got %v", expectedPaths, actualPaths)
		}
	}
	for path, expectedContent := range expected {
		if actual[path] != expectedContent {
			return fmt.Errorf("file content changed: %s", path)
		}
	}
	return nil
}
