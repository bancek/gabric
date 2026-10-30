# The rsync transport

On Unix, Gabric runs rsync without the system `ssh` binary: the remote shell
rsync is given is the Gabric binary itself, re-executed in a reserved sidecar
mode. The sidecar only proxies rsync's stdio over inherited file descriptors;
the Gabric process that started local rsync owns the remote side. This works
with both GNU rsync and openrsync.

Both paths require a local `rsync`. Windows and other platforms without
inherited file descriptors use the rsh resolver installed by the transport.
`gabricssh` installs an SSH fallback; it also requires local `ssh`.
`gabricdocker` and a custom remote runner such as `LocalRemote` install
`NoRsyncRsh`, so they need the native transport. This document explains the native
transport and why it works this way. It is a companion to the rsync section of
[architecture.md](architecture.md) and to
[ssh/gabricssh.md](ssh/gabricssh.md).

```text
Gabric process                         sidecar (same binary, re-exec)
  rsync --rsh '<gabric> gabric-rsync-rsh' ...
        │                                        │
        │  fd 3  socketpair (protocol, both ways)│
        │  fd 4  pipe  (argv, rsync -> Gabric)   │
        │  fd 5  pipe  (status, Gabric -> sidecar)
        ▼                                        ▼
      rsync  ── stdio ────────────────────────►  sidecar
        │                                        │
        └────── Gabric remote runner ────────────┘
            (runner installed by the transport,
             SSH via gabricssh)
              remote `rsync --server …`
```

---

## How rsync drives a remote shell

When the destination (or source) is `host:path`, rsync builds the remote server
command and hands it to the `--rsh` program. The two implementations behave
differently, and the differences drove most of the design.

### GNU rsync

From `main.c::do_cmd` and `pipe.c::piped_child` (verified against 3.2.7):

- `--rsh` is `execvp`'d directly, after being split into arguments with a
  simple quote-aware parser (`'` and `"` pairs only, no backslash escapes).
- The remote command is passed as **separate argv elements**:
  `[rsh…, -l <user>, <machine>, <rsync_path>, --server, <opts…>, ".", <path>]`.
  `--rsync-path=sudo rsync` arrives as one element, `"sudo rsync"`.
- stdio is wired to the child's stdin/stdout via **two pipes**.
- The child's stdin is set blocking; stdout may be non-blocking.
- Inherited file descriptors are **not** closed, so extra fds survive into the
  remote shell.

### openrsync (the macOS default)

From `main.c` and `fargs.c`:

- `--rsh`/`-e` is split on **whitespace only, with no quote handling**. A quoted
  shell script is torn into many argv tokens.
- The remote command is again separate argv elements, but the shape is
  `[rsh…, <host>, <rsync_path>, --server, <opts…>, …]`. There is **no
  `-l <user>`** (the user is part of `<host>`).
- stdio is **one bidirectional `socketpair`**, dup2'd to both fd 0 and fd 1.
- That socketpair is **non-blocking**.
- Inherited file descriptors are not closed either.

Two consequences matter:

1. Any inline shell script in `--rsh` must survive both parsers. GNU's is
   quote-aware; openrsync's is not. That is not portable.
2. The remote shell must be able to cope with non-blocking fds. `cat` cannot;
   a Go program can (Go's runtime poller handles `EAGAIN`).

---

## The design

### Sidecar mode

`rsyncRshCommand` sets `--rsh` to the current executable plus a reserved first
argument:

```text
--rsh '<gabric> gabric-rsync-rsh'
```

That string is a two-token command for both rsync parsers. The reserved
argument is detected in a Gabric package `init()` (`rsync_sidecar.go`), which
runs before the user's `main`-package `init()`:

```go
func init() {
	if len(os.Args) >= 2 && os.Args[1] == rsyncSidecarArg {
		os.Exit(runRsyncSidecar(os.Args[2:]))
	}
}
```

The sidecar only proxies, so the remote transport stays entirely in the Gabric
process and remains fully customizable.

### File descriptors

The Gabric process creates three fds and passes them to local rsync with
`ExtraFiles`, which places them at fd 3, 4 and 5 in rsync. rsync passes them on
to the sidecar:

| fd  | kind         | direction        | purpose                |
| --- | ------------ | ---------------- | ---------------------- |
| 3   | `socketpair` | both             | raw rsync protocol     |
| 4   | pipe         | rsync → Gabric   | the NUL-framed argv    |
| 5   | pipe         | Gabric → sidecar | the remote exit status |

The socketpair is created **non-blocking** on purpose: `os.File.Close` can then
unblock the SSH session's stdin copier through the runtime poller. A blocking
socketpair can make `Close` itself hang while another goroutine is blocked in a
read.

### The sidecar loop

```text
write:  <count>\0 <arg>\0 <arg>\0 …   → fd 4, then close fd 4
spawn:  copy stdin → fd 3; on EOF, shutdown(fd 3, SHUT_WR)
main:   copy fd 3 → stdout
exit:   read one line from fd 5, exit with it
```

### The Gabric side

`rsyncTransport.Serve`:

1. reads the framed argv from fd 4,
2. joins the tokens after the host into the remote shell command (this handles
   GNU's `-l user host …` and openrsync's `host …` alike),
3. runs it through `GetRemoteRunner(ctx)`, binding the socketpair to the
   runner's stdin/stdout and `os.Stderr` to its stderr (the `gabricssh` runner
   uses a pooled session),
