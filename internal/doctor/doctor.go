package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kenfdev/agw/internal/workspace"
)

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
	Name   string
	Status CheckStatus
	Detail string
}

type Report struct {
	WorkspaceID string
	State       State
	Checks      []Check
	NextAction  string
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
		info, err := os.Stat(project.Path)
		if err != nil || !info.IsDir() {
			report.add("project path", CheckFail, project.Path)
			report.State = StateBroken
			report.NextAction = fmt.Sprintf("fix project path for %s", project.Name)
			return report
		}
		report.add("project path", CheckPass, project.Path)
	}
	promptPath := filepath.Join(dir, "prompt.md")
	if _, err := os.Stat(promptPath); err != nil {
		report.add("prompt", CheckFail, "prompt.md missing")
		report.State = StateNeedsPrepare
		report.NextAction = fmt.Sprintf("agw workspace prepare %s --output %s", report.WorkspaceID, promptPath)
		return report
	}
	report.add("prompt", CheckPass, promptPath)
	return report
}

func (r *Report) add(name string, status CheckStatus, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
}
