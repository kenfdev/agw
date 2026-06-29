package compose

import (
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Services map[string]Service `yaml:"services"`
	Networks map[string]Network `yaml:"networks"`
}

type Service struct {
	Build   any
	Volumes []string
}

type Network struct {
	External bool
	Name     string
}

func Load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var file File
	if err := yaml.Unmarshal(b, &file); err != nil {
		return File{}, err
	}
	return file, nil
}

func (s *Service) UnmarshalYAML(value *yaml.Node) error {
	type rawService struct {
		Build   any         `yaml:"build"`
		Volumes []volumeRef `yaml:"volumes"`
	}
	var raw rawService
	if err := value.Decode(&raw); err != nil {
		return err
	}
	s.Build = raw.Build
	s.Volumes = make([]string, 0, len(raw.Volumes))
	for _, volume := range raw.Volumes {
		s.Volumes = append(s.Volumes, volume.String())
	}
	return nil
}

type volumeRef struct {
	value string
}

func (v *volumeRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		v.value = value.Value
		return nil
	}

	var raw struct {
		Source string `yaml:"source"`
		Target string `yaml:"target"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	if raw.Source == "" || raw.Target == "" {
		v.value = ""
		return nil
	}
	v.value = raw.Source + ":" + raw.Target
	return nil
}

func (v volumeRef) String() string {
	return v.value
}

func (n *Network) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		External externalRef `yaml:"external"`
		Name     string      `yaml:"name"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	n.External = raw.External.Enabled
	n.Name = raw.Name
	if n.Name == "" {
		n.Name = raw.External.Name
	}
	return nil
}

type externalRef struct {
	Enabled bool
	Name    string
}

func (e *externalRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var enabled bool
		if err := value.Decode(&enabled); err != nil {
			return err
		}
		e.Enabled = enabled
		return nil
	}

	var raw struct {
		Name string `yaml:"name"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	e.Enabled = true
	e.Name = raw.Name
	return nil
}
