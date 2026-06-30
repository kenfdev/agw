package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestWorkspacePrepareWritesPromptToOutputFile(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	def := workspace.Definition{
		ID:        "agw",
		Workspace: workspace.Workspace{Dir: filepath.Join(root, "storage", "agw")},
		Container: workspace.Container{
			Service: "dev",
			Workdir: "/workspace",
		},
		Projects: []workspace.Project{{Name: "agw", HostPath: wsDir, ContainerPath: "/workspace"}},
		Networks: &workspace.Networks{
			Attach: []workspace.NetworkAttachment{{Name: "acme_default"}},
		},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "go.mod"), []byte("module example.com/agw"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wsDir, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".devcontainer", "devcontainer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env"), []byte("TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".env.example"), []byte("TOKEN="), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, ".hiddenconfig"), []byte("TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(root, "prompt.md")
	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"prepare", "agw", "--config", cfgPath, "--output", outPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, want := range []string{"agw", "go.mod", "acme_default"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"#### `.env`", "#### `.hiddenconfig`"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("forbidden file included in output: %q", forbidden)
		}
	}
	for _, allowed := range []string{"#### `.env.example`", "#### `.devcontainer/devcontainer.json`"} {
		if !strings.Contains(out, allowed) {
			t.Fatalf("allowed hidden file missing in output: %q", allowed)
		}
	}
}

