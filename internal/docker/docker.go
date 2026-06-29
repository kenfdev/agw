package docker

import (
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
}

type CLI struct{}

var runNetworkInspect = func(name string) ([]byte, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	return cmd.CombinedOutput()
}

func (CLI) ComposeConfig(dir string) error {
	cmd := exec.Command("docker", "compose", "config")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func (CLI) NetworkExists(name string) (bool, error) {
	out, err := runNetworkInspect(name)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok && isNetworkNotFoundOutput(out) {
			return false, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, fmt.Errorf("%w: %s", exitErr, out)
		}
		return false, err
	}
	return true, nil
}

func isNetworkNotFoundOutput(output []byte) bool {
	out := strings.ToLower(strings.TrimSpace(string(output)))
	return strings.Contains(out, "no such network") || (strings.Contains(out, "network") && strings.Contains(out, "not found"))
}
