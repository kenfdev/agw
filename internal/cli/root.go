package cli

import "github.com/spf13/cobra"

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
	return cmd
}
