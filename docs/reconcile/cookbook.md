# Reconcile cookbook

Recipes for using `reconcile` and its [modules](modules.md) with `gabric`. For the
engine model see [README.md](README.md)/[architecture.md](architecture.md) and for
the builder [api.md](api.md).

For a complete, buildable project that combines these recipes, see
[`examples/homelab`](../../examples/homelab). Remote
examples assume `gabricssh.InstallDefault()` in `main`.

Examples use a local input alias:

```go
func S[T any](value T) input.Input[T] {
	return input.Static(value)
}
```

The library call is `input.Static`. File modes use `modules.FileMode`. See
[inputs.md](inputs.md) for the input model.

## A bootstrap task

Bring a host to a baseline with packages, sshd hardening, forwarding, and units,
all in one task backed by one group node.

```go
var sshTopic = reconcile.NewTopic("ssh")

// Restart ssh once when the hardening file changes.
var _ = systemd.RestartEffect(S("ssh"), sshTopic)

// apt-get update is a node; package nodes Need it (see the next recipe).
var aptUpdate = apt.Update().Build().Node()

var packages = []*reconcile.Node[bool]{
	pkg.Package(S("ufw")).Build().ApplyNeeds(aptUpdate).Node(),
	pkg.Package(S("fail2ban")).Build().ApplyNeeds(aptUpdate).Node(),
	pkg.Package(S("unattended-upgrades")).Build().ApplyNeeds(aptUpdate).Node(),
	pkg.Package(S("ca-certificates")).Build().ApplyNeeds(aptUpdate).Node(),
	pkg.Package(S("curl")).Build().ApplyNeeds(aptUpdate).Node(),
	pkg.Package(S("rsync")).Build().ApplyNeeds(aptUpdate).Node(),
}

var sshdHardening = modules.File(S("/etc/ssh/sshd_config.d/99-hardening.conf")).
	Content(S([]byte(hardening))).Mode(modules.FileMode(0o644)).
	Build().Notify(sshTopic).Node()

var autoReboot = modules.File(S("/etc/apt/apt.conf.d/51-auto-reboot")).
	Content(S([]byte(autoRebootConf))).Mode(modules.FileMode(0o644)).Build().Node()

var ipForward = linux.Sysctl(S("net.ipv4.ip_forward"), S("1")).Node()

var unattendedUpgrades = service.Service(S("unattended-upgrades")).Build().Node()
var fail2ban = service.Service(S("fail2ban")).Started().Build().Node()

// One node per task: reading it runs (and memoizes) every part.
var bootstrapNode = reconcile.Group("bootstrap").
	Contains(packages...).
	Contains(sshdHardening, autoReboot).
	Contains(ipForward).
	Contains(unattendedUpgrades, fail2ban).
	Node()

var Bootstrap = gabric.With(gabric.WithRemoteSudo(&gabric.Sudo{})).NodeTask(bootstrapNode)
```

For cross-distro code, use `pkg.Update().Build().Node()` as the dependency instead of
`aptUpdate`; it selects `apt.Update` or `apk.Update` for the target. Add
`.MaxAge(S(24 * time.Hour))` to refresh only when the target package cache is older.

`NodeTask` evaluates the node and flushes its effects at the end of the task, so
the ssh restart runs once if the hardening file changed, and stays attached to
the `ssh` topic, not to the task order.

## Configure the local machine

Modules use `gabric.TargetOps`, so the same graph can run locally. Use
`LocalTarget` for native local modules, for example declaratively creating files
and directories. Use `LocalRemoteTarget` plus `WithRemoteSudo` when the local run
should match a remote Unix host; it keeps remote-style `/bin/sh`, coreutils,
working directories, and real sudo:

```go
var localRoot = gabric.With(
	gabric.WithReconcile,
	gabric.WithLocalRemoteTarget,
	gabric.WithRemoteSudo(&gabric.Sudo{}),
)

var Configure = localRoot.NodeTask(
	modules.File(S("/etc/motd")).Content(S([]byte("managed\n"))).Mode(modules.FileMode(0o644)).Build().Node(),
)
```

