package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

type lifecycleRunner interface {
	Build(dir string) error
	Up(dir string) error
	UpDetached(dir string) error
	UpDetachedWithOptions(dir string, opts docker.UpOptions) error
	Down(dir string) error
	Stop(dir string) error
	Logs(dir string, service string) (string, error)
	Attach(dir string, service string) error
	RunShell(dir string, command string) error
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
	ServiceRunning(dir string, service string) (bool, error)
}

var newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
	return docker.CLI{Out: stdout, Err: stderr}
}

func newLifecycleStartCommand() *cobra.Command {
	var configPath string
	var daemon bool
	var build bool
	var forceRecreate bool

	cmd := &cobra.Command{
		Use:   "start <workspace>",
		Short: "Start the AGW workspace and attach to it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			return startLocatedWorkspace(cmd.OutOrStdout(), located, runner, daemon, docker.UpOptions{Build: build, ForceRecreate: forceRecreate})
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "start without attaching")
	cmd.Flags().BoolVar(&build, "build", false, "build images before starting")
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "recreate containers even if their configuration and image have not changed")
	return cmd
}

func startLocatedWorkspace(out io.Writer, located workspace.LocatedDefinition, runner lifecycleRunner, daemon bool, upOptions docker.UpOptions) error {
	service := strings.TrimSpace(located.Definition.Container.Service)
	if service == "" {
		return fmt.Errorf("workspace %q has no service configured", located.Definition.ID)
	}
	dir := filepath.Dir(located.Path)
	report := doctor.Diagnose(located, runner)
	switch report.State {
	case doctor.StateRunning:
		if err := runProjectLifecycleStartCommands(runner, located.Definition); err != nil {
			return err
		}
		if upOptions.Build || upOptions.ForceRecreate {
			if err := runLifecycleStartCommand(runner, located.Definition, dir, upOptions); err != nil {
				return err
			}
		}
		if daemon {
			return nil
		}
		return runner.Attach(dir, service)
	case doctor.StateNotRunning:
		if err := runProjectLifecycleStartCommands(runner, located.Definition); err != nil {
			return err
		}
		if strings.TrimSpace(located.Definition.Lifecycle.Start) == "" && !upOptions.Build {
			if err := runner.Build(dir); err != nil {
				return err
			}
		}
		if err := runLifecycleStartCommand(runner, located.Definition, dir, upOptions); err != nil {
			return err
		}
		if daemon {
			return nil
		}
		return runner.Attach(dir, service)
	default:
		if err := writeDoctorReport(out, report); err != nil {
			return err
		}
		return fmt.Errorf("workspace %s is not ready to start: %s", report.WorkspaceID, report.State)
	}
}

func runLifecycleStartCommand(runner lifecycleRunner, def workspace.Definition, dir string, opts docker.UpOptions) error {
	if command := strings.TrimSpace(def.Lifecycle.Start); command != "" {
		return runner.RunShell(dir, command)
	}
	return runner.UpDetachedWithOptions(dir, opts)
}

func runProjectLifecycleStartCommands(runner lifecycleRunner, def workspace.Definition) error {
	for _, project := range def.Projects {
		command := strings.TrimSpace(project.Lifecycle.Start)
		if command == "" {
			continue
		}
		if err := runner.RunShell(project.HostPath, command); err != nil {
			return fmt.Errorf("start project %s (%s): %w", project.Name, project.HostPath, err)
		}
	}
	return nil
}

func runProjectLifecycleStopCommands(runner lifecycleRunner, def workspace.Definition) error {
	var errs []error
	for i := len(def.Projects) - 1; i >= 0; i-- {
		project := def.Projects[i]
		command := strings.TrimSpace(project.Lifecycle.Stop)
		if command == "" {
			continue
		}
		if err := runner.RunShell(project.HostPath, command); err != nil {
			errs = append(errs, fmt.Errorf("stop project %s (%s): %w", project.Name, project.HostPath, err))
		}
	}
	return errors.Join(errs...)
}

func newLifecycleStopCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "stop <workspace>",
		Short: "Stop the AGW workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err := runner.Stop(filepath.Dir(located.Path)); err != nil {
				return err
			}
			return runProjectLifecycleStopCommands(runner, located.Definition)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleBuildCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "build <workspace>",
		Short: "Run external Docker CLI build for the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			return runner.Build(filepath.Dir(located.Path))
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleUpCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "up <workspace>",
		Short: "Run external Docker CLI up for the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			return runner.Up(filepath.Dir(located.Path))
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleDownCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "down <workspace>",
		Short: "Run external Docker CLI down for the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			return runner.Down(filepath.Dir(located.Path))
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleAttachCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "attach <workspace>",
		Short: "Run external Docker CLI exec for the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			service := located.Definition.Container.Service
			if strings.TrimSpace(service) == "" {
				return fmt.Errorf("workspace %q has no service configured", located.Definition.ID)
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			return runner.Attach(filepath.Dir(located.Path), service)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleStatusCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "status [workspace]",
		Short: "Show workspace lifecycle status",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(io.Discard, io.Discard)
			report := doctor.Diagnose(located, runner)
			return writeLifecycleStatus(cmd.OutOrStdout(), located, runner, report)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newLifecycleListCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List known workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := listDefinitions(configPath)
			if err != nil {
				return err
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Definition.ID < items[j].Definition.ID })
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			runner := newLifecycleRunner(io.Discard, io.Discard)
			if _, err := fmt.Fprintln(out, "WORKSPACE\tSTATE\tSERVICE\tDIR"); err != nil {
				return err
			}
			for _, item := range items {
				report := doctor.Diagnose(item, runner)
				_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", item.Definition.ID, report.State, item.Definition.Container.Service, item.Definition.Workspace.Dir)
				if err != nil {
					return err
				}
			}
			return out.Flush()
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func writeLifecycleStatus(out io.Writer, located workspace.LocatedDefinition, runner lifecycleRunner, report doctor.Report) error {
	if _, err := fmt.Fprintf(out, "Workspace: %s\n", located.Definition.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "State: %s\n", report.State); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Service: %s\n", located.Definition.Container.Service); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Directory: %s\n", filepath.Dir(located.Path)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nNetworks:"); err != nil {
		return err
	}
	attachments := networkCandidates(located.Definition)
	if len(attachments) == 0 {
		if _, err := fmt.Fprintln(out, "  none"); err != nil {
			return err
		}
	}
	for _, network := range attachments {
		exists, err := runner.NetworkExists(network)
		status := "missing"
		if err != nil {
			status = "unknown: " + err.Error()
		} else if exists {
			status = "available"
		}
		if _, err := fmt.Fprintf(out, "  Network %s: %s\n", network, status); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "\nChecks:"); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(out, "  %s %s: %s\n", checkStatusSymbol(check.Status), check.Name, check.Detail); err != nil {
			return err
		}
	}
	next := report.NextAction
	if strings.TrimSpace(next) == "" {
		next = "none"
	}
	if _, err := fmt.Fprintln(out, "\nNext:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  %s\n", next)
	return err
}

func listDefinitions(path string) ([]workspace.LocatedDefinition, error) {
	p := path
	var err error
	if p == "" {
		p, err = config.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	cfg, err := config.Load(p)
	if err != nil {
		return nil, err
	}
	registry := workspace.Registry{Roots: cfg.WorkspaceRoots}
	return registry.List()
}

func findLocatedDefinition(path, workspaceID string) (workspace.LocatedDefinition, error) {
	p := path
	var err error
	if p == "" {
		p, err = config.DefaultPath()
		if err != nil {
			return workspace.LocatedDefinition{}, err
		}
	}
	cfg, err := config.Load(p)
	if err != nil {
		return workspace.LocatedDefinition{}, err
	}
	registry := workspace.Registry{Roots: cfg.WorkspaceRoots}
	return registry.Find(workspaceID)
}
