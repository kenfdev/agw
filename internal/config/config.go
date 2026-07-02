package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WorkspaceRoot   string          `yaml:"workspaceRoot,omitempty"`
	WorkspaceRoots  []string        `yaml:"workspaceRoots,omitempty"`
	PathMappings    []PathMapping   `yaml:"pathMappings,omitempty"`
	BaseEnvironment BaseEnvironment `yaml:"baseEnvironment,omitempty"`
}

type BaseEnvironment struct {
	GuidancePath string `yaml:"guidancePath,omitempty"`
	Image        string `yaml:"image,omitempty"`
	Build        Build  `yaml:"build,omitempty"`
}

type Build struct {
	Context    string `yaml:"context,omitempty"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
}

type PathMapping struct {
	SourceRoot      string `yaml:"sourceRoot"`
	WorkspacePrefix string `yaml:"workspacePrefix"`
}

func DefaultPath() (string, error) {
	if path := os.Getenv("AGW_CONFIG"); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agw", "config.yaml"), nil
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.WorkspaceRoot == "" {
		switch len(cfg.WorkspaceRoots) {
		case 0:
		case 1:
			cfg.WorkspaceRoot = cfg.WorkspaceRoots[0]
		default:
			return Config{}, fmt.Errorf("config uses legacy workspaceRoots with multiple entries; choose one and set workspaceRoot")
		}
	}
	cfg.WorkspaceRoot = normalizePath(cfg.WorkspaceRoot)
	if cfg.WorkspaceRoot != "" {
		cfg.WorkspaceRoots = []string{cfg.WorkspaceRoot}
	} else {
		cfg.WorkspaceRoots = nil
	}
	return cfg, nil
}

func normalizePath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	path = os.ExpandEnv(path)
	return filepath.Clean(path)
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if cfg.WorkspaceRoot == "" && len(cfg.WorkspaceRoots) == 1 {
		cfg.WorkspaceRoot = cfg.WorkspaceRoots[0]
	}
	cfg.WorkspaceRoot = normalizePath(cfg.WorkspaceRoot)
	cfg.WorkspaceRoots = nil
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (c Config) Root() string {
	if c.WorkspaceRoot != "" {
		return c.WorkspaceRoot
	}
	if len(c.WorkspaceRoots) == 1 {
		return c.WorkspaceRoots[0]
	}
	return ""
}
