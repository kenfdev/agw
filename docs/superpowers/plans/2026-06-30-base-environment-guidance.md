# Base Environment Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Include reusable natural-language base environment guidance in `agw workspace prepare`, with global defaults and workspace-specific additions.

**Architecture:** Extend the existing YAML models with optional `baseEnvironment` sections, add a small CLI-side resolver that reads configured guidance relative to the config or workspace definition file, then pass the resolved content into `prepare.Render`. Keep generation adaptive: AGW only reads guidance and injects it into the prompt and agent JSON.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, Cobra CLI, existing `go test` focused package tests.

## Global Constraints

- Do not add Dockerfile templating.
- Do not run an LLM inside AGW.
- Do not install tools directly from AGW.
- Do not edit target project repositories.
- Missing configured guidance files must fail `workspace prepare` clearly.
- `includeGlobal` defaults to true when omitted.
- Use `go run ./cmd/agw ...` when checking CLI behavior in this repository.

---

## File Structure

- `internal/config/config.go`: add global `BaseEnvironment` config model.
- `internal/config/config_test.go`: verify global YAML load/save.
- `internal/workspace/model.go`: add workspace `BaseEnvironment` model with defaulted `IncludeGlobal`.
- `internal/workspace/workspace_test.go`: verify workspace YAML load/save and default behavior.
- `internal/prepare/prepare.go`: accept resolved guidance and render the prompt section.
- `internal/prepare/prepare_test.go`: verify prompt output for guidance combinations.
- `internal/cli/workspace.go`: resolve/read guidance during `workspace prepare` and include metadata in agent JSON.
- `internal/cli/workspace_test.go`: verify CLI JSON metadata, prompt injection, missing files, and `includeGlobal: false`.

### Task 1: Add Config And Workspace Models

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/workspace/model.go`
- Modify: `internal/workspace/workspace_test.go`

**Interfaces:**
- Produces: `config.BaseEnvironment{GuidancePath string}`
- Produces: `workspace.BaseEnvironment{IncludeGlobal *bool, GuidancePath string}`
- Produces: `workspace.Definition.IncludeGlobalBaseEnvironment() bool`

- [ ] **Step 1: Write failing config model test**

Add this case to `TestSaveAndLoadConfig` in `internal/config/config_test.go`:

```go
want := Config{
	WorkspaceRoots: []string{"/tmp/agw-root"},
	PathMappings: []PathMapping{{
		SourceRoot:      "/Users/me/ghq",
		WorkspacePrefix: "workspaces",
	}},
	BaseEnvironment: BaseEnvironment{
		GuidancePath: "base-environment.md",
	},
}
```

Then add:

```go
if got.BaseEnvironment.GuidancePath != "base-environment.md" {
	t.Fatalf("BaseEnvironment.GuidancePath = %q", got.BaseEnvironment.GuidancePath)
}
```

- [ ] **Step 2: Run config test to verify it fails**

Run: `go test ./internal/config -run TestSaveAndLoadConfig`

Expected: FAIL with `undefined: BaseEnvironment`.

- [ ] **Step 3: Implement global config model**

In `internal/config/config.go`, add the field and type:

```go
type Config struct {
	WorkspaceRoots  []string        `yaml:"workspaceRoots"`
	PathMappings    []PathMapping   `yaml:"pathMappings,omitempty"`
	BaseEnvironment BaseEnvironment `yaml:"baseEnvironment,omitempty"`
}

type BaseEnvironment struct {
	GuidancePath string `yaml:"guidancePath,omitempty"`
}
```

- [ ] **Step 4: Run config test to verify it passes**

Run: `go test ./internal/config -run TestSaveAndLoadConfig`

Expected: PASS.

- [ ] **Step 5: Write failing workspace model tests**

In `internal/workspace/workspace_test.go`, add:

```go
func TestLoadSaveDefinitionPreservesBaseEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	includeGlobal := false
	want := Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: Workspace{Dir: "workspaces/agw"},
		Container: Container{Service: "dev", Workdir: "/workspace"},
		BaseEnvironment: BaseEnvironment{
			IncludeGlobal: &includeGlobal,
			GuidancePath:  "environment.md",
		},
	}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.BaseEnvironment.IncludeGlobal == nil || *got.BaseEnvironment.IncludeGlobal {
		t.Fatalf("IncludeGlobal = %#v, want false", got.BaseEnvironment.IncludeGlobal)
	}
	if got.BaseEnvironment.GuidancePath != "environment.md" {
		t.Fatalf("GuidancePath = %q", got.BaseEnvironment.GuidancePath)
	}
	if got.IncludeGlobalBaseEnvironment() {
		t.Fatal("IncludeGlobalBaseEnvironment() = true, want false")
	}
}

