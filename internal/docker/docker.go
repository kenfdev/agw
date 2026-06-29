package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	ComposeConfig(dir string) error
	NetworkExists(name string) (bool, error)
}

type CLI struct {
	Out  io.Writer
	Err  io.Writer
	Exec func(dir string, name string, args ...string) error
}

var runNetworkInspect = func(name string) ([]byte, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	return cmd.CombinedOutput()
}

func (c CLI) ComposeConfig(dir string) error {
	return c.compose(dir, "docker", "compose", "config")
}

func (c CLI) Build(dir string) error {
	return c.compose(dir, "docker", "compose", "build")
}

func (c CLI) Up(dir string) error {
	return c.compose(dir, "docker", "compose", "up")
}

func (c CLI) Down(dir string) error {
	return c.compose(dir, "docker", "compose", "down")
}

func (c CLI) Attach(dir string, service string) error {
	return c.compose(dir, "docker", "compose", "exec", service, "bash")
}

func (c CLI) compose(dir string, args ...string) error {
	if c.Exec != nil {
		return c.Exec(dir, args[0], args[1:]...)
	}
	out := c.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := c.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
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
