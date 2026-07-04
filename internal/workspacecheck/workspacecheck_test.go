package workspacecheck

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/compose"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestValidateComposeReportsMissingService(t *testing.T) {
	ws := t.TempDir()
	file := compose.File{
		Services: map[string]compose.Service{"web": {}},
	}
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	checks := ValidateCompose(ws, def, file, fakeRunner{})

	check := findCheck(t, checks, CodeService)
	if check.Status != StatusFail {
		t.Fatalf("service status = %q, want %q", check.Status, StatusFail)
	}
	if check.Detail != "service dev not found in compose.yaml" {
		t.Fatalf("service detail = %q", check.Detail)
	}
}

func TestValidateComposeReportsMissingBuildDockerfile(t *testing.T) {
	ws := t.TempDir()
	file := compose.File{
		Services: map[string]compose.Service{
			"dev": {Build: "."},
		},
	}
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	checks := ValidateCompose(ws, def, file, fakeRunner{})

	check := findCheck(t, checks, CodeDockerfile)
	if check.Status != StatusFail {
		t.Fatalf("Dockerfile status = %q, want %q", check.Status, StatusFail)
	}
	if !strings.Contains(check.Detail, "Dockerfile not found") {
		t.Fatalf("Dockerfile detail = %q", check.Detail)
	}
}

func TestValidateComposeReportsMissingProjectMount(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	file := compose.File{
		Services: map[string]compose.Service{
			"dev": {Build: ".", Volumes: []string{"/other:/workspace"}},
		},
	}
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Projects:  []workspace.Project{{Name: "agw", HostPath: "/src/agw", ContainerPath: "/workspace"}},
	}

	checks := ValidateCompose(ws, def, file, fakeRunner{})

	check := findCheck(t, checks, CodeProjectMount)
	if check.Status != StatusFail {
		t.Fatalf("project mount status = %q, want %q", check.Status, StatusFail)
	}
	if check.Detail != "missing volume /src/agw:/workspace for project agw" {
		t.Fatalf("project mount detail = %q", check.Detail)
	}
}

func TestValidateComposeReportsMissingExternalNetwork(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	file := compose.File{
		Services: map[string]compose.Service{
			"dev": {Build: ".", Volumes: []string{"/src/agw:/workspace"}, Networks: []string{"app"}},
		},
		Networks: map[string]compose.Network{
			"app": {External: true, Name: "acme_default"},
		},
	}
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Projects:  []workspace.Project{{Name: "agw", HostPath: "/src/agw", ContainerPath: "/workspace"}},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}

	checks := ValidateCompose(ws, def, file, fakeRunner{missingNetworks: map[string]bool{"acme_default": true}})

	check := findCheck(t, checks, CodeExternalNetwork)
	if check.Status != StatusFail {
		t.Fatalf("external network status = %q, want %q", check.Status, StatusFail)
	}
	if check.Detail != "external network acme_default not found" {
		t.Fatalf("external network detail = %q", check.Detail)
	}
}

func TestValidateComposeReportsComposeConfigFailure(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "Dockerfile"), "FROM alpine\n")
	file := compose.File{
		Services: map[string]compose.Service{
			"dev": {Build: "."},
		},
	}
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	checks := ValidateCompose(ws, def, file, fakeRunner{composeErr: errors.New("invalid compose")})

	check := findCheck(t, checks, CodeComposeConfig)
	if check.Status != StatusFail {
		t.Fatalf("compose config status = %q, want %q", check.Status, StatusFail)
	}
	if check.Detail != "docker compose config: invalid compose" {
		t.Fatalf("compose config detail = %q", check.Detail)
	}
}

type fakeRunner struct {
	composeErr      error
	missingNetworks map[string]bool
}

func (r fakeRunner) ComposeConfig(string) error {
	return r.composeErr
}

func (r fakeRunner) NetworkExists(name string) (bool, error) {
	return !r.missingNetworks[name], nil
}

func findCheck(t *testing.T, checks []Check, code Code) Check {
	t.Helper()
	for _, check := range checks {
		if check.Code == code {
			return check
		}
	}
	t.Fatalf("missing check %q in %#v", code, checks)
	return Check{}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
