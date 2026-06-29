package tui

import (
	"strings"
	"testing"

	"github.com/kenfdev/agw/internal/workspace"
)

func TestModelInitialViewContainsWorkspace(t *testing.T) {
	model := NewModel([]workspace.LocatedDefinition{{Definition: workspace.Definition{ID: "agw"}}})
	view := model.View()
	if !strings.Contains(view, "agw") {
		t.Fatalf("view = %q", view)
	}
}
