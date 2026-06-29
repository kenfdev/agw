package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestLifecycleBuildUsesRunnerWithWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"build", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.buildDir != wsPath {
		t.Fatalf("build dir = %q, want %q", runner.buildDir, wsPath)
	}
}

func TestLifecycleUpUsesRunnerWithWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"up", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.upDir != wsPath {
		t.Fatalf("up dir = %q, want %q", runner.upDir, wsPath)
	}
}

func TestLifecycleDownUsesRunnerWithWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"down", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.downDir != wsPath {
		t.Fatalf("down dir = %q, want %q", runner.downDir, wsPath)
	}
}

func TestLifecycleAttachUsesConfiguredService(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"attach", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.attachDir != wsPath {
		t.Fatalf("attach dir = %q, want %q", runner.attachDir, wsPath)
	}
	if runner.attachService != "dev" {
		t.Fatalf("attach service = %q, want %q", runner.attachService, "dev")
	}
}

func TestLifecycleStatusShowsServiceAndNetworks(t *testing.T) {
	root := t.TempDir()
	def := workspace.Definition{
		ID:        "agw",
		Storage:   workspace.Storage{Path: filepath.Join(root, "storage", "agw")},
		Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
		Networks: &workspace.Networks{
			Attach: []workspace.NetworkAttachment{
				{Name: "acme_default"},
				{Name: "missing"},
			},
		},
	}
	cfgPath, _ := mustWriteLifecycleDefinition(t, root, "agw", def)

	runner := &lifecycleFakeRunner{
		networkExists: map[string]bool{
			"acme_default": true,
			"missing":      false,
		},
	}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"Workspace: agw", "Service: dev", "Network acme_default exists: true", "Network missing exists: false"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestLifecycleListShowsWorkspaceIdsAndStorage(t *testing.T) {
	root := t.TempDir()
	storageA := filepath.Join("storage", "alpha")
	_, err := createLifecycleDefinition(t, root, "alpha", "dev", storageA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = createLifecycleDefinition(t, root, "beta", "api", filepath.Join("storage", "beta"))
	if err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	list := out.String()
	if !strings.Contains(list, "alpha\t"+storageA) {
		t.Fatalf("list output missing alpha: %s", list)
	}
	if !strings.Contains(list, "beta\tstorage/beta") {
		t.Fatalf("list output missing beta: %s", list)
	}
}

func mustWriteLifecycleWorkspace(t *testing.T, root, id, service string) (string, string) {
	t.Helper()
	defPath, wsPath := createLifecycleDir(t, root, id)
	storage := filepath.Join(root, "storage", id)
	def := workspace.Definition{
		ID:      id,
		Storage: workspace.Storage{Path: storage},
		Container: workspace.Container{
			Service:       service,
			WorkspaceRoot: "/workspace",
		},
	}
	mustWriteLifecycleDefinitionFile(t, defPath, def)
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}
	return cfgPath, wsPath
}

func mustWriteLifecycleDefinition(t *testing.T, root, id string, def workspace.Definition) (string, string) {
	t.Helper()
	defPath, wsPath := createLifecycleDir(t, root, id)
	mustWriteLifecycleDefinitionFile(t, defPath, def)
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}
	return cfgPath, wsPath
}

func createLifecycleDefinition(t *testing.T, root, id, service, storage string) (string, error) {
	t.Helper()
	defPath, wsPath := createLifecycleDir(t, root, id)
	def := workspace.Definition{
		ID:      id,
		Storage: workspace.Storage{Path: filepath.Join(storage)},
		Container: workspace.Container{
			Service:       service,
			WorkspaceRoot: "/workspace",
		},
	}
	if err := workspace.SaveDefinition(defPath, def); err != nil {
		return "", err
	}
	return wsPath, nil
}

func createLifecycleDir(t *testing.T, root, id string) (string, string) {
	t.Helper()
	wsPath := filepath.Join(root, "workspaces", id)
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wsPath, "agw.yaml"), wsPath
}

func mustWriteLifecycleDefinitionFile(t *testing.T, path string, def workspace.Definition) {
	t.Helper()
	if err := workspace.SaveDefinition(path, def); err != nil {
		t.Fatal(err)
	}
}

type lifecycleFakeRunner struct {
	buildDir      string
	upDir         string
	downDir       string
	attachDir     string
	attachService string

	networkExists map[string]bool
}

func (r *lifecycleFakeRunner) Build(dir string) error {
	r.buildDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Up(dir string) error {
	r.upDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Down(dir string) error {
	r.downDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Attach(dir string, service string) error {
	r.attachDir = dir
	r.attachService = service
	return nil
}

func (r *lifecycleFakeRunner) ComposeConfig(string) error { return nil }

func (r *lifecycleFakeRunner) NetworkExists(name string) (bool, error) {
	if r.networkExists == nil {
		return false, nil
	}
	return r.networkExists[name], nil
}

func (r *lifecycleFakeRunner) ServiceRunning(dir string, service string) (bool, error) {
	return false, nil
}
