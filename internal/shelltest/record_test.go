//go:build unix

package shelltest

import (
	"context"
	"slices"
	"testing"
)

func TestRunScriptRecordsBuiltin(t *testing.T) {
	core := New()

	if _, err := core.RunScript(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	cmds := core.Commands()
	if len(cmds) != 1 {
		t.Fatalf("len(Commands()) = %d, want 1", len(cmds))
	}

	cmd := cmds[0]
	if cmd.Name != "echo" {
		t.Errorf("Name = %q, want %q", cmd.Name, "echo")
	}
	if len(cmd.Args) != 1 || cmd.Args[0] != "hi" {
		t.Errorf("Args = %v, want [hi]", cmd.Args)
	}
	if cmd.Dir != "/" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/")
	}
	if cmd.Sudo != nil {
		t.Errorf("Sudo = %v, want nil", cmd.Sudo)
	}

	NewAssertions(t, core.Commands).AssertExecuted("echo", "hi")
}

func TestRunScriptRecordsOrder(t *testing.T) {
	core := New()

	if _, err := core.RunScript(context.Background(), "echo one; echo two", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	cmds := core.Commands()
	if len(cmds) != 2 {
		t.Fatalf("len(Commands()) = %d, want 2", len(cmds))
	}

	if cmds[0].Name != "echo" || cmds[0].Args[0] != "one" {
		t.Errorf("first command = %+v, want echo one", cmds[0])
	}
	if cmds[1].Name != "echo" || cmds[1].Args[0] != "two" {
		t.Errorf("second command = %+v, want echo two", cmds[1])
	}
}

func TestRunScriptRecordsDir(t *testing.T) {
	core := New()

	if _, err := core.RunScript(context.Background(), "cd / && echo hi", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	cmds := core.Commands()
	if len(cmds) != 2 {
		t.Fatalf("len(Commands()) = %d, want 2", len(cmds))
	}

	if cmds[0].Name != "cd" || cmds[0].Dir != "/" {
		t.Errorf("cd command = %+v, want dir /", cmds[0])
	}
	if cmds[1].Name != "echo" || cmds[1].Dir != "/" {
		t.Errorf("echo command = %+v, want dir /", cmds[1])
	}
}

func TestRunArgsEnvRecordsEnv(t *testing.T) {
	core := New()

	if _, err := core.RunArgsEnv(
		context.Background(),
		"/",
		[]string{"mkdir", "/srv/env"},
		[]string{"GABRIC_TEST_ENV=value"},
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("RunArgsEnv() error: %v", err)
	}

	cmds := core.Commands()
	if len(cmds) != 1 {
		t.Fatalf("len(Commands()) = %d, want 1", len(cmds))
	}
	if got, want := cmds[0].Env, []string{"GABRIC_TEST_ENV=value"}; !slices.Equal(got, want) {
		t.Errorf("ExecEnv = %v, want %v", got, want)
	}
}

func TestCommandsCopy(t *testing.T) {
	core := New()

	if _, err := core.RunScript(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	cmds := core.Commands()
	cmds[0].Name = "changed"
	cmds[0].Args[0] = "changed"

	again := core.Commands()
	if again[0].Name != "echo" || again[0].Args[0] != "hi" {
		t.Errorf("Commands() shares state: %+v", again[0])
	}
}

func TestCommandsCopyEnv(t *testing.T) {
	core := New()

	if _, err := core.RunArgsEnv(
		context.Background(),
		"/",
		[]string{"mkdir", "/srv/env"},
		[]string{"GABRIC_TEST_ENV=value"},
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("RunArgsEnv() error: %v", err)
	}

	cmds := core.Commands()
	cmds[0].Env[0] = "GABRIC_TEST_ENV=changed"

	again := core.Commands()
	if got, want := again[0].Env, []string{"GABRIC_TEST_ENV=value"}; !slices.Equal(got, want) {
		t.Errorf("ExecEnv after mutating Commands() = %v, want %v", got, want)
	}
}

func TestReset(t *testing.T) {
	core := New()

	if _, err := core.RunScript(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	core.Reset()

	if cmds := core.Commands(); len(cmds) != 0 {
		t.Errorf("Commands() after Reset() = %v, want empty", cmds)
	}
}
