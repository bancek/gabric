package gabric

import (
	"context"
	"io"
	"os"
)

type execCall struct {
	cmd           []string
	env           ExecEnv
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	captureStdout bool
	captureStderr bool
	extraFiles    []*os.File
	afterStart    func()
}

type execFunc func(ctx context.Context, call execCall) (stdout, stderr []byte, err error)

// ExecOps runs a command either locally or remotely. The backend is selected by
// Local(ctx).Exec or Remote(ctx).Exec.
//
// By default stdout and stderr are inherited from the process. Use Capture,
// CaptureStdout or CaptureStderr to capture the output instead. WithEnv
// adds per-command environment entries on top of the context's exec
// environment.
type ExecOps struct {
	ctx           context.Context
	run           execFunc
	cmd           []string
	env           ExecEnv
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	captureStdout bool
	captureStderr bool
	extraFiles    []*os.File
	afterStart    func()
	logErrors     bool
	stdoutBytes   []byte
	stderrBytes   []byte
}

func newExec(ctx context.Context, run execFunc, cmd []string) *ExecOps {
	return &ExecOps{
		ctx: ctx,
		run: run,
		cmd: cmd,
	}
}

// WithEnv adds environment entries for this command. Entries override the
// context's exec environment for the same keys. Set several by chaining or by
// passing multiple entries.
func (e *ExecOps) WithEnv(vars ...EnvVar) *ExecOps {
	e.env = e.env.With(vars...)
	return e
}

// Stdin sets the command's standard input. It defaults to nil, which means no
// input. Pass os.Stdin to forward the process' standard input.
func (e *ExecOps) Stdin(stdin io.Reader) *ExecOps {
	e.stdin = stdin
	return e
}

// Stdout redirects the command's standard output to w, instead of inheriting
// the process stdout (the default) or capturing it. An explicit writer takes
// precedence over CaptureStdout/Quiet.
func (e *ExecOps) Stdout(stdout io.Writer) *ExecOps {
	e.stdout = stdout
	return e
}

// Stderr redirects the command's standard error to w, instead of inheriting the
// process stderr (the default) or capturing it. An explicit writer takes
// precedence over CaptureStderr/LogErrors/Quiet.
func (e *ExecOps) Stderr(stderr io.Writer) *ExecOps {
	e.stderr = stderr
	return e
}

// Capture captures both stdout and stderr.
func (e *ExecOps) Capture() *ExecOps {
	e.captureStdout = true
	e.captureStderr = true
	return e
}

// CaptureStdout captures stdout. Stderr is still inherited from the process.
func (e *ExecOps) CaptureStdout() *ExecOps {
	e.captureStdout = true
	return e
}

// CaptureStderr captures stderr. Stdout is still inherited from the process.
func (e *ExecOps) CaptureStderr() *ExecOps {
	e.captureStderr = true
	return e
}

// LogErrors captures stderr and logs it when the command fails.
func (e *ExecOps) LogErrors() *ExecOps {
	e.captureStderr = true
	e.logErrors = true
	return e
}

// Quiet captures stdout and stderr, and logs stderr when the command fails.
func (e *ExecOps) Quiet() *ExecOps {
	return e.CaptureStdout().LogErrors()
}

// Run runs the command and panics on failure.
func (e *ExecOps) Run() *ExecOps {
	if err := e.RunErr(); err != nil {
		panic(err)
	}
	return e
}

// RunErr runs the command and stores the captured output.
func (e *ExecOps) RunErr() error {
	stdout, stderr, err := e.run(e.ctx, execCall{
		cmd:           e.cmd,
		env:           e.env,
		stdin:         e.stdin,
		stdout:        e.stdout,
		stderr:        e.stderr,
		captureStdout: e.captureStdout,
		captureStderr: e.captureStderr,
		extraFiles:    e.extraFiles,
		afterStart:    e.afterStart,
	})

	e.stdoutBytes = stdout
	e.stderrBytes = stderr

	if err != nil && e.logErrors {
		log := Logger(e.ctx).With("error", err)
		if len(stderr) > 0 {
			log = log.With("stderr", string(stderr))
		}
		log.Error("Exec failed")
	}

	return err
}

// StdoutBytes returns the captured stdout.
func (e *ExecOps) StdoutBytes() []byte {
	return e.stdoutBytes
}

// StdoutStr returns the captured stdout as a string.
func (e *ExecOps) StdoutStr() string {
	return string(e.stdoutBytes)
}

// StderrBytes returns the captured stderr.
func (e *ExecOps) StderrBytes() []byte {
	return e.stderrBytes
}

// StderrStr returns the captured stderr as a string.
func (e *ExecOps) StderrStr() string {
	return string(e.stderrBytes)
}
