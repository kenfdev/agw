package workspace

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	ID        string    `yaml:"id"`
	Name      string    `yaml:"name"`
	Storage   Storage   `yaml:"storage"`
	Container Container `yaml:"container"`
	Projects  []Project `yaml:"projects"`
	Networks  *Networks `yaml:"networks,omitempty"`
}

type Storage struct {
	Path string `yaml:"path"`
}

type Container struct {
	Service       string `yaml:"service"`
	WorkspaceRoot string `yaml:"workspaceRoot"`
}

type Project struct {
	Name      string `yaml:"name"`
	Path      string `yaml:"path"`
	MountPath string `yaml:"mountPath"`
}

type Networks struct {
	Attach []NetworkAttachment `yaml:"attach,omitempty"`
}

type NetworkAttachment struct {
	Name    string   `yaml:"name"`
	Aliases []string `yaml:"aliases,omitempty"`
}

func LoadDefinition(path string) (Definition, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	var def Definition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func SaveDefinition(path string, def Definition) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
