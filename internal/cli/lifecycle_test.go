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

func TestLifecycleStartBuildsUpsAndAttachesWhenNotRunning(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")
	mustWriteStartWorkspaceFiles(t, wsPath, "dev", "")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"start", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.buildDir != wsPath {
		t.Fatalf("build dir = %q, want %q", runner.buildDir, wsPath)
	}
	if runner.upDetachedDir != wsPath {
		t.Fatalf("up detached dir = %q, want %q", runner.upDetachedDir, wsPath)
	}
	if runner.attachDir != wsPath {
		t.Fatalf("attach dir = %q, want %q", runner.attachDir, wsPath)
	}
	if runner.attachService != "dev" {
		t.Fatalf("attach service = %q, want %q", runner.attachService, "dev")
	}
}

func TestLifecycleStartOnlyAttachesWhenAlreadyRunning(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")
	mustWriteStartWorkspaceFiles(t, wsPath, "dev", "")

	runner := &lifecycleFakeRunner{serviceRunning: true}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"start", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.buildDir != "" {
		t.Fatalf("build dir = %q, want empty", runner.buildDir)
	}
	if runner.upDir != "" {
		t.Fatalf("up dir = %q, want empty", runner.upDir)
	}
	if runner.upDetachedDir != "" {
		t.Fatalf("up detached dir = %q, want empty", runner.upDetachedDir)
	}
	if runner.attachDir != wsPath {
		t.Fatalf("attach dir = %q, want %q", runner.attachDir, wsPath)
	}
	if runner.attachService != "dev" {
		t.Fatalf("attach service = %q, want %q", runner.attachService, "dev")
	}
}

func TestLifecycleStartStopsBeforeBuildWhenExternalNetworkMissing(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspaceWithNetworks(t, root, "agw", "dev", []workspace.NetworkAttachment{{Name: "target_default"}})
	mustWriteStartWorkspaceFiles(t, wsPath, "dev", "target_default")

	runner := &lifecycleFakeRunner{networkExists: map[string]bool{"target_default": false}}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"start", "agw", "--config", cfgPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected start to fail")
	}
	if runner.buildDir != "" {
		t.Fatalf("build dir = %q, want empty", runner.buildDir)
	}
	if runner.upDetachedDir != "" {
		t.Fatalf("up detached dir = %q, want empty", runner.upDetachedDir)
	}
	if !strings.Contains(out.String(), "external network") {
		t.Fatalf("start output missing doctor report:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "workspace agw is not ready to start") {
		t.Fatalf("error = %q", err)
	}
}

func TestLifecycleStopUsesRunnerWithWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	cfgPath, wsPath := mustWriteLifecycleWorkspace(t, root, "agw", "dev")

	runner := &lifecycleFakeRunner{}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	cmd.SetArgs([]string{"stop", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.stopDir != wsPath {
		t.Fatalf("stop dir = %q, want %q", runner.stopDir, wsPath)
	}
	if runner.downDir != "" {
		t.Fatalf("down dir = %q, want empty", runner.downDir)
	}
}

func TestLifecycleStatusShowsLifecycleSummary(t *testing.T) {
	root := t.TempDir()
	def := workspace.Definition{
		ID:        "agw",
		Workspace: workspace.Workspace{Dir: filepath.Join(root, "storage", "agw")},
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		Networks: &workspace.Networks{
			Attach: []workspace.NetworkAttachment{
				{Name: "acme_default"},
			},
		},
	}
	cfgPath, wsPath := mustWriteLifecycleDefinition(t, root, "agw", def)
	mustWriteStartWorkspaceFiles(t, wsPath, "dev", "acme_default")

	runner := &lifecycleFakeRunner{
		serviceRunning: true,
		networkExists: map[string]bool{
			"acme_default": true,
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
	for _, want := range []string{
		"Workspace: agw",
		"State: running",
		"Service: dev",
		"Directory: " + wsPath,
		"Network acme_default: available",
		"Next:",
		"agw start agw",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestLifecycleStatusWithoutWorkspaceShowsHelp(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Usage:\n  agw status [workspace]") {
		t.Fatalf("status help missing usage:\n%s", output)
	}
}

func TestLifecycleListShowsHeaderAndLifecycleState(t *testing.T) {
	root := t.TempDir()
	storageA := filepath.Join("storage", "alpha")
	wsA, err := createLifecycleDefinition(t, root, "alpha", "dev", storageA)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteStartWorkspaceFiles(t, wsA, "dev", "")
	wsB, err := createLifecycleDefinition(t, root, "beta", "api", filepath.Join("storage", "beta"))
	if err != nil {
		t.Fatal(err)
	}
	mustWriteStartWorkspaceFiles(t, wsB, "api", "")

	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	runner := &lifecycleFakeRunner{serviceRunningByDir: map[string]bool{wsA: true, wsB: false}}
	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(_ io.Writer, _ io.Writer) lifecycleRunner { return runner }
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	cmd := NewRootCommand("test")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	list := out.String()
	for _, want := range []string{
		"WORKSPACE\tSTATE\tSERVICE\tDIR",
		"alpha\trunning\tdev\t" + storageA,
		"beta\tnot-running\tapi\tstorage/beta",
	} {
		if !strings.Contains(list, want) {
			t.Fatalf("list output missing %q:\n%s", want, list)
		}
	}
}

func TestLifecycleHelpDescribesExternalDockerCLI(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"start", "--help"}, want: "Start the AGW workspace"},
		{args: []string{"stop", "--help"}, want: "Stop the AGW workspace"},
		{args: []string{"build", "--help"}, want: "Run external Docker CLI build"},
		{args: []string{"up", "--help"}, want: "Run external Docker CLI up"},
		{args: []string{"down", "--help"}, want: "Run external Docker CLI down"},
		{args: []string{"attach", "--help"}, want: "Run external Docker CLI exec"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := NewRootCommand("test")
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("help output missing %q:\n%s", tc.want, out.String())
			}
		})
	}
}

