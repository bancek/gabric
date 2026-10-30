//go:build unix

package gabricssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/internal/terminal"
	"github.com/bancek/gabric/internal/testutils"
)

func TestShellPTY(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))
	ctx := remoteServerContext(t, srv, false)

	var out bytes.Buffer

	err := Shell(ctx).
		Stdin(strings.NewReader("exit\n")).
		Stdout(&out).
		Stderr(io.Discard).
		Pty(true).
		Term("vt100").
		Size(120, 40).
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got := srv.PTYTerm(); got != "vt100" {
		t.Errorf("PTYTerm() = %q, want %q", got, "vt100")
	}

	w, h := srv.PTYSize()
	if w != 120 || h != 40 {
		t.Errorf("PTYSize() = %dx%d, want 120x40", w, h)
	}
}

func TestShellDefaultNoPTY(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))
	ctx := remoteServerContext(t, srv, false)

	var out bytes.Buffer

	err := Shell(ctx).
		Stdin(strings.NewReader("echo marker\nexit\n")).
		Stdout(&out).
		Stderr(io.Discard).
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got := srv.PTYTerm(); got != "" {
		t.Errorf("PTYTerm() = %q, want no PTY requested", got)
	}

	if !strings.Contains(out.String(), "marker") {
		t.Errorf("output = %q, want it to contain %q", out.String(), "marker")
	}
}

func TestShellResize(t *testing.T) {
	started := make(chan struct{})

	srv := startSSHServer(t, sshtest.WithExecHandler(func(_ string, stdin io.Reader, _, _ io.Writer) int {
		close(started)
		io.Copy(io.Discard, stdin)
		return 0
	}))
	ctx := remoteServerContext(t, srv, false)

	stdinRead, stdinWrite := io.Pipe()
	defer stdinWrite.Close()

	resize := make(chan terminal.Size, 1)

	ops := Shell(ctx).
		Stdin(stdinRead).
		Stdout(io.Discard).
		Stderr(io.Discard).
		Term("xterm").
		Size(80, 24)
	ops.resize = resize

	errCh := make(chan error, 1)

	go func() {
		errCh <- ops.RunErr()
	}()

	<-started

	resize <- terminal.Size{Width: 132, Height: 43}

	testutils.Eventually(t, 5*time.Second, func() bool {
		w, h := srv.LastWindowSize()
		return w == 132 && h == 43
	})

	stdinWrite.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}
}

func TestShellNoHost(t *testing.T) {
	err := Shell(context.Background()).Pty(false).RunErr()
	if err != nil {
		return
	}

	t.Fatal("RunErr() without host = nil, want error")
}

func TestShellExitError(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(func(_ string, _ io.Reader, _, _ io.Writer) int {
		return 3
	}))
	ctx := remoteServerContext(t, srv, false)

	err := Shell(ctx).
		Stdin(strings.NewReader("")).
		Stdout(io.Discard).
		Stderr(io.Discard).
		Pty(false).
		RunErr()

	remoteErr, ok := errors.AsType[*gabric.ExecExitError](err)
	if !ok {
		t.Fatalf("RunErr() error = %T, want *gabric.ExecExitError", err)
	}
	if remoteErr.Status != 3 {
		t.Errorf("Status = %d, want 3", remoteErr.Status)
	}

	if _, ok := errors.AsType[*ssh.ExitError](err); !ok {
		t.Errorf("RunErr() error = %T, want it to unwrap to *ssh.ExitError", err)
	}
}
