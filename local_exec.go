package gabric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/kballard/go-shellquote"
)

func (o *LocalOps) Exec(cmd ...string) *ExecOps {
	return newExec(o.ctx, runLocalExec, cmd)
}

func runLocalExec(ctx context.Context, call execCall) ([]byte, []byte, error) {
	var (
		stdoutBuffer bytes.Buffer
		stderrBuffer bytes.Buffer
		stdoutWriter io.Writer
		stderrWriter io.Writer
	)

	switch {
	case call.stdout != nil:
		stdoutWriter = call.stdout
	case call.captureStdout:
		stdoutWriter = &stdoutBuffer
	default:
		stdoutWriter = os.Stdout
	}

	switch {
	case call.stderr != nil:
		stderrWriter = call.stderr
	case call.captureStderr:
		stderrWriter = &stderrBuffer
	default:
		stderrWriter = os.Stderr
	}

	err := GetLocalBackend(ctx).Exec(ctx, ExecRequest{
		Cmd:        call.cmd,
		Dir:        ResolveLocalPath(ctx, "."),
		Env:        GetLocalExecEnv(ctx).Merge(call.env),
		Stdin:      call.stdin,
		Stdout:     stdoutWriter,
		Stderr:     stderrWriter,
		ExtraFiles: call.extraFiles,
		AfterStart: call.afterStart,
	})

	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}

func (realLocalBackend) Exec(ctx context.Context, req ExecRequest) error {
	fullCmd := shellquote.Join(req.Cmd...)
	dir := req.Dir

	log := Logger(ctx).With("cmd", fullCmd, "dir", dir)
	if req.Env.Len() != 0 {
		log = log.With("env", req.Env)
	}
	log.Debug("Local exec")

	c := exec.CommandContext(ctx, req.Cmd[0], req.Cmd[1:]...)

	c.Env = os.Environ()

	if req.Env.Len() != 0 {
		c.Env = mergeEnv(c.Env, req.Env.Environ())
	}

	c.Dir = dir
	c.Stdin = req.Stdin
	c.Stdout = req.Stdout
	c.Stderr = req.Stderr
	c.ExtraFiles = req.ExtraFiles

	err := c.Start()
	if err != nil {
		return err
	}

	if req.AfterStart != nil {
		req.AfterStart()
	}

	err = c.Wait()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return &ExecExitError{Status: exitErr.ExitCode(), Err: err}
		}

		return err
	}

	return nil
}
