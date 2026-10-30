# Cookbook

Short, practical recipes for common Gabric use cases. For the execution model
see [architecture.md](architecture.md); for signatures see [api.md](api.md) and
`go doc`.

Examples assume a task body signature `func(ctx context.Context)`. Imports are
omitted for brevity. Remote examples assume `gabricssh.InstallDefault()` in
`main`; local examples do not need it.

## Hosts

### Choose the host from the task argument

Use the task argument to override the host from `-ssh-host`, and fall back to `-ssh-host` when
none is given:

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	if arg := gabric.GetCurrentTaskArg(ctx); arg != "" {
		host, err := gabricssh.ResolveHost(ctx, arg)
		gabric.Must(err)
		ctx = gabricssh.NewHostContext(ctx, host)
	}

	gabric.Remote(ctx).Exec("uname", "-a").Run()
})
```

```bash
go run . -ssh-host deploy@example.com deploy:deploy@staging.example.com
```

### Default host when `-ssh-host` is not given

Main middleware runs before dispatch, so you can supply a fallback host:

```go
gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	if gabric.HasHost(ctx) {
		return next(ctx)
	}

	host, err := gabricssh.ResolveHost(ctx, "deploy@example.com")
	if err != nil {
		return err
	}

	return next(gabricssh.NewHostContext(ctx, host))
}))
```

### Run a task against another host

Tasks are functions, so call one with a derived context:

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	gabric.Remote(ctx).Rsync().Local("./app").Remote("/srv/app").Upload().Run()
})

var DeployBoth = gabric.Task("deploy_both", func(ctx context.Context) {
	blue, err := gabricssh.ResolveHost(ctx, "deploy@blue.example.com")
	gabric.Must(err)
	Deploy(gabricssh.NewHostContext(ctx, blue))

	green, err := gabricssh.ResolveHost(ctx, "deploy@green.example.com")
	gabric.Must(err)
	Deploy(gabricssh.NewHostContext(ctx, green))
})
```

### Override host fields

Derive a new immutable host with the settings you need:

```go
host := gabricssh.GetHost(ctx).
	WithPort(2222).
	WithUser("root").
	WithPrivateKeysPaths([]string{"/home/me/.ssh/deploy_key"}).
	WithKnownHostsPaths([]string{"/home/me/.ssh/known_hosts"}).
	WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
	WithDisplayName("web1")

ctx = gabricssh.NewHostContext(ctx, host)
```

`IdentityFile ~/...` from `~/.ssh/config` is expanded by `ResolveHost`, but
paths you set yourself are used as-is (use absolute paths or
`os.UserHomeDir`). Host private keys are tried before `Config.PrivateKeysPaths`;
host known_hosts and strict-checking values override config defaults.

### Jump host

Tunnel through another SSH host by setting `Transport` to a
`sshremote.JumpTransport` (Gabric dials it directly). When resolving through
`gabricssh.ResolveHost`, `ProxyJump` and the recognized `ProxyCommand` forms are
turned into the matching `Transport` automatically:

```go
jump := sshremote.NewHost("jump.example.com").
	WithUser("deploy").
	WithPrivateKeysPaths([]string{"/home/me/.ssh/jump_key"})

target := sshremote.NewHost("10.0.0.5").
	WithUser("deploy").
	WithPrivateKeysPaths([]string{"/home/me/.ssh/app_key"}).
	WithTransport(sshremote.JumpTransport{Host: jump})

ctx = gabricssh.NewHostContext(ctx, target)
```

`~/.ssh/config` `Host` aliases (including jump hosts) are also honored when you
resolve through `gabricssh.ResolveHost`.

## Remote config

For the model see [ssh/sshremote.md](ssh/sshremote.md).

### Global tweaks in main middleware

Main middleware runs after the installed transport setup, so changes apply
everywhere:

```go
gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	config := gabricssh.GetConfig(ctx).
		WithConnectionTimeout(time.Minute).
		WithAgentSocket("") // don't use the ssh agent

	return next(gabricssh.NewConfigContext(ctx, config))
}))
```

