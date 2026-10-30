//go:build unix

package gabricssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/sshtest"
)

// TestRsyncLocalIntegration runs a real rsync through the SSH transport
// against an in-process SSH server whose exec handler spawns the command
// locally. No Docker required.
func TestRsyncLocalIntegration(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available")
	}

	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))

	ctx := remoteServerContext(t, srv, false)

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	remoteFile := filepath.Join(t.TempDir(), "upload.txt")

	t.Logf("local=%s remote=%s", localFile, remoteFile)

	if err := gabric.Remote(ctx).Rsync().Local(localFile).Remote(remoteFile).Upload().RunErr(); err != nil {
		t.Fatalf("Rsync() error: %v", err)
	}

	b, err := os.ReadFile(remoteFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}
}

// TestRsyncLocalIntegrationCapture exercises output capture on the local
// rsync process. It runs a dry-run itemize (`-n -i`) against the in-process SSH
// server and checks the itemized path is captured, then runs a real upload with
// both streams captured.
func TestRsyncLocalIntegrationCapture(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available")
	}

	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))

	ctx := remoteServerContext(t, srv, false)

	localFile := filepath.Join(t.TempDir(), "capture.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	remoteFile := filepath.Join(t.TempDir(), "capture.txt")

	dry := gabric.Remote(ctx).Rsync().
		Local(localFile).
		Remote(remoteFile).
		Upload().
		AddFlags("-n", "-i").
		CaptureStdout().
		Run()

	if out := dry.StdoutStr(); !strings.Contains(out, "capture.txt") {
		t.Errorf("dry-run StdoutStr() = %q, want it to contain %q", out, "capture.txt")
	}

	if _, err := os.Stat(remoteFile); !os.IsNotExist(err) {
		t.Errorf("dry-run created remote file: stat error = %v", err)
	}

	captured := gabric.Remote(ctx).Rsync().
		Local(localFile).
		Remote(remoteFile).
		Upload().
		Capture().
		Run()

	if captured.StdoutStr() == "" {
		t.Error("StdoutStr() is empty with Capture(), want rsync output")
	}
}
