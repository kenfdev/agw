package workspacecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/compose"
	"github.com/kenfdev/agw/internal/mount"
	"github.com/kenfdev/agw/internal/workspace"
)

type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusWarn Status = "warn"
	StatusSkip Status = "skip"
)

type Code string

const (
	CodeService         Code = "service"
	CodeDockerfile      Code = "dockerfile"
	CodeProjectMount    Code = "project_mount"
	CodeExternalNetwork Code = "external_network"
	CodeComposeConfig   Code = "compose_config"
)

type Check struct {
	Code   Code
	Name   string
	Status Status
	Detail string
}

type Runner interface {
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
}

func ValidateCompose(workspaceDir string, def workspace.Definition, file compose.File, runner Runner) []Check {
	var checks []Check
	service, ok := file.Services[def.Container.Service]
	if !ok {
		return append(checks, fail(CodeService, "service", fmt.Sprintf("service %s not found in compose.yaml", def.Container.Service)))
	}
	checks = append(checks, pass(CodeService, "service", def.Container.Service))

	dockerfileStatus, dockerfileDetail, err := checkBuildDockerfile(workspaceDir, service)
	if err != nil {
		return append(checks, Check{Code: CodeDockerfile, Name: "Dockerfile", Status: StatusFail, Detail: err.Error()})
	}
	checks = append(checks, Check{Code: CodeDockerfile, Name: "Dockerfile", Status: dockerfileStatus, Detail: dockerfileDetail})

	for _, project := range def.Projects {
		containerPath, err := ResolveProjectMount(service, project)
		if err != nil {
			return append(checks, fail(CodeProjectMount, "project mount", err.Error()))
		}
		checks = append(checks, pass(CodeProjectMount, "project mount", fmt.Sprintf("%s -> %s", project.Name, containerPath)))
	}

	checkedNetworks := map[string]struct{}{}
	attachments := selectedNetworks(def)
	if len(attachments) == 0 {
		attachments = composeExternalNetworkAttachments(file, service)
		if len(attachments) == 0 {
			checks = append(checks, pass(CodeExternalNetwork, "external networks", "none required"))
		}
	}
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			return append(checks, fail(CodeExternalNetwork, "external network", "selected network name must not be blank"))
		}
		key, network, ok := findComposeNetwork(file.Networks, name)
		if !ok || !network.External {
			return append(checks, fail(CodeExternalNetwork, "external network", fmt.Sprintf("selected network %s must be declared as external in compose.yaml", name)))
		}
		if !serviceHasNetworkAttachment(service, key, network) {
			return append(checks, fail(CodeExternalNetwork, "external network", fmt.Sprintf("service %s must attach to selected network %s", def.Container.Service, name)))
		}
		resolvedName := composeNetworkName(key, network)
		exists, err := runner.NetworkExists(resolvedName)
		if err != nil {
			checks = append(checks, Check{Code: CodeExternalNetwork, Name: "external network", Status: StatusWarn, Detail: err.Error()})
			continue
		}
		if !exists {
			return append(checks, fail(CodeExternalNetwork, "external network", fmt.Sprintf("external network %s not found", resolvedName)))
		}
		checks = append(checks, pass(CodeExternalNetwork, "external network", resolvedName))
		checkedNetworks[resolvedName] = struct{}{}
	}

	for key, network := range file.Networks {
		if !network.External {
			continue
		}
		name := composeNetworkName(key, network)
		if _, ok := checkedNetworks[name]; ok {
			continue
		}
		exists, err := runner.NetworkExists(name)
		if err != nil {
			checks = append(checks, Check{Code: CodeExternalNetwork, Name: "external network", Status: StatusWarn, Detail: err.Error()})
			continue
		}
		if !exists {
			return append(checks, fail(CodeExternalNetwork, "external network", fmt.Sprintf("external network %s not found", name)))
		}
		checks = append(checks, pass(CodeExternalNetwork, "external network", name))
	}

	if err := runner.ComposeConfig(workspaceDir); err != nil {
		return append(checks, fail(CodeComposeConfig, "compose config", fmt.Sprintf("docker compose config: %v", err)))
	}
	checks = append(checks, pass(CodeComposeConfig, "compose config", "docker compose config"))
	return checks
}

