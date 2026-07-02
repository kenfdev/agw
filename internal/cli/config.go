package cli

import (
	"fmt"

	"github.com/kenfdev/agw/internal/config"
	"github.com/spf13/cobra"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage AGW configuration"}
	var configPath string
	var root string
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize AGW configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				return fmt.Errorf("--root is required")
			}
			path := configPath
			if path == "" {
				var err error
				path, err = config.DefaultPath()
				if err != nil {
					return err
				}
			}
			return config.Save(path, config.Config{WorkspaceRoot: root})
		},
	}
	initCmd.Flags().StringVar(&configPath, "config", "", "config file path")
	initCmd.Flags().StringVar(&root, "root", "", "AGW root directory")
	cmd.AddCommand(initCmd)
	return cmd
}