func mustWriteLifecycleWorkspace(t *testing.T, root, id, service string) (string, string) {
	t.Helper()
	defPath, wsPath := createLifecycleDir(t, root, id)
	storage := filepath.Join(root, "storage", id)
	def := workspace.Definition{
		ID:        id,
		Workspace: workspace.Workspace{Dir: storage},
		Container: workspace.Container{
			Service: service,
			Workdir: "/workspace",
		},
	}
	mustWriteLifecycleDefinitionFile(t, defPath, def)
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}
	return cfgPath, wsPath
}

func mustWriteLifecycleWorkspaceWithNetworks(t *testing.T, root, id, service string, networks []workspace.NetworkAttachment) (string, string) {
	t.Helper()
	defPath, wsPath := createLifecycleDir(t, root, id)
	storage := filepath.Join(root, "storage", id)
	def := workspace.Definition{
		ID:        id,
		Workspace: workspace.Workspace{Dir: storage},
		Container: workspace.Container{
			Service: service,
			Workdir: "/workspace",
		},
		Networks: &workspace.Networks{Attach: networks},
	}
	mustWriteLifecycleDefinitionFile(t, defPath, def)
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}
	return cfgPath, wsPath
}

func mustWriteStartWorkspaceFiles(t *testing.T, wsPath, service, externalNetwork string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(wsPath, "prompt.md"), []byte("prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsPath, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  " + service + ":\n    build: .\n"
	if externalNetwork != "" {
		compose += "    networks:\n      - target\nnetworks:\n  target:\n    external: true\n    name: " + externalNetwork + "\n"
	}
	if err := os.WriteFile(filepath.Join(wsPath, "compose.yaml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
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
		ID:        id,
		Workspace: workspace.Workspace{Dir: filepath.Join(storage)},
		Container: workspace.Container{
			Service: service,
			Workdir: "/workspace",
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
	upDetachedDir string
	downDir       string
	stopDir       string
	attachDir     string
	attachService string

	networkExists       map[string]bool
	serviceRunning      bool
	serviceRunningByDir map[string]bool
}

func (r *lifecycleFakeRunner) Build(dir string) error {
	r.buildDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Up(dir string) error {
	r.upDir = dir
	return nil
}

func (r *lifecycleFakeRunner) UpDetached(dir string) error {
	r.upDetachedDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Down(dir string) error {
	r.downDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Stop(dir string) error {
	r.stopDir = dir
	return nil
}

func (r *lifecycleFakeRunner) Logs(dir string, service string) (string, error) {
	_ = dir
	_ = service
	return "", nil
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
	if r.serviceRunningByDir != nil {
		return r.serviceRunningByDir[dir], nil
	}
	return r.serviceRunning, nil
}