Pooling limits live on the immutable remote config. Set the config before the
first remote operation; the remote manager itself is only a cache and creates
each pool from the config passed to `GetRemote`:

```go
config := sshremote.NewConfig().
	WithIdleConnections(2).
	WithMaxConnections(10)

ctx := gabricssh.NewConfigContext(context.Background(), config)

gabric.Main(gabric.WithContext(ctx))
```

### Per-branch config

`Config` is immutable and its pointer is part of the `Remote` identity, so derive
a context with a new config instance to scope it to one branch:

```go
config := gabricssh.GetConfig(ctx).
	WithConnectionTimeout(5 * time.Second)

// A new config pointer also gets its own Remote and pool.
host := gabricssh.GetHost(ctx).Clone()

branch := gabricssh.NewConfigContext(ctx, config)
branch = gabricssh.NewHostContext(branch, host)

gabric.Remote(branch).Exec("systemctl", "status", "app").Run()
```

> **Gotcha:** the `*Config` pointer is the pool identity. Equal values in two
> different instances use two different pools; reuse one instance for branches
> that should share a pool. See [ssh/sshremote.md](ssh/sshremote.md#pooling).

> **Precedence:** host ssh_config values take precedence over config defaults;
> see [ssh/sshremote.md](ssh/sshremote.md#hosts-and-resolution) for the full rules.

### Log fingerprints instead of verifying

Set the callback with `Config.WithHostKeyCallback`; it is used for every
connection regardless of the host key policy, which helps for first-time
access through proxies:

```go
gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	config := gabricssh.GetConfig(ctx).WithHostKeyCallback(func(hc sshremote.HostConfig) (ssh.HostKeyCallback, error) {
		return func(hostname string, addr net.Addr, key ssh.PublicKey) error {
			gabric.Logger(ctx).Info("host key",
				"host", hostname,
				"fingerprint", ssh.FingerprintSHA256(key),
			)
			return nil
		}, nil
	})

	return next(gabricssh.NewConfigContext(ctx, config))
}))
```

### Trust on first use

Add unknown host keys, but never accept a mismatch:

```go
host := gabricssh.GetHost(ctx).
	WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew)

ctx = gabricssh.NewHostContext(ctx, host)
```

Host-based overrides take precedence. For hosts without an ssh_config
`StrictHostKeyChecking`, derive the config with
`config = config.WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingAcceptNew)`.
`KnownHostsAcceptNewCallback(paths...)` exposes the same behavior when building
a custom `WithHostKeyCallback`.

## Sudo and working directories

### Global sudo

```go
var _ = gabric.Use(gabric.WithRemoteSudo(&gabric.Sudo{}))
```

### Sudo as another user or with flags

```go
var _ = gabric.Use(gabric.WithRemoteSudo(&gabric.Sudo{
	User:  "deploy",
	Flags: []string{"-n"},
}))
```

### Sudo for a subset of steps

```go
var Install = gabric.Task("install", func(ctx context.Context) {
	sudoCtx := gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})

	gabric.Remote(sudoCtx).Put("/etc/app/config").String(config).Mode(0o600).Run()
	gabric.Remote(sudoCtx).Exec("systemctl", "restart", "app").Run()
})
```

### Working directories

```go
gabric.Remote(ctx).WithWorkingDir("/srv/app", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("git", "pull").Run()
})

gabric.Local(ctx).WithWorkingDir("./build", func(ctx context.Context) {
	gabric.Local(ctx).Exec("make").Run()
})
```

Use the `*Err` form when the callback should return an error:

```go
err := gabric.Remote(ctx).WithWorkingDirErr("/srv/app", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("git", "pull").RunErr()
})
```

### Local sudo with `LocalRemoteTarget`

See the [built-in target comparison](api.md#target-operations) for command,
file-op, and sudo differences.

Use `LocalTarget` for native local work, such as using modules to create local
files and directories: Go `exec`/`os`, local paths, no implicit shell, and no
sudo. Its target operations work on Windows; individual module support still
depends on the module. Use `LocalRemoteTarget` when the local run should match a
remote Unix host (`/bin/sh`, coreutils, remote paths and environment, and sudo),
or when you need rsync. It is also the target for running a binary built
for a remote machine in place. `LocalRemoteTarget` is Unix-like only;
`LocalTarget` has no rsync.

For a whole run:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalRemoteTarget))

var _ = gabric.Use(gabric.WithRemoteSudo(&gabric.Sudo{}))
```

For a single scope:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})

gabric.TargetOps(ctx).Put("/etc/app/config").String(config).Mode(0o600).Run()
```

Or for a task builder:

```go
var localRoot = gabric.With(
	gabric.WithLocalRemoteTarget,
	gabric.WithRemoteSudo(&gabric.Sudo{}),
)
```

Caveats:

- real sudo is required; `/bin/sh` and coreutils must exist (Unix-like);
- remote working-directory and environment semantics still apply;
- direct `Local(ctx)` / `Remote(ctx)` calls do not follow `TargetOps`;
- use passwordless sudo (or `sudo -n`) for non-interactive runs.

### Build once, run on the target

One approach is to build on the operator, upload the directory, and run the
binary in local mode on the target:

```go
var local = gabric.Default.FlagSet.Bool("local", false, "run against the local machine")

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

```go
func deployLocal(ctx context.Context) error {
	ctx = gabric.NewRemoteWorkingDirContext(ctx, "")
	ctx = gabric.NewRemoteSudoContext(ctx, nil)

	err := gabric.Local(ctx).WithWorkingDirErr(".", func(ctx context.Context) error {
		return gabric.Local(ctx).
			Exec("go", "build", "-trimpath", "-o", "bin/homelab", ".").
			WithEnv(
				gabric.Env("GOOS", "linux"),
				gabric.Env("GOARCH", "arm64"),
				gabric.Env("CGO_ENABLED", "0"),
			).
			RunErr()
	})
	if err != nil {
		return err
	}

	err = gabric.Remote(ctx).Rsync().
		Local("./").
		Remote("/home/deploy/homelab").
		Upload().
		Exclude(".git", ".vagrant", "secrets/keys.txt").
		RunErr()
	if err != nil {
		return err
	}

	return gabric.Remote(ctx).WithWorkingDirErr(
		"/home/deploy/homelab",
		func(ctx context.Context) error {
			return gabric.Remote(ctx).Exec("./bin/homelab", "-local", "all").RunErr()
		})
}
```

Notes:

- Runtime inputs (e.g. `secrets/`, `srv/`) must travel with the binary
  unless embedded.
- Build for the target architecture with `CGO_ENABLED=0`; the homelab
  `deploy-local` task maps `uname -m` and builds `linux/<arch>`.
- The remote side needs `rsync` before the upload; install it first if the
  graph's bootstrap has not run.
- In homelab example, local mode needs `age` and `rsync` on the target.
- `LocalRemote` is Unix-only for this use.

See the [homelab example](../examples/homelab) for the complete
`deploy-local`, VM, and e2e tasks, and the
[Reconcile cookbook](reconcile/cookbook.md) for running the same graph locally.

## Environment and arguments

### Application values from flags

Register application flags on the runner and attach them in main middleware:

```go
var (
	stageFlag   = gabric.Default.FlagSet.String("stage", "staging", "deployment stage")
	versionFlag = gabric.Default.FlagSet.String("version", "", "release version")
)

type deployConfig struct {
	Stage   string
	Version string
}

type deployConfigKey struct{}

func main() {
	gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
		ctx = context.WithValue(ctx, deployConfigKey{}, deployConfig{
			Stage:   *stageFlag,
			Version: *versionFlag,
		})
		return next(ctx)
	}))
}
```

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	cfg, _ := ctx.Value(deployConfigKey{}).(deployConfig)
	gabric.Remote(ctx).Exec("echo", cfg.Stage, cfg.Version).Run()
})
```

