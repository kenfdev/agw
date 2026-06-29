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

	composeSource := filepath.Join(generatedDir, "compose.yaml")
	if _, err := os.Stat(composeSource); err != nil {
		return fmt.Errorf("generated compose.yaml not found: %w", err)
	}

	if err := copyRegularFiles(workspaceDir, generatedDir); err != nil {
		return err
	}

	composeFile, err := compose.Load(filepath.Join(workspaceDir, "compose.yaml"))
	if err != nil {
		return err
	}
	if err := validateCompose(workspaceDir, def, composeFile, runner); err != nil {
		return err
	}
	return nil
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
	service, ok := file.Services[def.Container.Service]
	if !ok {
		return fmt.Errorf("service %s not found in compose.yaml", def.Container.Service)
	}

	for _, project := range def.Projects {
		if !hasVolumeMount(service.Volumes, project.Path, project.MountPath) {
			required := project.Path + ":" + project.MountPath
			return fmt.Errorf("missing volume %s for project %s", required, project.Name)
		}
	}

	for key, network := range file.Networks {
		if !network.External {
			continue
		}
		name := network.Name
		if name == "" {
			name = key
		}
		exists, err := runner.NetworkExists(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("external network %s not found", name)
		}
	}

	if err := runner.ComposeConfig(workspaceDir); err != nil {
		return fmt.Errorf("docker compose config: %w", err)
	}
	return nil
}

func hasVolumeMount(volumes []string, source, target string) bool {
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) >= 2 && parts[0] == source && parts[1] == target {
			return true
		}
	}
	return false
}
