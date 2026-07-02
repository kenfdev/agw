package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/config"
)

func TestSuggestStoragePathUsesMapping(t *testing.T) {
	got, ok := SuggestStoragePath("/Users/me/ghq/github.com/kenfdev/agw", []config.PathMapping{{
		SourceRoot:      "/Users/me/ghq",
		WorkspacePrefix: "workspaces",
	}})
	if !ok {
		t.Fatal("expected suggestion")
	}
	if got != filepath.Join("workspaces", "github.com", "kenfdev", "agw") {
		t.Fatalf("suggestion = %q", got)
	}
}

func TestSuggestStoragePathAllowsRelativePathStartingWithDotDotText(t *testing.T) {
	got, ok := SuggestStoragePath("/Users/me/ghq/..foo", []config.PathMapping{{
		SourceRoot:      "/Users/me/ghq",
		WorkspacePrefix: "workspaces",
	}})
	if !ok {
		t.Fatal("expected suggestion")
	}
	if got != filepath.Join("workspaces", "..foo") {
		t.Fatalf("suggestion = %q", got)
	}
}

func TestSuggestStoragePathRejectsEscapingRelativePath(t *testing.T) {
	_, ok := SuggestStoragePath("/Users/me/other", []config.PathMapping{{
		SourceRoot:      "/Users/me/ghq",
		WorkspacePrefix: "workspaces",
	}})
	if ok {
		t.Fatal("expected no suggestion for path outside source root")
	}
}

func TestLoadSaveDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	want := Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: Workspace{Dir: "workspaces/github.com/kenfdev/agw"},
		Container: Container{
			Service: "dev",
			Workdir: "/workspace",
		},
		Projects: []Project{{Name: "agw", HostPath: "/src/agw", ContainerPath: "/workspace"}},
	}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotYAML := string(b)
	for _, want := range []string{"workspace:\n    dir:", "workdir:", "hostPath:", "containerPath:"} {
		if !strings.Contains(gotYAML, want) {
			t.Fatalf("saved YAML missing %q:\n%s", want, gotYAML)
		}
	}
	for _, legacy := range []string{"storage:", "workspaceRoot:", "mountPath:"} {
		if strings.Contains(gotYAML, legacy) {
			t.Fatalf("saved YAML contains legacy field %q:\n%s", legacy, gotYAML)
		}
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agw" || got.Workspace.Dir != "workspaces/github.com/kenfdev/agw" || got.Container.Workdir != "/workspace" || got.Projects[0].ContainerPath != "/workspace" {
		t.Fatalf("definition = %#v", got)
	}
}

func TestLoadDefinitionAcceptsLegacyFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	legacy := []byte(`id: agw
name: AGW
storage:
    path: workspaces/github.com/kenfdev/agw
container:
    service: dev
    workspaceRoot: /workspace
projects:
    - name: agw
      path: /src/agw
      mountPath: /workspace
`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Dir != "workspaces/github.com/kenfdev/agw" {
		t.Fatalf("Workspace.Dir = %q", got.Workspace.Dir)
	}
	if got.Container.Workdir != "/workspace" {
		t.Fatalf("Container.Workdir = %q", got.Container.Workdir)
	}
	if len(got.Projects) != 1 || got.Projects[0].HostPath != "/src/agw" || got.Projects[0].ContainerPath != "/workspace" {
		t.Fatalf("Projects = %#v", got.Projects)
	}
}

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

func TestLoadSaveDefinitionPreservesLifecycleStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	want := Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: Workspace{Dir: "workspaces/agw"},
		Container: Container{Service: "dev", Workdir: "/workspace"},
		Lifecycle: Lifecycle{
			Start: "op run --env-file=.env.1password -- docker compose up -d",
		},
	}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotYAML := string(b)
	if !strings.Contains(gotYAML, "lifecycle:") || !strings.Contains(gotYAML, "start: op run --env-file=.env.1password -- docker compose up -d") {
		t.Fatalf("saved YAML missing lifecycle start:\n%s", gotYAML)
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle.Start != want.Lifecycle.Start {
		t.Fatalf("Lifecycle.Start = %q, want %q", got.Lifecycle.Start, want.Lifecycle.Start)
	}
}

func TestLoadSaveDefinitionPreservesProjectLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	want := Definition{
		ID:        "agw",
		Name:      "AGW",
		Workspace: Workspace{Dir: "workspaces/agw"},
		Container: Container{Service: "dev", Workdir: "/workspace"},
		Projects: []Project{
			{
				Name:          "api",
				HostPath:      "/src/api",
				ContainerPath: "/workspace/api",
				Lifecycle: Lifecycle{
					Start: "docker compose up -d",
					Stop:  "docker compose down",
				},
			},
		},
	}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotYAML := string(b)
	for _, want := range []string{
		"lifecycle:",
		"start: docker compose up -d",
		"stop: docker compose down",
	} {
		if !strings.Contains(gotYAML, want) {
			t.Fatalf("saved YAML missing %q:\n%s", want, gotYAML)
		}
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("Projects count = %d, want 1", len(got.Projects))
	}
	if got.Projects[0].Lifecycle.Start != want.Projects[0].Lifecycle.Start {
		t.Fatalf("Project Lifecycle.Start = %q, want %q", got.Projects[0].Lifecycle.Start, want.Projects[0].Lifecycle.Start)
	}
	if got.Projects[0].Lifecycle.Stop != want.Projects[0].Lifecycle.Stop {
		t.Fatalf("Project Lifecycle.Stop = %q, want %q", got.Projects[0].Lifecycle.Stop, want.Projects[0].Lifecycle.Stop)
	}
}

func TestDefinitionDefaultsIncludeGlobalBaseEnvironment(t *testing.T) {
	def := Definition{}
	if !def.IncludeGlobalBaseEnvironment() {
		t.Fatal("IncludeGlobalBaseEnvironment() = false, want true")
	}
}

func TestRegistryFindAndList(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "a", "b", "agw.yaml")
	def := Definition{
		ID: "target",
		Projects: []Project{
			{Name: "app", HostPath: "/src", ContainerPath: "/workspace"},
		},
	}
	if err := SaveDefinition(defPath, def); err != nil {
		t.Fatalf("SaveDefinition() error = %v", err)
	}

	registry := Registry{Roots: []string{root}}
	list, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() count = %d", len(list))
	}
	if list[0].Path != defPath {
		t.Fatalf("List() path = %q", list[0].Path)
	}

	item, err := registry.Find("target")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if item.Path != defPath {
		t.Fatalf("Find() path = %q", item.Path)
	}
}

func TestRegistryFindMissing(t *testing.T) {
	registry := Registry{Roots: []string{t.TempDir()}}
	_, err := registry.Find("missing")
	if err == nil || err.Error() != "workspace not found" {
		t.Fatalf("Find() error = %v", err)
	}
}
