# SSH transport

Gabric core has no SSH dependency. SSH support is provided by two packages:

- [`gabricssh`](gabricssh.md) is the Gabric integration: flags, context setup,
  runner, shell, port forwarding, tasks and the rsync fallback.
- [`sshremote`](sshremote.md) is the low-level SSH client: hosts, config,
  pooling, authentication and host keys. It is context-free.

## Install

```go
package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

`gabricssh.Install(g)` installs on an explicit `*gabric.Gabric`. Calling
`Install` twice panics.

## Flags

| Flag            | Meaning                            |
| --------------- | ---------------------------------- |
| `-ssh-host`     | SSH host, in `user@host:port` form |
| `-ssh-identity` | SSH identity file; repeatable      |

The `-H`, `-port` and `-i` flags were removed. Put the port in `-ssh-host` and
use `-ssh-identity` for keys.

## Ways to use it

| Goal                                               | API                                     |
| -------------------------------------------------- | --------------------------------------- |
| Run tasks with `-ssh-host`                         | `Install` plus `gabric.Remote(ctx)`     |
| Run one operation with a managed manager           | `gabricssh.WithRemote(ctx, host, next)` |
| Run one operation with existing config and manager | `gabricssh.Remote(ctx, host)`           |
| Make `TargetOps` and Reconcile modules use SSH     | `gabricssh.Target(host)`                |
| Run a whole scope with SSH setup                   | `WithSSH` plus `WithHost`               |
| Build a runner without installing context          | `gabricssh.Runner()`                    |

One-shot with managed config and manager:

```go
err := gabricssh.WithRemote(ctx, "deploy@example.com", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("uname", "-a").RunErr()
})
```

One-shot with existing config and manager:

```go
gabricssh.Remote(ctx, "deploy@example.com").Exec("uname", "-a").Run()
```

Target operations:

```go
ctx = gabric.NewTargetContext(ctx, gabricssh.Target("deploy@example.com"))
gabric.TargetOps(ctx).Exec("uname", "-a").Run()
```

For a whole run:

```go
err := gabric.Run(ctx, calls,
	gabricssh.WithSSH,
	gabricssh.WithHost("deploy@example.com"),
)
```

`WithSSH` installs config, manager, runner and the rsync fallback. `WithHost`
only sets the host. See [gabricssh.md](gabricssh.md#choosing-a-form).

## Example

[`examples/ssh/main.go`](../../examples/ssh/main.go) is a minimal runnable
example:

```bash
go run ./examples/ssh -ssh-host deploy@example.com hello_remote
```

It has `hello_local`, `hello_remote`, `hello`, `ssh` and `forward` tasks.

[`examples/apidemo/ssh/main.go`](../../examples/apidemo/ssh/main.go) runs the full
remote API demo:

```bash
go run ./examples/apidemo/ssh -ssh-host deploy@example.com api
```

## End-to-end

`./gab e2e-api-demo` runs the shared apidemo against Docker and SSH.
`./gab e2e` includes this task.

## Rsync

SSH uses the native rsync transport when available. `WithSSH` and
`NewRunnerContext` install the SSH rsh resolver for platforms where the native
transport is unavailable. A custom runner has no fallback.

## Shell and forwarding

- [shell.md](shell.md): the interactive shell task and terminal behavior.
- [forward.md](forward.md): the local port forwarding task and connection
  lifecycle.

## Without SSH

`gabric.Main()` without a transport runs tasks that use `gabric.Local(ctx)`.
The default target remains `Remote`, and remote operations need an installed
runner. Use `LocalRemote` or `gabricssh` for remote operations without and with
SSH.
