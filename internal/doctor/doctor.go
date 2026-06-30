package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/compose"
	"github.com/kenfdev/agw/internal/workspace"
)

var statFile = os.Stat

type State string

const (
	StateDefined      State = "defined"
	StateNeedsPrepare State = "needs-prepare"
	StateNeedsApply   State = "needs-apply"
	StateReadyToBuild State = "ready-to-build"
	StateNotRunning   State = "not-running"
	StateRunning      State = "running"
	StateBroken       State = "broken"
)

type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckFail CheckStatus = "fail"
	CheckWarn CheckStatus = "warn"
	CheckSkip CheckStatus = "skip"
)

type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail"`
}

type Report struct {
	WorkspaceID string  `json:"workspaceId"`
	State       State   `json:"state"`
	Checks      []Check `json:"checks"`
	NextAction  string  `json:"nextAction"`
}

type Runner interface {
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
	ServiceRunning(dir string, service string) (bool, error)
}

func Diagnose(located workspace.LocatedDefinition, runner Runner) Report {
	dir := filepath.Dir(located.Path)
	report := Report{WorkspaceID: located.Definition.ID, State: StateDefined}
	report.add("workspace definition", CheckPass, located.Path)
	for _, project := range located.Definition.Projects {
		info, err := os.Stat(project.HostPath)
		if err != nil || !info.IsDir() {
			report.add("project path", CheckFail, project.HostPath)
			report.State = StateBroken
			report.NextAction = fmt.Sprintf("fix project path for %s", project.Name)
			return report
		}
		report.add("project path", CheckPass, project.HostPath)
	}
	promptPath := filepath.Join(dir, "prompt.md")
	if _, err := os.Stat(promptPath); err != nil {
		report.add("prompt", CheckFail, "prompt.md missing")
		report.State = StateNeedsPrepare
		report.NextAction = fmt.Sprintf("agw workspace prepare %s --output %s", report.WorkspaceID, promptPath)
		return report
	}
	report.add("prompt", CheckPass, promptPath)

	composePath := filepath.Join(dir, "compose.yaml")
	if _, err := statFile(composePath); err != nil {
		if os.IsNotExist(err) {
			return fail(&report, "compose.yaml", "compose.yaml missing", workspaceApplyAction(report.WorkspaceID), StateNeedsApply)
		}
		return fail(&report, "compose.yaml", err.Error(), fmt.Sprintf("fix generated workspace files for %s", report.WorkspaceID), StateBroken)
	}
	report.add("compose.yaml", CheckPass, composePath)

	composeFile, err := compose.Load(composePath)
	if err != nil {
		return fail(&report, "compose parse", err.Error(), "fix compose.yaml", StateBroken)
	}
	service, ok := composeFile.Services[located.Definition.Container.Service]
	if !ok {
		return fail(&report, "service", fmt.Sprintf("service %s not found in compose.yaml", located.Definition.Container.Service), "run agw workspace apply", StateBroken)
	}
	report.add("service", CheckPass, located.Definition.Container.Service)

	dockerfileStatus, dockerfileDetail, err := checkBuildDockerfile(dir, service)
	if err != nil {
		state := StateBroken
		next := "fix compose.yaml"
		if os.IsNotExist(err) {
			state = StateNeedsApply
			next = workspaceApplyAction(report.WorkspaceID)
		}
		return fail(&report, "Dockerfile", err.Error(), next, state)
	}
	report.add("Dockerfile", dockerfileStatus, dockerfileDetail)

	for _, project := range located.Definition.Projects {
		if !hasVolumeMount(service.Volumes, project.HostPath, project.ContainerPath) {
			required := project.HostPath + ":" + project.ContainerPath
			return fail(&report, "project mount", fmt.Sprintf("missing volume %s for project %s", required, project.Name), "run agw workspace apply", StateBroken)
		}
		report.add("project mount", CheckPass, project.Name)
	}

	attachments := selectedNetworks(located.Definition)
	if len(attachments) == 0 {
		report.add("external networks", CheckPass, "none required")
	}
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			return fail(&report, "external network", "selected network name must not be blank", "fix workspace definition", StateBroken)
		}
		key, network, ok := findComposeNetwork(composeFile.Networks, name)
		if !ok || !network.External {
			return fail(&report, "external network", fmt.Sprintf("selected network %s must be declared as external in compose.yaml", name), "run agw workspace apply", StateBroken)
		}
		if !serviceHasNetworkAttachment(service, key, network) {
			return fail(&report, "external network", fmt.Sprintf("service %s must attach to selected network %s", located.Definition.Container.Service, name), "run agw workspace apply", StateBroken)
		}
		resolvedName := composeNetworkName(key, network)
		exists, err := runner.NetworkExists(resolvedName)
		if err != nil {
			report.add("external network", CheckWarn, err.Error())
			continue
		}
		if !exists {
			next := fmt.Sprintf("start the base project services, then run agw start %s", report.WorkspaceID)
			return fail(&report, "external network", fmt.Sprintf("external network %s not found", resolvedName), next, StateBroken)
		}
		report.add("external network", CheckPass, resolvedName)
	}

	if err := runner.ComposeConfig(dir); err != nil {
		return fail(&report, "compose config", err.Error(), "fix compose.yaml", StateBroken)
	}
	report.add("compose config", CheckPass, "docker compose config")

	running, err := runner.ServiceRunning(dir, located.Definition.Container.Service)
	if err != nil {
		report.add("runtime", CheckWarn, err.Error())
		report.State = StateNotRunning
		report.NextAction = fmt.Sprintf("agw build %s && agw up %s", report.WorkspaceID, report.WorkspaceID)
		return report
	}

	if running {
		report.add("runtime", CheckPass, "service is running")
		report.State = StateRunning
		report.NextAction = fmt.Sprintf("agw start %s", report.WorkspaceID)
		return report
	}

	report.add("runtime", CheckFail, "service is not running")
	report.State = StateNotRunning
	report.NextAction = fmt.Sprintf("agw start %s", report.WorkspaceID)
	return report
}

func (r *Report) add(name string, status CheckStatus, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
}

func fail(report *Report, name, detail, next string, state State) Report {
	report.add(name, CheckFail, detail)
	report.State = state
	report.NextAction = next
	return *report
}

func checkBuildDockerfile(workspaceDir string, service compose.Service) (CheckStatus, string, error) {
	contextDir, dockerfile, ok := buildPaths(service.Build)
	if !ok {
		return CheckSkip, "service has no build", nil
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
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("%s not found for service build: is a directory", dockerfile)
	}
	return CheckPass, "build Dockerfile exists", nil
}

func workspaceApplyAction(workspaceID string) string {
	return fmt.Sprintf("agw workspace apply %s <generated-dir>", workspaceID)
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

func hasVolumeMount(volumes []string, source, target string) bool {
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) >= 2 && parts[0] == source && parts[1] == target {
			return true
		}
	}
	return false
}

func selectedNetworks(def workspace.Definition) []workspace.NetworkAttachment {
	if def.Networks == nil {
		return nil
	}
	return def.Networks.Attach
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
