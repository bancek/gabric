# Reconcile modules

Ready-made reconciler nodes for common system resources. Base modules live in
`github.com/bancek/gabric/reconcile/modules`; domain modules live in its
subpackages (`linux`, `ufw`, `ufwdocker`, `systemd`, `service`, `apt`, `apk`,
`pkg`, `git`, `docker`, `httpfile`, `github`, `rsync`). Each is assembled from
the node builder in [api.md](api.md).

See [`examples/homelab`](../../examples/homelab) for most of
these modules used in a complete, buildable project.

Module e2e tests exercise modules against real Docker, LocalRemote, and Local
targets. See [module e2e testing](module-e2e-testing.md) for when to add one
and how to run the suites.

## Inputs

Module builders take `input.Input[T]` values. Static values use
`input.Static`; the examples use a local alias:

```go
func S[T any](value T) input.Input[T] {
	return input.Static(value)
}
```

File modes use `modules.FileMode`. List literals use `input.List(...)`; an
existing slice uses `input.Static(slice)`. Boolean flags accept zero or one
input, so `.CreateOnly()`, `.CreateOnly(input.False)` and `.CreateOnly(node)`
all work. `input.WithName(primaryInput, "...")` overrides the resource name.

See [Inputs](inputs.md) for the full model.

## The pattern

A module is a builder that configures a resource and hands off to the Reconcile
node builder with `Build`:

```go
modules.File(S("/etc/ssh/sshd_config.d/99-hardening.conf")).
	Content(S([]byte(hardening))).Mode(modules.FileMode(0o644)).
	Build().              // *reconcile.ResourceBuilder
	Notify(sshTopic).     // change signal (optional)
	Node()                // *reconcile.Node
```

On the `*reconcile.ResourceBuilder` you can add `Notify`, `CheckNeeds`/
`ApplyNeeds`, `ApplyInvalidates`, and finally `Node`. Modules with no options
return a `*reconcile.ResourceBuilder` (or a `*reconcile.Node`) directly.

- **Value type.** Every resource module yields `bool`: `true` when it changed,
  `false` when already satisfied. (Reads such as `git.RemoteRevisionData`,
  `git.LocalRevisionData`, `ufw.StatusData`, `FileContentData` and
  `FileExistsData` yield their value; `git.RepoGroup` is a Void `Group` cover with the
  revision on `LocalRevision`.)
- **Composable data nodes.** Modules are assembled from small, reusable,
  read-only data nodes. See
  [composable data nodes](#composable-data-nodes) for the index and the scope
  rule.
- **Notifications.** A module never calls `Notify` itself; add it after `.Build()`.

## Composable data nodes

Modules are assembled from small read-only nodes. Each is created with
`reconcile.Data(…)`, runs in a dry run, and is memoized once per run.

- **Files:** `FileContentData`, `FileExistsData`, `PathExistsData`,
  `DirExistsData`, `FileHashData`, `FileStatData`, `FileSatisfiedData`,
  `TemplateData`, `httpfile.SatisfiedData`, `DirSatisfiedData`,
  `FileHashMatchesData`, `FileContentMatchesData`, `FileModeMatchesData`,
  `FileOwnerMatchesData`.
- **Users:** `linux.UserPasswdData`, `linux.UserGroupData`, `linux.UserGroupsData`,
  `linux.UserSatisfiedData`, `linux.SSHAuthorizedKeysContentData`,
  `linux.SSHAuthorizedKeysDirPathData`, `linux.SSHAuthorizedKeysFilePathData`.
- **OS facts:** `linux.OSReleaseData`.
- **Units:** `systemd.EnabledData`, `systemd.ActiveData`.
- **Git:** `git.RemoteRevisionData`, `git.LocalRevisionData`,
  `git.TrackingRevisionData`.
- **UFW / Docker:** `ufw.StatusData`, `ufw.ShowAddedData`,
  `ufwdocker.InstalledData`, `docker.NetworkExistsData`.
- **Packages / sysctl:** `apt.InstalledData`, `apt.VersionData`,
  `apt.CandidateData`, `apt.SatisfiedData`, `apk.InstalledData`,
  `apk.VersionData`, `apk.CandidateData`, `apk.SatisfiedData`,
  `linux.SysctlRuntimeData`,
  `linux.SysctlDropinData`, `linux.SysctlSatisfiedData`.
- **Compose / rsync:** `docker.ComposeSatisfiedData`, `rsync.SatisfiedData`.
- **Commands:** `CommandOutputData`, `CommandStderrData`, `IsExitStatus`.

Command-backed data nodes treat only their documented non-zero exit as an
absent or negative result. `IsExitStatus(err, status...)` checks for a command
exit error with one of those statuses; transport, start and other errors
propagate.

**Scope.** A data node is memoized per run by identity. Build it outside
`Check`/`Apply` to share one read, or inside to force a fresh probe after a
mutation (for example, reading the account again after `useradd`). The match
data nodes take the base node, so one `FileStatData` can feed both mode and owner
checks.

## Target operations

Modules use `gabric.TargetOps(ctx)`. `gabric.Main` and `gabric.Run` default to a
remote target, which needs an installed transport such as `gabricssh`. See the
[built-in target comparison](../api.md#target-operations) for command, file-op,
and sudo differences. Use `WithLocalTarget` or `NewTargetContext` to apply
modules locally:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalTarget))
```

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)
```