`gabric.WithLocalRemoteTarget` is equivalent middleware for a whole run.
`LocalTarget` runs commands directly through Go `exec`/`os` with local paths and
no implicit shell or sudo. It works on Windows for target operations; individual
module support still depends on the module. Use it for ordinary local Reconcile
work, such as files and directories.

Use `LocalRemoteTarget` when the local run should match a remote Unix host:
`/bin/sh`, coreutils, remote working directories and environment, and
`WithRemoteSudo`. Use this target to run a binary built for a remote machine in
place. It needs `/bin/sh` and coreutils, so it is Unix-like only; use
it when you need local sudo or rsync. There is no separate local-sudo API;
remote sudo is the sudo setting for remote-style operations, including
`LocalRemote`.

Custom data nodes, effects, and task logic follow the selected target only when
they call `gabric.TargetOps`. `Remote` and `Local` are explicit escapes.

## Default unprivileged, sudo per node and effect

Run the task without a sudo base, then opt individual nodes and effects into
sudo. `Node.With` and `Effect.With` take the same `middleware.Middleware` used by
tasks:

```go
var sudo = gabric.WithRemoteSudo(&gabric.Sudo{})

var aptUpdate = apt.Update().Build().Node().With(sudo)

var ufwPkg = pkg.Package(S("ufw")).
	Build().
	ApplyNeeds(aptUpdate).
	Node().
	With(sudo)

var _ = systemd.RestartEffect(S("ssh"), sshTopic).With(sudo)
```

`Node.With` applies the middleware to the node's whole evaluation, including
`ApplyNeeds`, `CheckNeeds`, and dependencies, so `aptUpdate` above also runs with
sudo. It returns a distinct node identity; use the wrapped node in groups,
dependencies, and `ApplyInvalidates`.

Node state is memoized by identity, not context. If the same dependency is read
by both privileged and unprivileged nodes, wrap the dependency itself (as with
`aptUpdate`) so its context is consistent.

Effects run at flush, outside node middleware, so they need their own
`Effect.With`. It replaces the effect's topic subscriptions and may skip the
effect by not calling `next`. Build effects with `With` before the first flush.

## Run the same graph over SSH or locally

A graph that uses `TargetOps` can target either an SSH host or the machine
running the binary. Add a flag and select the target in main middleware:

```go
var local = gabric.Default.FlagSet.Bool("local", false, "run locally")

func main() {
	gabricssh.InstallDefault()
	gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
		if *local {
			ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
		}
		return next(ctx)
	}))
}
```

Remote mode runs the binary on the operator and connects over SSH. Local mode
runs the binary on the target with `-local`; the graph, nodes, and reports are
the same. Runtime inputs that are not embedded must travel with the binary:
`secrets/`, `srv/www/`, and any other files read from the working directory.

For a complete build/upload/run path and cross-transport e2e, see the
[homelab example](../../examples/homelab) and its `deploy-local`,
`e2e`, and `vm-*` tasks. Run the e2e from that directory with `./gab e2e`; it is
local-only and not part of CI.

## `apt-get update` only when something is installed

`ApplyNeeds` reads its dependencies **in the apply path**: only when the node
drifts (and inside `Do`), so a dry run skips them.

```go
var aptUpdate = apt.Update().Build().Node()
var ufwPkg = pkg.Package(S("ufw")).Build().ApplyNeeds(aptUpdate).Node()
```

Reading `ufw` probes with `dpkg-query`; only if it is missing does `aptUpdate`
run, then the install. If `ufw` is already installed, `apt-get update` never runs.
`aptUpdate` is memoized, so many `pkg.Package(…).ApplyNeeds(aptUpdate)` nodes share one
update.

## Dynamic file content (memoized once per run)

Put a runtime read in a **data node**, then pass it to `Content`. The data
node is memoized per run, so the read happens once and is available in a dry run
(reading it directly in a `Check`/`Apply` closure would run it twice):

