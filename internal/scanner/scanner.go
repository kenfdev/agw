package scanner

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/kenfdev/agw/internal/workspace"
)

type FileSnapshot struct {
	Path    string
	Content string
}

type ProjectSnapshot struct {
	Project workspace.Project
	Files   []FileSnapshot
}

var candidateFiles = []string{
	"Dockerfile", "compose.yaml", "docker-compose.yaml",
	".devcontainer/devcontainer.json",
	"package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json",
	"pyproject.toml", "requirements.txt", "uv.lock", "poetry.lock",
	"go.mod", "go.sum", "Gemfile", "Gemfile.lock", "README.md", ".env.example",
}

func ScanProject(project workspace.Project) (ProjectSnapshot, error) {
	if project.Path == "" {
		return ProjectSnapshot{}, errors.New("project path is empty")
	}
	var files []FileSnapshot
	for _, rel := range candidateFiles {
		full := filepath.Join(project.Path, rel)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return ProjectSnapshot{}, err
		}
		files = append(files, FileSnapshot{Path: filepath.ToSlash(rel), Content: string(b)})
	}
	return ProjectSnapshot{Project: project, Files: files}, nil
}
