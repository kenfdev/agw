package apply

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kenfdev/agw/internal/compose"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/kenfdev/agw/internal/workspacecheck"
)

func Apply(workspaceDir string, def workspace.Definition, generatedDir string, runner docker.Runner) error {
	workspaceDir, err := filepath.Abs(filepath.Clean(workspaceDir))
	if err != nil {
		return err
	}
	generatedDir, err = filepath.Abs(filepath.Clean(generatedDir))
	if err != nil {
		return err
	}
	if pathsOverlap(workspaceDir, generatedDir) {
		return fmt.Errorf("generated directory must not overlap workspace directory")
	}

	composeSource := filepath.Join(generatedDir, "compose.yaml")
	if _, err := os.Stat(composeSource); err != nil {
		return fmt.Errorf("generated compose.yaml not found: %w", err)
	}

	composeFile, err := compose.Load(composeSource)
	if err != nil {
		return err
	}
	if err := validateCompose(generatedDir, def, composeFile, runner); err != nil {
		return err
	}

	return copyRegularFiles(workspaceDir, generatedDir)
}

func copyRegularFiles(workspaceDir, generatedDir string) error {
	backupDir := filepath.Join(workspaceDir, ".agw", "backups", time.Now().UTC().Format("20060102T150405.000000000Z"))

	return filepath.WalkDir(generatedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(generatedDir, path)
		if err != nil {
			return err
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("generated file escapes generated directory: %s", path)
		}

		dest := filepath.Join(workspaceDir, rel)
		if err := ensureInside(workspaceDir, dest); err != nil {
			return err
		}

		if existing, err := os.Stat(dest); err == nil && existing.Mode().IsRegular() {
			backupPath := filepath.Join(backupDir, rel)
			if err := ensureInside(workspaceDir, backupPath); err != nil {
				return err
			}
			if err := copyFile(dest, backupPath, existing.Mode().Perm()); err != nil {
				return err
			}
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		return copyFile(path, dest, info.Mode().Perm())
	})
}

func ensureInside(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes workspace directory: %s", path)
	}
	return nil
}

func copyFile(src, dest string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func validateCompose(workspaceDir string, def workspace.Definition, file compose.File, runner docker.Runner) error {
	for _, check := range workspacecheck.ValidateCompose(workspaceDir, def, file, runner) {
		if check.Status == workspacecheck.StatusFail || check.Status == workspacecheck.StatusWarn {
			return fmt.Errorf("%s", check.Detail)
		}
	}
	return nil
}

func pathsOverlap(a, b string) bool {
	return pathContains(a, b) || pathContains(b, a)
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
