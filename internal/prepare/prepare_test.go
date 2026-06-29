package prepare

import (
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/scanner"
	"github.com/kenfdev/agw/internal/workspace"
)

func TestRenderPromptContainsConstraintsAndFiles(t *testing.T) {
	out, err := Render(Input{
		Definition: workspace.Definition{
			ID: "agw", Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
			Projects: []workspace.Project{{Name: "agw", Path: "/src/agw", MountPath: "/workspace"}},
		},
		Projects: []scanner.ProjectSnapshot{{
			Project: workspace.Project{Name: "agw", Path: "/src/agw", MountPath: "/workspace"},
			Files:   []scanner.FileSnapshot{{Path: "go.mod", Content: "module example.com/agw"}},
		}},
		NetworkCandidates: []string{"acme_default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sidecar", "Do not edit target project files", "go.mod", "acme_default"} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt missing %q:\n%s", want, out)
		}
	}
}
