//go:build unix

package gabric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRsyncTransportServeRunner runs the remote side through a custom runner,
// without a real rsync or SSH connection.
func TestRsyncTransportServeRunner(t *testing.T) {
	transport, err := newRsyncTransport()
	if err != nil {
		t.Fatalf("newRsyncTransport() error: %v", err)
	}
	defer transport.Close()

	args := []string{"-l", "deploy", "example.com", "rsync", "--server", ".", "/home/app"}
	if err := writeRsyncArgs(transport.argsWrite, args); err != nil {
		t.Fatalf("writeRsyncArgs() error: %v", err)
	}

	var (
		gotCmd    string
		gotStdin  io.Reader
		gotStdout io.Writer
	)

	remoteErr := &ExecExitError{Status: 7}

	runner := func(_ context.Context, cmd string, stdin io.Reader, stdout, _ io.Writer) error {
		gotCmd = cmd
		gotStdin = stdin
		gotStdout = stdout

		return remoteErr
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)

	if err := transport.Serve(ctx); !errors.Is(err, remoteErr) {
		t.Fatalf("Serve() error = %v, want %v", err, remoteErr)
	}

	if want := "rsync --server . /home/app"; gotCmd != want {
		t.Errorf("runner command = %q, want %q", gotCmd, want)
	}

	if gotStdin != transport.dataParent {
		t.Error("runner stdin is not the transport data file")
	}

	if gotStdout != transport.dataParent {
		t.Error("runner stdout is not the transport data file")
	}

	if status := readRsyncStatus(transport.statusRead); status != 7 {
		t.Errorf("remote status = %d, want 7", status)
	}
}

// TestRsyncSidecarBridge exercises the sidecar re-exec end to end over the
// inherited file descriptors, without a real rsync or SSH connection.
func TestRsyncSidecarBridge(t *testing.T) {
	dataParent, dataChild, err := newSocketPair()
	if err != nil {
		t.Fatalf("newSocketPair() error: %v", err)
	}
	defer dataParent.Close()

	argsRead, argsWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	defer argsRead.Close()

	statusRead, statusWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	defer statusWrite.Close()

	cmd := exec.Command(os.Args[0], rsyncSidecarArg,
		"-l", "deploy", "example.com", "rsync", "--server", "-logDtpre", ".", "/home/app")
	cmd.Stdin = strings.NewReader("from-rsync")
	cmd.ExtraFiles = []*os.File{dataChild, argsWrite, statusRead}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	dataChild.Close()
	argsWrite.Close()
	statusRead.Close()

	command, err := readRsyncServerCommand(argsRead)
	if err != nil {
		t.Fatalf("readRsyncServerCommand() error: %v", err)
	}

	if want := "rsync --server -logDtpre . /home/app"; command != want {
		t.Errorf("command = %q, want %q", command, want)
	}

	if _, err := dataParent.Write([]byte("from-remote")); err != nil {
		t.Fatalf("dataParent.Write() error: %v", err)
	}

	localToRemote := make([]byte, len("from-rsync"))
	if _, err := io.ReadFull(dataParent, localToRemote); err != nil {
		t.Fatalf("dataParent.Read() error: %v", err)
	}

	if string(localToRemote) != "from-rsync" {
		t.Errorf("data forwarded to remote = %q, want %q", string(localToRemote), "from-rsync")
	}

	if err := dataParent.Close(); err != nil {
		t.Fatalf("dataParent.Close() error: %v", err)
	}

	if _, err := io.WriteString(statusWrite, "7\n"); err != nil {
		t.Fatalf("statusWrite.Write() error: %v", err)
	}

	statusWrite.Close()

	err = cmd.Wait()

	if code := cmd.ProcessState.ExitCode(); code != 7 {
		t.Fatalf("Wait() = %v, exit status = %d, want 7", err, code)
	}

	if stdout.String() != "from-remote" {
		t.Errorf("sidecar stdout = %q, want %q", stdout.String(), "from-remote")
	}
}
