package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/prepare"
	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func NewWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Manage AGW workspaces"}
	cmd.AddCommand(newWorkspacePrepareCommand())
	cmd.AddCommand(newWorkspaceNewCommand())
	return cmd
}

func parseWorkspaceProject(value string) (string, string, string, error) {
	namePart, rest, found := strings.Cut(value, "=")
	if !found {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=path:mountPath)", value)
	}
	if namePart == "" {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=path:mountPath)", value)
	}

	pathPart, mountPart, found := strings.Cut(rest, ":")
	if !found {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=path:mountPath)", value)
	}
	if pathPart == "" || mountPart == "" {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=path:mountPath)", value)
	}
	return namePart, pathPart, mountPart, nil
}

func newWorkspaceNewCommand() *cobra.Command {
	var (
		id            string
		name          string
		root          string
		storage       string
		service       string
		workspaceRoot string
		projectFlags  []string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a workspace definition file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if root == "" {
				return fmt.Errorf("--root is required")
			}
			if storage == "" {
				return fmt.Errorf("--storage is required")
			}
			if service == "" {
				return fmt.Errorf("--service is required")
			}
			if workspaceRoot == "" {
				return fmt.Errorf("--workspace-root is required")
			}

			projects := make([]workspace.Project, 0, len(projectFlags))
			for _, raw := range projectFlags {
				projectName, projectPath, projectMountPath, err := parseWorkspaceProject(raw)
				if err != nil {
					return err
				}
				projects = append(projects, workspace.Project{
					Name:      projectName,
					Path:      projectPath,
					MountPath: projectMountPath,
				})
			}

			def := workspace.Definition{
				ID:      id,
				Name:    name,
				Storage: workspace.Storage{Path: storage},
				Container: workspace.Container{
					Service:       service,
					WorkspaceRoot: workspaceRoot,
				},
				Projects: projects,
			}

			defPath := filepath.Join(root, storage, "agw.yaml")
			return workspace.SaveDefinition(defPath, def)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "workspace id")
	cmd.Flags().StringVar(&name, "name", "", "workspace name")
	cmd.Flags().StringVar(&root, "root", "", "AGW root directory")
	cmd.Flags().StringVar(&storage, "storage", "", "workspace storage path")
	cmd.Flags().StringSliceVar(&projectFlags, "project", nil, "project definitions in name=path:mountPath format")
	cmd.Flags().StringVar(&service, "service", "", "container service name")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "container workspace root path")
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
