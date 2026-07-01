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
	"github.com/kenfdev/agw/internal/mount"
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
	service, ok := file.Services[def.Container.Service]
	if !ok {
		return fmt.Errorf("service %s not found in compose.yaml", def.Container.Service)
	}
	if err := validateBuildDockerfile(workspaceDir, service); err != nil {
		return err
	}

	for _, project := range def.Projects {
		if !mount.HasVolumeMount(service.Volumes, project.HostPath, project.ContainerPath) {
			required := project.HostPath + ":" + project.ContainerPath
			return fmt.Errorf("missing volume %s for project %s", required, project.Name)
		}
	}

	checkedNetworks := map[string]struct{}{}
	for _, attachment := range selectedNetworks(def) {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			return fmt.Errorf("selected network name must not be blank")
		}
		key, network, ok := findComposeNetwork(file.Networks, name)
		if !ok || !network.External {
			return fmt.Errorf("selected network %s must be declared as external in compose.yaml", name)
		}
		if !serviceHasNetworkAttachment(service, key, network) {
			return fmt.Errorf("service %s must attach to selected network %s", def.Container.Service, name)
		}
		resolvedName := composeNetworkName(key, network)
		if err := validateExternalNetwork(resolvedName, runner); err != nil {
			return err
		}
		checkedNetworks[resolvedName] = struct{}{}
	}

	for key, network := range file.Networks {
		if !network.External {
			continue
		}
		name := network.Name
		if name == "" {
			name = key
		}
		if _, ok := checkedNetworks[name]; ok {
			continue
		}
		if err := validateExternalNetwork(name, runner); err != nil {
			return err
		}
	}

	if err := runner.ComposeConfig(workspaceDir); err != nil {
		return fmt.Errorf("docker compose config: %w", err)
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

func validateBuildDockerfile(workspaceDir string, service compose.Service) error {
	contextDir, dockerfile, ok := buildPaths(service.Build)
	if !ok {
		return nil
	}
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if contextDir == "" {
		contextDir = "."
	}
	path := filepath.Join(workspaceDir, contextDir, dockerfile)
	if err := ensureInside(workspaceDir, path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found for service build: %w", dockerfile, err)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s not found for service build: is a directory", dockerfile)
	}
	return nil
}

func buildPaths(build any) (string, string, bool) {
	switch value := build.(type) {
	case nil:
		return "", "", false
	case string:
		return value, "Dockerfile", true
	case map[string]any:
		return stringMapValue(value, "context"), stringMapValue(value, "dockerfile"), true
	case map[any]any:
		return anyMapValue(value, "context"), anyMapValue(value, "dockerfile"), true
	default:
		return "", "", false
	}
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func anyMapValue(values map[any]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func selectedNetworks(def workspace.Definition) []workspace.NetworkAttachment {
	if def.Networks == nil {
		return nil
	}
	return def.Networks.Attach
}

func findComposeNetwork(networks map[string]compose.Network, selected string) (string, compose.Network, bool) {
	for key, network := range networks {
		name := composeNetworkName(key, network)
		if key == selected || name == selected {
			return key, network, true
		}
	}
	return "", compose.Network{}, false
}

func validateExternalNetwork(name string, runner docker.Runner) error {
	exists, err := runner.NetworkExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("external network %s not found", name)
	}
	return nil
}

func serviceHasNetworkAttachment(service compose.Service, key string, network compose.Network) bool {
	resolvedName := composeNetworkName(key, network)
	for _, attached := range service.Networks {
		if attached == key || attached == resolvedName {
			return true
		}
	}
	return false
}

func composeNetworkName(key string, network compose.Network) string {
	if network.Name != "" {
		return network.Name
	}
	return key
}