```bash
go run . -stage staging -version 1.2.3 deploy
```

### Environment for a local command

```go
ctx = gabric.NewLocalExecEnvContext(ctx, gabric.Env("GOFLAGS", "-trimpath"))
gabric.Local(ctx).Exec("go", "build", "./...").Run()

// or scoped to a callback
gabric.Local(ctx).WithExecEnv(
	gabric.NewExecEnv(gabric.Env("CGO_ENABLED", "0")),
	func(ctx context.Context) {
		gabric.Local(ctx).Exec("go", "build", "./...").Run()
	},
)
```

### Environment for a remote command

Set a target-wide environment in main middleware or in the task:

```go
ctx = gabric.NewRemoteExecEnvContext(ctx,
	gabric.Env("DEBIAN_FRONTEND", "noninteractive"),
	gabric.SensitiveEnv("GITHUB_TOKEN", token),
)

gabric.Remote(ctx).Exec("apt-get", "install", "-y", "ufw").Run()
```

The values are exported inside the remote shell command, so they reach the
command even through sudo. For one command, use `Exec.WithEnv`:

```go
gabric.Remote(ctx).Exec("apt-get", "install", "-y", "ufw").
	WithEnv(gabric.Env("DEBIAN_FRONTEND", "noninteractive")).
	Run()
```

