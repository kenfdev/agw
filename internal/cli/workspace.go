package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	cmd.AddCommand(newWorkspaceNetworkCommand())
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
		from          string
		configPath    string
		projectFlags  []string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a workspace definition file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if from != "" {
				return runWorkspaceNewFrom(cmd, configPath, root, from, id, name, storage, service, workspaceRoot)
			}
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
	cmd.Flags().StringVar(&from, "from", "", "project path to create a standalone workspace from")
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().StringSliceVar(&projectFlags, "project", nil, "project definitions in name=path:mountPath format")
	cmd.Flags().StringVar(&service, "service", "", "container service name")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "container workspace root path")
	return cmd
}

func runWorkspaceNewFrom(cmd *cobra.Command, configPath, root, from, id, name, storage, service, workspaceRoot string) error {
	projectPath, err := filepath.Abs(filepath.Clean(from))
	if err != nil {
		return err
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("--from must be an existing directory")
	}

	var cfg config.Config
	if root == "" {
		path := configPath
		if path == "" {
			var err error
			path, err = config.DefaultPath()
			if err != nil {
				return err
			}
		}
		cfg, err = config.Load(path)
		if err != nil {
			return err
		}
		if len(cfg.WorkspaceRoots) == 0 {
			return fmt.Errorf("config has no workspace roots")
		}
		root = cfg.WorkspaceRoots[0]
	} else if configPath != "" {
		cfg, _ = config.Load(configPath)
	}

	if id == "" {
		id = filepath.Base(projectPath)
	}
	if name == "" {
		name = id
	}
	if service == "" {
		service = "dev"
	}
	if workspaceRoot == "" {
		workspaceRoot = "/workspace"
	}
	if storage == "" {
		if suggested, ok := workspace.SuggestStoragePath(projectPath, cfg.PathMappings); ok {
			storage = suggested
		} else {
			storage = filepath.Join("workspaces", id)
		}
	}

	defRoot, err := workspaceDefinitionPath(root, storage)
	if err != nil {
		return err
	}
	def := workspace.Definition{
		ID:      id,
		Name:    name,
		Storage: workspace.Storage{Path: storage},
		Container: workspace.Container{
			Service:       service,
			WorkspaceRoot: workspaceRoot,
		},
		Projects: []workspace.Project{{
			Name:      id,
			Path:      projectPath,
			MountPath: workspaceRoot,
		}},
	}
	defPath := filepath.Join(defRoot, "agw.yaml")
	if err := workspace.SaveDefinition(defPath, def); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created standalone workspace %s at %s\n", id, defPath); err != nil {
		return err
	}
	if hints := detectContainerSetup(projectPath); len(hints) > 0 {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nFound possible container setup:"); err != nil {
			return err
		}
		for _, hint := range hints {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", hint); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nExternal networks are optional. Add one only when this sidecar must reach existing project services."); err != nil {
			return err
		}
	}
	return nil
}

func detectContainerSetup(projectPath string) []string {
	candidates := []string{
		"compose.yaml",
		"docker-compose.yaml",
		"Dockerfile",
		".devcontainer/devcontainer.json",
	}
	var found []string
	for _, rel := range candidates {
		info, err := os.Stat(filepath.Join(projectPath, rel))
		if err == nil && !info.IsDir() {
			found = append(found, filepath.ToSlash(rel))
		}
	}
	sort.Strings(found)
	return found
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

func newWorkspaceNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "network", Short: "Manage workspace external networks"}
	cmd.AddCommand(newWorkspaceNetworkAddCommand())
	return cmd
}

func newWorkspaceNetworkAddCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "add <workspace> <network>",
		Short: "Select an external Docker network for a workspace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			network := strings.TrimSpace(args[1])
			if network == "" {
				return fmt.Errorf("network name must not be blank")
			}

			def := located.Definition
			if def.Networks == nil {
				def.Networks = &workspace.Networks{}
			}
			for _, existing := range def.Networks.Attach {
				if existing.Name == network {
					return workspace.SaveDefinition(located.Path, def)
				}
			}
			def.Networks.Attach = append(def.Networks.Attach, workspace.NetworkAttachment{Name: network})
			return workspace.SaveDefinition(located.Path, def)
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
