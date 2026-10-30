package gabric

import (
	"testing"
)

func TestRemoteExecShell(t *testing.T) {
	_, asserts, ctx := localRemoteContext(t)

	out := Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
	if out != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}

	asserts.AssertExecuted("echo", "hi")
}

func TestRemoteExecShellSudo(t *testing.T) {
	_, asserts, ctx := localRemoteContextWithExec(t, nil, true)

	out := Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
	if out != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}

	cmd := asserts.AssertExecuted("echo", "hi")
	if cmd.Sudo == nil {
		t.Error("Sudo = nil, want metadata")
	}
}
