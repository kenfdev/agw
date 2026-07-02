package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kenfdev/agw/internal/base"
	"github.com/kenfdev/agw/internal/config"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/spf13/cobra"
)

type baseRunner interface {
	BuildImage(contextDir string, dockerfile string, image string) error
	InspectImage(image string) (docker.ImageInfo, bool, error)
}

var now = time.Now

func NewBaseCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "base", Short: "Manage the AGW base environment image"}
	cmd.AddCommand(newBaseBuildCommand())
	cmd.AddCommand(newBaseStatusCommand())
	return cmd
}

func newBaseBuildCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the configured AGW base environment image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigForPath(configPath)
			if err != nil {
				return err
			}
			baseCfg, err := base.Resolve(cfg)
			if err != nil {
				return err
			}
			if _, err := os.Stat(baseCfg.Dockerfile); err != nil {
				return fmt.Errorf("base environment Dockerfile %s: %w", baseCfg.Dockerfile, err)
			}
			return newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr()).BuildImage(baseCfg.ContextDir, baseCfg.Dockerfile, baseCfg.Image)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func newBaseStatusCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the configured AGW base environment image status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigForPath(configPath)
			if err != nil {
				return err
			}
			baseCfg, err := base.Resolve(cfg)
			if err != nil {
				return err
			}
			status := inspectBaseImage(baseCfg, newLifecycleRunner(io.Discard, io.Discard), now())
			return writeBaseStatus(cmd.OutOrStdout(), status)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	return cmd
}

func loadConfigForPath(configPath string) (config.Config, error) {
	path := configPath
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return config.Config{}, err
		}
	}
	return config.Load(path)
}

func inspectBaseImage(baseCfg base.Config, runner baseRunner, current time.Time) base.Status {
	status := base.Status{Config: baseCfg}
	info, exists, err := runner.InspectImage(baseCfg.Image)
	if err != nil {
		status.Status = base.StatusUnknown
		status.Error = err.Error()
		return status
	}
	if !exists {
		status.Status = base.StatusMissing
		return status
	}
	status.Status = base.StatusAvailable
	if !info.CreatedAt.IsZero() {
		createdAt := info.CreatedAt
		status.CreatedAt = &createdAt
		status.Age = base.FormatAge(current, info.CreatedAt)
	}
	return status
}

func writeBaseStatus(out io.Writer, status base.Status) error {
	if _, err := fmt.Fprintf(out, "Base image: %s\n", status.Config.Image); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Build context: %s\n", status.Config.ContextDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Dockerfile: %s\n", status.Config.Dockerfile); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Image status: %s\n", status.Status); err != nil {
		return err
	}
	if status.CreatedAt != nil {
		if _, err := fmt.Fprintf(out, "Created: %s\n", status.CreatedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	if status.Age != "" {
		if _, err := fmt.Fprintf(out, "Age: %s\n", status.Age); err != nil {
			return err
		}
	}
	if status.Error != "" {
		if _, err := fmt.Fprintf(out, "Error: %s\n", status.Error); err != nil {
			return err
		}
	}
	return nil
}