```go
var hostArch = reconcile.Data("host-arch", func(ctx context.Context) (string, error) {
	out, err := gabric.TargetOps(ctx).Exec("dpkg", "--print-architecture").
		Capture().Run().StdoutStr()
	return strings.TrimSpace(out), err
}).Node()

var dockerSource = modules.File(S("/etc/apt/sources.list.d/docker.list")).
	Content(reconcile.Data("docker-source", func(ctx context.Context) ([]byte, error) {
		arch, err := hostArch.Value(ctx) // one evaluation per run
		if err != nil {
			return nil, err
		}
		return []byte(fmt.Sprintf("deb [arch=%s] …\n", arch)), nil
	}).Node()).
	Mode(modules.FileMode(0o644)).Build().Node()
```

See [api.md](api.md#data) for the memoization note. For `TemplateData`, the
template text is an input too: `TemplateData(S(path), S(tmpl), data)`. Use
[`reconcile.Map`](api.md#map) to turn a read into the value the template
expects.

## Restart / reload on change

A config file notifies a topic; one effect reloads the service once, only when
something changed. Multiple producers notify the same topic:

```go
var dockerTopic = reconcile.NewTopic("docker")

var _ = reconcile.NewEffect("restart-docker", []*reconcile.Topic{dockerTopic},
	func(ctx context.Context) error {
		return gabric.TargetOps(ctx).Exec("systemctl", "restart", "docker").RunErr()
	})

var daemonConf = modules.File(S("/etc/docker/daemon.json")).
	Content(S([]byte(dockerDaemon))).Mode(modules.FileMode(0o644)).
	Build().Notify(dockerTopic).Node()
```

`systemd.RestartEffect`/`modules.ExecEffect` are sugar over the same
`reconcile.NewEffect` (see [modules.md](modules.md#effects)):

```go
var _ = systemd.RestartEffect(S("docker"), dockerTopic)
var _ = modules.ExecEffect("reload-www").Topics(wwwTopic).
	Exec(input.List("docker", "compose", "-f", "/srv/www/compose.yaml", "exec", "-T", "www", "nginx", "-s", "reload")).
	Effect()
```

Effects are for reactions that run _once per change at flush_. When a node should
not run unless something changed, use `Filter`/`IfChanged` (see the next recipe).

## Only when it changed (`Filter` / `IfChanged`)

A node can be evaluated only when a condition holds. `Filter` takes a function
condition (`ValueMatches`/`ValueEquals`/`NodesEqual` build one from node
values; see [api.md](api.md#valuematches--valueequals--nodesequal)). `IfChanged`
takes a `*reconcile.Node[bool]`, for example a resource's value, which is `true`
when it applied and `false` when it was already satisfied.
When the condition is false the source is **not evaluated** (no side effects) and
the node yields the zero value. Invalidating the gate does not invalidate the
hidden source; invalidate the source (or start a new `Run`) to force a re-probe.

```go
// Only redeploy when the checked-out revision changed.
var redeploy = reconcile.Filter(deploy, reconcile.ValueMatches(repo.LocalRevision, func(rev string) bool {
	return rev != deployedRev
}))

// Only write the marker after the download ran (download is a *Node[bool]).
var writeRevision = reconcile.IfChanged(
	modules.File(S("/srv/app/.revision")).Content(S([]byte(rev))).Mode(modules.FileMode(0o644)).Build().Node(),
	download,
)
```

Unlike a topic/effect (a reaction that runs **once at flush when something
changed**; see "Restart / reload on change"), `Filter`/`IfChanged` means the work
does not run unless the condition holds. The condition is a data node,
so it runs in a dry run; the gated source's probe runs too (its mutation does not).

## A git-backed service

`git.Repo` clones/updates a repository; read the checked-out revision with
`repo.LocalRevision`. The repo cover is a group, not an owner: start a new
`Run` to re-check the remote from scratch.

```go
var repo = git.Repo(S("https://github.com/example/app.git"), S("/srv/app/app")).Build()

revision, err := repo.LocalRevision.Value(ctx)

// A new, isolated run resets all state and re-checks the remote.
err = reconcile.Run(ctx, func(ctx context.Context) error {
	_, err := repo.Value(ctx)
	return err
})
```

`repo.LocalRevision` is the **observed** revision: `git.Reset` invalidates it after
it applies and the cover reads it last, so reading it once the repo has been read
reflects the post-reset checkout (the probe `git.Reset` compares against is the
separate `repo.LocalRevisionProbe`). Read it after `repo.Value(ctx)` to get the
revision checked out. Invalidating the cover does **not** reset its
parts.

A `Compose` project can key its rebuild on that revision by putting it in a
service label (via the compose file's template), so a new upstream commit makes
`docker compose up` recreate the container; see
[modules.md](modules.md#dockercompose). Alternatively, gate the rebuild directly on the
revision with `Filter`/`IfChanged` (see "Only when it changed").

## Pin a downloaded file to a revision (`github.RawFile`)

`github.RawFile` downloads `owner/repo`'s `path` at a pinned `ref` (a commit,
since tags can move) from `raw.githubusercontent.com` to a remote destination,
records the installed ref in a marker file next to it, and re-downloads only
when the ref changes or the file is missing:

```go
const ufwDockerRef = "78366b6afe6e566cd53f7e55341889c2e3c863e7"

var script = github.RawFile(S("chaifeng"), S("ufw-docker"), S("ufw-docker"), S(ufwDockerRef)).
	Dest(S("/usr/local/bin/ufw-docker")).Mode(modules.FileMode(0o755)).Build()

// reading script.Node ensures the file is present at that ref
```

It records the ref in `Dest + ".ref"` (override with `.RefFile`), and writes the
marker **only after a successful download**, so a failed transfer is retried and
never advances the marker. To force a re-download, start a new `Run` (or
invalidate `script.Ref`, the data node the download is gated on). Bump
`ufwDockerRef` to update the file. See
[modules.md#githubrawfile](modules.md#githubrawfile); `ufwdocker.UfwDocker` is built on it.

## A revision marker, by hand

`github.RawFile` is a small group you can build from primitives. The design:
read the installed ref with a **data** node, gate the download on it, and write
the marker only after the download succeeds. `httpfile.File(...).ForceWrite()` downloads
unconditionally when evaluated; `FileContentData`/`FileExistsData` read the marker
and the file.

```go
const ref = "abc123"
const url = "https://example.com/app.bin"
const dest = "/usr/local/bin/app"
const refFile = dest + ".ref"

installedRef := modules.FileContentData(S(refFile)) // data: current ref ("" if none)
destExists := modules.FileExistsData(S(dest))       // data: is dest present?

// ForceWrite drops the probe, so it downloads whenever it is evaluated.
downloadMaybe := httpfile.File(S(url)).Path(S(dest)).Mode(modules.FileMode(0o755)).ForceWrite().Build().Node()
// Download only when the ref changed or the file is missing.
download := reconcile.Filter(downloadMaybe, func(ctx context.Context) (bool, error) {
	cur, err := installedRef.Value(ctx)
	if err != nil {
		return false, err
	}
	if cur != ref {
		return true, nil
	}
	exists, err := destExists.Value(ctx)
	if err != nil {
		return false, err
	}
	return !exists, nil
})
// Write the marker only after a download (download's bool is true).
writeRef := reconcile.IfChanged(
	modules.File(S(refFile)).Content(S([]byte(ref))).Mode(modules.FileMode(0o644)).Build().Node(),
	download,
)

// A cover: reading it ensures both (start a new Run to reset).
var script = reconcile.Group("script:" + dest).
	Contains(download, writeRef).
	Node()
```

This is the "data + resource on the same file" pattern: a data node reads the
marker, a resource writes it, and the gate makes them agree. It is also why
`IfChanged` takes a node (the download's changed `bool`) while `writeRef`'s source
runs only after a successful download.

See also [api.md#filter--ifchanged](api.md#filter--ifchanged).

## Dry run

`gabric -dry-run` marks the run context; Reconcile probes but never mutates:

```bash
go run . -dry-run all
```

For library use, mark the context yourself:

```go
ctx = dryrun.With(ctx)
```

## Shared runs and hosts

Register the global middleware so every task in an invocation shares one run
(nodes evaluated at most once, effects flushed once):

```go
var _ = gabric.Use(gabric.WithReconcile)
```

A run can span several hosts when each host uses distinct nodes. The unsafe case
is evaluating the same node or node graph with different host contexts in one
run: node state is memoized by node identity, so one host's result can be reused
for another. A host-specific node can call `gabricssh.NewHostContext` itself, and a
per-host graph factory produces distinct nodes.

Reusing one node task across hosts is safe when each call gets a fresh run. A
`NodeTask` on a builder with `WithReconcile` does that for each call:

```go
var appTask = gabric.With(gabric.WithReconcile).NodeTask(appNode)

appTask(gabricssh.NewHostContext(ctx, staging))
```

`NodeTask` evaluates the node and flushes its effects; the middleware supplies
the run. When the global middleware is also registered, this is a nested run, so
it does not share node values with the global one. Nested runs inherit
`dryrun.Enabled(ctx)` and the flush bound, so a dry run still probes the other host's
graph without applying. `NodeTaskErr` is the error-first form.

Independent targets run concurrently with `gabric.Parallel`. Because each call
gets its own run, the same `appTask` can target staging or production hosts
without sharing node values:

```go
gabric.Parallel(ctx,
	func(ctx context.Context) {
		appTask(gabricssh.NewHostContext(ctx, app1))
	},
	func(ctx context.Context) {
		appTask(gabricssh.NewHostContext(ctx, app2))
	},
)
```

`ParallelErr` is the error-first form. Alternatively, build a distinct node
graph per host and evaluate those nodes in a shared run.

## Memoize a value shared by `Check` and `Apply`

A resource's value comes from `Apply`, so anything the **probe** also needs must
be a data node (read once per run, dry-run safe) rather than a closure that runs
twice:

```go
var version = reconcile.Data("version", func(ctx context.Context) (string, error) {
	return readVersion(ctx)
}).Node()

var app = reconcile.Resource("app",
	reconcile.ValueEquals(version, desired), // reads the memoized data node
	func(ctx context.Context) (string, error) {
		return upgrade(ctx)
	}).
	Node()
```

## Invalidate a read your own mutation changed

A resource's `Apply` often changes state that a **data node** also reads. A git
`reset` changes HEAD, and `ufw enable` changes the status. When a later consumer
must see the new state, the mutator declares `.ApplyInvalidates(reader)`, so the
read re-probes after the change (see [api.md](api.md#applyinvalidates)).

The mutator's own `Check` usually reads that same node to decide whether to
apply, and the engine rejects invalidating a node the in-flight node reads
(`ErrConcurrentUse`). Use **two** nodes for the state: a _probe_ the resource
reads and an _observed_ node it invalidates. Have the cover read the observed one
**last**, after the mutation:

```go
// One read, cloned: a probe the resource checks, an observed node it invalidates.
probe := reconcile.Data("app-version", readVersion).Node()
observed := probe.Clone()

var app = reconcile.ResourceBool("app",
	reconcile.ValueEquals(probe, desired), // the mutator checks the probe…
	func(ctx context.Context) error { return upgrade(ctx) }).
	ApplyInvalidates(observed). // …and invalidates the observed node (not the probe)
	Node()

// The cover reads the mutator and both reads, the private probe included, and
// reads the observed node last so it re-probes after a change. It is a
// group (not an owner).
var cover = reconcile.Group("app").
	Contains(probe, app).
	Contains(observed).
	Node()
```

Reading `cover` runs `app` (which invalidates `observed` on drift) and then reads
`observed`, re-probing the version. The cover is a group: it is not
memoized and owns nothing, so `cover.Invalidate(ctx)` does not reset the parts.
To reset the whole thing, start a new `Run` (or invalidate `probe` and `observed`
individually). `ufw.Firewall` and `git.Repo` use this pattern with their
probes (`Probe`, `LocalRevisionProbe`); see [modules.md](modules.md#ufwfirewall) and
[modules.md](modules.md#gitrepo).

If nothing outside the mutator reads the aliased state, a single node is fine:
the probe/observed split only matters when a later reader must see the new value.