func TestDefinitionDefaultsIncludeGlobalBaseEnvironment(t *testing.T) {
	def := Definition{}
	if !def.IncludeGlobalBaseEnvironment() {
		t.Fatal("IncludeGlobalBaseEnvironment() = false, want true")
	}
}
```

- [ ] **Step 6: Run workspace model tests to verify they fail**

Run: `go test ./internal/workspace -run 'TestLoadSaveDefinitionPreservesBaseEnvironment|TestDefinitionDefaultsIncludeGlobalBaseEnvironment'`

Expected: FAIL with `undefined: BaseEnvironment`.

- [ ] **Step 7: Implement workspace model**

In `internal/workspace/model.go`, add to `Definition`:

```go
BaseEnvironment BaseEnvironment `yaml:"baseEnvironment,omitempty"`
```

Add the type and helper:

```go
type BaseEnvironment struct {
	IncludeGlobal *bool  `yaml:"includeGlobal,omitempty"`
	GuidancePath  string `yaml:"guidancePath,omitempty"`
}

func (d Definition) IncludeGlobalBaseEnvironment() bool {
	return d.BaseEnvironment.IncludeGlobal == nil || *d.BaseEnvironment.IncludeGlobal
}
```

- [ ] **Step 8: Run model tests**

Run: `go test ./internal/config ./internal/workspace`

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/workspace/model.go internal/workspace/workspace_test.go
git commit -m "feat: add base environment config models"
```

### Task 2: Render Guidance In Prepare Prompts

**Files:**
- Modify: `internal/prepare/prepare.go`
- Modify: `internal/prepare/prepare_test.go`

**Interfaces:**
- Consumes: `prepare.Input`
- Produces: `prepare.BaseEnvironmentGuidance{Global, Workspace string}`
- Produces: `prepare.Render` section `## Base Environment Guidance`

- [ ] **Step 1: Write failing prepare render test**

In `internal/prepare/prepare_test.go`, add:

```go
func TestRenderPromptContainsBaseEnvironmentGuidance(t *testing.T) {
	out, err := Render(Input{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		},
		BaseEnvironment: BaseEnvironmentGuidance{
			Global:    "Prefer my devcontainer-features repository.",
			Workspace: "This workspace needs PostgreSQL client tools.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Base Environment Guidance",
		"not a fixed template",
		"### Global Guidance",
		"Prefer my devcontainer-features repository.",
		"### Workspace Guidance",
		"This workspace needs PostgreSQL client tools.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt missing %q:\n%s", want, out)
		}
	}
}

func TestRenderPromptOmitsBaseEnvironmentSectionWhenEmpty(t *testing.T) {
	out, err := Render(Input{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## Base Environment Guidance") {
		t.Fatalf("unexpected guidance section:\n%s", out)
	}
}
```

- [ ] **Step 2: Run prepare tests to verify failure**

Run: `go test ./internal/prepare -run BaseEnvironment`

Expected: FAIL with `undefined: BaseEnvironmentGuidance`.

- [ ] **Step 3: Implement render input and prompt section**

In `internal/prepare/prepare.go`, add:

```go
type BaseEnvironmentGuidance struct {
	Global    string
	Workspace string
}
```

Add to `Input`:

```go
BaseEnvironment BaseEnvironmentGuidance
```

In `Render`, after the candidate Docker networks section and before `## Required Output`, add:

```go
if def := input.BaseEnvironment; def.Global != "" || def.Workspace != "" {
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Base Environment Guidance")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Use this guidance when generating the workspace Dockerfile and Compose files.")
	fmt.Fprintln(&b, "It is not a fixed template. Adapt it to the project's base image, package manager, language runtime, and build constraints.")
	fmt.Fprintln(&b, "Reference snippets are examples, not content to paste blindly.")
	if def.Global != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "### Global Guidance")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, def.Global)
	}
	if def.Workspace != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "### Workspace Guidance")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, def.Workspace)
	}
}
```

