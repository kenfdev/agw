package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorCommandPrintsWorkspaceState(t *testing.T) {
	root, configPath := createDoctorFixture(t, "agw")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "agw", "--config", configPath})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Workspace: agw", "State:", "Checks:", "Next:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
	}
	_ = root
}

func TestDoctorAllPrintsMultipleWorkspaces(t *testing.T) {
	_, configPath := createDoctorFixture(t, "agw", "api")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "--all", "--config", configPath})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Workspace: agw") || !strings.Contains(got, "Workspace: api") {
		t.Fatalf("doctor --all output:\n%s", got)
	}
}

func TestDoctorCommandPrintsJSONReport(t *testing.T) {
	_, configPath := createDoctorFixture(t, "agw")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "agw", "--config", configPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var got struct {
		WorkspaceID string `json:"workspaceId"`
		State       string `json:"state"`
		Checks      []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
		NextAction string `json:"nextAction"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("doctor JSON is invalid: %v\n%s", err, out.String())
	}
	if got.WorkspaceID != "agw" {
		t.Fatalf("workspaceId = %q, want agw", got.WorkspaceID)
	}
	if got.State != "needs-prepare" {
		t.Fatalf("state = %q, want needs-prepare", got.State)
	}
	if got.NextAction == "" {
		t.Fatal("nextAction is empty")
	}
	if len(got.Checks) == 0 || got.Checks[0].Name != "workspace definition" || got.Checks[0].Status != "pass" {
		t.Fatalf("checks = %#v", got.Checks)
	}
}

func TestDoctorJSONDoesNotIncludeLifecycleCommandOutput(t *testing.T) {
	root, configPath := createDoctorFixture(t, "agw")
	workspaceDir := filepath.Join(root, "workspaces", "agw")
	mustWriteCLI(t, filepath.Join(workspaceDir, "prompt.md"), "prompt")
	mustWriteCLI(t, filepath.Join(workspaceDir, "Dockerfile"), "FROM alpine\n")
	mustWriteCLI(t, filepath.Join(workspaceDir, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - "+filepath.Join(root, "projects", "agw")+":/workspace\n")

	oldRunner := newLifecycleRunner
	newLifecycleRunner = func(stdout, stderr io.Writer) lifecycleRunner {
		return noisyDoctorRunner{out: stdout}
	}
	defer func() { newLifecycleRunner = oldRunner }()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "agw", "--config", configPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "compose noise") {
		t.Fatalf("JSON stdout includes lifecycle command output:\n%s", out.String())
	}
	var got struct {
		WorkspaceID string `json:"workspaceId"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("doctor JSON is invalid: %v\n%s", err, out.String())
	}
	if got.WorkspaceID != "agw" {
		t.Fatalf("workspaceId = %q, want agw", got.WorkspaceID)
	}
}

func TestDoctorCommandSuggestsMatchingWorkspaceIDs(t *testing.T) {
	_, configPath := createDoctorFixture(t, "agw-api", "agw-web", "other")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "agw", "--config", configPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "workspace not found") {
		t.Fatalf("error = %q", err)
	}
	if !strings.Contains(err.Error(), "did you mean: agw-api, agw-web") {
		t.Fatalf("error = %q", err)
	}
}

type noisyDoctorRunner struct {
	out io.Writer
}

func (r noisyDoctorRunner) Build(string) error                  { return nil }
func (r noisyDoctorRunner) Up(string) error                     { return nil }
func (r noisyDoctorRunner) UpDetached(string) error             { return nil }
func (r noisyDoctorRunner) Down(string) error                   { return nil }
func (r noisyDoctorRunner) Stop(string) error                   { return nil }
func (r noisyDoctorRunner) Logs(string, string) (string, error) { return "", nil }
func (r noisyDoctorRunner) Attach(string, string) error         { return nil }
func (r noisyDoctorRunner) NetworkExists(string) (bool, error)  { return true, nil }
func (r noisyDoctorRunner) ServiceRunning(string, string) (bool, error) {
	return false, nil
}

func (r noisyDoctorRunner) ComposeConfig(string) error {
	_, err := fmt.Fprintln(r.out, "compose noise")
	return err
}

func createDoctorFixture(t *testing.T, ids ...string) (string, string) {
	t.Helper()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	runCommand(t, []string{"config", "init", "--config", configPath, "--root", root})
	for _, id := range ids {
		project := filepath.Join(root, "projects", id)
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		runCommand(t, []string{"workspace", "new", "--root", root, "--id", id, "--name", id, "--workspace-dir", "workspaces/" + id, "--project", id + "=" + project + ":/workspace", "--service", "dev", "--workdir", "/workspace"})
	}
	return root, configPath
}

func runCommand(t *testing.T, args []string) {
	t.Helper()
	cmd := NewRootCommand("test")
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
