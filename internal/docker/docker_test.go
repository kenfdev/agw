package docker

import (
	"os/exec"
	"testing"
)

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

func commandExitError() error {
	if _, err := exec.Command("sh", "-c", "exit 1").CombinedOutput(); err != nil {
		return err
	}
	return exec.ErrNotFound
}