`SensitiveEnv` values are redacted by logs and formatting; they are not hidden
from the target process or the remote command line.

For target operations, configure working-directory and exec-environment
overlays that `TargetOps` applies to the selected target:

```go
ctx = gabric.NewTargetOpsWorkingDirContext(ctx, "/srv/app")
ctx = gabric.NewTargetOpsExecEnvContext(ctx,
	gabric.Env("DEBIAN_FRONTEND", "noninteractive"),
)
```

`NewTargetOpsExecEnvContext` sets the exec-env overlay;
`AppendTargetOpsExecEnvContext` adds to it. `ops.WithWorkingDir(...)` and
`ops.WithExecEnv(...)` override the target overlays for their callback. The
overlays apply only to `TargetOps` consumers, not direct `Local(ctx)` /
`Remote(ctx)` calls.

### Task arguments

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	stage := gabric.GetCurrentTaskArg(ctx)
	gabric.Logger(ctx).Info("deploying", "stage", stage)
})
```

```bash
go run . deploy:staging
```

## Composition and middleware

### Gate a deploy

Return `nil` without calling `next` to skip the task:

```go
func skipWhenDeployed(releaseDir string) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		exists, err := gabric.Remote(ctx).Exists(releaseDir).Dir().RunErr()
		if err != nil {
			return err
		}

		if exists {
			gabric.Logger(ctx).Info("already deployed", "dir", releaseDir)
			return nil
		}

		return next(ctx)
	}
}

var Deploy = gabric.With(skipWhenDeployed("/srv/app/current")).Task("deploy", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()
})
```

Middleware that scopes sudo, a working directory, a host or config uses the same
pattern. Middlewares run in declaration order; the builder is immutable, so a
shared base can be extended safely. See
[api.md](api.md#with-taskbuilder-and-middleware) for the builder and
examples. Calling a registered task from another task re-enters its middleware.

## Error handling

### Probe, then decide

Use `RunErr`/`*Err` when you need to react instead of aborting:

```go
var Check = gabric.Task("check", func(ctx context.Context) {
	if err := gabric.Remote(ctx).Exec("nginx", "-t").RunErr(); err != nil {
		gabric.Logger(ctx).Error("invalid nginx config", "error", err)
	}
})
```

### Error-first tasks and `Must`

```go
var Dist = gabric.TaskErr("dist", func(ctx context.Context) error {
	return gabric.Local(ctx).Exec("go", "build", "./...").RunErr()
})

