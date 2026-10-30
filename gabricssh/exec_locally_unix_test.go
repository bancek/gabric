//go:build unix

package gabricssh

import (
	"io"
	"os"
	"os/exec"
)

// execLocally runs cmd through /bin/sh with the SSH channel stdio. The child's
// stdin is an *os.File so cmd.Wait does not block on the stdin copier after
// the command exits: like sshd, the handler must report the exit status even
// while the client keeps its input channel open.
func execLocally(cmd string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = stdout
	c.Stderr = stderr

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return 1
	}

	c.Stdin = stdinRead

	if err := c.Start(); err != nil {
		stdinRead.Close()
		stdinWrite.Close()

		return 1
	}

	stdinRead.Close()

	go func() {
		io.Copy(stdinWrite, stdin)
		stdinWrite.Close()
	}()

	if err := c.Wait(); err != nil {
		return c.ProcessState.ExitCode()
	}

	return 0
}
