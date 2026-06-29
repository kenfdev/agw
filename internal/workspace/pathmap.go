package workspace

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/config"
)

func SuggestStoragePath(projectPath string, mappings []config.PathMapping) (string, bool) {
	cleanProject := filepath.Clean(projectPath)
	for _, m := range mappings {
		root := filepath.Clean(m.SourceRoot)
		rel, err := filepath.Rel(root, cleanProject)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.Join(m.WorkspacePrefix, rel), true
		}
	}
	return "", false
}

type LocatedDefinition struct {
	Definition Definition
	Root       string
	Path       string
}

type Registry struct {
	Roots []string
}

func (r Registry) List() ([]LocatedDefinition, error) {
	var out []LocatedDefinition
	for _, root := range r.Roots {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "agw.yaml" {
				return nil
			}
			def, err := LoadDefinition(path)
			if err != nil {
				return err
			}
			out = append(out, LocatedDefinition{Definition: def, Root: root, Path: path})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r Registry) Find(id string) (LocatedDefinition, error) {
	items, err := r.List()
	if err != nil {
		return LocatedDefinition{}, err
	}
	for _, item := range items {
		if item.Definition.ID == id {
			return item, nil
		}
	}
	return LocatedDefinition{}, errors.New("workspace not found")
}
