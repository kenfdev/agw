package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

type lifecycleRunner interface {
	Build(dir string) error
	Up(dir string) error
	Down(dir string) error
	Attach(dir string, service string) error
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
	ServiceRunning(dir string, service string) (bool, error)
}

var newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
	return docker.CLI{Out: stdout, Err: stderr}
}

func newLifecycleBuildCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "build <workspace>",
		Short: "Build the workspace compose services",
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
		Short: "Start workspace containers",
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
		Short: "Stop workspace containers",
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
		Short: "Attach to a workspace container",
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
		Use:   "status <workspace>",
		Short: "Show workspace lifecycle status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
			out := cmd.OutOrStdout()

			_, err = fmt.Fprintln(out, "Workspace:", located.Definition.ID)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, "Service:", located.Definition.Container.Service)
			if err != nil {
				return err
			}
			attachments := networkCandidates(located.Definition)
			if len(attachments) == 0 {
				_, err = fmt.Fprintln(out, "Networks: none")
				return err
			}
			for _, network := range attachments {
				exists, err := runner.NetworkExists(network)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(out, "Network %s exists: %t\n", network, exists)
				if err != nil {
					return err
				}
			}
			return nil
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
			out := cmd.OutOrStdout()
			for _, item := range items {
				_, err := fmt.Fprintf(out, "%s\t%s\n", item.Definition.ID, item.Definition.Storage.Path)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
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
