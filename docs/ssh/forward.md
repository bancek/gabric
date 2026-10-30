# Local port forwarding over Gabric

Gabric can open an SSH **local port forward** (`ssh -L`) using its own SSH
stack, so all SSH access is configured through `gabricssh` and `sshremote`
(`-ssh-host`, ssh_config, keys, jump hosts, known_hosts) instead of the system
`ssh` binary.

The motivating use case is reaching a service bound to `127.0.0.1` on the
remote host, such as a web UI, an admin console, or a database, from the
operator's machine without exposing it publicly. Define a task for the service
and run it:

```bash
go run . -ssh-host deploy@example.com forward:127.0.0.1:8080   # then open http://127.0.0.1:8080
```

## Usage

Register the optional task and run it:

```go
package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

var Forward = gabricssh.RegisterForwardTask(gabric.Default)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

```bash
# bind 127.0.0.1:9090 locally, forward to 127.0.0.1:8080 as seen by the host
go run . -ssh-host deploy@example.com forward:127.0.0.1:9090,127.0.0.1:8080

# a single address applies to both sides (local 127.0.0.1:8080)
go run . -ssh-host deploy@example.com forward:127.0.0.1:8080
```

From library code, use `gabricssh.Forward`:

```go
gabricssh.Forward(ctx).
	Local("127.0.0.1:8080").
	Remote("127.0.0.1:8080").
	Run()                                 // panic-style
err := gabricssh.Forward(ctx).
	Local("127.0.0.1:0").
	Remote("127.0.0.1:8080").
	RunErr()                              // error-first
```

`Run`/`RunErr` block until `ctx` is cancelled, forwarding connections as they
arrive. `RunErr` returns `ctx.Err()` on cancellation; per-connection errors
(such as a refused remote target) are logged and do not stop the forward.

The builder mirrors `gabricssh.Shell` and `Rsync`:

| Method          | Default       | Meaning                                                |
| --------------- | ------------- | ------------------------------------------------------ |
| `Local(addr)`   | `127.0.0.1:0` | local bind address; `0` picks an ephemeral port        |
| `Remote(addr)`  | required      | target `host:port` as reached from the SSH server      |
| `AllowRemote()` | off           | permit a non-loopback `Local` bind                     |
| `LocalAddr()`   | n/a           | the actual bound address (useful when the port is `0`) |

The actual bound address is logged at info level when the listener starts:

```text
level=INFO msg="Local forward listening" local=127.0.0.1:8080 remote=127.0.0.1:8080
```

## Connection ownership

A forward owns a **dedicated** SSH connection, created with
`sshremote.WithNewRemoteClient` (so hosts, keys, jump hosts, host keys and config
all apply), rather than borrowing one from the manager's pool. This matters
because a forward is long-lived:

- It is not counted against the `gabricssh` manager's `MaxConnections`, so it cannot
  starve other operations on the same host.
- Cancelling the forward closes only its own connection. The manager's pooled
  connections (and any operations using them) are unaffected.

## Safety

A port forward is an easy way to expose an internal service by accident, so the
local side binds loopback (`127.0.0.1`) by default and a non-loopback bind is
refused unless `AllowRemote()` is called:

```go
gabricssh.Forward(ctx).
	AllowRemote().          // explicit opt-in
	Local("0.0.0.0:8080").
	Remote("127.0.0.1:8080").
	Run()
```

## How it works

`ForwardOps.RunErr`:

1. Validates the host, the remote address, and (unless `AllowRemote`) that the
   local address is loopback.
2. Binds the local listener with `net.Listen("tcp", localAddr)` and records the
   actual address.
3. Acquires a dedicated connection via `sshremote.WithNewRemoteClient` and runs the
   accept loop with it; cancellation closes the connection.
4. For each accepted connection, opens a `direct-tcpip` channel with
   `client.DialContext(ctx, "tcp", remoteAddr)` and copies bytes in both
   directions (`io.Copy`), closing both ends when either finishes.
5. On `ctx` cancellation, closes the listener, drains in-flight connections and
   returns `ctx.Err()`.

It mirrors `sshremote.ForwardTransport`, which reaches another address _through
a jump host_ while establishing the client connection. A local forward binds a
**local** listener and sends accepted connections to an address as reached from
the SSH server.

## Limitations

- One forward per `Run`. Run several in goroutines (each with its own context)
  to forward multiple ports.
- No dynamic forwarding / SOCKS5 (`ssh -D`) and no remote forwarding (`ssh -R`).
- No `~` escape sequences; stop it with Ctrl-C.
- The local listener is plain TCP; there is no per-connection authentication
  beyond what the remote target requires.

## Testing

- `internal/sshtest` implements `direct-tcpip` channels (`WithDirectTCPIP`).
- `gabricssh/forward_test.go` runs a local echo server, forwards an ephemeral
  local port to it through `sshtest`, and asserts a round trip, the
  dedicated-vs-pooled connection behavior, cancellation, the loopback default
  and task registration.
- `internal/gabricsshe2e` forwards to a real OpenSSH server in Docker and reaches it
  through a `sshremote.DialTransport` bound to the forwarded address.
