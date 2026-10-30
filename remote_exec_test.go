package gabric

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestBuildRemoteCommand(t *testing.T) {
	tests := []struct {
		name       string
		workingDir string
		sudo       *Sudo
		env        ExecEnv
		cmd        []string
		log        bool
		want       string
	}{
		{
			name: "plain",
			cmd:  []string{"echo", "hi"},
			want: "/bin/sh -c 'echo hi'",
		},
		{
			name:       "working dir",
			workingDir: "/srv/app",
			cmd:        []string{"ls"},
			want:       "/bin/sh -c 'cd /srv/app >/dev/null && ls'",
		},
		{
			name: "sudo",
			sudo: &Sudo{},
			cmd:  []string{"echo", "hi"},
			want: "sudo /bin/sh -c 'echo hi'",
		},
		{
			name:       "sudo and working dir",
			workingDir: "/srv/app",
			sudo:       &Sudo{},
			cmd:        []string{"echo", "hi"},
			want:       "sudo /bin/sh -c 'cd /srv/app >/dev/null && echo hi'",
		},
		{
			name: "sudo user and flags",
			sudo: &Sudo{User: "deploy", Flags: []string{"-n", "-E"}},
			cmd:  []string{"echo", "hi"},
			want: "sudo -n -E -u deploy /bin/sh -c 'echo hi'",
		},
		{
			name: "env",
			env:  NewExecEnv(Env("GABRIC_TEST_ENV", "value")),
			cmd:  []string{"echo", "hi"},
			want: "/bin/sh -c 'export GABRIC_TEST_ENV=value; echo hi'",
		},
		{
			name:       "env and working dir",
			workingDir: "/srv/app",
			env:        NewExecEnv(Env("GABRIC_TEST_ENV", "value")),
			cmd:        []string{"echo", "hi"},
			want:       "/bin/sh -c 'export GABRIC_TEST_ENV=value; cd /srv/app >/dev/null && echo hi'",
		},
		{
			name: "sudo and env",
			sudo: &Sudo{},
			env:  NewExecEnv(SensitiveEnv("GABRIC_TEST_SECRET", "secret")),
			cmd:  []string{"echo", "hi"},
			want: "sudo /bin/sh -c 'export GABRIC_TEST_SECRET=secret; echo hi'",
		},
		{
			name: "log env",
			env:  NewExecEnv(Env("GABRIC_TEST_ENV", "value")),
			cmd:  []string{"echo", "hi"},
			log:  true,
			want: "/bin/sh -c 'export GABRIC_TEST_ENV=value; echo hi'",
		},
		{
			name: "log sensitive env",
			env:  NewExecEnv(SensitiveEnv("GABRIC_TEST_SECRET", "two words")),
			cmd:  []string{"echo", "hi"},
			log:  true,
			want: "/bin/sh -c 'export GABRIC_TEST_SECRET=<sensitive>; echo hi'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildRemoteCommand(tt.workingDir, tt.sudo, tt.env, tt.cmd, tt.log); got != tt.want {
				t.Errorf("buildRemoteCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoteExecEnvPrefix(t *testing.T) {
	env := NewExecEnv(
		Env("A", "1"),
		Env("B", "two words"),
		Env("C", "it's"),
	)

	if got, want := remoteExecEnvPrefix(env, false), `export A=1 B='two words' C=it\'s; `; got != want {
		t.Errorf("remoteExecEnvPrefix() = %q, want %q", got, want)
	}
	if got := remoteExecEnvPrefix(ExecEnv{}, false); got != "" {
		t.Errorf("remoteExecEnvPrefix(empty) = %q, want empty", got)
	}
}

func TestRemoteExecEnvPrefixLog(t *testing.T) {
	env := NewExecEnv(
		Env("A", "1"),
		Env("B", "two words"),
		SensitiveEnv("TOKEN", "two words"),
	)

	if got, want := remoteExecEnvPrefix(env, true), `export A=1 B='two words' TOKEN=<sensitive>; `; got != want {
		t.Errorf("remoteExecEnvPrefix(log) = %q, want %q", got, want)
	}
	if got := remoteExecEnvPrefix(ExecEnv{}, true); got != "" {
		t.Errorf("remoteExecEnvPrefix(empty, log) = %q, want empty", got)
	}
}

func TestRemoteExecSudoSpec(t *testing.T) {
	cmdCh := make(chan string, 1)

	_, _, ctx := localRemoteContextWithExec(t, func(cmd string, _ io.Reader, _, _ io.Writer) int {
		select {
		case cmdCh <- cmd:
		default:
		}
		return 0
	}, false)
	ctx = NewRemoteSudoContext(ctx, &Sudo{User: "deploy", Flags: []string{"-n"}})

	if err := Remote(ctx).Exec("echo", "hi").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	select {
	case cmd := <-cmdCh:
		if want := "sudo -n -u deploy /bin/sh -c 'echo hi'"; cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
	default:
		t.Fatal("exec handler was not called")
	}
}

func TestRemoteExecCapture(t *testing.T) {
	_, _, ctx := localRemoteContextWithExec(t, func(_ string, _ io.Reader, stdout, stderr io.Writer) int {
		io.WriteString(stdout, "out")
		io.WriteString(stderr, "err")
		return 0
	}, false)

	e := Remote(ctx).Exec("ignored").Capture()

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if e.StdoutStr() != "out" {
		t.Errorf("StdoutStr() = %q, want %q", e.StdoutStr(), "out")
	}

	if e.StderrStr() != "err" {
		t.Errorf("StderrStr() = %q, want %q", e.StderrStr(), "err")
	}
}

func TestRemoteExecStdoutWriter(t *testing.T) {
	_, _, ctx := localRemoteContextWithExec(t, func(_ string, _ io.Reader, stdout, _ io.Writer) int {
		io.WriteString(stdout, "out")
		return 0
	}, false)

	var buf bytes.Buffer

	if err := Remote(ctx).Exec("ignored").Stdout(&buf).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got := buf.String(); got != "out" {
		t.Errorf("Stdout(w) = %q, want %q", got, "out")
	}
}

func TestRemoteExecStdin(t *testing.T) {
	_, _, ctx := localRemoteContextWithExec(t, func(_ string, stdin io.Reader, stdout, _ io.Writer) int {
		io.Copy(stdout, stdin)
		return 0
	}, false)

	out := Remote(ctx).Exec("cat").Stdin(strings.NewReader("input")).CaptureStdout().Run().StdoutStr()

	if out != "input" {
		t.Errorf("StdoutStr() = %q, want %q", out, "input")
	}
}

func TestRemoteExecExitError(t *testing.T) {
	_, _, ctx := localRemoteContextWithExec(t, func(_ string, _ io.Reader, _, _ io.Writer) int {
		return 3
	}, false)

	err := Remote(ctx).Exec("ignored").RunErr()

	if exitErr, ok := errors.AsType[*ExecExitError](err); !ok {
		t.Fatalf("error = %T, want *ExecExitError", err)
	} else if exitErr.Status != 3 {
		t.Errorf("Status = %d, want 3", exitErr.Status)
	}
}

func TestRemoteExecSudo(t *testing.T) {
	cmdCh := make(chan string, 1)

	_, _, ctx := localRemoteContextWithExec(t, func(cmd string, _ io.Reader, _, _ io.Writer) int {
		select {
		case cmdCh <- cmd:
		default:
		}
		return 0
	}, true)

	if err := Remote(ctx).Exec("echo", "hi").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	select {
	case cmd := <-cmdCh:
		if want := "sudo /bin/sh -c 'echo hi'"; cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
	default:
		t.Fatal("exec handler was not called")
	}
}
