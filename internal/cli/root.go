package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kenfdev/agw/internal/base"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/prepare"
	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/tui"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:           "agw",
		Short:         "Agent Workspace",
		Long:          "Agent Workspace prepares workspace files and can call the external Docker CLI for sidecar development containers.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceTUI(cmd, configPath)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.AddCommand(NewConfigCommand())
	cmd.AddCommand(NewBaseCommand())
	cmd.AddCommand(NewWorkspaceCommand())
	cmd.AddCommand(
		newLifecycleStartCommand(),
		newLifecycleRestartCommand(),
		newLifecycleStopCommand(),
		newDoctorCommand(),
		newLifecycleBuildCommand(),
		newLifecycleUpCommand(),
		newLifecycleDownCommand(),
		newLifecycleAttachCommand(),
		newLifecycleStatusCommand(),
		newLifecycleListCommand(),
	)
	return cmd
}

var runWorkspaceTUI = func(cmd *cobra.Command, configPath string) error {
	items, err := listDefinitions(configPath)
	if err != nil {
		return err
	}
	actions := &tuiActions{out: cmd.OutOrStdout(), err: cmd.ErrOrStderr(), configPath: configPath}
	reports := make([]doctor.Report, 0, len(items))
	for _, item := range items {
		report, _ := actions.Refresh(item)
		reports = append(reports, report)
	}
	baseStatus, _ := actions.BaseStatus()
	finalModel, err := tui.RunWithReportsAndBaseStatusResult(items, reports, baseStatus, actions)
	if err != nil {
		return fmt.Errorf("tui failed: %w", err)
	}
	if item, ok := finalModel.RequestedShellWorkspace(); ok {
		_, err := actions.OpenShell(item)
		return err
	}
	return nil
}

type tuiActions struct {
	out        io.Writer
	err        io.Writer
	configPath string
}

func (a *tuiActions) Status(item workspace.LocatedDefinition) (string, error) {
	runner := newLifecycleRunner(a.out, a.err)
	if _, err := fmt.Fprintln(a.out, "Workspace:", item.Definition.ID); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(a.out, "Service:", item.Definition.Container.Service); err != nil {
		return "", err
	}
	attachments := networkCandidates(item.Definition)
	if len(attachments) == 0 {
		_, err := fmt.Fprintln(a.out, "Networks: none")
		return "", err
	}
	for _, network := range attachments {
		exists, err := runner.NetworkExists(network)
		if err != nil {
			return "", err
		}
		if _, err := fmt.Fprintf(a.out, "Network %s exists: %t\n", network, exists); err != nil {
			return "", err
		}
	}
	return "", nil
}

func (a *tuiActions) Build(item workspace.LocatedDefinition) (string, error) {
	runner, output := newCapturedLifecycleRunner()
	err := runner.Build(filepath.Dir(item.Path))
	return lifecycleActionResult(item.Definition.ID, output.String()), lifecycleActionError(err, output.String())
}

func (a *tuiActions) BuildStream(item workspace.LocatedDefinition, out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	err := runner.Build(filepath.Dir(item.Path))
	return item.Definition.ID, err
}

func (a *tuiActions) BaseStatus() (*base.Status, error) {
	return configuredBaseStatus(a.configPath, newLifecycleRunner(io.Discard, io.Discard))
}

func (a *tuiActions) BuildBase() (string, error) {
	runner, output := newCapturedLifecycleRunner()
	if err := buildConfiguredBaseImage(a.configPath, runner); err != nil {
		return "", lifecycleActionError(err, output.String())
	}
	status, err := a.BaseStatus()
	if err != nil {
		return "", err
	}
	if status == nil {
		return lifecycleActionResult("", output.String()), nil
	}
	return lifecycleActionResult(status.Config.Image, output.String()), nil
}

func (a *tuiActions) BuildBaseStream(out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	if err := buildConfiguredBaseImage(a.configPath, runner); err != nil {
		return "", err
	}
	status, err := a.BaseStatus()
	if err != nil {
		return "", err
	}
	if status == nil {
		return "", nil
	}
	return status.Config.Image, nil
}

func (a *tuiActions) Start(item workspace.LocatedDefinition) (string, error) {
	runner, output := newCapturedLifecycleRunner()
	if err := startLocatedWorkspace(output, item, runner, docker.UpOptions{}); err != nil {
		return "", lifecycleActionError(err, output.String())
	}
	return lifecycleActionResult(item.Definition.ID, output.String()), nil
}

func (a *tuiActions) StartStream(item workspace.LocatedDefinition, out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	if err := startLocatedWorkspace(out, item, runner, docker.UpOptions{}); err != nil {
		return "", err
	}
	return item.Definition.ID, nil
}

