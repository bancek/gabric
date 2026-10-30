package gabric

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
)

func (o *RemoteOps) Exec(cmd ...string) *ExecOps {
	return newExec(o.ctx, runRemoteExec, cmd)
}

func buildRemoteCommand(workingDir string, sudo *Sudo, env ExecEnv, cmd []string, log bool) string {
	fullCmd := shellquote.Join(cmd...)

	if workingDir != "" {
		fullCmd = shellquote.Join("cd", workingDir) + " >/dev/null && " + fullCmd
	}

	if prefix := remoteExecEnvPrefix(env, log); prefix != "" {
		fullCmd = prefix + fullCmd
	}

	fullParts := sudo.WrapCommand([]string{"/bin/sh", "-c", fullCmd})

	return shellquote.Join(fullParts...)
}

// remoteExecEnvPrefix returns export assignments for env. It is inserted
// inside the wrapped shell command so sudo cannot drop it. When log is true,
// sensitive values are replaced by the redaction marker without shell quoting.
func remoteExecEnvPrefix(env ExecEnv, log bool) string {
	if env.Len() == 0 {
		return ""
	}

	entries := env.Environ()
	if log {
		entries = env.EnvironLog()
	}

	assignments := make([]string, 0, len(entries))

	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		if log && env.IsSensitive(key) {
			assignments = append(assignments, key+"="+value)
			continue
		}

		assignments = append(assignments, key+"="+shellquote.Join(value))
	}

	return "export " + strings.Join(assignments, " ") + "; "
}

func runRemoteExec(ctx context.Context, call execCall) (stdout []byte, stderr []byte, err error) {
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

	workingDir := GetRemoteWorkingDir(ctx)
	env := GetRemoteExecEnv(ctx).Merge(call.env)
	backendCtx := withoutRemoteWorkingDir(ctx)

	err = GetRemoteBackend(backendCtx).Exec(backendCtx, ExecRequest{
		Cmd:    call.cmd,
		Dir:    workingDir,
		Env:    env,
		Stdin:  call.stdin,
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
	})

	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}

func (realRemoteBackend) Exec(ctx context.Context, req ExecRequest) error {
	sudo := GetRemoteSudo(ctx)
	fullCmd := buildRemoteCommand(req.Dir, sudo, req.Env, req.Cmd, false)

	Logger(ctx).
		With("cmd", buildRemoteCommand(req.Dir, sudo, req.Env, req.Cmd, true), "dir", req.Dir).
		Debug("Remote exec")

	return GetRemoteRunner(ctx)(ctx, fullCmd, req.Stdin, req.Stdout, req.Stderr)
}

func (r realRemoteBackend) execQuiet(ctx context.Context, cmd []string, stdin io.Reader) error {
	dir := GetRemoteWorkingDir(ctx)

	return r.Exec(ctx, ExecRequest{
		Cmd:    cmd,
		Dir:    dir,
		Env:    GetRemoteExecEnv(ctx),
		Stdin:  stdin,
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}
