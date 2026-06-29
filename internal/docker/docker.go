package docker

import (
	"bufio"
	"encoding/json"
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

type Network struct {
	Name   string
	Labels map[string]string
}

type CLI struct {
	In   io.Reader
	Out  io.Writer
	Err  io.Writer
	Exec func(dir string, name string, args ...string) error
}

var runNetworkInspect = func(name string) ([]byte, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	return cmd.CombinedOutput()
}

var runNetworkList = func() ([]byte, error) {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{json .}}")
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
	var lastErr error
	for _, shell := range []string{"bash", "zsh", "sh"} {
		if err := c.compose(dir, "docker", "compose", "exec", service, shell); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (c CLI) compose(dir string, args ...string) error {
	if c.Exec != nil {
		return c.Exec(dir, args[0], args[1:]...)
	}
	out := c.Out
	if out == nil {
		out = os.Stdout
	}
	in := c.In
	if in == nil {
		in = os.Stdin
	}
	errOut := c.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdin = in
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

func (CLI) ListNetworks() ([]Network, error) {
	out, err := runNetworkList()
	if err != nil {
		return nil, err
	}
	var networks []Network
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var payload struct {
			Name   string          `json:"Name"`
			Labels json.RawMessage `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			return nil, err
		}
		networks = append(networks, Network{
			Name:   payload.Name,
			Labels: parseLabelsFromJSON(payload.Labels),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return networks, nil
}

func parseLabelsFromJSON(raw json.RawMessage) map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]string{}
	}
	var rawLabels string
	if err := json.Unmarshal(raw, &rawLabels); err != nil {
		return map[string]string{}
	}
	labels := strings.TrimSpace(rawLabels)
	if labels == "" {
		return map[string]string{}
	}

	parts := strings.Split(labels, ",")
	out := make(map[string]string, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(kv[1])
	}
	return out
}

func isNetworkNotFoundOutput(output []byte) bool {
	out := strings.ToLower(strings.TrimSpace(string(output)))
	return strings.Contains(out, "no such network") || (strings.Contains(out, "network") && strings.Contains(out, "not found"))
}
