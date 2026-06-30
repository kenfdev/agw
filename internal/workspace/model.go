package workspace

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	ID              string          `yaml:"id"`
	Name            string          `yaml:"name"`
	Workspace       Workspace       `yaml:"workspace"`
	Storage         Storage         `yaml:"storage,omitempty"`
	Container       Container       `yaml:"container"`
	BaseEnvironment BaseEnvironment `yaml:"baseEnvironment,omitempty"`
	Projects        []Project       `yaml:"projects"`
	Networks        *Networks       `yaml:"networks,omitempty"`
}

type Workspace struct {
	Dir string `yaml:"dir"`
}

type Storage struct {
	Path string `yaml:"path,omitempty"`
}

type Container struct {
	Service       string `yaml:"service"`
	Workdir       string `yaml:"workdir"`
	WorkspaceRoot string `yaml:"workspaceRoot,omitempty"`
}

type BaseEnvironment struct {
	IncludeGlobal *bool  `yaml:"includeGlobal,omitempty"`
	GuidancePath  string `yaml:"guidancePath,omitempty"`
}

func (d Definition) IncludeGlobalBaseEnvironment() bool {
	return d.BaseEnvironment.IncludeGlobal == nil || *d.BaseEnvironment.IncludeGlobal
}

type Project struct {
	Name          string `yaml:"name"`
	HostPath      string `yaml:"hostPath"`
	ContainerPath string `yaml:"containerPath"`
	Path          string `yaml:"path,omitempty"`
	MountPath     string `yaml:"mountPath,omitempty"`
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
	return normalizeDefinition(def), nil
}

func SaveDefinition(path string, def Definition) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	def = normalizeDefinition(def)
	clearLegacyFields(&def)
	b, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func normalizeDefinition(def Definition) Definition {
	if def.Workspace.Dir == "" {
		def.Workspace.Dir = def.Storage.Path
	}
	if def.Container.Workdir == "" {
		def.Container.Workdir = def.Container.WorkspaceRoot
	}
	for i := range def.Projects {
		if def.Projects[i].HostPath == "" {
			def.Projects[i].HostPath = def.Projects[i].Path
		}
		if def.Projects[i].ContainerPath == "" {
			def.Projects[i].ContainerPath = def.Projects[i].MountPath
		}
	}
	return def
}

func clearLegacyFields(def *Definition) {
	def.Storage = Storage{}
	def.Container.WorkspaceRoot = ""
	for i := range def.Projects {
		def.Projects[i].Path = ""
		def.Projects[i].MountPath = ""
	}
}
