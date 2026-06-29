package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/apply"
	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/prepare"
	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

var newDockerRunner = func() docker.Runner {
	return docker.CLI{}
}

var newDockerCLI = func() docker.CLI {
	return docker.CLI{}
}

func NewWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Manage AGW workspaces"}
	cmd.AddCommand(newWorkspacePrepareCommand())
	cmd.AddCommand(newWorkspaceNewCommand())
	cmd.AddCommand(newWorkspaceApplyCommand())
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

func workspaceDefinitionPath(root, storage string) (string, error) {
	if filepath.IsAbs(storage) {
		return "", fmt.Errorf("--storage must be a relative path")
	}

	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}

	storagePath, err := filepath.Abs(filepath.Join(root, filepath.Clean(storage)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, storagePath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--storage must not escape --root")
	}

	return storagePath, nil
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

			defRoot, err := workspaceDefinitionPath(root, storage)
			if err != nil {
				return err
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

			defPath := filepath.Join(defRoot, "agw.yaml")
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

			availableNetworks, err := newDockerCLI().ListNetworks()
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: unable to list Docker networks: %v\n", err)
				availableNetworks = nil
			}

			prompt, err := prepare.Render(prepare.Input{
				Definition:        located.Definition,
				Projects:          projectSnapshots,
				NetworkCandidates: networkCandidatesForPrepare(located.Definition, availableNetworks),
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

func newWorkspaceApplyCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "apply <workspace> <generated-dir>",
		Short: "Apply generated workspace files",
		Args:  cobra.ExactArgs(2),
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

			workspaceDir := filepath.Dir(located.Path)
			return apply.Apply(workspaceDir, located.Definition, args[1], newDockerRunner())
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
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

func networkCandidatesForPrepare(def workspace.Definition, discovered []docker.Network) []string {
	candidates := networkCandidates(def)
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		seen[candidate] = struct{}{}
	}

	composeNetworks := make([]string, 0)
	otherNetworks := make([]string, 0)
	for _, network := range discovered {
		name := strings.TrimSpace(network.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if hasComposeLabel(network.Labels) {
			composeNetworks = append(composeNetworks, name)
			continue
		}
		otherNetworks = append(otherNetworks, name)
	}
	return append(append(candidates, composeNetworks...), otherNetworks...)
}

func hasComposeLabel(labels map[string]string) bool {
	for key := range labels {
		if strings.HasPrefix(key, "com.docker.compose.") {
			return true
		}
	}
	return false
}
