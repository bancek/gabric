# Interactive shell over Gabric

Gabric can open an interactive login shell on a remote host using its own SSH
stack, so all SSH access is configured through `gabricssh` and `sshremote`
(`-ssh-host`, ssh_config, keys, jump hosts, known_hosts) instead of the system
`ssh` binary.

## Usage

Register the optional task and run it:

```go
package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

var Ssh = gabricssh.RegisterSshTask(gabric.Default)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

```bash
go run . -ssh-host deploy@example.com ssh
```

From library code, use `gabricssh.Shell`:

```go
gabricssh.Shell(ctx).Run()               // panic-style
err := gabricssh.Shell(ctx).RunErr()     // error-first
```

The builder mirrors `Exec`/`Rsync` and supports overriding the defaults:

```go
gabricssh.Shell(ctx).
	Stdin(customIn).
	Stdout(customOut).
	Stderr(customErr).
	Term("vt100").
	Size(120, 40).
	Pty(false).
	Run()
```

| Method                    | Default                                          |
| ------------------------- | ------------------------------------------------ |
| `Stdin`/`Stdout`/`Stderr` | `os.Stdin`/`os.Stdout`/`os.Stderr`               |
| `Term`                    | `$TERM`, else `xterm`                            |
| `Size`                    | local terminal size, else 80×24                  |
| `Pty`                     | on when stdin is a terminal (derived by `Stdin`) |

Defaults are captured when `gabricssh.Shell(ctx)` is called. `Stdin` derives the
PTY default from the input; call `Pty(true)`/`Pty(false)` after `Stdin` to
override (for example to force a PTY with piped input, or disable it for a
terminal). When stdin is not a terminal the login shell runs without a PTY and
exits at EOF, so commands can be piped in:

```bash
echo "echo hello" | go run . -ssh-host deploy@example.com ssh
```

## How it works

`ShellOps.RunErr` uses the pooled connection from the `gabricssh` manager
(`gabricssh.GetRemote(ctx).WithSession`) and:

1. Puts the local terminal into raw mode when stdin is a TTY (best effort),
   restoring it with `defer` when the shell exits or panics.
2. Requests a remote PTY (when enabled) with
   `session.RequestPty(term, height, width, modes)`.
3. Wires the process stdio to the session.
4. Forwards terminal resizes: `SIGWINCH` on the local TTY re-reads the size with
   `term.GetSize` and sends `session.WindowChange(height, width)`.
5. Starts the login shell with `session.Shell()` and waits for it
   (`session.Wait()`). A non-zero remote exit status is returned as
   `*gabric.ExecExitError` and still unwraps to `*ssh.ExitError`.

Because the session lives on a pooled connection, the host, private keys,
proxy/jump hosts and known hosts all come from Gabric's context-scoped remote
configuration.

## Terminal behavior

Raw mode disables the local line discipline (`ICANON`, `ECHO`, `ISIG`), so the
remote PTY owns editing, echo, and signals, as it does in `ssh`:

- **Ctrl-C** is forwarded as `0x03`; the _remote_ PTY delivers `SIGINT` to the
  remote foreground process. Gabric itself is not interrupted.
- **Ctrl-D** is forwarded as `0x04`; the remote shell sees EOF and logs out.
- **Ctrl-Z**, arrows, backspace, etc. are passed through to the remote.

Without raw mode the local terminal would instead buffer input until Enter,
echo locally, and turn Ctrl-C into a local `SIGINT`. Raw mode prevents that.

When stdin is not a TTY (a pipe, or in tests) there is no line discipline, raw
mode is skipped, and the shell still works.

## Message of the day

`gabricssh.Shell(ctx)` sends an SSH **shell** request, so sshd treats it as an
interactive login: the message of the day (and PAM login messages) are printed,
as with `ssh host`, including with piped stdin (`echo cmd | ssh host`)
and `ssh -T host`. OpenSSH only skips these when a **command** is given
(`check_quietlogin` returns quiet for `command != NULL`).

To run without the MOTD, run a command instead of a shell. `Remote(ctx).Exec(...)`
maps to an `exec` request (like `ssh host cmd`). Server-side options are
`PrintMotd no` / `UsePAM no`, or a remote `~/.hushlogin` file.

## Relationship to `gabricssh`

It uses the same host resolution and connection pooling as
`gabric.Remote(ctx).Exec`, `Put` and rsync. `-ssh-host` (or
`gabricssh.ResolveHost`), `~/.ssh/config`, private keys, the ssh agent,
`ProxyJump`, `ProxyCommand` and known_hosts are all honored.
There is no dependency on the system `ssh` command.

Host key verification can be customized with `Config.WithHostKeyCallback` (used
regardless of the host key policy); see
[sshremote.md](sshremote.md#host-key-verification) for fingerprint
logging and trust-on-first-use examples. See
[sshremote.md](sshremote.md#how-ssh_config-matching-works) for ssh_config precedence
and `IdentityFile` behavior.

## Limitations

- No local signal-cleanup handler: the terminal is restored via `defer` on
  normal return and panics, but not if the process is killed.
- No `~` escape sequences (`~.`, `~^Z`); this is a plain forwarded session.
- Starts the login shell in the remote home directory; it ignores the sudo and
  working-directory context settings (like `ssh host`).
- Resize handling uses `SIGWINCH`, which is unix-only; on other platforms the
  initial size is used for the whole session.

## Testing

- `internal/sshtest` accepts `pty-req`, `window-change` and `shell` requests and
  records the PTY term/size and the last window size.
- `gabricssh/shell_test.go` exercises the builder against `sshtest` with
  injected IO: a non-PTY command round-trip, PTY negotiation (`Term`/`Size`),
  resize forwarding and exit-status wrapping.
- `gabricssh/tasks_test.go` covers task registration.
- `internal/gabricsshe2e` runs `gabricssh.Shell(ctx)` against a real OpenSSH
  server in Docker, with and without a PTY.
- Raw mode and `TIOCGWINSZ` need a real local PTY, so they are not unit-tested;
  the code path is covered by the e2e `Pty(true)` test and manual use.
