package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/base"
	"github.com/kenfdev/agw/internal/compose"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/kenfdev/agw/internal/workspacecheck"
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
	WorkspaceID     string       `json:"workspaceId"`
	State           State        `json:"state"`
	Checks          []Check      `json:"checks"`
	NextAction      string       `json:"nextAction"`
	BaseEnvironment *base.Status `json:"baseEnvironment,omitempty"`
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
	composePath := filepath.Join(dir, "compose.yaml")
	if _, err := statFile(composePath); err != nil {
		if os.IsNotExist(err) {
			return fail(&report, "compose.yaml", "compose.yaml missing", workspacePrepareAction(report.WorkspaceID), StateNeedsPrepare)
		}
		return fail(&report, "compose.yaml", err.Error(), fmt.Sprintf("fix generated workspace files for %s", report.WorkspaceID), StateBroken)
	}
	report.add("compose.yaml", CheckPass, composePath)

	composeFile, err := compose.Load(composePath)
	if err != nil {
		return fail(&report, "compose parse", err.Error(), "fix compose.yaml", StateBroken)
	}
	for _, check := range workspacecheck.ValidateCompose(dir, located.Definition, composeFile, runner) {
		detail := doctorCheckDetail(check)
		if check.Status == workspacecheck.StatusFail {
			next, state := nextActionForValidationFailure(report.WorkspaceID, check, detail)
			return fail(&report, check.Name, detail, next, state)
		}
		report.add(check.Name, doctorCheckStatus(check.Status), detail)
	}

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

func workspaceApplyAction(workspaceID string) string {
	return fmt.Sprintf("agw workspace apply %s <generated-dir>", workspaceID)
}

func workspacePrepareAction(workspaceID string) string {
	return fmt.Sprintf("agw workspace prepare %s --agent-json", workspaceID)
}

func doctorCheckStatus(status workspacecheck.Status) CheckStatus {
	switch status {
	case workspacecheck.StatusPass:
		return CheckPass
	case workspacecheck.StatusFail:
		return CheckFail
	case workspacecheck.StatusWarn:
		return CheckWarn
	case workspacecheck.StatusSkip:
		return CheckSkip
	default:
		return CheckSkip
	}
}

func doctorCheckDetail(check workspacecheck.Check) string {
	if check.Code == workspacecheck.CodeComposeConfig {
		return strings.TrimPrefix(check.Detail, "docker compose config: ")
	}
	return check.Detail
}

func nextActionForValidationFailure(workspaceID string, check workspacecheck.Check, detail string) (string, State) {
	switch check.Code {
	case workspacecheck.CodeService, workspacecheck.CodeProjectMount:
		return "run agw workspace apply", StateBroken
	case workspacecheck.CodeDockerfile:
		if strings.Contains(detail, "not found for service build") {
			return workspaceApplyAction(workspaceID), StateNeedsApply
		}
		return "fix compose.yaml", StateBroken
	case workspacecheck.CodeExternalNetwork:
		if detail == "selected network name must not be blank" {
			return "fix workspace definition", StateBroken
		}
		if strings.HasPrefix(detail, "external network ") && strings.HasSuffix(detail, " not found") {
			return fmt.Sprintf("start the base project services, then run agw start %s", workspaceID), StateBroken
		}
		return "run agw workspace apply", StateBroken
	case workspacecheck.CodeComposeConfig:
		return "fix compose.yaml", StateBroken
	default:
		return "fix compose.yaml", StateBroken
	}
}
