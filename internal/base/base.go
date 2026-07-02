package base

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kenfdev/agw/internal/config"
)

type Config struct {
	Image      string
	ContextDir string
	Dockerfile string
}

type ImageStatus string

const (
	StatusUnavailable ImageStatus = ""
	StatusAvailable   ImageStatus = "available"
	StatusMissing     ImageStatus = "missing"
	StatusUnknown     ImageStatus = "unknown"
)

type Status struct {
	Config    Config      `json:"config"`
	Status    ImageStatus `json:"status"`
	CreatedAt *time.Time  `json:"createdAt,omitempty"`
	Age       string      `json:"age,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func Resolve(cfg config.Config) (Config, error) {
	env := cfg.BaseEnvironment
	if strings.TrimSpace(env.Image) == "" {
		return Config{}, fmt.Errorf("baseEnvironment.image is not configured")
	}
	if strings.TrimSpace(env.Build.Context) == "" {
		return Config{}, fmt.Errorf("baseEnvironment.build.context is required when baseEnvironment.image is set")
	}
	if strings.TrimSpace(env.Build.Dockerfile) == "" {
		return Config{}, fmt.Errorf("baseEnvironment.build.dockerfile is required when baseEnvironment.image is set")
	}
	root := cfg.Root()
	if strings.TrimSpace(root) == "" {
		return Config{}, fmt.Errorf("workspaceRoot is required")
	}

	contextDir := env.Build.Context
	if !filepath.IsAbs(contextDir) {
		contextDir = filepath.Join(root, contextDir)
	}
	contextDir = filepath.Clean(contextDir)

	dockerfile := env.Build.Dockerfile
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(contextDir, dockerfile)
	}
	dockerfile = filepath.Clean(dockerfile)

	return Config{
		Image:      strings.TrimSpace(env.Image),
		ContextDir: contextDir,
		Dockerfile: dockerfile,
	}, nil
}

func Optional(cfg config.Config) (Config, bool, error) {
	if strings.TrimSpace(cfg.BaseEnvironment.Image) == "" {
		return Config{}, false, nil
	}
	resolved, err := Resolve(cfg)
	return resolved, true, err
}

func FormatAge(now time.Time, createdAt time.Time) string {
	if createdAt.IsZero() {
		return ""
	}
	if now.Before(createdAt) {
		return "0s"
	}
	d := now.Sub(createdAt).Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	if d < time.Hour {
		return d.Truncate(time.Second).String()
	}
	return d.Truncate(time.Minute).String()
}
