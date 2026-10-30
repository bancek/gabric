//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShC(t *testing.T) {
	core := New()

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sh -c 'echo hi'", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("echo", "hi")
	asserts.AssertNotExecuted("sh")
}

func TestSlashShC(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "/bin/sh -c 'echo hi'", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertNotExecuted("/bin/sh")
}

func TestBashC(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "bash -c 'echo hi'", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	NewAssertions(t, core.Commands).AssertNotExecuted("bash")
}

func TestNestedSh(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	script := `sh -c "sh -c 'echo nested'"`

	code, err := core.RunScript(context.Background(), script, nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "nested\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("echo", "nested")
	asserts.AssertNotExecuted("sh")
}

func TestShExitStatus(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "sh -c 'exit 3'", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestNestedShExitStatus(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), `sh -c "sh -c 'exit 4'"`, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 4 {
		t.Errorf("exit code = %d, want 4", code)
	}
}

func TestUnknownCommandAfterSh(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sh -c 'not-a-real-command'", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 127 {
		t.Errorf("exit code = %d, want 127", code)
	}

	if !strings.Contains(stderr.String(), "not-a-real-command: command not found") {
		t.Errorf("stderr = %q, want command not found", stderr.String())
	}

	NewAssertions(t, core.Commands).AssertExecuted("not-a-real-command")
}

func TestUnknownCommandIsNotExecuted(t *testing.T) {
	core := New()

	path := filepath.Join(t.TempDir(), "touched")

	code, err := core.RunScript(context.Background(), "sh -c 'touch "+path+"'", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 127 {
		t.Errorf("exit code = %d, want 127", code)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("unknown command executed: Stat() error = %v", err)
	}
}

func TestUnsupportedShInvocation(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sh -e", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}

	if !strings.Contains(stderr.String(), "unsupported shell invocation") {
		t.Errorf("stderr = %q, want unsupported shell invocation", stderr.String())
	}
}

func TestSudoEcho(t *testing.T) {
	core := New()

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo echo hi", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("echo", "hi")
	asserts.AssertNotExecuted("sudo")

	cmd, ok := asserts.Find("echo", "hi")
	if !ok {
		t.Fatal("echo command not recorded")
	}
	if cmd.Sudo == nil {
		t.Fatal("Sudo = nil, want metadata")
	}
	if cmd.Sudo.User != "" || len(cmd.Sudo.Flags) != 0 {
		t.Errorf("Sudo = %+v, want empty metadata", *cmd.Sudo)
	}
}

func TestSudoShC(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo /bin/sh -c 'echo hi'", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("echo", "hi")
	asserts.AssertNotExecuted("sudo")

	if cmd, ok := asserts.Find("echo", "hi"); !ok || cmd.Sudo == nil {
		t.Errorf("echo command = %+v, want sudo metadata", cmd)
	}
}

func TestSudoNestedSh(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	script := `sudo sh -c "sh -c 'echo nested'"`

	code, err := core.RunScript(context.Background(), script, nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "nested\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	asserts := NewAssertions(t, core.Commands)
	asserts.AssertExecuted("echo", "nested")
	asserts.AssertNotExecuted("sudo")
	asserts.AssertNotExecuted("sh")

	if cmd, ok := asserts.Find("echo", "nested"); !ok || cmd.Sudo == nil {
		t.Errorf("echo command = %+v, want sudo metadata", cmd)
	}
}

func TestSudoExitStatus(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "sudo sh -c 'exit 3'", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestNestedSudo(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo sudo echo hi", nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if cmd, ok := NewAssertions(t, core.Commands).Find("echo", "hi"); !ok || cmd.Sudo == nil {
		t.Errorf("echo command = %+v, want sudo metadata", cmd)
	}
}

func TestSudoMissingCommand(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stderr.String(), "sudo: missing command") {
		t.Errorf("stderr = %q, want missing command", stderr.String())
	}
}

func TestSudoOptions(t *testing.T) {
	core := New()

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo -n -u app echo hi", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	cmd, ok := NewAssertions(t, core.Commands).Find("echo", "hi")
	if !ok {
		t.Fatal("echo command not recorded")
	}
	if cmd.Sudo == nil {
		t.Fatal("Sudo = nil, want metadata")
	}
	if cmd.Sudo.User != "app" {
		t.Errorf("Sudo.User = %q, want app", cmd.Sudo.User)
	}
	if len(cmd.Sudo.Flags) != 1 || cmd.Sudo.Flags[0] != "-n" {
		t.Errorf("Sudo.Flags = %v, want [-n]", cmd.Sudo.Flags)
	}
}

func TestSudoMissingUserArgument(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "sudo -u", nil, nil, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}

	if !strings.Contains(stderr.String(), "requires an argument") {
		t.Errorf("stderr = %q, want missing argument error", stderr.String())
	}
}

func TestSudoUnknownCommandIsNotExecuted(t *testing.T) {
	core := New()

	path := filepath.Join(t.TempDir(), "touched")

	code, err := core.RunScript(context.Background(), "sudo touch "+path, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 127 {
		t.Errorf("exit code = %d, want 127", code)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("unknown command executed: Stat() error = %v", err)
	}

	if cmd, ok := NewAssertions(t, core.Commands).Find("touch", path); !ok || cmd.Sudo == nil {
		t.Errorf("touch command = %+v, want sudo metadata", cmd)
	}
}