Direct Reconcile use must set a target explicitly.

`LocalTarget` uses `Local` and the local exec env / working-dir contexts;
`RemoteTarget` and `LocalRemoteTarget` use the remote ones. Target ops accept
target-specific working-directory and exec-environment overlays:

```go
ctx = gabric.NewTargetOpsWorkingDirContext(ctx, "/srv/app")
ctx = gabric.NewTargetOpsExecEnvContext(ctx, gabric.Env("DEBIAN_FRONTEND", "noninteractive"))
```

`ops.WithWorkingDir(...)` and `ops.WithExecEnv(...)` override those overlays for
their callback.

Use `LocalTarget` for native local modules, for example files and directories:
Go `exec`/`os`, local paths, no implicit shell, and no sudo. It also works on
Windows for target operations; individual module support still depends on the
module and available tools. It runs as the user running Gabric, so remote sudo
does not apply; run with the privileges the modules need.

Use `LocalRemoteTarget` when module execution should match a remote Unix host:
the same `/bin/sh` commands, coreutils, working directories, environment, and
sudo behavior. It is the target for running a binary built for a remote machine in
place. It needs `/bin/sh` and coreutils, so it is Unix-like only; use it when
you need local sudo or rsync.

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalRemoteTarget))
```

or:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
```

For privileged local modules, pair `LocalRemoteTarget` with the existing remote
sudo setting:

```go
var localRoot = gabric.With(
	gabric.WithReconcile,
	gabric.WithLocalRemoteTarget,
	gabric.WithRemoteSudo(&gabric.Sudo{}),
)
```

If remote sudo is configured, `LocalRemote` still needs working sudo
permissions. Without it, commands run as the current user. `LocalRemote` expects
`/bin/sh` and the Unix tools used by the modules. It covers commands, files, and
rsync. SSH `Shell` and `Forward` live in `gabricssh`.

Remote SSH works with macOS targets. Module support depends on the tools
installed on the target. `pkg`, `service`, `ufw`, `linux.Sysctl`, and `linux.User` are
Linux-specific.

Do not evaluate the same node under local and remote targets in one run; use
separate runs or distinct nodes.

Modules use `TargetOps`, so they follow the selected target. Custom Reconcile
data nodes, effects, and task logic that should follow the target must use
`gabric.TargetOps` too. `Remote` and `Local` are explicit escapes for
intentionally target-independent reads or writes.

`Rsync` is not part of `gabric.Ops` because a local target has no rsync backend.
The `Rsync` module uses `gabric.TargetRsyncOps` and fails with
`ErrNoRsyncTarget` for a plain `Local` target, so it works with `Remote` and
`LocalRemote` (the latter needs the native transport, Unix only).
`git.RemoteRevisionData` and `git.LocalRevisionData` keep their upstream versus
checked-out meaning; they are not target selectors.

## Modules

### `Command`

```go
modules.Command(input.List("apt-get", "update")) // *reconcile.ResourceBuilder[bool]
```

Runs the argv unconditionally (no probe); a dry run skips it. Node
`modules.command:<cmd>`.

### `CommandOutputData` / `CommandStderrData`

```go
modules.CommandOutputData(input.List("uname", "-r"))              // *reconcile.Node[string]
modules.CommandStderrData(input.List("docker", "compose", "ps")) // *reconcile.Node[string]
```

Read-only **data nodes** (memoized once per run). They run the command on the
target and yield its stdout or stderr. Use only side-effect-free commands: data
nodes run during a dry run. Nodes `modules.command-output:<cmd>` /
`modules.command-stderr:<cmd>`.

### pkg.Package

```go
pkg.Package(S("ufw"))             // *pkg.PackageBuilder
pkg.Package(S("curl")).Latest()   // upgrade to the current apt or apk candidate
```

`.Build() *reconcile.ResourceBuilder[bool]` (value = changed). The cover selects
`apt.Package` or `apk.Package` with `reconcile.DynamicResource`, so it reports as
`pkg.package:<name>`. `pkg.ManagerData` reads `linux.OSReleaseData` and maps
Debian-like releases to apt and Alpine to apk. The probe reads `dpkg-query` or
`apk info` status (and, with `.Latest()`, the apt or apk candidate); `Apply` runs
`apt-get install -y` or `apk add`. It does **not** run `apt-get update` or
`apk update`. Add a `pkg.Update().Build().Node()` (or an implementation-specific
update node) as an `.ApplyNeeds` dependency (see the cookbook).

