package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kenfdev/agw/internal/apply"
	"github.com/kenfdev/agw/internal/base"
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
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=hostPath:containerPath)", value)
	}
	if namePart == "" {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=hostPath:containerPath)", value)
	}

	pathPart, mountPart, found := strings.Cut(rest, ":")
	if !found {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=hostPath:containerPath)", value)
	}
	if pathPart == "" || mountPart == "" {
		return "", "", "", fmt.Errorf("invalid project flag value %q (expected name=hostPath:containerPath)", value)
	}
	return namePart, pathPart, mountPart, nil
}

func workspaceDefinitionPath(root, storage string) (string, error) {
	if filepath.IsAbs(storage) {
		return "", fmt.Errorf("--workspace-dir must be a relative path")
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
		return "", fmt.Errorf("--workspace-dir must not escape --root")
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
				return fmt.Errorf("--workspace-dir is required")
			}
			if service == "" {
				return fmt.Errorf("--service is required")
			}
			if workspaceRoot == "" {
				return fmt.Errorf("--workdir is required")
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
					Name:          projectName,
					HostPath:      projectPath,
					ContainerPath: projectMountPath,
				})
			}

			def := workspace.Definition{
				ID:        id,
				Name:      name,
				Workspace: workspace.Workspace{Dir: storage},
				Container: workspace.Container{
					Service: service,
					Workdir: workspaceRoot,
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
	cmd.Flags().StringVar(&storage, "workspace-dir", "", "AGW workspace directory under --root")
	cmd.Flags().StringVar(&storage, "storage", "", "legacy alias for --workspace-dir")
	cmd.Flags().StringVar(&from, "from", "", "project path to create a standalone workspace from")
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().StringSliceVar(&projectFlags, "project", nil, "project definitions in name=hostPath:containerPath format")
	cmd.Flags().StringVar(&service, "service", "", "container service name")
	cmd.Flags().StringVar(&workspaceRoot, "workdir", "", "container working directory")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "legacy alias for --workdir")
	_ = cmd.Flags().MarkDeprecated("storage", "use --workspace-dir instead")
	_ = cmd.Flags().MarkDeprecated("workspace-root", "use --workdir instead")
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
		if cfg.Root() == "" {
			return fmt.Errorf("config has no workspace root")
		}
		root = cfg.Root()
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
		ID:        id,
		Name:      name,
		Workspace: workspace.Workspace{Dir: storage},
		Container: workspace.Container{
			Service: service,
			Workdir: workspaceRoot,
		},
		Projects: []workspace.Project{{
			Name:          id,
			HostPath:      projectPath,
			ContainerPath: workspaceRoot,
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
	var agentJSON bool

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

			registry := workspace.Registry{Roots: []string{cfg.Root()}}
			located, err := registry.Find(args[0])
			if err != nil {
				return err
			}
			baseGuidance, baseEnvironmentMeta, err := loadBaseEnvironmentGuidance(path, cfg, located)
			if err != nil {
				return err
			}
			baseStatus := baseStatusForPrepare(cfg)
			baseEnvironmentMeta.Image = baseStatusImage(baseStatus)
			if baseStatus != nil {
				baseEnvironmentMeta.BuildContext = baseStatus.Config.ContextDir
				baseEnvironmentMeta.Dockerfile = baseStatus.Config.Dockerfile
				baseEnvironmentMeta.ImageStatus = string(baseStatus.Status)
				if baseStatus.CreatedAt != nil {
					baseEnvironmentMeta.ImageCreatedAt = baseStatus.CreatedAt.Format(time.RFC3339)
				}
				baseEnvironmentMeta.ImageAge = baseStatus.Age
				baseEnvironmentMeta.ImageError = baseStatus.Error
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
				BaseEnvironment:   baseGuidance,
				BaseImage:         baseStatus,
			})
			if err != nil {
				return err
			}

			if agentJSON {
				if outputPath != "" {
					if err := os.WriteFile(outputPath, []byte(prompt), 0o644); err != nil {
						return err
					}
				}
				return writeJSON(cmd.OutOrStdout(), newPrepareAgentPacket(located, prompt, outputPath, networkCandidatesForPrepare(located.Definition, availableNetworks), baseEnvironmentMeta))
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
	cmd.Flags().BoolVar(&agentJSON, "agent-json", false, "print machine-readable agent preparation context")
	return cmd
}

type prepareAgentPacket struct {
	WorkspaceID            string                     `json:"workspaceId"`
	WorkspaceDir           string                     `json:"workspaceDir"`
	PromptPath             string                     `json:"promptPath,omitempty"`
	Service                string                     `json:"service"`
	Workdir                string                     `json:"workdir"`
	Mode                   string                     `json:"mode"`
	Prompt                 string                     `json:"prompt"`
	ExpectedGeneratedFiles []string                   `json:"expectedGeneratedFiles"`
	SelectedNetworks       []string                   `json:"selectedNetworks"`
	NetworkCandidates      []string                   `json:"networkCandidates"`
	BaseEnvironment        baseEnvironmentAgentPacket `json:"baseEnvironment"`
	SafetyRules            []string                   `json:"safetyRules"`
	NextCommands           []string                   `json:"nextCommands"`
}

type baseEnvironmentAgentPacket struct {
	GlobalGuidancePath    string `json:"globalGuidancePath,omitempty"`
	WorkspaceGuidancePath string `json:"workspaceGuidancePath,omitempty"`
	IncludeGlobal         bool   `json:"includeGlobal"`
	Image                 string `json:"image,omitempty"`
	BuildContext          string `json:"buildContext,omitempty"`
	Dockerfile            string `json:"dockerfile,omitempty"`
	ImageStatus           string `json:"imageStatus,omitempty"`
	ImageCreatedAt        string `json:"imageCreatedAt,omitempty"`
	ImageAge              string `json:"imageAge,omitempty"`
	ImageError            string `json:"imageError,omitempty"`
}

func newPrepareAgentPacket(located workspace.LocatedDefinition, prompt, promptPath string, candidates []string, baseEnvironment baseEnvironmentAgentPacket) prepareAgentPacket {
	def := located.Definition
	selected := networkCandidates(def)
	mode := "standalone-sidecar"
	if len(selected) > 0 {
		mode = "attached-sidecar"
	}
	return prepareAgentPacket{
		WorkspaceID:            def.ID,
		WorkspaceDir:           filepath.Dir(located.Path),
		PromptPath:             promptPath,
		Service:                def.Container.Service,
		Workdir:                def.Container.Workdir,
		Mode:                   mode,
		Prompt:                 prompt,
		ExpectedGeneratedFiles: []string{"Dockerfile", "compose.yaml"},
		SelectedNetworks:       selected,
		NetworkCandidates:      candidates,
		BaseEnvironment:        baseEnvironment,
		SafetyRules: []string{
			"Do not edit target project files.",
			"Generate files for the AGW workspace directory only.",
			"Use external networks only when selected in the AGW workspace definition.",
			"Ask questions before generating files if important information is missing.",
		},
		NextCommands: []string{
			fmt.Sprintf("agw workspace apply %s <generated-dir>", def.ID),
			fmt.Sprintf("agw doctor %s --json", def.ID),
			fmt.Sprintf("agw start %s", def.ID),
		},
	}
}

func baseStatusForPrepare(cfg config.Config) *base.Status {
	baseCfg, ok, err := base.Optional(cfg)
	if err != nil {
		status := base.Status{Status: base.StatusUnknown, Error: err.Error()}
		return &status
	}
	if !ok {
		return nil
	}
	status := inspectBaseImage(baseCfg, newLifecycleRunner(io.Discard, io.Discard), now())
	return &status
}

func baseStatusImage(status *base.Status) string {
	if status == nil {
		return ""
	}
	return status.Config.Image
}

func loadBaseEnvironmentGuidance(configPath string, cfg config.Config, located workspace.LocatedDefinition) (prepare.BaseEnvironmentGuidance, baseEnvironmentAgentPacket, error) {
	includeGlobal := located.Definition.IncludeGlobalBaseEnvironment()
	meta := baseEnvironmentAgentPacket{IncludeGlobal: includeGlobal}
	var guidance prepare.BaseEnvironmentGuidance

	if includeGlobal && cfg.BaseEnvironment.GuidancePath != "" {
		path := resolveRelativeToFile(configPath, cfg.BaseEnvironment.GuidancePath)
		content, err := os.ReadFile(path)
		if err != nil {
			return guidance, meta, fmt.Errorf("read global base environment guidance %s: %w", path, err)
		}
		meta.GlobalGuidancePath = path
		guidance.Global = string(content)
	}

	if located.Definition.BaseEnvironment.GuidancePath != "" {
		path := resolveRelativeToFile(located.Path, located.Definition.BaseEnvironment.GuidancePath)
		content, err := os.ReadFile(path)
		if err != nil {
			return guidance, meta, fmt.Errorf("read workspace base environment guidance %s: %w", path, err)
		}
		meta.WorkspaceGuidancePath = path
		guidance.Workspace = string(content)
	}

	return guidance, meta, nil
}

func resolveRelativeToFile(baseFile, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(baseFile), path))
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

			registry := workspace.Registry{Roots: []string{cfg.Root()}}
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