func TestWorkspacePreparePrintsAgentJSON(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "agw")
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/agw"), 0o644); err != nil {
		t.Fatal(err)
	}

	def := workspace.Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: workspace.Workspace{Dir: filepath.Join("workspaces", "agw")},
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		Projects:  []workspace.Project{{Name: "agw", HostPath: projectDir, ContainerPath: "/workspace"}},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "manual"}}},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := NewWorkspaceCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"prepare", "agw", "--config", cfgPath, "--agent-json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var got struct {
		WorkspaceID            string   `json:"workspaceId"`
		WorkspaceDir           string   `json:"workspaceDir"`
		Service                string   `json:"service"`
		Workdir                string   `json:"workdir"`
		Mode                   string   `json:"mode"`
		Prompt                 string   `json:"prompt"`
		ExpectedGeneratedFiles []string `json:"expectedGeneratedFiles"`
		SelectedNetworks       []string `json:"selectedNetworks"`
		NetworkCandidates      []string `json:"networkCandidates"`
		SafetyRules            []string `json:"safetyRules"`
		NextCommands           []string `json:"nextCommands"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("agent JSON is invalid: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if got.WorkspaceID != "agw" {
		t.Fatalf("workspaceId = %q, want agw", got.WorkspaceID)
	}
	if got.WorkspaceDir != wsDir {
		t.Fatalf("workspaceDir = %q, want %q", got.WorkspaceDir, wsDir)
	}
	if got.Service != "dev" || got.Workdir != "/workspace" {
		t.Fatalf("service/workdir = %q/%q", got.Service, got.Workdir)
	}
	if got.Mode != "attached-sidecar" {
		t.Fatalf("mode = %q, want attached-sidecar", got.Mode)
	}
	if !strings.Contains(got.Prompt, "AGW Workspace Preparation: agw") {
		t.Fatalf("prompt missing title:\n%s", got.Prompt)
	}
	if !containsString(got.ExpectedGeneratedFiles, "Dockerfile") || !containsString(got.ExpectedGeneratedFiles, "compose.yaml") {
		t.Fatalf("expectedGeneratedFiles = %#v", got.ExpectedGeneratedFiles)
	}
	if !containsString(got.SelectedNetworks, "manual") || !containsString(got.NetworkCandidates, "manual") {
		t.Fatalf("selected/candidate networks = %#v / %#v", got.SelectedNetworks, got.NetworkCandidates)
	}
	if !containsString(got.SafetyRules, "Do not edit target project files.") {
		t.Fatalf("safetyRules = %#v", got.SafetyRules)
	}
	if !containsString(got.NextCommands, "agw workspace apply agw <generated-dir>") || !containsString(got.NextCommands, "agw doctor agw --json") {
		t.Fatalf("nextCommands = %#v", got.NextCommands)
	}
}

func TestWorkspaceApplyValidatesGeneratedDirectoryBeforeCopying(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "workspaces", "agw")
	genDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	def := workspace.Definition{
		ID: "agw",
		Container: workspace.Container{
			Service: "dev",
		},
	}
	if err := workspace.SaveDefinition(filepath.Join(wsDir, "agw.yaml"), def); err != nil {
		t.Fatal(err)
	}
	mustWriteCLI(t, filepath.Join(genDir, "Dockerfile"), "FROM alpine\n")
	mustWriteCLI(t, filepath.Join(genDir, "compose.yaml"), "services:\n  dev:\n    build: .\n")

	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	var composeDir string
	oldRunner := newDockerRunner
	newDockerRunner = func() docker.Runner {
		return cliFakeRunner{composeDir: &composeDir}
	}
	defer func() { newDockerRunner = oldRunner }()

	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"apply", "agw", genDir, "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if composeDir != genDir {
		t.Fatalf("ComposeConfig dir = %q, want %q", composeDir, genDir)
	}
	if _, err := os.Stat(filepath.Join(wsDir, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkCandidatesForPreparePrefersComposeNetworks(t *testing.T) {
	def := workspace.Definition{
		Networks: &workspace.Networks{
			Attach: []workspace.NetworkAttachment{
				{Name: "manual"},
			},
		},
	}
	candidates := networkCandidatesForPrepare(def, []docker.Network{
		{Name: "acme_default", Labels: map[string]string{"com.docker.compose.project": "acme"}},
		{Name: "db-net", Labels: map[string]string{"com.example": "keep"}},
		{Name: "manual", Labels: map[string]string{"com.docker.compose.network": "manual"}},
	})

	want := []string{"manual", "acme_default", "db-net"}
	if len(candidates) != len(want) {
		t.Fatalf("candidates len = %d, want %d", len(candidates), len(want))
	}
	for i, wantName := range want {
		if candidates[i] != wantName {
			t.Fatalf("candidate[%d] = %q, want %q", i, candidates[i], wantName)
		}
	}
}

func TestWorkspaceNetworkAddPersistsExternalNetworkSelection(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := workspace.Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: workspace.Workspace{Dir: filepath.Join("workspaces", "agw")},
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
	}
	defPath := filepath.Join(wsDir, "agw.yaml")
	if err := workspace.SaveDefinition(defPath, def); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"network", "add", "agw", "api_default", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	loaded, err := workspace.LoadDefinition(defPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Networks == nil || len(loaded.Networks.Attach) != 1 {
		t.Fatalf("Networks = %#v", loaded.Networks)
	}
	if loaded.Networks.Attach[0].Name != "api_default" {
		t.Fatalf("network = %q", loaded.Networks.Attach[0].Name)
	}
}

func TestWorkspaceNetworkAddDoesNotDuplicateExistingSelection(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, "workspaces", "agw")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := workspace.Definition{
		ID:        "agw",
		Container: workspace.Container{Service: "dev", Workdir: "/workspace"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "api_default"}}},
	}
	defPath := filepath.Join(wsDir, "agw.yaml")
	if err := workspace.SaveDefinition(defPath, def); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := config.Save(cfgPath, config.Config{WorkspaceRoots: []string{root}}); err != nil {
		t.Fatal(err)
	}

	cmd := NewWorkspaceCommand()
	cmd.SetArgs([]string{"network", "add", "agw", "api_default", "--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	loaded, err := workspace.LoadDefinition(defPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Networks == nil || len(loaded.Networks.Attach) != 1 {
		t.Fatalf("Networks = %#v", loaded.Networks)
	}
}

func mustWriteCLI(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type cliFakeRunner struct {
	composeDir *string
}

func (r cliFakeRunner) ComposeConfig(dir string) error {
	*r.composeDir = dir
	return nil
}

func (cliFakeRunner) NetworkExists(string) (bool, error) { return true, nil }
