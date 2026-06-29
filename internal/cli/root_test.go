package cli

import (
	"bytes"
	"testing"
)

func TestRootCommandShowsHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Agent Workspace")) {
		t.Fatalf("help output did not contain product name: %s", out.String())
	}
}
