package docker

import (
	"fmt"
	"os/exec"
)

type Runner interface {
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
}

type CLI struct{}

func (CLI) ComposeConfig(dir string) error {
	cmd := exec.Command("docker", "compose", "config")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func (CLI) NetworkExists(name string) (bool, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