func (a *tuiActions) Restart(item workspace.LocatedDefinition, opts docker.UpOptions) (string, error) {
	runner, output := newCapturedLifecycleRunner()
	if err := restartLocatedWorkspace(output, item, runner, opts); err != nil {
		return "", lifecycleActionError(err, output.String())
	}
	return lifecycleActionResult(item.Definition.ID, output.String()), nil
}

func (a *tuiActions) RestartStream(item workspace.LocatedDefinition, opts docker.UpOptions, out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	if err := restartLocatedWorkspace(out, item, runner, opts); err != nil {
		return "", err
	}
	return item.Definition.ID, nil
}

func (a *tuiActions) Up(item workspace.LocatedDefinition) (string, error) {
	runner, output := newCapturedLifecycleRunner()
	err := runner.Up(filepath.Dir(item.Path))
	return lifecycleActionResult(item.Definition.ID, output.String()), lifecycleActionError(err, output.String())
}

func (a *tuiActions) UpStream(item workspace.LocatedDefinition, out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	err := runner.Up(filepath.Dir(item.Path))
	return item.Definition.ID, err
}

func (a *tuiActions) Down(item workspace.LocatedDefinition) (string, error) {
	runner, output := newCapturedLifecycleRunner()
	err := runner.Down(filepath.Dir(item.Path))
	return lifecycleActionResult(item.Definition.ID, output.String()), lifecycleActionError(err, output.String())
}

func (a *tuiActions) DownStream(item workspace.LocatedDefinition, out io.Writer) (string, error) {
	runner := newLifecycleRunner(out, out)
	err := runner.Down(filepath.Dir(item.Path))
	return item.Definition.ID, err
}

func (a *tuiActions) Attach(item workspace.LocatedDefinition) (string, error) {
	return a.OpenShell(item)
}

func (a *tuiActions) OpenShell(item workspace.LocatedDefinition) (string, error) {
	service := strings.TrimSpace(item.Definition.Container.Service)
	if service == "" {
		return "", fmt.Errorf("workspace %q has no service configured", item.Definition.ID)
	}
	return "", newLifecycleRunner(a.out, a.err).Attach(filepath.Dir(item.Path), service)
}

func (a *tuiActions) Logs(item workspace.LocatedDefinition) (string, error) {
	service := strings.TrimSpace(item.Definition.Container.Service)
	if service == "" {
		return "", fmt.Errorf("workspace %q has no service configured", item.Definition.ID)
	}
	return newLifecycleRunner(a.out, a.err).Logs(filepath.Dir(item.Path), service)
}

func (a *tuiActions) Prepare(item workspace.LocatedDefinition) (string, error) {
	projectSnapshots := make([]scanner.ProjectSnapshot, 0, len(item.Definition.Projects))
	for _, project := range item.Definition.Projects {
		snapshot, err := scanner.ScanProject(project)
		if err != nil {
			return "", err
		}
		projectSnapshots = append(projectSnapshots, snapshot)
	}

	availableNetworks, err := newDockerCLI().ListNetworks()
	if err != nil {
		_, _ = fmt.Fprintf(a.err, "Warning: unable to list Docker networks: %v\n", err)
		availableNetworks = nil
	}

	prompt, err := prepare.Render(prepare.Input{
		Definition:        item.Definition,
		Projects:          projectSnapshots,
		NetworkCandidates: networkCandidatesForPrepare(item.Definition, availableNetworks),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("rendered preparation prompt for %s (%d bytes); use workspace prepare --agent-json for agent generation", item.Definition.ID, len(prompt)), nil
}

func (a *tuiActions) Refresh(item workspace.LocatedDefinition) (doctor.Report, error) {
	runner := newLifecycleRunner(io.Discard, io.Discard)
	return doctor.Diagnose(item, runner), nil
}

func (a *tuiActions) OpenPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path must not be blank")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return path, nil
}

func (a *tuiActions) EditPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path must not be blank")
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", editor, path)
	} else {
		cmd = exec.Command("sh", "-c", editor+" \"$1\"", "agw-editor", path)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = a.out
	cmd.Stderr = a.err
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return path, nil
}

func (a *tuiActions) CopyText(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text must not be blank")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		cmd = exec.Command("sh", "-c", "command -v wl-copy >/dev/null 2>&1 && wl-copy || xclip -selection clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return text, nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newCapturedLifecycleRunner() (lifecycleRunner, *lockedBuffer) {
	output := &lockedBuffer{}
	return newLifecycleRunner(output, output), output
}

func lifecycleActionResult(label, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return label
	}
	if strings.TrimSpace(label) == "" {
		return output
	}
	return label + "\n" + output
}

func lifecycleActionError(err error, output string) error {
	if err == nil {
		return nil
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, output)
}