var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	gabric.Must(Dist(ctx))
	gabric.Remote(ctx).Rsync().Local("./app").Remote("/srv/app").Upload().Run()
})
```

The task wrapper recovers panics into errors; `Main` logs a failed task and
exits non-zero.

## Concurrency

Each goroutine gets its own context (and pooled connection):

```go
var DeployAll = gabric.Task("deploy_all", func(ctx context.Context) {
	hosts := []*sshremote.Host{blue, green}

	var wg sync.WaitGroup
	errs := make(chan error, len(hosts))

	for _, host := range hosts {
		wg.Add(1)

		go func() {
			defer wg.Done()

			hostCtx := gabricssh.NewHostContext(ctx, host)
			errs <- gabric.Remote(hostCtx).Exec("systemctl", "restart", "app").RunErr()
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		gabric.Must(err)
	}
})
```

For independent tasks that share one context, `gabric.Parallel` runs them
concurrently and joins their failures, for example a CI task group:

```go
var CI = gabric.Task("ci", func(ctx context.Context) {
	gabric.Parallel(ctx, CheckGofmt, Vet, Test)
})
```

`Parallel` takes panic-style task callables and panics with the joined errors;
`ParallelErr` takes error-first functions and returns `errors.Join`. A
`TaskFunc` is an alias of `func(ctx) error`, so both plain functions and
`TaskFunc` values work with either form. `Parallel` passes one context to every
task, so derive per-branch contexts inside each task. See
[architecture.md](architecture.md#concurrency) for the Reconcile run caveat and
[reconcile/README.md](reconcile/README.md#using-with-gabric) for node-level
guidance.

## Library use

`gabric.Main` is optional. Build a context yourself and dispatch tasks
programmatically:

```go
func main() {
	config, err := sshremote.NewDefaultConfig()
	if err != nil {
		panic(err)
	}

	log := slog.New(logger.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()
	ctx = logger.NewLoggerContext(ctx, log)
	ctx = gabricssh.NewConfigContext(ctx, config)

	host, err := gabricssh.ResolveHost(ctx, "deploy@example.com")
	if err != nil {
		panic(err)
	}
	ctx = gabricssh.NewHostContext(ctx, host)

	manager := sshremote.NewRemoteManager()
	defer manager.Close()
	ctx = gabricssh.NewManagerContext(ctx, manager)
	ctx = gabricssh.NewRunnerContext(ctx)

	g := gabric.New()
	g.Task("deploy", func(ctx context.Context) {
		gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()
	})

	calls, err := g.ParseTasks([]string{"deploy:staging"})
	if err != nil {
		log.Error("invalid task", "error", err)
		os.Exit(1)
	}

	if err := g.Run(ctx, calls); err != nil {
		log.Error("task failed", "error", err)
		os.Exit(1)
	}
}
```

Store your own application values in the context too:

```go
type appContextKeyType int

var appContextKey = appContextKeyType(0)

gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	return next(context.WithValue(ctx, appContextKey, loadApp()))
}))
```

## Shell, forwarding and rsync

### Interactive shell

```go
var Ssh = gabricssh.RegisterSshTask(gabric.Default)
```

```bash
go run . -ssh-host deploy@example.com ssh
```

From library code, use `gabricssh.Shell(ctx)`; see
[ssh/gabricssh.md](ssh/gabricssh.md) and [ssh/shell.md](ssh/shell.md).

### Local port forwarding

```go
var Forward = gabricssh.RegisterForwardTask(gabric.Default)
```

```bash
go run . -ssh-host deploy@example.com forward:127.0.0.1:8080,127.0.0.1:8080
```

The task argument is `<local>,<remote>` in `host:port` form; a single address
applies to both sides. From library code, use `gabricssh.Forward(ctx)`; see
[ssh/gabricssh.md](ssh/gabricssh.md) and [ssh/forward.md](ssh/forward.md).

### Rsync

For builder options, see [api.md](api.md#rsync). With a sudo context, rsync runs
the remote side with the configured sudo prefix followed by `rsync`
automatically:

```go
gabric.Remote(gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})).Rsync().
	Local("./app").
	Remote("/srv/app").
	Upload().
	Run()
```

The local rsync process's output can be captured and read back. This is useful
for a dry-run drift check with `--dry-run --itemize-changes`:

```go
out := gabric.Remote(ctx).Rsync().
	Local("./app").
	Remote("/srv/app").
	Upload().
	AddFlags("-n", "-i").
	CaptureStdout().
	Run().
	StdoutStr()

if strings.Contains(out, "deleting") || strings.Contains(out, ">f") {
	// the remote tree drifted; run the real transfer
}
```

`Capture` captures both local streams; `CaptureStdout`/`CaptureStderr` capture
one. See [api.md](api.md#rsync) for the capture variants and
[rsync-transport.md](rsync-transport.md) for the transport.
