//go:build unix

package sshremote

import (
	"os/exec"
	"syscall"
)

// configureCommand puts the command in its own process group so the whole
// process tree can be killed on close.
func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killCommand kills the command's process group.
func killCommand(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
