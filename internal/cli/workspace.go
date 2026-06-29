package cli

import (
	"fmt"
	"os"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/prepare"
	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func NewWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Manage AGW workspaces"}
	cmd.AddCommand(newWorkspacePrepareCommand())
	return cmd
}

func newWorkspacePrepareCommand() *cobra.Command {
	var configPath string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "prepare <workspace>",
		Short: "Render the workspace preparation prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPath
			if path == "" {
				var err error
				path, err = config.DefaultPath()
				if err != nil {
					return err
				}
			}

			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			registry := workspace.Registry{Roots: cfg.WorkspaceRoots}
			located, err := registry.Find(args[0])
			if err != nil {
				return err
			}

			projectSnapshots := make([]scanner.ProjectSnapshot, 0, len(located.Definition.Projects))
			for _, project := range located.Definition.Projects {
				snapshot, err := scanner.ScanProject(project)
				if err != nil {
					return err
				}
				projectSnapshots = append(projectSnapshots, snapshot)
			}

			prompt, err := prepare.Render(prepare.Input{
				Definition:        located.Definition,
				Projects:          projectSnapshots,
				NetworkCandidates: networkCandidates(located.Definition),
			})
			if err != nil {
				return err
			}

			if outputPath != "" {
				return os.WriteFile(outputPath, []byte(prompt), 0o644)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), prompt)
			return err
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().StringVar(&outputPath, "output", "", "output file path")
	return cmd
}

func networkCandidates(def workspace.Definition) []string {
	if def.Networks == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, attachment := range def.Networks.Attach {
		if attachment.Name == "" {
			continue
		}
		if _, ok := seen[attachment.Name]; ok {
			continue
		}
		seen[attachment.Name] = struct{}{}
		out = append(out, attachment.Name)
	}
	return out
}
