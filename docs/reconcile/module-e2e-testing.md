# Module e2e testing

Module e2e tests exercise Reconcile modules against real targets. They are for
behavior that cannot be covered by unit tests or the in-memory
[`gabrictest`](../testing.md#high-level-harness-gabrictest) harness.

This guide is for module authors. The general test harness API lives in
[gabrice2e.md](../gabrice2e.md). The overall test strategy lives in
[testing.md](../testing.md).

## When to write an e2e test

Write a module e2e test when the module depends on real target behavior:

- package managers such as apt or apk
- systemd services and effects
- ufw or Docker
- real git, rsync, stat, or shell commands
- files and HTTP downloads observed on a real filesystem

Keep these in unit tests or `gabrictest`:

- exact command arguments
- report names
- path construction
- file mode and ownership calculations
- unsupported manager errors
- command output parsing

Base `File`, `Dir`, `TemplateData`, data-node, command, and effect behavior is
already covered by base e2e tests. Module tests should not repeat it.

There is no SSH target for module e2e. SSH transport and target-operation
coverage lives in the `gabrice2e` and SSH e2e suites.

## Running

```bash
./gab e2e-modules
./gab e2e-modules-heavy
```

`e2e-modules` runs every test matching `^Test.*E2E$` under
`./reconcile/modules/...` with `GABRIC_E2E=1`.

`e2e-modules-heavy` also sets `GABRIC_E2E_HEAVY=1` and includes privileged
systemd, ufw, Docker-in-Docker, and installer tests.

Run one package or one test directly:

```bash
GABRIC_E2E=1 go test -p 1 -run TestAptE2E -count=1 -v ./reconcile/modules/apt
GABRIC_E2E=1 GABRIC_E2E_HEAVY=1 \
  go test -p 1 -run TestSystemdE2E -count=1 -v ./reconcile/modules/systemd
```

The suite is manual only. It is not part of CI and is not run by `./gab e2e`.

Set `GABRIC_E2E_DOCKER_DISABLED=1` to skip Docker-backed targets. Local and
LocalRemote base targets still run.

Use `-p 1` when running several packages. It keeps package test binaries
sequential so they do not build or start the same images concurrently.

## Gating and file layout

Module e2e tests live next to the module:

```text
apt/apt.go
apt/apt_test.go
apt/apt_e2e_test.go
```

The e2e file uses `package apt_test` and imports the module plus
`reconcile/modules/moduletest`.

Every e2e test must be gated. Use:

- `moduletest.Require(t)` for normal Docker tests
- `moduletest.RequireHeavy(t)` for privileged systemd, ufw, Docker-in-Docker,
  and installer tests
- a `ForEach...` helper, which calls `Require` or `RequireHeavy` internally

Do not add a package-level `TestMain` skip. It would skip the package unit tests
as well.

Name tests so they end in `E2E`, because the manual task uses
`^Test.*E2E$`.

## Targets

The `moduletest.Target` wraps a `gabrice2e` target with the effective user,
group, and capabilities:

```go
type Target struct {
    *gabrice2e.Test

    User, Group string

    Root       bool
    POSIXModes bool
    Symlinks   bool
    Linux      bool
    Systemd    bool
    Docker     bool
    Apt        bool
    Git        bool
    Rsync      bool
    Curl       bool
}
```

Use `g.POSIXModes` to guard mode and ownership assertions. Use `g.Root` to guard
operations that need root.

`g.Ops()` returns the target-selected `gabric.Ops`. `g.Path(name)` returns a
path under the target working directory, or the absolute path unchanged.
`g.Container()` returns the Docker container, or `nil` for Local and
LocalRemote.

### Target helpers

| Helper                     | Targets                                                      | Notes                    |
| -------------------------- | ------------------------------------------------------------ | ------------------------ |
| `ForEachBaseTarget`        | Docker Alpine and Ubuntu, root and `app`, LocalRemote, Local | Base modules             |
| `ForEachLinuxTarget`       | AlpineTools, UbuntuTools, root                               | User and authorized keys |
| `ForEachLinuxSysctlTarget` | UbuntuTools, privileged                                      | Heavy                    |
| `ForEachUbuntuTarget`      | Ubuntu, root                                                 | apt and package tests    |
| `ForEachAlpineTarget`      | Alpine, root                                                 | apk and package tests    |
| `ForEachToolsTarget`       | AlpineTools, UbuntuTools, root and `app`                     | git and rsync            |
| `ForEachHTTPTarget`        | Tools targets plus the `HTTP` file server                    | httpfile and github      |
| `ForEachUfwDockerTarget`   | UbuntuUfwDocker, privileged                                  | Heavy                    |
| `ForEachSystemdTarget`     | UbuntuSystemd, privileged                                    | Heavy                    |
| `ForEachDockerTarget`      | Any Docker-capable image                                     | Heavy, starts Docker     |

Direct constructors are also available:

```go
moduletest.Docker(t, moduletest.Ubuntu)
moduletest.Local(t)
moduletest.LocalRemote(t)
```

Call `moduletest.Require(t)` before a direct constructor. The helper functions
already do it.

The non-root tools targets use the `app` user. Their working directory is under
`/tmp`, so git, HTTP downloads, and rsync can write there without root.

### Local and LocalRemote

`ForEachBaseTarget` includes Local and LocalRemote. Those targets operate on the
host filesystem, so module tests must stay limited to base file and data
behavior there.

Do not run package manager, user, systemd, ufw, Docker, or other host-mutating
module tests against Local or LocalRemote. A bug in such a test would change the
machine running it.

## Writing a test

A resource test normally has four steps:

1. Dry run on a clean target.
2. First real run.
3. Second real run.
4. Assert external state.

```go
package file_test

import (
    "testing"

    "github.com/bancek/gabric/input"
    "github.com/bancek/gabric/reconcile/modules"
    "github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestFileE2E(t *testing.T) {
    moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
        t := g.T()
        path := g.Path("app.conf")
        file := modules.File(input.Static(path)).Content(input.Static([]byte("hello\n")))
        if g.POSIXModes {
            file = file.Mode(modules.FileMode(0o640))
        }

        node := file.Build().Node()

        moduletest.AssertDryRun(t, g, node)
        moduletest.AssertConverges(t, g, node)
        moduletest.AssertFile(t, g, path, []byte("hello\n"))
    })
}
```

`AssertDryRun` checks that the report contains `would-change` and no `changed` or
`failed` entries.

`AssertConverges` checks that the first run changes something and the second run
has no changes and at least one satisfied resource.

`AssertFile` reads the target file and compares its content.

### External state

Assert the module's outcome with target operations or public data nodes, not by
re-reading the module's internal report.

When the module contract fixes a target path, assert that literal path. Do not
read a module data node to compute the assertion. That would hide a path bug and
couple the test to the implementation.

```go
stat, err := g.Ops().Stat(path).RunErr()
if err != nil {
    t.Fatal(err)
}
if got, want := stat.Mode.Perm(), fs.FileMode(0o640); got != want {
    t.Errorf("mode = %v, want %v", got, want)
}
```

For package installs, use the package manager directly, such as `dpkg-query`,
`apk info`, or `apt-cache`.

For systemd state, `systemd.EnabledData` and `systemd.ActiveData` are public
data nodes. Use a raw `systemctl` check when the data node itself is under test,
or when there is no data node, such as `InvocationID`.

### Data nodes

Data nodes do not appear in the report. Test their value directly:

```go
hash, err := g.RunNodeErr(g.Context(), modules.FileHashData(input.Static(path)))
if err != nil {
    t.Fatal(err)
}
```

Test present and missing paths, documented non-zero exits, and transport errors.
Do not use `AssertConverges` for a data node.

### Groups and covers

Groups and covers are not reported. Their member resources are.

For a group such as `git.Repo`, assert the external result and the report of the
members when it matters.

`AssertConverges` requires a satisfied resource on the second run. A group whose
members are all filtered out can report nothing. In that case, use
`g.RunReportRef` and assert the statuses directly.

### Non-convergent modules

Some resources apply on every evaluation:

- `modules.Command`
- `httpfile.File(...).ForceWrite()`
- parts of `ufwdocker.UfwDockerGroup` before its install steps are present

Do not use `AssertConverges` for these. Assert the side effect after the first
run, assert the dry run did not perform it, and assert the documented second-run
behavior.

### Dependencies and effects

`ApplyNeeds` is lazy. It runs only when the resource applies, and it is not read
on a satisfied second run.

`CheckNeeds` is unconditional. Do not use a non-convergent `Command` as a
`CheckNeeds` dependency when the test expects a clean second run.

Effects are not in the report. Assert their side effects:

- `ExecEffect`: marker file or captured command
- `systemd.RestartEffect`: `systemctl show -p InvocationID --value`
- `ufwdocker.RestartEffect`: ufw invocation ID or the loaded firewall rules

A dry run must not run effects.

## HTTP tests

For httpfile and github tests use `ForEachHTTPTarget`. The helper starts:

- the tools target
- an `HTTP` file-server container on the same Docker network
- the target's client connection to that server

Write files into the server before the module runs:

```go
moduletest.ForEachHTTPTarget(t, func(g *moduletest.Target, server *moduletest.FileServer) {
    t := g.T()
    server.Put("app.bin", []byte("hello\n"))

    node := httpfile.File(input.Static(server.URL + "/app.bin")).
        Path(input.Static(g.Path("app.bin"))).
        Build().
        Node()

    moduletest.AssertConverges(t, g, node)
})
```

`FileServer.Put` writes through the file server's target operations. It creates
parent directories. The server can start before any file exists.

For GitHub raw files, use `BaseURL(input.Static(server.URL))` and create the
expected path tree below the server:

```go
server.Put("owner/repo/v1/bin/app", content)

script := github.RawFile(input.Static("owner"), input.Static("repo"), input.Static("bin/app"), input.Static("v1")).
    BaseURL(input.Static(server.URL)).
    Dest(input.Static(g.Path("app"))).
    Build()
```

No host volume, host port, or `host.docker.internal` mapping is involved.

## Systemd and Docker

Use `ForEachSystemdTarget` for systemd module tests. The helper starts the
privileged systemd container and waits until systemd reports `running` or
`degraded`.

Use `ForEachDockerTarget` for Docker-in-Docker tests. The helper:

- starts a privileged container
- sets `DOCKER_TLS_CERTDIR=`
- waits for `docker info`

`ForEachDockerTarget` is heavy. It requires `GABRIC_E2E_HEAVY`.

Use `ForEachUfwDockerTarget` for UFW Docker integration. It starts the
`UbuntuUfwDocker` image with the systemd options.

Systemd containers need privileged mode, host cgroup namespace, `/run` and
`/run/lock` tmpfs, and the systemd stop signal. The helpers set these options.

## Images

The harness images are in `reconcile/modules/moduletest/testdata`:

| Image             | Purpose                                 |
| ----------------- | --------------------------------------- |
| `Alpine`          | Minimal Alpine target                   |
| `AlpineUser`      | Alpine as `app`                         |
| `Ubuntu`          | Minimal Ubuntu target                   |
| `UbuntuUser`      | Ubuntu as `app`                         |
| `AlpineTools`     | Alpine with git, rsync, curl, and `app` |
| `UbuntuTools`     | Ubuntu with git, rsync, curl, and `app` |
| `HTTP`            | Alpine file server with BusyBox `httpd` |
| `UbuntuSystemd`   | Ubuntu with systemd, cron, and ufw      |
| `UbuntuUfwDocker` | Ubuntu systemd with Docker and ufw      |
| `Dind`            | Docker-in-Docker with Compose           |

Images are pinned to explicit versions. Do not use `latest`.

Add a new image only when the existing target cannot support the test. Prefer a
missing command or a small image variant over a general-purpose test image.

## Debugging

- `g.Container().Logs()` returns the container logs.
- `gabrice2e` dumps logs when a container fails or exits immediately.
- Images stay cached across runs.
- Containers use `--rm` and are stopped by `t.Cleanup`.
- Networks and temporary directories are cleaned up by the harness.
- Inside this repository, use `internal/testutils.Eventually` for readiness
  checks instead of sleeps.

To inspect a failed test, run the package alone with `GABRIC_E2E=1` and the
focused test regex. Add `GABRIC_E2E_HEAVY=1` for privileged tests.

## Checklist

- Test file is named `<module>_e2e_test.go`.
- package is `<module>_test`.
- Test name ends in `E2E`.
- Test uses `Require`, `RequireHeavy`, or a `ForEach...` helper.
- A dry run is checked where the module is convergent.
- The first run changes state.
- The second run is satisfied where the module is convergent.
- External state is asserted with target operations or a public data node.
- Data nodes are tested directly, not through `AssertConverges`.
- Groups and covers are checked through their members or external state.
- Non-convergent modules and effects are tested explicitly.
- Heavy tests require `GABRIC_E2E_HEAVY`.
- The test does not add CI coverage.
