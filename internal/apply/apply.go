package apply

import (
	"errors"
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

var renameFile = os.Rename

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
	plan, err := planRegularFiles(workspaceDir, generatedDir)
	if err != nil {
		return err
	}
	return executePlan(workspaceDir, plan)
}

func executePlan(workspaceDir string, plan applyPlan) error {
	if len(plan.files) == 0 {
		return nil
	}
	backupDir := filepath.Join(workspaceDir, ".agw", "backups", time.Now().UTC().Format("20060102T150405.000000000Z"))
	tmpRoot := filepath.Join(workspaceDir, ".agw", "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return err
	}
	stagingDir, err := os.MkdirTemp(tmpRoot, "apply-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)

	for i := range plan.files {
		staged := filepath.Join(stagingDir, plan.files[i].rel)
		if err := ensureInside(stagingDir, staged); err != nil {
			return err
		}
		if err := copyFile(plan.files[i].source, staged, plan.files[i].mode); err != nil {
			return err
		}
		plan.files[i].staged = staged
	}

	for i := range plan.files {
		file := &plan.files[i]
		if file.existingRegular {
			file.backup = filepath.Join(backupDir, file.rel)
			if err := ensureInside(workspaceDir, file.backup); err != nil {
				return err
			}
			if err := copyFile(file.dest, file.backup, file.existingMode); err != nil {
				return err
			}
		}
	}

	var installed []applyFile
	for _, file := range plan.files {
		if err := os.MkdirAll(filepath.Dir(file.dest), 0o755); err != nil {
			if rollbackErr := rollbackInstalled(installed); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}
		if err := renameFile(file.staged, file.dest); err != nil {
			if rollbackErr := rollbackInstalled(installed); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			return err
		}
		installed = append(installed, file)
	}
	return nil
}

type applyPlan struct {
	files []applyFile
}

type applyFile struct {
	rel             string
	source          string
	dest            string
	staged          string
	backup          string
	mode            fs.FileMode
	existingRegular bool
	existingMode    fs.FileMode
}

func rollbackInstalled(installed []applyFile) error {
	var errs []error
	for i := len(installed) - 1; i >= 0; i-- {
		file := installed[i]
		if file.existingRegular {
			if err := copyFile(file.backup, file.dest, file.existingMode); err != nil {
				errs = append(errs, fmt.Errorf("restore %s: %w", filepath.ToSlash(file.rel), err))
			}
			continue
		}
		if err := os.Remove(file.dest); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s: %w", filepath.ToSlash(file.rel), err))
		}
	}
	return errors.Join(errs...)
}

func planRegularFiles(workspaceDir, generatedDir string) (applyPlan, error) {
	var plan applyPlan
	err := filepath.WalkDir(generatedDir, func(path string, d fs.DirEntry, err error) error {
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

		file := applyFile{
			rel:    rel,
			source: path,
			dest:   dest,
			mode:   info.Mode().Perm(),
		}
		if existing, err := os.Stat(dest); err == nil && existing.Mode().IsRegular() {
			file.existingRegular = true
			file.existingMode = existing.Mode().Perm()
		} else if err == nil {
			return fmt.Errorf("destination %s exists and is not a regular file", filepath.ToSlash(rel))
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}

		plan.files = append(plan.files, file)
		return nil
	})
	if err != nil {
		return applyPlan{}, err
	}
	return plan, nil
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
