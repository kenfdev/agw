package prepare

import (
	"bytes"
	"fmt"

	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/workspace"
)

type Input struct {
	Definition        workspace.Definition
	Projects          []scanner.ProjectSnapshot
	NetworkCandidates []string
	BaseEnvironment   BaseEnvironmentGuidance
}

type BaseEnvironmentGuidance struct {
	Global    string
	Workspace string
}

func Render(input Input) (string, error) {
	var b bytes.Buffer
	def := input.Definition

	fmt.Fprintf(&b, "# AGW Workspace Preparation: %s\n\n", def.ID)
	fmt.Fprintln(&b, "Create sidecar Docker development workspace files for AGW.")
	fmt.Fprintln(&b, "Do not edit target project files. Generate files for the AGW workspace directory only.")
	fmt.Fprintln(&b, "Do not require Docker settings in the target project.")
	fmt.Fprintln(&b, "Generate a standalone compose.yaml when no external networks are selected.")
	fmt.Fprintln(&b, "Use external networks only when selected in the AGW workspace definition.")
	fmt.Fprintln(&b, "When startup must be wrapped by another CLI, set `lifecycle.start` in `agw.yaml` to the exact command AGW should run from the workspace directory.")
	fmt.Fprintln(&b, "When project-owned services should be started or stopped with AGW, set `projects[].lifecycle.start` and `projects[].lifecycle.stop` to exact commands AGW should run from each project host path.")
	fmt.Fprintln(&b, "If important information is missing, ask questions before generating files.")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Service: `%s`\n\n", def.Container.Service)
	fmt.Fprintf(&b, "Container workdir: `%s`\n\n", def.Container.Workdir)

	fmt.Fprintln(&b, "## Projects")
	for _, p := range def.Projects {
		fmt.Fprintf(&b, "- `%s`: host `%s`, container `%s`\n", p.Name, p.HostPath, p.ContainerPath)
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Candidate Docker Networks")
	if len(input.NetworkCandidates) == 0 {
		fmt.Fprintln(&b, "- None detected")
	} else {
		for _, n := range input.NetworkCandidates {
			fmt.Fprintf(&b, "- `%s`\n", n)
		}
	}

	if input.BaseEnvironment.Global != "" || input.BaseEnvironment.Workspace != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "## Base Environment Guidance")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Use this guidance when generating the workspace Dockerfile and Compose files.")
		fmt.Fprintln(&b, "It is not a fixed template. Adapt it to the project's base image, package manager, language runtime, and build constraints.")
		fmt.Fprintln(&b, "Reference snippets are examples, not content to paste blindly.")
		if input.BaseEnvironment.Global != "" {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "### Global Guidance")
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, input.BaseEnvironment.Global)
		}
		if input.BaseEnvironment.Workspace != "" {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "### Workspace Guidance")
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, input.BaseEnvironment.Workspace)
		}
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Required Output")
	fmt.Fprintln(&b, "- `Dockerfile`")
	fmt.Fprintln(&b, "- `compose.yaml` with external networks when selected")
	fmt.Fprintln(&b, "- Optional `.env.example` and `README.md`")

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Project Files")
	for _, project := range input.Projects {
		fmt.Fprintf(&b, "\n### %s\n", project.Project.Name)
		for _, file := range project.Files {
			fmt.Fprintf(&b, "\n#### `%s`\n\n```text\n%s\n```\n", file.Path, file.Content)
		}
	}

	return b.String(), nil
}