4. closes the socketpair and writes the remote exit status to fd 5. The status
   comes from the runner's `*ExecExitError`.

`RsyncOps.RunErr` starts local rsync and `Serve` concurrently, joins the errors,
and prefers the remote `*ExecExitError` when the remote command failed.

---

## Why not the alternatives

- **System `ssh` (`--rsh 'ssh …'`).** The original behaviour. Correct, but it
  ties rsync to the system `ssh` and its config, which the design removes. It
  also requires `known_hosts` handling to line up with Gabric's own host-key
  checking.
- **Inline shell script (`--rsh "sh -c '…'"`).** The first attempt. Fails on
  openrsync (whitespace-only splitting) and on non-blocking fds (`cat` gets
  `EAGAIN`). Also fragile to quote.
- **A `ProxyCommand` that points at Gabric.** Leaves the SSH work in the system
  `ssh` binary, so it still depends on `ssh`.
- **Passing a connected socket path and using `socat`/`nc`.** Needs an external
  connector binary that cannot be assumed.
- **The sidecar in the same process.** rsync insists on spawning a program for
  the remote shell; there is no fd-passing entry point. The sidecar is the
  smallest thing that can be spawned while keeping SSH in the parent.

---

## Problems encountered (and their fixes)

These bugs shaped the final design, roughly in the order they appeared. Each
one is a trap for anyone reimplementing this.

### 1. `cat: stdin: Resource temporarily unavailable`

rsync leaves the remote shell's fds non-blocking (it expects `ssh` to retry on
`EAGAIN`). `cat` treats `EAGAIN` as fatal. Confirmed by dumping fd flags from
the helper with `python3 -c 'fcntl…'`: fd 0 was `O_RDWR|O_NONBLOCK`.

_Fix:_ use a Go sidecar, which reads/writes non-blocking fds through the
runtime poller. (`--blocking-io` was tried first; it is GNU-only and does not
address openrsync.)

### 2. openrsync shreds a quoted `--rsh`

openrsync splits `-e` on whitespace only. The quoted `sh -c '…'` script became
dozens of bogus argv tokens. The inline-script approach therefore cannot work,
which motivated the sidecar. Its `--rsh` value is two harmless tokens.

### 3. Args pipe never reached EOF

`readRsyncServerCommand` first used `io.ReadAll` on the args pipe. It hung
forever because the intermediate rsync process inherits the **write** end of
that pipe and keeps it open for the whole transfer; EOF does not arrive until
after the transfer has finished.

_Fix:_ frame the argv with a leading count and read exactly that many
NUL-terminated fields. Framing also tolerates the still-open write end.

### 4. Remote command never exited, no exit-status

Nothing half-closed the SSH channel's stdin when local rsync finished, so the
remote exec path (`/bin/sh -c "rsync --server …"`) never saw EOF. That kept the
remote process's stdin copier alive, so `cmd.Wait` never returned and the SSH
server never sent exit-status; the client's `Session.Wait` hung.

_Fix:_ the sidecar calls `shutdown(fd 3, SHUT_WR)` when its stdin hits EOF, so
`session.Stdin` returns EOF and the ssh library half-closes the channel.

### 5. The test harness deadlocked on `os/exec` stdin

`internal/sshtest`'s exec handler ran the command with `cmd.Stdin = ch` (the SSH
channel). `os/exec` spawns a goroutine to copy stdin and waits for it in
`cmd.Wait`; since the channel stays open, the handler never returned. Real sshd
reaps the process without waiting on a stdin copier.

_Fix (test-only):_ give the child an `*os.File` stdin (via `os.Pipe`) so
`os/exec` passes the fd directly and does not wait on a copier.

### 6. `dataParent.Close()` itself blocked

The socketpair fds were blocking, so Go could not interrupt the SSH stdin
copier's in-flight `read` when the Gabric side closed the socket. `Close`
blocked, the sidecar never got EOF, and openrsync did not finish.

_Fix:_ create the socketpair non-blocking, so the runtime poller owns it,
`Close` unblocks readers, and the transfer completes.

### Debugging notes

Useful when debugging this code:

- dump fd flags from inside the spawned helper (`fcntl(F_GETFL)` via
  `python3`/`perl`), and check which fds exist via `/dev/fd/N`;
- log stages to a temp file from both the sidecar and the parent;
- `ps -axo pid,ppid,command` to see the rsync → sidecar process tree;
- `sample <pid>` to see which syscall a process is parked in;
- `SIGQUIT` for a Go goroutine dump.

---

## Limitations

- **Unix only.** It needs inherited file descriptors. Elsewhere Gabric uses the
  rsh resolver installed by the transport. `gabricssh` provides an SSH fallback;
  `gabricdocker` and custom remote runners install `NoRsyncRsh`, so rsync fails
  instead.
- **One extra short-lived process per transfer** (the same binary in sidecar
  mode). It re-execs the user's binary, so the user's `main`-package `init` is
  skipped, but packages imported before Gabric still initialize.
- **openrsync cannot handle an executable path containing spaces** (its `-e`
  splitter ignores quotes).
- **No daemon mode / `--protect-args`.** The transport assumes the plain
  remote-shell path, which is all Gabric uses.