func FirstFailure(checks []Check) (Check, bool) {
	for _, check := range checks {
		if check.Status == StatusFail {
			return check, true
		}
	}
	return Check{}, false
}

func ResolveProjectMount(service compose.Service, project workspace.Project) (string, error) {
	if project.ContainerPath != "" {
		if mount.HasVolumeMount(service.Volumes, project.HostPath, project.ContainerPath) {
			return project.ContainerPath, nil
		}
		required := project.HostPath + ":" + project.ContainerPath
		return "", fmt.Errorf("missing volume %s for project %s", required, project.Name)
	}

	targets := mount.VolumeTargets(service.Volumes, project.HostPath)
	switch len(targets) {
	case 0:
		return "", fmt.Errorf("missing volume for project %s host path %s", project.Name, project.HostPath)
	case 1:
		return targets[0], nil
	default:
		return "", fmt.Errorf("multiple volumes for project %s host path %s; set containerPath in agw.yaml", project.Name, project.HostPath)
	}
}

func pass(code Code, name, detail string) Check {
	return Check{Code: code, Name: name, Status: StatusPass, Detail: detail}
}

func fail(code Code, name, detail string) Check {
	return Check{Code: code, Name: name, Status: StatusFail, Detail: detail}
}

func checkBuildDockerfile(workspaceDir string, service compose.Service) (Status, string, error) {
	contextDir, dockerfile, ok := buildPaths(service.Build)
	if !ok {
		return StatusSkip, "service has no build", nil
	}
	if contextDir == "" {
		contextDir = "."
	}
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	path := filepath.Join(workspaceDir, contextDir, dockerfile)
	if err := ensureInside(workspaceDir, path); err != nil {
		return "", "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("%s not found for service build: %w", dockerfile, err)
		}
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("%s not found for service build: is a directory", dockerfile)
	}
	return StatusPass, "build Dockerfile exists", nil
}

func buildPaths(build any) (string, string, bool) {
	switch value := build.(type) {
	case nil:
		return "", "", false
	case string:
		return value, "Dockerfile", true
	case map[string]any:
		return stringMapValue(value, "context"), stringMapValue(value, "dockerfile"), true
	case map[any]any:
		return anyMapValue(value, "context"), anyMapValue(value, "dockerfile"), true
	default:
		return "", "", false
	}
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func anyMapValue(values map[any]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func ensureInside(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes workspace directory: %s", path)
	}
	return nil
}

func selectedNetworks(def workspace.Definition) []workspace.NetworkAttachment {
	if def.Networks == nil {
		return nil
	}
	return def.Networks.Attach
}

func composeExternalNetworkAttachments(file compose.File, service compose.Service) []workspace.NetworkAttachment {
	var attachments []workspace.NetworkAttachment
	for key, network := range file.Networks {
		if !network.External || !serviceHasNetworkAttachment(service, key, network) {
			continue
		}
		attachments = append(attachments, workspace.NetworkAttachment{Name: composeNetworkName(key, network)})
	}
	return attachments
}

func findComposeNetwork(networks map[string]compose.Network, selected string) (string, compose.Network, bool) {
	for key, network := range networks {
		name := composeNetworkName(key, network)
		if key == selected || name == selected {
			return key, network, true
		}
	}
	return "", compose.Network{}, false
}

func serviceHasNetworkAttachment(service compose.Service, key string, network compose.Network) bool {
	resolvedName := composeNetworkName(key, network)
	for _, attached := range service.Networks {
		if attached == key || attached == resolvedName {
			return true
		}
	}
	return false
}

func composeNetworkName(key string, network compose.Network) string {
	if network.Name != "" {
		return network.Name
	}
	return key
}
