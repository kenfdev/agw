package docker

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// NetworkExists behavior: only "network not found" should return (false, nil);
// daemon/permission/and other inspect failures should return an error.
func TestNetworkExistsReturnsFalseForMissingNetwork(t *testing.T) {
	old := runNetworkInspect
	defer func() { runNetworkInspect = old }()

	runNetworkInspect = func(_ string) ([]byte, error) {
		return []byte("Error response from daemon: network missing not found"), commandExitError()
	}

	exists, err := CLI{}.NetworkExists("missing")
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if exists {
		t.Fatal("expected missing network to be false")
	}
}

func TestNetworkExistsReturnsErrorForOtherExitErrors(t *testing.T) {
	old := runNetworkInspect
	defer func() { runNetworkInspect = old }()

	out := []string{
		"Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
		"docker: 'network' is not a docker command.",
	}
	for _, message := range out {
		message := message
		runNetworkInspect = func(_ string) ([]byte, error) {
			return []byte(message), commandExitError()
		}

		_, err := CLI{}.NetworkExists("bad")
		if err == nil {
			t.Fatalf("expected error for inspect failure: %q", message)
		}
	}
}

func TestNetworkExistsReturnsErrorForExecuteError(t *testing.T) {
	old := runNetworkInspect
	defer func() { runNetworkInspect = old }()

	runNetworkInspect = func(_ string) ([]byte, error) {
		return nil, exec.ErrNotFound
	}

	_, err := CLI{}.NetworkExists("bad")
	if err == nil {
		t.Fatal("expected execution error")
	}
}

func TestBuildUsesDockerComposeBuild(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.Build("/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "build"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestUpUsesDockerComposeUp(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.Up("/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "up"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestDownUsesDockerComposeDown(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.Down("/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "down"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestAttachUsesDockerComposeExec(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.Attach("/tmp/ws", "dev"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "exec", "dev", "bash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestAttachFallsBackFromBashToZshToSh(t *testing.T) {
	var got [][]string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		call := append([]string{name}, args...)
		got = append(got, call)
		shell := args[len(args)-1]
		if shell == "sh" {
			return nil
		}
		return errors.New(shell + " not found")
	}}

	if err := cli.Attach("/tmp/ws", "dev"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"docker", "compose", "exec", "dev", "bash"},
		{"docker", "compose", "exec", "dev", "zsh"},
		{"docker", "compose", "exec", "dev", "sh"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestComposeCommandReceivesConfiguredStdin(t *testing.T) {
	var out bytes.Buffer
	cli := CLI{
		In:  strings.NewReader("hello\n"),
		Out: &out,
	}

	if err := cli.compose(t.TempDir(), "cat"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func commandExitError() error {
	if _, err := exec.Command("sh", "-c", "exit 1").CombinedOutput(); err != nil {
		return err
	}
	return exec.ErrNotFound
}
