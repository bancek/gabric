# The `sshremote` package

The `sshremote` package implements SSH: hosts and resolution, connection
configuration, pooling, authentication and host key verification. It is the
low-level client. Gabric's SSH integration, including `Remote(ctx)` operations,
the interactive shell and port forwarding, is in [`gabricssh`](gabricssh.md).

The package is **context-free**: it reads no `context.Context` values.
`gabricssh` keeps the host and immutable connection config in its context,
combines them into a `HostConfig`, and passes that plus an explicit
`*slog.Logger` to operations, so `sshremote` can also be used standalone. A
different config instance selects a different `Remote` and pool; the config also
carries the pooling limits.

For the context layout see
[../architecture.md](../architecture.md#context-is-the-execution-environment).

## Overview

| Type            | Role                                                                                           |
| --------------- | ---------------------------------------------------------------------------------------------- |
| `Host`          | An SSH target plus per-host ssh_config values: hostname, port, user, keys, known hosts, proxy. |
| `Config`        | Immutable defaults: keys, ssh_config/known_hosts, agent, timeouts, pool limits.                |
| `HostConfig`    | Resolves effective settings from a `Host` and a `Config`.                                      |
| `RemoteManager` | Caches one `Remote` per host and config identity.                                              |
| `Remote`        | A pooled SSH connection to one host and config.                                                |
| `RemoteClient`  | A single SSH client, carried by a dialer (direct, jump, command, session or stdio).            |

`Remote` exposes `WithSession` (taking a `*slog.Logger`). Gabric uses it to run
remote commands. The `Remote` owns its connection config. File operations
are shell/coreutils commands over the same sessions; there is no SFTP path.
Callers that need a raw `*ssh.Client` use `NewRemoteClient` or
`WithNewRemoteClient` for a dedicated connection.

### Advanced API

These escape hatches are supported for custom integrations but are not needed
for normal operation:

- `RemoteClient`, `NewRemoteClient` and `WithNewRemoteClient` manage a dedicated
  connection outside the pool.
- `NewRemote` constructs a pooled `Remote` without a manager.
- `HostSSHAuth` and `GetSSHSigners` build authentication methods for custom SSH
  clients.
- `HostConfig.SSHHost`, `SSHArgs`, `SSHCommand` and `ProxyCommand` render
  system-ssh fallback arguments.

Normal code should use `RemoteManager.GetRemote`, `Remote.WithSession`, or
`WithNewRemoteClient` for a scoped dedicated connection.

## Hosts and resolution

`Config.ResolveHost(hostStr)` parses the host string and merges matching
`~/.ssh/config` settings:

```go
config, _ := sshremote.NewDefaultConfig()

host, _ := config.ResolveHost("deploy@example.com")
hc := sshremote.NewHostConfig(host, config)
remote := sshremote.NewRemote(hc)
defer remote.Close()
```

Gabric code normally uses `gabricssh.ResolveHost` or
`gabricssh.NewHostNameContext`, which resolve through the context config. See
[gabricssh.md](gabricssh.md).

`ResolveHost` merges `Hostname`, `User`, `Port`, every matching `IdentityFile`,
`UserKnownHostsFile`, `StrictHostKeyChecking`, `ProxyCommand` and `ProxyJump`,
and expands a leading `~`. It stores only the ssh_config-derived values on the
`Host`; it does not copy `Config` defaults. A `UserKnownHostsFile` value may
list several whitespace-separated files; only the first matching directive is
used, matching OpenSSH. `sshremote.ParseHost("user@host:2222")` parses without
ssh_config.

`Host` is immutable. Use `NewHost`/`With...` to build one. `Port()` returns the
effective port (22 by default) and `User()` the effective user (the current OS
user when unset). `Clone()` returns an independent copy; the `Transport` is kept
as-is, so clone or derive the jump host separately if it needs to change.

`HostConfig` resolves a `Host` and a `Config` at use time:

```go
hc := sshremote.NewHostConfig(host, config)
fmt.Println(hc.PrivateKeysPaths())
fmt.Println(hc.KnownHostsPaths())
fmt.Println(hc.StrictHostKeyChecking())
```

The precedence is:

- **private keys**: the host keys first, then the config keys;
- **known_hosts**: the host value when set, otherwise the config defaults;
- **strict checking**: the host value when set, otherwise the config default;
- **host key callback**: the config callback when set, otherwise the resolved
  strict policy and known_hosts files.

### How ssh_config matching works

`github.com/kevinburke/ssh_config` parses root-level directives into an implicit
`Host *` at the beginning of the file. It then applies:

- `Get(alias, key)` walks matching blocks in file order and returns the **first**
  value;
- `GetAll(alias, key)` walks all matching blocks in file order and returns
  **every** value;
- matching covers root directives, exact hosts, wildcards, negation and
  `Match Host`.

For example:

```sshconfig
# root defaults
IdentityFile ~/.ssh/id_ed25519
UserKnownHostsFile ~/.ssh/known_hosts
StrictHostKeyChecking yes

Host *.internal
    IdentityFile ~/.ssh/internal_key

Host app.internal
    User deploy
```

For `app.internal`:

- `IdentityFile` returns both `id_ed25519` and `internal_key`, in file order.
  `IdentityFile` is additive; a host-specific value does not replace a root
  value.
- `StrictHostKeyChecking` and `UserKnownHostsFile` use first-match semantics.
  The root value wins over a later host-specific value. A wildcard block before
  an exact block also wins.
- `HostName`, `User` and `Port` are also first-match.
- To make a host-specific single-value setting win, put its block before the
  more general block, matching OpenSSH.
- `Include` ordering is approximate; the library searches included files
  unordered.

`ResolveHost` uses `GetAll` for `IdentityFile` and `Get` for
`UserKnownHostsFile`, `StrictHostKeyChecking`, `HostName`, `User` and `Port`.

### Jump hosts

A `Host` reaches its destination through `Transport`: `nil` connects directly,
and `sshremote.JumpTransport{Host: jump}` tunnels through another SSH host (Gabric
dials it directly). `ProxyCommand` keeps the original ssh_config value for the
system-ssh fallback (rsync `--rsh`).

`ProxyJump` is read from ssh_config and resolved into the `Transport` jump chain,
matching OpenSSH. A comma-separated list (`ProxyJump a,b`) is a chain: Gabric
dials `a`, then `b`, then the target. `target.Transport` jumps to `b` and
`b.Transport` jumps to `a`. The `none` value disables it.

Nested jump configuration follows OpenSSH: only the outermost (first) listed hop
follows its own `ProxyCommand`/`ProxyJump`; later hops' proxy directives are
suppressed by the explicit chain, but the hops themselves are still used. So
`ProxyJump a,b` with `Host a / ProxyJump c` dials `c → a → b → target`.

Jump chains are resolved recursively; a loop (e.g. `Host a / ProxyJump b` with
`Host b / ProxyJump a`) is reported as an error instead of recursing forever.

`ProxyCommand` is recognized when it invokes ssh as `ssh [flags] <dest>`,
`ssh [flags] -W %h:%p <dest>`, or `ssh [flags] <dest> nc %h %p` (flags `-l`,
`-p`, `-i`), and resolved to a jump host. When the forwarded address is not the
target itself (`ssh [flags] -W <host:port> <dest>`, `ssh [flags] <dest> nc <host>
<port>`), it becomes a `sshremote.ForwardTransport` through that jump. A trailing
command that is not a recognized connector (`ssh [flags] <dest> <cmd>`) becomes a
`sshremote.SessionCommandTransport`: `<cmd>` runs in a session on `dest` and its
stdio is the transport. A standalone `nc <host> <port>` is recognized as a
`sshremote.DialTransport`, which dials that address directly (the target keeps its
own SSH identity); `nc %h %p` is a direct connection. Any other value
(unmodeled ssh flags, other commands) is preserved verbatim in
`Host.ProxyCommand()` and executed locally as a `sshremote.CommandTransport`, run
with `$SHELL -c` on unix, with `%h`/`%p`/`%r`/`%n`/`%%` expanded, and its
process group killed on close, as ssh does. `ProxyCommand -` uses the process
stdin/stdout as the transport (`sshremote.StdioTransport`).

`ProxyCommand` and `ProxyJump` are mutually exclusive: whichever appears first in
the config wins (matching OpenSSH). Ordering through an `Include` is approximate.

## `Config`

`Config` carries the connection settings:

`Config` is immutable. Settings are read through getters such as
`PrivateKeysPaths()`, `SSHConfigPath()`, `SSHConfigContent()`,
`KnownHostsPaths()`, `AgentSocket()`, `StrictHostKeyChecking()`,
`ConnectionTimeout()`, `SessionOpenTimeout()`, `IdleConnections()` and
`MaxConnections()`. Changes use `With...` methods and return a new `*Config`.

- `PrivateKeysPaths`: default private keys, the files from
  `DefaultSSHPrivateKeyFiles` (`~/.ssh/id_ed25519`, `id_ecdsa`, `id_rsa`) that
  exist. `HostConfig` tries them after the host's ssh_config `IdentityFile`
  entries.
- `SSHConfigPath`: the ssh_config file. `SSHConfigContent`: in-memory ssh_config
  content; setting it clears the path, and setting the path clears it.
  `KnownHostsPaths`: default known_hosts files used when the host has no
  ssh_config `UserKnownHostsFile`.
- `AgentSocket`: the ssh agent socket (`$SSH_AUTH_SOCK`).
- `StrictHostKeyChecking`: the default host key policy for hosts without an
  ssh_config `StrictHostKeyChecking` (`yes`, `ask`, `accept-new` or `no`; the
  zero value is `yes`).
- `ConnectionTimeout`: TCP dial and SSH handshake timeout.
- `SessionOpenTimeout`: opening an SSH session only; `0` disables it.
- `IdleConnections` / `MaxConnections`: per-Remote pool limits.
- the host key callback, set with `WithHostKeyCallback`.

`sshremote.NewDefaultConfig()` builds it from the user's environment.

Use `WithSSHConfigContent(content []byte)` to parse ssh_config content without a
file:

```go
config = config.WithSSHConfigContent([]byte(`
Host example
  HostName example.com
`))

host, err := config.ResolveHost("example")
```

The content is copied and parsed once per `Config`. `Include` directives in
in-memory content are resolved by the ssh_config library without a config-file
base directory.

`ConnectionTimeout` and `SessionOpenTimeout` default to 30 seconds through
`NewDefaultConfig()`.

`Clone()` returns an independent copy. `Equal(other)` compares settings,
including the callback. The `*Config` pointer is the pool identity; changing a
setting uses a new pointer. The ssh_config source is parsed once per `Config`
instance; edits to a file after the first resolution are not observed. A new
`Config` (`Clone` or either ssh_config setter) parses it again.

### In-memory signers

Use `WithSigners` when the private key is already in memory, comes from a
secret store, or is backed by a custom signer:

```go
signer, err := ssh.NewSignerFromKey(privateKey)
if err != nil {
	return err
}

config := sshremote.NewConfig().WithSigners(signer)
host := sshremote.NewHost("example.com").WithSigners(signer)
```

`Config.WithSigners` and `Host.WithSigners` replace the in-memory signer list.
`Config.Signers`, `Host.Signers` and `HostConfig.Signers` return copies.
`HostConfig.Signers` returns host signers followed by config signers.

`GetSSHSigners` uses this order:

1. agent signers;
2. host in-memory signers;
3. config in-memory signers;
4. host private key files;
5. config private key files.

`Clone` copies the signer slice and `Config.Equal` compares signer identity.
In-memory signers are available to the native SSH client only. The system
`ssh` fallback is built from key paths, so it cannot use them.

### Setting the host key callback

The callback is private and set through an immutable setter that returns a new
config:

```go
config = config.WithHostKeyCallback(func(hc sshremote.HostConfig) (ssh.HostKeyCallback, error) {
	/* ... */
})
```

`WithHostKeyCallback` allocates a new holder, and the pointer is the version:
the returned config compares **unequal** to the original, so pooled connections
are rebuilt with the new callback. `Clone` copies the pointer, so a clone is
still equal. `WithHostKeyCallback(nil)` clears the callback and compares equal
to a config that never had one.

Because it returns a new config, callers must keep the result. In `gabricssh`,
store it with `NewConfigContext`; see [gabricssh.md](gabricssh.md).

### Per-branch config

Most per-host differences (user, port, keys, known_hosts, strict checking,
proxy) already come from the resolved `Host` and ssh_config. For explicit
low-level use, build a separate `HostConfig`:

```go
hostConfig := config.WithConnectionTimeout(5 * time.Second)
host := host.Clone()

remote := sshremote.NewRemote(sshremote.NewHostConfig(host, hostConfig))
defer remote.Close()
```

In Gabric, derive a context branch with `gabricssh.NewConfigContext` and
`gabricssh.NewHostContext`; see [gabricssh.md](gabricssh.md).

The manager caches `Remote`s by host identity and config identity. A new
`*Config` instance selects a separate `Remote`, so branches with different
configs do not rebuild one another's connections. Keep the same config instance
for branches that should share a pool.

## Pooling

The config carries the pooling limits (`IdleConnections`, `MaxConnections`);
the defaults are 5/5. The manager caches one `Remote` per host identity and
`*Config` identity, and each `Remote` pools SSH connections using those limits.

The pool is internal to `sshremote`. It keeps idle resources in a mutex-protected
stack and uses a counting semaphore to bound checked-out resources. `Close`
cancels the pool-lifetime context, closes idle connections, and wakes callers
waiting for capacity; connections currently in use are closed when released.
Pool-owned connection establishment is not tied to the operation context, so a
cancelled operation does not discard a connection that finished dialing. The
connection attempt is bounded by `Config.ConnectionTimeout()` and cancelled if
the pool closes.

`IdleConnections` and `MaxConnections` are `-1` for unlimited or positive
limits; invalid values panic during `NewRemote`.

`WithSession` opens the operation's real session on a checked-out connection;
there is no separate probe session. If opening it fails because a recycled
connection's transport has died, that connection is discarded and the call
tries another pooled connection. A failure on a freshly established connection
is returned. An `*ssh.OpenChannelError` means the server answered and refused
the session, so the connection is healthy and is kept.

Two `*Host` values that set only hostname, effective port and effective user
share a host identity when those values match. A host with any other connection
field set uses pointer identity. To use a different config, provide a different `*Config`
instance; equal-but-distinct config instances intentionally use separate pools.

Because connections live behind the context, two branches with different hosts
get independent connections and can run concurrently.

For a connection that should **not** be pooled, `NewRemoteClient(ctx,
hostConfig, logger)` returns a single `*RemoteClient` (the caller closes it),
and `WithNewRemoteClient(ctx, hostConfig, logger, f)` runs `f(*ssh.Client)`
with a dedicated connection, closing it (and returning `ctx.Err()`) when the
context is cancelled. Gabric uses `WithNewRemoteClient` for local port
forwarding.

## Cancellation

The context passed to an operation bounds both connection establishment and the
operation itself. If it is cancelled during the TCP dial or the SSH handshake,
the operation returns `ctx.Err()` and the half-open connection is discarded.

Once connected, cancellation aborts the running operation and the error returned
is `ctx.Err()`. If cancellation happens before the session opens, `WithSession`
closes and discards the connection to unblock `NewSession`. After the session is
open, it closes only the session channel; the pooled SSH connection stays
healthy and is reused by the next operation. A dedicated connection created with
`WithNewRemoteClient` is closed when it returns or when its context is
cancelled.

It is the same context a task receives, so cancelling a task (or a derived
context) stops its remote work promptly.

## Authentication

`HostSSHAuth` / `GetSSHSigners` take a `HostConfig` and build the authentication
methods: the ssh agent (if `AgentSocket` is set) plus the effective private keys.
`GetSSHSigners` returns `ErrAgentUnavailable` when the agent socket cannot be
dialed or listed; `HostSSHAuth` logs that failure and retries with the agent
disabled, authenticating with the private keys instead.
Passphrase-protected keys are skipped only when an agent is available.
The agent connection is opened per `Sign` rather than held open; prompts are
triggered by `Sign`, not by connecting, so re-dialing does not add prompts.

## Host key verification

`HostConfig.StrictHostKeyChecking()` controls verification against
`HostConfig.KnownHostsPaths()`. The host value from ssh_config takes precedence;
the config policy is the fallback:

- `StrictHostKeyCheckingYes` (the zero value) verifies known keys and rejects
  unknown hosts.
- `StrictHostKeyCheckingAsk` verifies the same way as
  `StrictHostKeyCheckingYes`. Gabric does not prompt interactively, so an unknown
  host is rejected rather than accepted.
- `StrictHostKeyCheckingAcceptNew` verifies known keys, appends an unknown key
  to the first known_hosts file and rejects changed keys. At least one
  known_hosts path is required.
- `StrictHostKeyCheckingNo` skips host key verification.

For example, trust new hosts once but never accept a changed key:

```go
host := sshremote.NewHost("example.com").
	WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew)

hc := sshremote.NewHostConfig(host, config)
```

Use `Config.WithHostKeyCallback` to own the policy entirely; it takes precedence
over the resolved strict policy. The callback receives the `HostConfig`, so it
can use the effective known hosts. This makes it possible to disable verification
and only observe keys, or to implement a custom trust policy:

```go
base := config.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)

config = base.WithHostKeyCallback(func(hc sshremote.HostConfig) (ssh.HostKeyCallback, error) {
	paths := hc.KnownHostsPaths()
	if len(paths) == 0 {
		return nil, fmt.Errorf("no known_hosts file configured")
	}

	cb, err := sshremote.KnownHostsCallback(paths...)
	if err != nil {
		return nil, err
	}

	return func(hostname string, addr net.Addr, key ssh.PublicKey) error {
		if err := cb(hostname, addr, key); err != nil {
			if !sshremote.IsUnknownHost(err) {
				return err
			}

			if err := sshremote.KnownHostsAppend(paths[0], sshremote.KnownHostsLine(hostname, key)); err != nil {
				return err
			}
		}

		return nil
	}, nil
})
```

`sshremote.IsUnknownHost` only matches an unknown host, so mismatches, revoked keys
and unrelated errors are returned unchanged. For first-time connections through
proxies (where the key is not yet known), a callback that logs
`ssh.FingerprintSHA256(key)` and accepts is enough.

### Known hosts helpers

- `KnownHostsCallback(paths...)`: a strict `ssh.HostKeyCallback` over one or
  more files.
- `KnownHostsAcceptNewCallback(paths...)`: verifies known keys and appends
  unknown keys to the first file, like `StrictHostKeyCheckingAcceptNew`.
- `KnownHostsLine(hostname, key)`: format a known_hosts entry.
- `KnownHostsAppend(path, line)`: append an entry (trust on first use).
- `IsUnknownHost(err)`: true only for "key is unknown".

See [cookbook.md](../cookbook.md) for full `WithHostKeyCallback` recipes.

## Design decisions

- **Connection pooling is internal to `sshremote`.** Keeping multiple pooled
  connections per host is intentional for concurrent large transfers; the pool
  is small enough to live beside the connection code and avoids a dispatcher
  goroutine on the hot path.
- **The SSH agent is re-dialed per signature** (`agentSigner` connects on each
  `Sign`) rather than holding a long-lived agent connection. Prompts are
  triggered by `Sign`, not by connecting, so this does not add prompts.
- **Remote file operations use coreutils.** `Put` streams to `dd`, `Get` reads
  with `cat`, `Exists` uses `test`, `Stat` uses `stat -L`, `Mkdir` uses `mkdir`,
  `Remove` uses `rm`, and `List` uses `ls -1Ap`, plus `chmod`/`chown`. `Stat`
  selects GNU or BSD `stat` with `uname -s` in one shell command; Linux needs a
  `stat` with `-c` support. This avoids a separate SFTP subsystem; a 200 MB
  benchmark showed `dd` roughly 4x faster than SFTP for uploads, and streaming
  avoids an extra copy and temp space.

## Limitations

- Only some `ProxyCommand` forms become native (jumps, forwards, sessions,
  standalone `nc`, and `-`); other values are executed via `$SHELL -c` (so they
  may depend on whatever they name, e.g. `ssh`/`nc`). `ProxyCommand`/`ProxyJump`
  precedence is first-in-file; ordering through an `Include` is approximate.
- `ask` is treated as `yes` because Gabric does not prompt for unknown host
  keys.
- Hosts that set only hostname, effective port and effective user share identity
  by those values; hosts with other connection fields use pointer identity.
