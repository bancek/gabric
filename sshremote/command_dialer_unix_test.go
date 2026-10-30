//go:build unix

package sshremote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/bancek/gabric/internal/testutils"
)

func TestCommandDialerCloseKillsProcessGroup(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")

	d := &commandDialer{
		host:    &Host{hostname: "127.0.0.1", port: 22},
		command: fmt.Sprintf("sleep 60 & echo $! > %q; wait", pidfile),
	}

	conn, err := d.Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	defer conn.Close()

	var childPID int

	testutils.Eventually(t, 5*time.Second, func() bool {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return false
		}

		childPID = pid

		return true
	})

	if err := d.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// The child (in the command's process group) must be gone.
	testutils.Eventually(t, 5*time.Second, func() bool {
		return syscall.Kill(childPID, 0) == syscall.ESRCH
	})
}
