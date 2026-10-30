package gabricssh

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/terminal"
)

// ShellOps builds an interactive login shell on the remote host. The zero value
// is not usable; create one with Shell.
type ShellOps struct {
	ctx    context.Context
	stdin  io.Reader
	inFile *os.File // non-nil when stdin is an *os.File
	inTTY  bool     // inFile != nil && terminal.IsTerminal(inFile)
	stdout io.Writer
	stderr io.Writer
	term   string
	width  int
	height int
	pty    bool
	resize <-chan terminal.Size
}

// Shell returns a builder for an interactive login shell on the remote host
// from the context. It defaults to the process standard streams, the local
// terminal type and size, and requests a PTY when stdin is a terminal.
func Shell(ctx context.Context) *ShellOps {
	s := &ShellOps{ctx: ctx}

	s.Stdin(os.Stdin)
	s.Stdout(os.Stdout)
	s.Stderr(os.Stderr)
	s.Term(defaultTerm())
	s.Size(defaultSize())

	return s
}

// Stdin sets the shell input and derives the PTY default from it: a PTY is
// requested when stdin is a terminal. Call Pty after Stdin to override.
func (s *ShellOps) Stdin(stdin io.Reader) *ShellOps {
	s.stdin = stdin
	s.inFile = nil

	if f, ok := stdin.(*os.File); ok {
		s.inFile = f
	}

	s.inTTY = s.inFile != nil && terminal.IsTerminal(s.inFile)
	s.pty = s.inTTY

	return s
}

// Stdout sets the shell output. It defaults to os.Stdout.
func (s *ShellOps) Stdout(stdout io.Writer) *ShellOps {
	s.stdout = stdout
	return s
}

// Stderr sets the shell error output. It defaults to os.Stderr.
func (s *ShellOps) Stderr(stderr io.Writer) *ShellOps {
	s.stderr = stderr
	return s
}

// Term sets the terminal type sent with the PTY request. It defaults to $TERM,
// or "xterm" when unset.
func (s *ShellOps) Term(term string) *ShellOps {
	s.term = term
	return s
}

// Size sets the initial terminal size in character cells. It defaults to the
// local terminal size, or 80x24 when there is none.
func (s *ShellOps) Size(width, height int) *ShellOps {
	s.width = width
	s.height = height
	return s
}

// Pty overrides whether a remote PTY is requested. By default it follows
// Stdin: a PTY is requested when stdin is a terminal.
func (s *ShellOps) Pty(enabled bool) *ShellOps {
	s.pty = enabled
	return s
}

// Run runs the shell and panics on failure.
func (s *ShellOps) Run() *ShellOps {
	if err := s.RunErr(); err != nil {
		panic(err)
	}
	return s
}

// RunErr opens the shell and returns the remote exit status. It returns an
// error when the context has no host.
func (s *ShellOps) RunErr() error {
	if !HasHost(s.ctx) {
		return errors.New("no host: pass -ssh-host")
	}

	if s.inTTY {
		if restore, err := terminal.MakeRaw(s.inFile); err == nil {
			defer restore()
		}
	}

	resize := s.resize
	if resize == nil && s.pty && s.inTTY {
		var stop func()
		resize, stop = terminal.Resize(s.inFile)
		defer stop()
	}

	host := GetHost(s.ctx)

	gabric.Logger(s.ctx).With("host", host.String(), "term", s.term, "width", s.width, "height", s.height, "pty", s.pty).Debug("Remote shell")

	err := GetRemote(s.ctx).WithSession(s.ctx, gabric.Logger(s.ctx), func(session *ssh.Session) error {
		if s.pty {
			modes := ssh.TerminalModes{
				ssh.ECHO:          1,
				ssh.TTY_OP_ISPEED: 14400,
				ssh.TTY_OP_OSPEED: 14400,
			}

			if err := session.RequestPty(s.term, s.height, s.width, modes); err != nil {
				return err
			}
		}

		session.Stdin = s.stdin
		session.Stdout = s.stdout
		session.Stderr = s.stderr

		if resize != nil {
			done := make(chan struct{})
			defer close(done)

			go func() {
				for {
					select {
					case <-done:
						return
					case size, ok := <-resize:
						if !ok {
							return
						}

						session.WindowChange(size.Height, size.Width)
					}
				}
			}()
		}

		if err := session.Shell(); err != nil {
			return err
		}

		return session.Wait()
	})
	if err != nil {
		if exitErr, ok := errors.AsType[*ssh.ExitError](err); ok {
			return &gabric.ExecExitError{
				Status: exitErr.ExitStatus(),
				Err:    err,
			}
		}

		return err
	}

	return nil
}

func defaultTerm() string {
	term := os.Getenv("TERM")
	if term == "" {
		return "xterm"
	}

	return term
}

func defaultSize() (width, height int) {
	if size, ok := terminal.GetSize(os.Stdin); ok {
		return size.Width, size.Height
	}

	return 80, 24
}