The probe reads are exposed as data nodes: `apt.InstalledData(S(name))`,
`apt.VersionData(S(name))`, `apt.CandidateData(S(name))`, and the matching
`apk.InstalledData`, `apk.VersionData`, `apk.CandidateData`. `InstalledData` and
`VersionData` treat a package-manager "not found" exit as absent;
`CandidateData` returns `""` when the candidate is unknown. Other errors
propagate. `apt.SatisfiedData(nameInput, latestInput)` and
`apk.SatisfiedData(nameInput, latestInput)` are the derived bool nodes the
resources check; they compose those reads and memoize the result per run.

### pkg.Update

```go
pkgUpdate := pkg.Update().Build().Node()
pkgUpdate = pkg.Update().MaxAge(S(24 * time.Hour)).Build().Node()
```

Refreshes the target package index using the selected package manager
(`apt.Update` or `apk.Update`). Without `MaxAge` it has no probe and always runs
when evaluated; a dry run skips it. `MaxAge` checks the selected package cache
and only refreshes when the cache is older than the duration. Node `pkg.update`.
Bind the node once and reuse it; each `pkg.Update()` call creates a distinct
node.

### apt.Update

```go
aptUpdate := apt.Update().Build().Node()
aptUpdate = apt.Update().MaxAge(S(24 * time.Hour)).Build().Node()
```

Refreshes the apt package index. Without `MaxAge` it has no probe and always runs
when evaluated; a dry run skips it. `MaxAge` uses the
`/var/lib/apt/periodic/update-success-stamp` mtime when it is fresh, otherwise
the `/var/lib/apt/lists/partial` directory, and refreshes only when the cache is
older than the duration. Node `apt.update`.

### apk.Update

```go
apkUpdate := apk.Update().Build().Node()
apkUpdate = apk.Update().MaxAge(S(24 * time.Hour)).Build().Node()
```

Refreshes the apk package index. Without `MaxAge` it has no probe and always runs
when evaluated; a dry run skips it. `MaxAge` reads the cached
`/var/cache/apk/APKINDEX.*` file mtimes and refreshes only when any of them is
older than the duration. Node `apk.update`.

### `File`

```go
modules.File(S(path)).Content(S(data)).Mode(modules.FileMode(0o644)).Build()
modules.File(S(path)).Content(contentNode).Build()
modules.File(pathNode).Content(contentNode).Build()
```

Options: `.Content(input.Input[[]byte])`, `.Mode(input.Input[os.FileMode])`,
`.User(input.Input[string])`, `.Group(input.Input[string])`, `.CreateOnly()`.
`.Build() *reconcile.ResourceBuilder[bool]` (value = changed). The
probe compares the content hash, mode and any configured owner; `Apply` writes
the file, creating its parent. A function or node for `.Content` is read once
per run. `FileSatisfiedData(pathInput, contentInput, modeInput, createOnlyInput)`
is the derived content/mode node.

With `LocalTarget`, the content hash is computed by streaming the local file in
Go. With `Remote`, `sha256sum` runs on the remote host; with `LocalRemote`, it
runs through the local remote-style shell. In both cases Gabric does not stream
the file. On Windows, content-only `File` (no `Mode`, `User` or `Group`) works
with `LocalTarget`; mode and ownership remain Unix-like.

### `FileContentData` / `FileExistsData` / `PathExistsData` / `DirExistsData`

```go
modules.FileContentData(S("/usr/local/bin/ufw-docker.ref")) // *reconcile.Node[string]
modules.FileExistsData(S("/usr/local/bin/ufw-docker"))      // *reconcile.Node[bool]
modules.PathExistsData(S("/srv/app"))                       // *reconcile.Node[bool]
modules.DirExistsData(S("/srv/app"))                        // *reconcile.Node[bool]
```

Read-only **data nodes** (memoized once per run). `FileContentData` yields the
target file's content, or `""` when the file does not exist (other read errors
propagate); `FileExistsData` reports whether the regular file exists;
`PathExistsData` reports whether the path exists at all (file, directory, …);
`DirExistsData` reports whether the path is a directory. Nodes
`modules.file-content-data:<path>` / `modules.file-exists-data:<path>` /
`modules.path-exists-data:<path>` / `modules.dir-exists-data:<path>`.

### `FileHashData`

```go
modules.FileHashData(S("/srv/app/app")) // *reconcile.Node[string]
```

Read-only data node (memoized once per run). Yields the target file's sha256, or
`""` when the file does not exist; other read errors propagate. Node
`modules.file-hash-data:<path>`.

`FileHashData` uses the same split as `File`: `LocalTarget` streams and hashes
locally; other targets run `sha256sum`, falling back to `shasum -a 256` when
`sha256sum` is unavailable.

### `FileStatData` / `FileModeMatchesData` / `FileOwnerMatchesData`

```go
stat := modules.FileStatData(S("/etc/app.conf")) // *reconcile.Node[FileStat]

modeOK := modules.FileModeMatchesData(stat, modules.FileMode(0o644))
ownerOK := modules.FileOwnerMatchesData(stat, S("root"), S("root"))
```

