package apply

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestApplyCopiesAndValidatesCompose(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{HostPath: "/src/agw", ContainerPath: "/workspace"}}}
	if err := Apply(ws, def, gen, &fakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyBacksUpExistingRegularFiles(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "old compose\n")
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{HostPath: "/src/agw", ContainerPath: "/workspace"}}}

	if err := Apply(ws, def, gen, &fakeRunner{}); err != nil {
		t.Fatal(err)
	}

	backupsRoot := filepath.Join(ws, ".agw", "backups")
	entries, err := os.ReadDir(backupsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("backup directories = %d", len(entries))
	}
	b, err := os.ReadFile(filepath.Join(backupsRoot, entries[0].Name(), "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old compose\n" {
		t.Fatalf("backup content = %q", string(b))
	}
}

func TestApplyRejectsMissingGeneratedCompose(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "services:\n  web:\n    build: .\n")
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	def := workspace.Definition{Container: workspace.Container{Service: "web"}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil {
		t.Fatal("expected missing generated compose.yaml to fail")
	}
	if !strings.Contains(err.Error(), "generated compose.yaml not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsSameWorkspaceAndGeneratedDirectory(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "services:\n  dev:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, ws, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "generated directory must not overlap workspace directory") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsNestedWorkspaceAndGeneratedDirectories(t *testing.T) {
	for _, tc := range []struct {
		name         string
		workspaceDir func(root string) string
		generatedDir func(root string) string
	}{
		{
			name:         "generated inside workspace",
			workspaceDir: func(root string) string { return filepath.Join(root, "workspace") },
			generatedDir: func(root string) string { return filepath.Join(root, "workspace", "generated") },
		},
		{
			name:         "workspace inside generated",
			workspaceDir: func(root string) string { return filepath.Join(root, "generated", "workspace") },
			generatedDir: func(root string) string { return filepath.Join(root, "generated") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			ws := tc.workspaceDir(root)
			gen := tc.generatedDir(root)
			mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

			err := Apply(ws, def, gen, &fakeRunner{})
			if err == nil || !strings.Contains(err.Error(), "generated directory must not overlap workspace directory") {
				t.Fatalf("Apply() error = %v", err)
			}
		})
	}
}

func TestApplyRejectsMissingService(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  web:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "service dev not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyValidationFailureLeavesWorkspaceUnchanged(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(ws, "compose.yaml"), "services:\n  dev:\n    image: stable\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  web:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "service dev not found") {
		t.Fatalf("Apply() error = %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ws, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "services:\n  dev:\n    image: stable\n" {
		t.Fatalf("workspace compose was mutated:\n%s", string(b))
	}
}

func TestApplyRejectsMissingProjectMount(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /other:/workspace\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{HostPath: "/src/agw", ContainerPath: "/workspace"}}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "missing volume /src/agw:/workspace") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyAcceptsVolumeOptions(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace:cached\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{HostPath: "/src/agw", ContainerPath: "/workspace"}}}

	if err := Apply(ws, def, gen, &fakeRunner{}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsMissingExternalNetwork(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    volumes:\n      - /src/agw:/workspace\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}, Projects: []workspace.Project{{HostPath: "/src/agw", ContainerPath: "/workspace"}}}

	err := Apply(ws, def, gen, &fakeRunner{missingNetworks: map[string]bool{"acme_default": true}})
	if err == nil || !strings.Contains(err.Error(), "external network acme_default not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRequiresSelectedNetworksToBeDeclaredExternal(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\nnetworks:\n  acme_default:\n    name: acme_default\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "selected network acme_default must be declared as external") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyChecksSelectedExternalNetworksExist(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    networks:\n      - app\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}
	runner := &fakeRunner{missingNetworks: map[string]bool{"acme_default": true}}

	err := Apply(ws, def, gen, runner)
	if err == nil || !strings.Contains(err.Error(), "external network acme_default not found") {
		t.Fatalf("Apply() error = %v", err)
	}
	if !reflect.DeepEqual(runner.checkedNetworks, []string{"acme_default"}) {
		t.Fatalf("checked networks = %#v", runner.checkedNetworks)
	}
}

func TestApplyRejectsBlankSelectedNetworkName(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: ""}}},
	}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "selected network name must not be blank") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRequiresServiceToAttachSelectedNetwork(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "service dev must attach to selected network acme_default") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyAcceptsSelectedNetworkAttachedByServiceListSyntax(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    networks:\n      - app\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}
	runner := &fakeRunner{}

	if err := Apply(ws, def, gen, runner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.checkedNetworks, []string{"acme_default"}) {
		t.Fatalf("checked networks = %#v", runner.checkedNetworks)
	}
}

func TestApplyAcceptsSelectedNetworkAttachedByServiceMappingSyntax(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n    networks:\n      app:\n        aliases:\n          - workspace\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")
	def := workspace.Definition{
		Container: workspace.Container{Service: "dev"},
		Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "acme_default"}}},
	}
	runner := &fakeRunner{}

	if err := Apply(ws, def, gen, runner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.checkedNetworks, []string{"acme_default"}) {
		t.Fatalf("checked networks = %#v", runner.checkedNetworks)
	}
}

func TestApplyRejectsMissingDockerfileForStringBuild(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "Dockerfile not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyRejectsMissingDockerfileForObjectBuild(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build:\n      context: docker\n      dockerfile: Dev.Dockerfile\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, &fakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "Dev.Dockerfile not found") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestApplyReturnsComposeConfigError(t *testing.T) {
	ws := t.TempDir()
	gen := t.TempDir()
	mustWrite(t, filepath.Join(gen, "Dockerfile"), "FROM alpine\n")
	mustWrite(t, filepath.Join(gen, "compose.yaml"), "services:\n  dev:\n    build: .\n")
	def := workspace.Definition{Container: workspace.Container{Service: "dev"}}

	err := Apply(ws, def, gen, &fakeRunner{composeErr: errors.New("bad compose")})
	if err == nil || !strings.Contains(err.Error(), "docker compose config") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeRunner struct {
	composeErr      error
	missingNetworks map[string]bool
	checkedNetworks []string
}

func (r *fakeRunner) ComposeConfig(string) error { return r.composeErr }
func (r *fakeRunner) NetworkExists(name string) (bool, error) {
	r.checkedNetworks = append(r.checkedNetworks, name)
	return !r.missingNetworks[name], nil
}
