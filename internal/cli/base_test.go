package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
)

func TestBaseBuildUsesConfiguredImageAndBuildPaths(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoot: root,
		BaseEnvironment: config.BaseEnvironment{
			Image: "agw-base:latest",
			Build: config.Build{Context: "base", Dockerfile: "Dockerfile"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	fake := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return fake
	}
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"base", "build", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if fake.buildImageName != "agw-base:latest" {
		t.Fatalf("image = %q", fake.buildImageName)
	}
	if fake.buildImageContext != baseDir {
		t.Fatalf("context = %q, want %q", fake.buildImageContext, baseDir)
	}
	if fake.buildImageDockerfile != filepath.Join(baseDir, "Dockerfile") {
		t.Fatalf("dockerfile = %q", fake.buildImageDockerfile)
	}
}

func TestBaseStatusReportsImageAge(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoot: root,
		BaseEnvironment: config.BaseEnvironment{
			Image: "agw-base:latest",
			Build: config.Build{Context: "base", Dockerfile: "Dockerfile"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC)
	fake := &lifecycleFakeRunner{
		inspectImageInfo:   docker.ImageInfo{CreatedAt: created},
		inspectImageExists: true,
	}
	oldRunner := newLifecycleRunner
	oldNow := now
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return fake
	}
	now = func() time.Time { return created.Add(2*time.Hour + 3*time.Minute) }
	defer func() {
		newLifecycleRunner = oldRunner
		now = oldNow
	}()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"base", "status", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"Base image: agw-base:latest", "Image status: available", "Age: 2h3m0s"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestBaseStatusReportsUnknownDockerError(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoot: root,
		BaseEnvironment: config.BaseEnvironment{
			Image: "agw-base:latest",
			Build: config.Build{Context: "base", Dockerfile: "Dockerfile"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleFakeRunner{inspectImageErr: errors.New("docker unavailable")}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return fake
	}
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"base", "status", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Image status: unknown") || !strings.Contains(output, "docker unavailable") {
		t.Fatalf("output =\n%s", output)
	}
}
