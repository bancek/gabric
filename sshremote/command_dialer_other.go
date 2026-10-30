//go:build !unix

package sshremote

import "os/exec"

func configureCommand(*exec.Cmd) {}

// killCommand kills the command's process. Process groups are not available on
// these platforms, so this is best effort.
func killCommand(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
