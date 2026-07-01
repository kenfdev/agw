package docker

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"runtime"
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

func TestUpDetachedUsesDockerComposeUpDetached(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.UpDetached("/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "up", "-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestUpDetachedWithOptionsUsesDockerComposeUpDetachedFlags(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.UpDetachedWithOptions("/tmp/ws", UpOptions{Build: true, ForceRecreate: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "up", "-d", "--build", "--force-recreate"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRunShellUsesPlatformShellInWorkspaceDir(t *testing.T) {
	var gotDir string
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		gotDir = dir
		got = append([]string{name}, args...)
		return nil
	}}
	command := "op run --env-file=.env.1password -- docker compose up -d"
	if err := cli.RunShell("/tmp/ws", command); err != nil {
		t.Fatal(err)
	}
	if gotDir != "/tmp/ws" {
		t.Fatalf("dir = %q, want /tmp/ws", gotDir)
	}
	want := []string{"sh", "-c", command}
	if runtime.GOOS == "windows" {
		want = []string{"cmd", "/C", command}
	}
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

func TestLogsUsesDockerComposeLogsWithTail(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if _, err := cli.Logs("/tmp/ws", "dev"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "logs", "--tail", "200", "dev"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestStopUsesDockerComposeStop(t *testing.T) {
	var got []string
	cli := CLI{Exec: func(dir string, name string, args ...string) error {
		_ = dir
		got = append([]string{name}, args...)
		return nil
	}}
	if err := cli.Stop("/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "stop"}
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

func TestServiceRunningUsesDockerComposePs(t *testing.T) {
	var got []string
	old := runComposePs
	defer func() { runComposePs = old }()

	runComposePs = func(dir string, service string) ([]byte, error) {
		got = append([]string{"docker", "compose", "ps", "--status", "running", "-q", service})
		return []byte("container-id\n"), nil
	}
	running, err := CLI{}.ServiceRunning("/tmp/ws", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Fatal("expected running")
	}
	want := []string{"docker", "compose", "ps", "--status", "running", "-q", "dev"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestServiceRunningReturnsTrueForNonEmptyOutput(t *testing.T) {
	old := runComposePs
	defer func() { runComposePs = old }()

	runComposePs = func(_ string, _ string) ([]byte, error) {
		return []byte("container-id\n"), nil
	}

	running, err := CLI{}.ServiceRunning("/tmp/ws", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Fatal("expected running")
	}
}

func TestServiceRunningReturnsFalseForEmptyOutput(t *testing.T) {
	old := runComposePs
	defer func() { runComposePs = old }()

	runComposePs = func(_ string, _ string) ([]byte, error) {
		return []byte{}, nil
	}

	running, err := CLI{}.ServiceRunning("/tmp/ws", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("expected not running")
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