`FileStatData` reads the target path metadata (`gabric.FileStat` plus
`Exists`) and normalizes a missing path to `Exists=false`. The match nodes take
the state node, so one `Stat` serves both checks. `FileModeMatchesData` compares
the permission bits; `FileOwnerMatchesData` checks the non-empty user/group fields.
Build `FileStatData` inside `Check`/`Apply` for a fresh read, or once outside to
share it.

### `FileHashMatchesData` / `FileContentMatchesData`

```go
hash := modules.FileHashData(S(path))
hashOK := modules.FileHashMatchesData(hash, S(want))
contentOK := modules.FileContentMatchesData(hash, S(content))
```

Derived data nodes over a `FileHashData` read. A missing file never matches.
`FileContentMatchesData` compares against the sha256 of `content`.

### `TemplateData`

```go
data := reconcile.Data("revision", func(ctx context.Context) (any, error) {
	return map[string]any{"Revision": rev}, nil
}).Node()

modules.TemplateData(S("/srv/app/compose.yaml"), S(tmpl), data) // *reconcile.Node[[]byte]
```

Renders a Go text template to a data node (memoized once per run). Both the
template text and the data are inputs. The template is parsed once per run, and
an invalid template returns an error. Node `modules.template-data:<path>`. A
typed data node can be passed directly, or derived with
[`reconcile.Map`](api.md#map):

```go
data := reconcile.Map[string, any](gitRevision, func(ctx context.Context, rev string) (any, error) {
	return map[string]any{"Revision": rev}, nil
})
composeFile := modules.File(S(path)).
	Content(modules.TemplateData(S(path), S(tmpl), data)).
	Mode(modules.FileMode(0o644)).
	Build().
	Node()
```

Use `File` with `Content` for the probe, apply, mode and owner. The rendered
content is memoized once per run and shared with `Apply`. The resulting file uses
the same local/remote hashing as any other `File`; content-only files (no `Mode`,
`User` or `Group`) work with `LocalTarget` on Windows, while mode and ownership
remain Unix-like. Static data can be wrapped in a `reconcile.Data` node.

### `Dir`

```go
modules.Dir(S("/srv/www")).Build()            // *reconcile.ResourceBuilder[bool]
modules.Dir(S(path)).Mode(modules.FileMode(0o755)).Build()
```

Options: `.Mode(input.Input[os.FileMode])`, `.User(input.Input[string])`,
`.Group(input.Input[string])`.
`Build() *reconcile.ResourceBuilder[bool]`.
The probe checks existence, mode and any configured owner; `Apply` is
`TargetOps(ctx).Mkdir(path).Recursive().Mode(mode).User(user).Group(group)`.
`DirSatisfiedData(pathInput, modeInput)` is the derived mode/existence node.

### linux.Sysctl

```go
linux.Sysctl(S("net.ipv4.ip_forward"), S("1")) // *reconcile.ResourceBuilder[bool]
```

The probe checks both the runtime value (`/proc/sys/…`) and the persisted drop-in;
`Apply` writes `/etc/sysctl.d/99-reconcile-<k>.conf` and runs `sysctl -p <dropin>`.
The node yields `bool` (changed) and does not notify by itself.

`linux.SysctlRuntimeData(S(key))` and `linux.SysctlDropinData(S(key))` expose the
two reads as data nodes; both return `""` when the value or file is unreadable.
`linux.SysctlSatisfiedData(keyInput, valueInput)` is the derived bool node the
resource checks.

### service.Service

```go
service.Service(S("unattended-upgrades")).Build() // enable
service.Service(S("fail2ban")).Started().Build()    // enable --now
service.Service(S("foo")).Enabled(input.False).Build()      // disable --now
```

`service.Service` is the generic cover; `systemd.Service` is the systemd
implementation. The cover selects `systemd.Service` with `reconcile.DynamicResource`, so
it reports as `service.service:<name>`. Options: `.Enabled(...input.Input[bool])`
and `.Started(...input.Input[bool])`. A service is enabled by default and not
started by default. `.Enabled(input.False)` with `.Started()` is an evaluation
error. `Build() *reconcile.ResourceBuilder[bool]`. The probe is `systemctl
is-enabled` (and `is-active` when `.Started()`); `Apply` runs `enable` /
`enable --now` / `disable --now`.

### `systemd.EnabledData` / `systemd.ActiveData`

```go
systemd.EnabledData(S("docker")) // *reconcile.Node[bool]
systemd.ActiveData(S("docker"))  // *reconcile.Node[bool]
```

Read-only data nodes (memoized once per run). `systemd.EnabledData` reads
`systemctl is-enabled`; `systemd.ActiveData` reads `systemctl is-active`. A disabled
or inactive unit is `false`, not an error. Nodes `systemd.enabled:<name>` /
`systemd.active:<name>`.

### linux.User

```go
linux.User(S("deploy")).Build()                      // login user, home, /bin/bash
linux.User(S("deploy")).Shell(S("/bin/sh")).CreateHome(input.False).Build()
linux.User(S("app")).System().Build()                // system account, no home
linux.User(S("deploy")).Groups(input.List("docker")).Build()     // additive supplementary groups
```

Options: `.Shell(input.Input[string])` (default `/bin/bash`),
`.Home(input.Input[string])`, `.System()` (no home by default),
`.CreateHome()`/`.CreateHome(input.False)`, `.Groups(input.Input[[]string])`
(additive).
`Build() *reconcile.ResourceBuilder[bool]`. The probe reads `getent passwd` (and
`id -nG` for groups); `Apply` runs `useradd --user-group` when the account is
missing, `usermod --shell`/`--home` on drift, and `usermod -aG` for groups. It
never grants sudo unless you add the `sudo` group.
`linux.UserSatisfiedData(nameInput, shellInput, homeInput, groupsInput)` is the
derived bool node the resource checks.

### linux.OSReleaseData

```go
release := linux.OSReleaseData() // *reconcile.Node[OSRelease]
```

Reads and parses `/etc/os-release` (memoized once per run, dry-run safe). Node
`linux.os-release`. `OSRelease` exposes `ID`, `IDLike`, `Name`, `PrettyName`,
`VersionID` and `VersionCodename`; `IsLike("debian")` checks `ID` and `IDLike`.
A missing file yields an empty value; other read errors propagate.

### linux.UserPasswdData / linux.UserGroupData / linux.UserGroupsData

```go
passwd := linux.UserPasswdData(S("deploy")) // *reconcile.Node[UserPasswd]
group := linux.UserGroupData(S("deploy"))   // *reconcile.Node[string]
groups := linux.UserGroupsData(S("deploy")) // *reconcile.Node[[]string]
```

Read-only data nodes (memoized once per run). `linux.UserPasswdData` parses
`getent passwd <name>` (`UserPasswd` exposes `Name`, `Password`, `UID`, `GID`,
`GECOS`, `Home`, `Shell` and `Exists`) and yields `Exists=false` when `getent`
reports the user is not found; other errors propagate. `linux.UserGroupData`
reads `id -gn`; `linux.UserGroupsData` reads `id -nG` and returns the primary and
supplementary group names. Nodes `linux.user-passwd:<name>`,
`linux.user-group:<name>`, `linux.user-groups:<name>`.

### linux.SSHAuthorizedKeys

```go
linux.SSHAuthorizedKeys(S("deploy"), input.List(publicKey))
linux.SSHAuthorizedKeys(S("deploy"),
	input.List("ssh-ed25519 AAAA… one", "ssh-ed25519 AAAA… two"),
)
```

Ensures `<home>/.ssh` (mode `0700`) and `<home>/.ssh/authorized_keys` (mode
`0600`), both owned by the user. The home comes from `getent passwd`.
`SSHAuthorizedKeys` returns an `SSHAuthorizedKeysGroup`; the group contains
separate `Dir` and `File` resources, and reading it runs the directory first.

```go
keys := linux.SSHAuthorizedKeys(S("deploy"), input.List(publicKey))
keys.Dir  // *reconcile.Node[bool]
keys.File // *reconcile.Node[bool]
```

The group reports the two resources separately as
`modules.dir:<user>` and `modules.file:<user>`.
Put the user resource before the keys in the enclosing group:

```go
user := linux.User(S("deploy")).Build().Node()
keys := linux.SSHAuthorizedKeys(S("deploy"), input.List(publicKey))

reconcile.Group("user").
	Contains(user, keys)
```

The paths are exposed as composable data nodes:

```go
passwd := linux.UserPasswdData(S("deploy"))
dir := linux.SSHAuthorizedKeysDirPathData(passwd)  // <home>/.ssh
file := linux.SSHAuthorizedKeysFilePathData(dir)   // <home>/.ssh/authorized_keys
```

Both error when the account or home is missing.
`linux.SSHAuthorizedKeysContentData(input.List(publicKey))` renders the keys as
file content (trimmed, blanks dropped, one per line) and yields
`*reconcile.Node[[]byte]`.

### docker.Network

```go
docker.Network(S("web")) // *reconcile.ResourceBuilder[bool]
```

Probe `docker network inspect <name>` (exit 0 = present); `Apply`
`docker network create <name>`. `docker.NetworkExistsData(S(name))` exposes the probe
as a `*reconcile.Node[bool]`.

### docker.DockerEngine

```go
engine := docker.DockerEngine().Build()
```

Installs Docker CE from Docker's official apt repository and starts
`docker.service`. It supports Ubuntu and Debian, using `linux.OSReleaseData` for
the codename and `dpkg --print-architecture` for the architecture.

The engine group exposes:

- `Keyring`: `/etc/apt/keyrings/docker.asc`
- `Source`: `/etc/apt/sources.list.d/docker.list`
- `Update`: the apt index refresh, read lazily by the package nodes
- `Packages`: `docker-ce`, `docker-ce-cli`, `containerd.io`,
  `docker-buildx-plugin`, `docker-compose-plugin`
- `Service`: `systemd.Service(S("docker")).Started()`

The keyring and source are applied before the packages, and the service starts
after the packages. The group cover is `docker.engine`; it does not include the
non-convergent apt update as a direct member. Daemon configuration is left to
the caller.

### rsync.Sync

```go
rsync.Sync(S("./srv/www")).Remote(S("/srv/www")).Build()
rsync.Sync(S("./srv")).Remote(S("/srv")).Delete().Exclude(input.List(".state/**")).Build()
```

Options: `.Remote(input.Input[string])`, `.Delete()`,
`.Exclude(input.Input[[]string])`.
`Build() *reconcile.ResourceBuilder[bool]`. The probe runs the transfer in dry-run
(`-n -i`) and compares the itemized output; `Apply` runs the real upload. The
transfer uses `gabric.TargetOps(ctx)`, so it follows the `Remote` / `LocalRemote`
target. The node is `rsync.sync:<local>-><remote>`.
`rsync.SatisfiedData(localInput, remoteInput, deleteInput, excludesInput)` is the
derived bool node the resource checks.

### ufw.Rule

```go
ufw.Rule().Port(S("22")).Proto(S("tcp")).Build()
ufw.Rule().Route().Port(S("80")).Proto(S("tcp")).Build()
ufw.Rule().In(S("wg0")).Build()
```

Options: `.Port(input.Input[string])`, `.Proto(input.Input[string])`,
`.Action(input.Input[string])` (default `allow`), `.From(input.Input[string])`,
`.In(input.Input[string])`, `.Out(input.Input[string])`, `.Route()`.
`Build() *reconcile.ResourceBuilder[bool]`. The probe compares the canonical
command against `ufw show added`; `Apply` runs it. Never notifies.
`ufw.ShowAddedData()` exposes the `ufw show added` read as a
`*reconcile.Node[string]`; a failed command yields `""`.

### ufw.Firewall

The firewall state is read from `ufw status verbose` and parsed into a
`ufw.StatusInfo` data node:

```go
info, err := ufw.StatusData().Value(ctx) // ufw.StatusInfo{Active, Incoming, Outgoing, Routed}
```

- `ufw.DefaultIncoming(status, policyInput) *reconcile.ResourceBuilder[bool]`:
  ensures `ufw default <policy> incoming`.
- `ufw.DefaultOutgoing(status, policyInput) *reconcile.ResourceBuilder[bool]`:
  ensures `ufw default <policy> outgoing`.
- `ufw.Enabled(status, enableInput) *reconcile.ResourceBuilder[bool]`: ensures
  `ufw --force enable` / `disable`.

Both default-policy builders panic if the policy input is nil.

`ufw.Firewall()` configures a **cover** over those data nodes and the rules:

```go
fw := ufw.Firewall().
	Rule(ufw.Rule().Port(S("22")).Proto(S("tcp"))).
	Incoming(S("deny")).Outgoing(S("allow")).
	Enabled(). // or Enabled(input.False)
	Build()
```

`.Build()` panics unless `.Enabled(...)` was called. The
cover evaluates **rules → probe → incoming → outgoing → enabled → status** (fixed
order, whatever the call order), so `ufw allow 22/tcp` lands before
`ufw default deny incoming` and `ufw --force enable`, so the firewall never locks
you out. Each default policy is its own node, so only the drifted policy is
applied. Reading the cover runs the whole group; it is not memoized and
owns nothing, so start a new `Run` to reset it.

The cover uses **two** status reads: an internal _probe_ the default/enabled
nodes share, and the _observed_ `fw.Status`, read **last**. The mutators
invalidate the observed status when they change (and the cover has not read it
yet), so reading `fw.Status` after the cover re-probes instead of returning the
pre-change value. This is the general
[probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed).

A rule owned by another service is built with `fw.AddRule`, which wires it to
invalidate the observed status when it applies; the returned node is grouped
into that service's node (it does **not** join the cover):

```go
var TraefikUfwRuleHTTP = fw.AddRule(ufw.Rule().Route().Port(S("80")).Proto(S("tcp")))
// ... reconcile.Group("traefik").Contains(TraefikUfwRuleHTTP, TraefikUfwRuleHTTPS)
```

### httpfile.File

```go
httpfile.File(S(url)).Path(S(path)).Mode(modules.FileMode(0o644)).Build()
httpfile.File(S(url)).Path(S("/usr/local/bin/ufw-docker")).Mode(modules.FileMode(0o755)).Build()
httpfile.File(S(url)).Path(S(path)).SHA256(S(hex)).Build()
```

Options: `.Path(input.Input[string])` (required),
`.Mode(input.Input[os.FileMode])`, `.User(input.Input[string])`,
`.Group(input.Input[string])`, `.SHA256(input.Input[string])`, `.ForceWrite()`.
`.Build() *reconcile.ResourceBuilder[bool]` (value = changed). The probe
compares the target `sha256sum` (when `.SHA256` is set) and mode; with no checksum
only existence and mode are checked, so a changed upstream file is **not**
re-downloaded. `Apply` creates the parent (like `Put(…).EnsureParent()`),
downloads to `<path>.tmp` with `curl -fsSL`, `mv`s it into place, then applies
mode/owner. The url and path inputs are required; a nil input panics in
`Build`.
`httpfile.SatisfiedData(targetInput, sha256Input, modeInput)` is the derived bool
node the probe uses when `.ForceWrite()` is not set.

`.ForceWrite()` drops the probe, so the node **always downloads when evaluated**.
Use it inside a gate ([`Filter`/`IfChanged`](api.md#filter--ifchanged)) to control
when. It cannot be combined with `.SHA256` (a forced write does not verify
content); the resource reports an error when both are set.

### git.Repo

Global data-node builders plus a group:

```go
remoteRevision     := git.RemoteRevisionData(S(repo)) // *reconcile.Node[string]
localRevisionProbe := git.LocalRevisionData(S(dir))   // *reconcile.Node[string]
localRevision      := localRevisionProbe.Clone()          // *reconcile.Node[string]
clone := git.Clone(S(repo), S(dir)).Node()
fetch := git.Fetch(remoteRevision, S(dir)).CheckNeeds(clone).Node()
reset := git.Reset(localRevisionProbe, remoteRevision, S(dir)).
	CheckNeeds(fetch).ApplyInvalidates(localRevision).Node()

repo := git.Repo(S(repo), S(dir)).Build() // *git.RepoGroup
```

| Builder                                                 | Node                          | Probe → Apply                                                    |
| ------------------------------------------------------- | ----------------------------- | ---------------------------------------------------------------- |
| `git.RemoteRevisionData(S(repo))`                       | `git.remote-revision:<repo>`  | data: `git ls-remote <repo> HEAD`                                |
| `git.LocalRevisionData(S(dir))`                         | `git.local-revision:<dir>`    | data: local `git -C <dir> rev-parse HEAD` (`""` when not a repo) |
| `git.TrackingRevisionData(S(dir))`                      | `git.tracking-revision:<dir>` | data: `git -C <dir> rev-parse @{u}` (`""` when no upstream)      |
| `git.Clone(S(repo), S(dir))`                            | `git.clone:<dir>`             | `dir/.git` exists → `git clone` (ensures the parent dir)         |
| `git.Fetch(remoteRevision, S(dir))`                     | `git.fetch:<dir>`             | tracking `@{u}` vs remote → `git fetch --prune`                  |
| `git.Reset(localRevisionProbe, remoteRevision, S(dir))` | `git.reset:<dir>`             | local HEAD vs remote → `git reset --hard @{u}`                   |

`git.Repo(S(repo), S(dir)).Build()` returns a `*git.RepoGroup`. `git.RepoGroup` embeds
`*reconcile.Node[reconcile.Void]`. The cover is a group of clone, fetch, reset,
remoteRevision, localRevisionProbe, and localRevision, so reading it runs all
parts. `git.RepoGroup` also exposes `RemoteRevision`, `LocalRevisionProbe`,
`LocalRevision`, `Clone`, `Fetch`, and `Reset`:

```go
repo := git.Repo(S("https://github.com/example/app.git"), S("/srv/app")).Build()
revision, err := repo.LocalRevision.Value(ctx) // local revision

// A new run resets all state and re-checks the remote.
err = reconcile.Run(ctx, func(ctx context.Context) error {
	_, err := repo.Value(ctx)
	return err
})
```

The checkout is read by **two** nodes, like UFW's status: an internal
`LocalRevisionProbe` that `git.Reset` probes, and the observed `LocalRevision` (a
[clone](cookbook.md#invalidate-a-read-your-own-mutation-changed) of the probe) it
invalidates when it applies. They must be distinct, because a mutator cannot
invalidate a node it reads. The cover reads the observed revision **last**, after
`git.Reset` invalidated it, so reading `repo.LocalRevision` after the cover
re-probes the post-reset HEAD instead of returning the pre-reset value. See the
[probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed).

```go
repo.Value(ctx)                    // clone/fetch/reset, invalidating LocalRevision
rev, err := repo.LocalRevision.Value(ctx) // post-reset HEAD
```

### docker.Compose

```go
docker.Compose(S("/srv/www/compose.yaml")).Build()
docker.Compose(S("/srv/koofr-heic/docker-compose.yml")).BuildImages().Build()
```

Ensures the compose project defined by `file` is up (`docker compose -f <file> up
-d --remove-orphans`, plus `--build` with `.BuildImages()`). Node `docker.compose:<file>`
(value = changed). Pass whichever compose file the project uses (`compose.yaml`,
`compose.yml`, `docker-compose.yaml`, `docker-compose.yml`).

The probe runs `docker compose -f <file> --progress json --dry-run up -d
--remove-orphans` and parses its stderr: any action Compose would take (create,
recreate, start, …) is drift, so a converged project reports OK
(`ok`). This needs a Compose that supports `--dry-run` and `--progress json`
(Compose v2.20+). A missing compose file is drift (the probe is skipped), and
when the project does drift Compose runs **twice** (probe, then `up`).
`docker.ComposeSatisfiedData(S(file))` is the derived bool node the resource checks.

The probe does not pass `--build`; build inputs are detected through the resolved
config (Compose's config hash), so put external inputs (a git revision, a
rendered value, …) in a service **label**. See below.

Put external inputs (a git revision, a rendered value, …) in a service **label**
via the file's template, so a change lands in Compose's config hash and `up`
recreates the service:

```yaml
services:
  koofrheic:
    build: koofr-heic
    labels:
      reconcile.revision: "{{ .Revision }}"
```

Then deploy the file with `File` and `TemplateData` (shaping the input with
`reconcile.Map`) and make Compose depend on it:

```go
data := reconcile.Map[string, any](gitLocalRev, func(ctx context.Context, rev string) (any, error) {
	return map[string]any{"Revision": rev}, nil
})
composeFile := modules.File(S("/srv/koofr-heic/compose.yaml")).
	Content(modules.TemplateData(S("/srv/koofr-heic/compose.yaml"), S(tmpl), data)).
	Build().
	Node()

docker.Compose(S("/srv/koofr-heic/compose.yaml")).BuildImages().
	Build().ApplyNeeds(composeFile).Node()
```

### github.RawFile

```go
script := github.RawFile(S("chaifeng"), S("ufw-docker"), S("ufw-docker"), S(ref)).
	Dest(S("/usr/local/bin/ufw-docker")).Mode(modules.FileMode(0o755)).Build()
```

Downloads `owner/repo`'s `path` at a pinned `ref` from
`https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>` to `Dest`, tracking
the installed ref in a ref file (default `<Dest>.ref`). It re-downloads only when
the ref changes or the file is missing, and writes the ref marker only after a
successful download. `github.RawFile(...).Build()` returns a `*github.RawFileGroup`:

- `Ref *reconcile.Node[string]`: the installed ref (the ref file's content)
- `Download *reconcile.Node[bool]`: downloads when the ref changed or the file is missing
- `WriteRef *reconcile.Node[bool]`: writes the ref marker after a download

Reading the embedded cover (a group of `Download` and `WriteRef`) ensures
the file is present; it is not memoized, so start a new `Run` to force a full
re-check (or invalidate `Ref`). Options: `.Dest(input.Input[string])` (required),
`.Mode(input.Input[os.FileMode])`, `.User(input.Input[string])`,
`.Group(input.Input[string])`, `.RefFile(input.Input[string])` (marker path),
`.BaseURL(input.Input[string])`.

The download is an `httpfile.File(...).ForceWrite()` gated by a `Filter` on the ref (or
a missing file), so the script is fetched only when needed.

### ufwdocker.UfwDocker

```go
u := ufwdocker.UfwDocker(S(ref)).Build() // *ufwdocker.UfwDockerGroup
// .BaseURL(input.Input[string]) overrides the GitHub raw host.
var _ = ufwdocker.RestartEffect() // systemctl restart ufw on ufw-docker changes
```

Installs [ufw-docker](https://github.com/chaifeng/ufw-docker): downloads the script
at the pinned `ref` (`github.RawFileGroup`), then runs `ufw-docker install` (probe:
`ufwdocker.InstalledData`, the `BEGIN UFW AND DOCKER` block in
`/etc/ufw/after.rules`) and `ufw-docker install-service` (probe:
`/etc/systemd/system/ufw-docker.service`).
Both notify `ufwdocker.Topic`; `ufwdocker.RestartEffect()` restarts ufw once.
`ufwdocker.UfwDockerGroup` exposes `Script` (`*github.RawFileGroup`), `Install` and `InstallService`.
The install steps read the script cover first, so a download that succeeded but
whose install failed retries install on the next run. The pinned `ref` must be a
ufw-docker revision that supports `install-service` (the `251123` tag predates
it), otherwise the service node drifts forever.

### Effects

Effects run a command when one of their topics is notified. The caller creates
the topics.

```go
modules.ExecEffect("reload:www").Topics(wwwTopic).
	Exec(input.List("docker", "compose", "-f", "/srv/www/compose.yaml", "exec", "-T", "www", "nginx", "-s", "reload")).
	Effect()

systemd.RestartEffect(S("ssh"), sshTopic) // systemctl restart ssh
systemd.DaemonReloadEffect(unitTopic)      // systemctl daemon-reload
```

`ExecEffect(name).Topics(…).Exec(cmdInput).Effect()` runs `cmdInput` over SSH
when any subscribed topic is notified. `systemd.Effect(nameInput, verb, topics…)`
is sugar for
`systemctl <verb> <name>` (verbs `restart`, `reload`, `try-restart`, `start`,
`stop`), with `systemd.RestartEffect`/`systemd.ReloadEffect`/`systemd.TryRestartEffect`/
`systemd.StartEffect`/`systemd.StopEffect` shorthands plus `systemd.DaemonReloadEffect`. `Effect()`
panics if no topics or no command were set.
