package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
	cmd.AddCommand(NewWorkspaceCommand())
	cmd.AddCommand(
		newLifecycleStartCommand(),
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
	actions := &tuiActions{out: cmd.OutOrStdout(), err: cmd.ErrOrStderr()}
	reports := make([]doctor.Report, 0, len(items))
	for _, item := range items {
		report, _ := actions.Refresh(item)
		reports = append(reports, report)
	}
	finalModel, err := tui.RunWithReportsResult(items, reports, actions)
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
	out io.Writer
	err io.Writer
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
	return "", newLifecycleRunner(a.out, a.err).Build(filepath.Dir(item.Path))
}

func (a *tuiActions) Up(item workspace.LocatedDefinition) (string, error) {
	return "", newLifecycleRunner(a.out, a.err).Up(filepath.Dir(item.Path))
}

func (a *tuiActions) Down(item workspace.LocatedDefinition) (string, error) {
	return "", newLifecycleRunner(a.out, a.err).Down(filepath.Dir(item.Path))
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
	return doctor.Diagnose(item, newLifecycleRunner(io.Discard, io.Discard)), nil
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
