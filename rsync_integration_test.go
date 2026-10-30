//go:build unix

package gabric

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRsyncLocalRemoteIntegration runs a real rsync through the local remote
// runner, without a host or SSH.
func TestRsyncLocalRemoteIntegration(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")

	if err := os.WriteFile(source, []byte("hello local"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	remote := filepath.Join(dir, "remote.txt")

	var commands []string

	runner := func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
		commands = append(commands, cmd)

		return localRemoteRunner(ctx, cmd, stdin, stdout, stderr)
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)

	if err := Remote(ctx).Rsync().Local(source).Remote(remote).Upload().RunErr(); err != nil {
		t.Fatalf("Rsync() upload error: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(commands))
	}

	if !strings.Contains(commands[0], "--server") {
		t.Errorf("runner command = %q, want a remote rsync server command", commands[0])
	}

	b, err := os.ReadFile(remote)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello local" {
		t.Errorf("remote content = %q, want %q", string(b), "hello local")
	}

	downloaded := filepath.Join(dir, "downloaded.txt")

	if err := LocalRemote(testContext(t)).Rsync().Local(downloaded).Remote(remote).Download().RunErr(); err != nil {
		t.Fatalf("Rsync() download error: %v", err)
	}

	b, err = os.ReadFile(downloaded)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello local" {
		t.Errorf("downloaded content = %q, want %q", string(b), "hello local")
	}
}
