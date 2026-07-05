package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func TestTUIActionsPrepareRendersPromptWithoutWritingWorkspacePromptFile(t *testing.T) {
	projectDir := t.TempDir()
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/agw"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := workspace.LocatedDefinition{
		Path: filepath.Join(workspaceDir, "agw.yaml"),
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: workspaceDir},
			Container: workspace.Container{
				Service: "dev",
				Workdir: "/workspace",
			},
			Projects: []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		},
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	actions := &tuiActions{out: &out, err: &errOut}

	result, err := actions.Prepare(item)
	if err != nil {
		t.Fatal(err)
	}

	promptPath := filepath.Join(workspaceDir, "prompt.md")
	if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
		t.Fatalf("prompt.md should not be written, stat err = %v", err)
	}
	if !strings.Contains(result, "rendered preparation prompt for agw") {
		t.Fatalf("result = %q", result)
	}
	if out.Len() != 0 {
		t.Fatalf("prepare wrote prompt to command output:\n%s", out.String())
	}
}

func TestTUIActionsRefreshDoesNotWriteLifecycleNoiseToTUIOutput(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	workspaceDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteCLI(t, filepath.Join(workspaceDir, "Dockerfile"), "FROM alpine\n")
	mustWriteCLI(t, filepath.Join(workspaceDir, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - "+projectDir+":/workspace\n")

	item := workspace.LocatedDefinition{
		Path: filepath.Join(workspaceDir, "agw.yaml"),
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: workspaceDir},
			Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
			Projects:  []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		},
	}

	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return noisyDoctorRunner{out: stdout}
	}
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	actions := &tuiActions{out: &out, err: &out}
	if _, err := actions.Refresh(item); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "compose noise") {
		t.Fatalf("refresh wrote lifecycle output to TUI stdout:\n%s", out.String())
	}
}

func TestTUIActionsBuildStartAndBaseBuildCaptureLifecycleOutput(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	workspaceDir := filepath.Join(root, "workspace")
	baseDir := filepath.Join(root, "base")
	for _, dir := range []string{projectDir, workspaceDir, baseDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteCLI(t, filepath.Join(workspaceDir, "Dockerfile"), "FROM alpine\n")
	mustWriteCLI(t, filepath.Join(workspaceDir, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - "+projectDir+":/workspace\n")
	mustWriteCLI(t, filepath.Join(baseDir, "Dockerfile"), "FROM alpine\n")
	cfgPath := filepath.Join(root, "config.yaml")
	mustWriteCLI(t, cfgPath, "workspaceRoot: "+root+"\nbaseEnvironment:\n  image: agw-base:latest\n  build:\n    context: base\n    dockerfile: Dockerfile\n")

	item := workspace.LocatedDefinition{
		Path: filepath.Join(workspaceDir, "agw.yaml"),
		Definition: workspace.Definition{
			ID:        "agw",
			Workspace: workspace.Workspace{Dir: workspaceDir},
			Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
			Projects:  []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		},
	}

	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return noisyLifecycleRunner{out: stdout, err: stderr}
	}
	defer func() { newLifecycleRunner = oldRunner }()

	var out bytes.Buffer
	actions := &tuiActions{out: &out, err: &out, configPath: cfgPath}

	buildResult, err := actions.Build(item)
	if err != nil {
		t.Fatal(err)
	}
	startResult, err := actions.Start(item)
	if err != nil {
		t.Fatal(err)
	}
	restartResult, err := actions.Restart(item, docker.UpOptions{Build: true, ForceRecreate: true})
	if err != nil {
		t.Fatal(err)
	}
	baseResult, err := actions.BuildBase()
	if err != nil {
		t.Fatal(err)
	}

	if out.Len() != 0 {
		t.Fatalf("lifecycle output escaped to TUI terminal:\n%s", out.String())
	}
	for _, got := range []string{buildResult, startResult, restartResult, baseResult} {
		if !strings.Contains(got, "lifecycle stdout") || !strings.Contains(got, "lifecycle stderr") {
			t.Fatalf("captured result missing lifecycle output:\n%s", got)
		}
	}
}

func TestRootCommandShowsHelp(t *testing.T) {
	oldRun := runWorkspaceTUI
	defer func() { runWorkspaceTUI = oldRun }()
	runWorkspaceTUI = func(cmd *cobra.Command, configPath string) error {
		t.Fatal("help should not run TUI")
		return nil
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Agent Workspace")) {
		t.Fatalf("help output did not contain product name: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("external Docker CLI")) {
		t.Fatalf("help output did not describe Docker boundary: %s", out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("tui")) {
		t.Fatalf("help output should not expose tui subcommand:\n%s", out.String())
	}
}

func TestRootCommandRunsTUIByDefault(t *testing.T) {
	oldRun := runWorkspaceTUI
	defer func() { runWorkspaceTUI = oldRun }()

	var called bool
	runWorkspaceTUI = func(cmd *cobra.Command, configPath string) error {
		called = true
		if configPath != "" {
			t.Fatalf("config path = %q, want empty", configPath)
		}
		return nil
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("root command did not run TUI")
	}
	if out.Len() != 0 {
		t.Fatalf("root command wrote unexpected output:\n%s", out.String())
	}
}

func TestRootCommandRejectsRemovedTUISubcommand(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"tui"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want unknown command")
	}
}

type noisyLifecycleRunner struct {
	out io.Writer
	err io.Writer
}

func (r noisyLifecycleRunner) Build(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: build")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: build")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) Up(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: up")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: up")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) UpDetached(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: up detached")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: up detached")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) UpDetachedWithOptions(string, docker.UpOptions) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: up detached")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: up detached")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) Down(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: down")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: down")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) Stop(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: stop")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: stop")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) Logs(string, string) (string, error) {
	return "container logs", nil
}

func (r noisyLifecycleRunner) Attach(string, string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: attach")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: attach")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) RunShell(string, string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: shell")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: shell")
	return errorsJoin(outErr, errErr)
}

func (r noisyLifecycleRunner) BuildImage(string, string, string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: base build")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: base build")
	return errorsJoin(outErr, errErr)
}

func (noisyLifecycleRunner) InspectImage(string) (docker.ImageInfo, bool, error) {
	return docker.ImageInfo{}, true, nil
}

func (r noisyLifecycleRunner) ComposeConfig(string) error {
	_, outErr := fmt.Fprintln(r.out, "lifecycle stdout: compose config")
	_, errErr := fmt.Fprintln(r.err, "lifecycle stderr: compose config")
	return errorsJoin(outErr, errErr)
}

func (noisyLifecycleRunner) NetworkExists(string) (bool, error) {
	return true, nil
}

func (noisyLifecycleRunner) ServiceRunning(string, string) (bool, error) {
	return false, nil
}

func errorsJoin(errs ...error) error {
	var parts []string
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
