//go:build unix

package shelltest

import (
	"context"
	"testing"
)

func TestAssertCommands(t *testing.T) {
	core := New()
	assertions := NewAssertions(t, core.Commands)

	if _, err := core.RunScript(context.Background(), "echo one; echo two", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	assertions.AssertCommands(
		Command{Name: "echo", Args: []string{"one"}, Dir: "/"},
		Command{Name: "echo", Args: []string{"two"}, Dir: "/"},
	)
}

func TestAssertions(t *testing.T) {
	core := New()
	assertions := NewAssertions(t, core.Commands)

	if _, err := core.RunScript(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	cmd := assertions.AssertExecuted("echo", "hi")
	assertions.AssertNotExecuted("echo", "bye")
	assertions.AssertFunc(func(cmd Command) bool {
		return cmd.Name == "echo" && cmd.Dir == "/"
	}, "echo in root")

	if cmd.Name != "echo" {
		t.Errorf("Name = %q, want %q", cmd.Name, "echo")
	}
}
