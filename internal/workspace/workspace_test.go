package workspace

import (
	"path/filepath"
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

func TestLoadSaveDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agw.yaml")
	want := Definition{
		ID:      "agw",
		Name:    "AGW",
		Storage: Storage{Path: "workspaces/github.com/kenfdev/agw"},
		Container: Container{
			Service:       "dev",
			WorkspaceRoot: "/workspace",
		},
		Projects: []Project{{Name: "agw", Path: "/src/agw", MountPath: "/workspace"}},
	}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agw" || got.Projects[0].MountPath != "/workspace" {
		t.Fatalf("definition = %#v", got)
	}
}

func TestRegistryFindAndList(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "a", "b", "agw.yaml")
	def := Definition{
		ID: "target",
		Projects: []Project{
			{Name: "app", Path: "/src", MountPath: "/workspace"},
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
