package scanner

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kenfdev/agw/internal/workspace"
)

const maxSnapshotBytes = 64 * 1024

type FileSnapshot struct {
	Path          string
	Content       string
	Truncated     bool
	OriginalBytes int64
}

type ProjectSnapshot struct {
	Project workspace.Project
	Files   []FileSnapshot
}

// Allowed project file snapshots by explicit allowlist.
// Hidden files are intentionally excluded unless explicitly listed here.
var hiddenAllowedFiles = map[string]struct{}{
	".env.example":                    {},
	".devcontainer/devcontainer.json": {},
}

var candidateFiles = []string{
	"Dockerfile", "compose.yaml", "docker-compose.yaml",
	"package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json",
	"pyproject.toml", "requirements.txt", "uv.lock", "poetry.lock",
	"go.mod", "go.sum", "Gemfile", "Gemfile.lock", "README.md", ".env.example",
	".devcontainer/devcontainer.json",
}

func ScanProject(project workspace.Project) (ProjectSnapshot, error) {
	if project.HostPath == "" {
		return ProjectSnapshot{}, errors.New("project path is empty")
	}
	info, err := os.Stat(project.HostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ProjectSnapshot{}, errors.New("project path must be an existing directory")
		}
		return ProjectSnapshot{}, err
	}
	if !info.IsDir() {
		return ProjectSnapshot{}, errors.New("project path must be an existing directory")
	}

	var files []FileSnapshot
	for _, rel := range candidateFiles {
		if !isAllowedCandidate(rel) {
			continue
		}
		full := filepath.Join(project.HostPath, rel)
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return ProjectSnapshot{}, err
		}
		if info.IsDir() {
			continue
		}
		content, truncated, err := readSnapshotFile(full, info.Size())
		if err != nil {
			return ProjectSnapshot{}, err
		}
		files = append(files, FileSnapshot{
			Path:          filepath.ToSlash(rel),
			Content:       content,
			Truncated:     truncated,
			OriginalBytes: info.Size(),
		})
	}
	return ProjectSnapshot{Project: project, Files: files}, nil
}

func readSnapshotFile(path string, size int64) (string, bool, error) {
	if size <= maxSnapshotBytes {
		b, err := os.ReadFile(path)
		return string(b), false, err
	}

	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, maxSnapshotBytes))
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

func isAllowedCandidate(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			_, allowed := hiddenAllowedFiles[path]
			return allowed
		}
	}
	return true
}
