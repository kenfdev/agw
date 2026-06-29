package cli

import (
	"fmt"

	"github.com/kenfdev/agw/internal/tui"
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "agw",
		Short:         "Agent Workspace",
		Long:          "Agent Workspace manages personal sidecar development containers.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(NewConfigCommand())
	cmd.AddCommand(NewWorkspaceCommand())
	cmd.AddCommand(
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
			if err := tui.Run(items); err != nil {
				return fmt.Errorf("tui failed: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}
