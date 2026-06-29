package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/prepare"
	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/tui"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "agw",
		Short:         "Agent Workspace",
		Long:          "Agent Workspace prepares workspace files and can call the external Docker CLI for sidecar development containers.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
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
		newTUICommand(),
	)
	return cmd
}

func newTUICommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open minimal workspace TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := tui.RunWithReports(items, reports, actions); err != nil {
				return fmt.Errorf("tui failed: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
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
	service := strings.TrimSpace(item.Definition.Container.Service)
	if service == "" {
		return "", fmt.Errorf("workspace %q has no service configured", item.Definition.ID)
	}
	return "", newLifecycleRunner(a.out, a.err).Attach(filepath.Dir(item.Path), service)
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
	promptPath := filepath.Join(filepath.Dir(item.Path), "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return "", err
	}
	return "wrote " + promptPath, nil
}

func (a *tuiActions) Refresh(item workspace.LocatedDefinition) (doctor.Report, error) {
	return doctor.Diagnose(item, newLifecycleRunner(a.out, a.err)), nil
}