- [ ] **Step 4: Run prepare tests**

Run: `go test ./internal/prepare`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/prepare/prepare.go internal/prepare/prepare_test.go
git commit -m "feat: render base environment guidance"
```

### Task 3: Resolve Guidance In Workspace Prepare CLI

**Files:**
- Modify: `internal/cli/workspace.go`
- Modify: `internal/cli/workspace_test.go`

**Interfaces:**
- Consumes: `config.Config.BaseEnvironment.GuidancePath`
- Consumes: `workspace.Definition.BaseEnvironment`
- Produces: `baseEnvironmentAgentPacket`
- Produces: helper `loadBaseEnvironmentGuidance(configPath string, cfg config.Config, located workspace.LocatedDefinition) (prepare.BaseEnvironmentGuidance, baseEnvironmentAgentPacket, error)`

- [ ] **Step 1: Write failing CLI agent JSON test**

In `internal/cli/workspace_test.go`, extend `TestWorkspacePreparePrintsAgentJSON`:

Write files before saving config:

```go
globalGuidance := filepath.Join(root, "base-environment.md")
if err := os.WriteFile(globalGuidance, []byte("Prefer devcontainer-features."), 0o644); err != nil {
	t.Fatal(err)
}
if err := os.WriteFile(filepath.Join(wsDir, "environment.md"), []byte("Install PostgreSQL client tools."), 0o644); err != nil {
	t.Fatal(err)
}
```

Set workspace definition:

```go
def.BaseEnvironment = workspace.BaseEnvironment{GuidancePath: "environment.md"}
```

Set config:

```go
if err := config.Save(cfgPath, config.Config{
	WorkspaceRoots: []string{root},
	BaseEnvironment: config.BaseEnvironment{GuidancePath: "base-environment.md"},
}); err != nil {
	t.Fatal(err)
}
```

Extend the local `got` struct:

```go
BaseEnvironment struct {
	GlobalGuidancePath    string `json:"globalGuidancePath,omitempty"`
	WorkspaceGuidancePath string `json:"workspaceGuidancePath,omitempty"`
	IncludeGlobal         bool   `json:"includeGlobal"`
} `json:"baseEnvironment"`
```

Add assertions:

```go
if !strings.Contains(got.Prompt, "Prefer devcontainer-features.") || !strings.Contains(got.Prompt, "Install PostgreSQL client tools.") {
	t.Fatalf("prompt missing base environment guidance:\n%s", got.Prompt)
}
if got.BaseEnvironment.GlobalGuidancePath != globalGuidance {
	t.Fatalf("globalGuidancePath = %q, want %q", got.BaseEnvironment.GlobalGuidancePath, globalGuidance)
}
if got.BaseEnvironment.WorkspaceGuidancePath != filepath.Join(wsDir, "environment.md") {
	t.Fatalf("workspaceGuidancePath = %q", got.BaseEnvironment.WorkspaceGuidancePath)
}
if !got.BaseEnvironment.IncludeGlobal {
	t.Fatal("includeGlobal = false, want true")
}
```

- [ ] **Step 2: Write failing missing guidance test**

In `internal/cli/workspace_test.go`, add:

```go
func TestWorkspacePrepareFailsWhenConfiguredGuidanceMissing(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "agw")
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := workspace.Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: workspace.Workspace{Dir: filepath.Join("workspaces", "agw")},
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		Projects:  []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoots: []string{root},
		BaseEnvironment: config.BaseEnvironment{GuidancePath: "missing.md"},
	}); err != nil {
		t.Fatal(err)
	}

	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"prepare", "agw", "--config", cfgPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing guidance error")
	}
	if !strings.Contains(err.Error(), "global base environment guidance") || !strings.Contains(err.Error(), "missing.md") {
		t.Fatalf("error = %v", err)
	}
}
```

- [ ] **Step 3: Write failing includeGlobal false test**

In `internal/cli/workspace_test.go`, add:

```go
func TestWorkspacePrepareIncludeGlobalFalseSkipsGlobalGuidance(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "agw")
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	includeGlobal := false
	if err := os.WriteFile(filepath.Join(wsDir, "environment.md"), []byte("Only workspace guidance."), 0o644); err != nil {
		t.Fatal(err)
	}
	def := workspace.Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: workspace.Workspace{Dir: filepath.Join("workspaces", "agw")},
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		BaseEnvironment: workspace.BaseEnvironment{
			IncludeGlobal: &includeGlobal,
			GuidancePath:  "environment.md",
		},
		Projects: []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{
		WorkspaceRoots: []string{root},
		BaseEnvironment: config.BaseEnvironment{GuidancePath: "missing-global.md"},
	}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewWorkspaceCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"prepare", "agw", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	if strings.Contains(got, "missing-global") {
		t.Fatalf("global guidance leaked into prompt:\n%s", got)
	}
	if !strings.Contains(got, "Only workspace guidance.") {
		t.Fatalf("workspace guidance missing:\n%s", got)
	}
}
```

- [ ] **Step 4: Run CLI tests to verify failure**

Run: `go test ./internal/cli -run 'TestWorkspacePrepare(PrintsAgentJSON|FailsWhenConfiguredGuidanceMissing|IncludeGlobalFalseSkipsGlobalGuidance)'`

Expected: FAIL until the resolver and JSON metadata exist.

- [ ] **Step 5: Implement resolver and metadata**

In `internal/cli/workspace.go`, add:

```go
type baseEnvironmentAgentPacket struct {
	GlobalGuidancePath    string `json:"globalGuidancePath,omitempty"`
	WorkspaceGuidancePath string `json:"workspaceGuidancePath,omitempty"`
	IncludeGlobal         bool   `json:"includeGlobal"`
}
```

Add to `prepareAgentPacket`:

```go
BaseEnvironment baseEnvironmentAgentPacket `json:"baseEnvironment"`
```

Add helpers:

```go
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
```

In `newWorkspacePrepareCommand`, after `located` is found and before `prepare.Render`, call:

```go
baseGuidance, baseEnvironmentMeta, err := loadBaseEnvironmentGuidance(path, cfg, located)
if err != nil {
	return err
}
```

Pass `BaseEnvironment: baseGuidance` into `prepare.Render`.

Change `newPrepareAgentPacket` signature to accept metadata:

```go
func newPrepareAgentPacket(located workspace.LocatedDefinition, prompt, promptPath string, candidates []string, baseEnvironment baseEnvironmentAgentPacket) prepareAgentPacket
```

Set:

```go
BaseEnvironment: baseEnvironment,
```

Update the call site.

- [ ] **Step 6: Run focused CLI tests**

Run: `go test ./internal/cli -run 'TestWorkspacePrepare(PrintsAgentJSON|FailsWhenConfiguredGuidanceMissing|IncludeGlobalFalseSkipsGlobalGuidance)'`

Expected: PASS.

- [ ] **Step 7: Run all focused package tests**

Run: `go test ./internal/config ./internal/workspace ./internal/prepare ./internal/cli`

Expected: PASS.

- [ ] **Step 8: Verify CLI manually with repository command**

Run: `go run ./cmd/agw workspace prepare --help`

Expected: command succeeds and still shows existing prepare flags.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/workspace.go internal/cli/workspace_test.go
git commit -m "feat: include base environment guidance in prepare"
```

### Task 4: Documentation And Final Verification

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: implemented config and workspace YAML fields.
- Produces: user-facing example for global and workspace guidance.

- [ ] **Step 1: Add README section**

Add after the config section in `README.md`:

```md
## Base environment guidance

AGW can include personal development-environment guidance in workspace
preparation prompts. This is natural-language guidance for the agent that
generates workspace files, not a shared Dockerfile template.

Global guidance in `config.yaml`:

```yaml
baseEnvironment:
  guidancePath: /Users/me/.config/agw/base-environment.md
```

Workspace-specific guidance in `agw.yaml`:

```yaml
baseEnvironment:
  includeGlobal: true
  guidancePath: environment.md
```

Set `includeGlobal: false` for a workspace that should ignore the global
guidance. Relative global paths are resolved from the config file directory.
Relative workspace paths are resolved from the workspace `agw.yaml` directory.
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run CLI smoke check**

Run: `go run ./cmd/agw workspace prepare --help`

Expected: PASS, no panic, help text prints.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document base environment guidance"
```
