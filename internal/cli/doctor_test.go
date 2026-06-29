package cli

import (
	"bytes"
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
		runCommand(t, []string{"workspace", "new", "--root", root, "--id", id, "--name", id, "--storage", "workspaces/" + id, "--project", id + "=" + project + ":/workspace", "--service", "dev", "--workspace-root", "/workspace"})
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
