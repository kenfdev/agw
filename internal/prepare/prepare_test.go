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

func TestRenderPromptNoNetworkCandidates(t *testing.T) {
	out, err := Render(Input{
		Definition: workspace.Definition{
			ID: "agw", Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
		},
		Projects:          nil,
		NetworkCandidates: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"None detected", "Generate a standalone compose.yaml", "Do not require Docker settings in the target project"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected no candidates message %q, got:\n%s", want, out)
		}
	}
}

func TestRenderPromptTreatsExternalNetworksAsSelectedOnly(t *testing.T) {
	out, err := Render(Input{
		Definition: workspace.Definition{
			ID:        "agw",
			Container: workspace.Container{Service: "dev", WorkspaceRoot: "/workspace"},
			Networks:  &workspace.Networks{Attach: []workspace.NetworkAttachment{{Name: "api_default"}}},
		},
		NetworkCandidates: []string{"api_default", "other_default"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Use external networks only when selected in the AGW workspace definition") {
		t.Fatalf("expected no candidates message, got:\n%s", out)
	}
}
