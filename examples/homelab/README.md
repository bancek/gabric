# homelab (reconcile example)

A real-world example of [gabric's `reconcile`
engine](../../docs/reconcile/README.md) and its
[`modules`](../../docs/reconcile/modules.md): converge a small personal
server to a described state: a reverse proxy (traefik), a static site (nginx),
a service built from source ([traefik/whoami](https://github.com/traefik/whoami)),
a WireGuard VPN (wg-easy), plus baseline hardening, Docker, a default-deny
firewall and encrypted secrets.

It follows the shape of a real deployment repo: one file per service, each a
`reconcile.Group` cover over the nodes that service owns; a single run shared by
every task (`Use(WithReconcileReport)`); topics and effects for restarts and
reloads; and `-dry-run` for a dry run. The same graph can converge a remote
host over SSH, run in place with `-local`, or be built and uploaded to a VM for
a local-mode run.

> Generic example. Hosts, domains, packages and the committed secret are
> placeholders. Adapt them before running against a real machine.

## Layout

| File             | What it contains                                                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `main.go`        | the `S` input alias, shared `WithRemoteSudo` base, the global `WithReconcileReport` run, `-local`, `-dry-run`, `ssh`/`forward` |
| `all.go`         | one `Group` cover over every service                                                                                           |
| `config.go`      | a `Data` node for site config (`-tls`, `-domain`, `-acme-email`) driving the templates                                         |
| `bootstrap.go`   | packages with lazy `ApplyNeeds(aptGetUpdate)`, `File(...).Notify(sshTopic)`, `systemd.RestartEffect`, `linux.Sysctl`           |
| `user.go`        | `linux.User` (docker group, `ApplyNeeds(dockerNode)`) + `linux.SSHAuthorizedKeys` with `CheckNeeds`                               |
| `docker.go`      | `docker.DockerEngine`, daemon config with `Notify`, and `docker.Network`                                                       |
| `firewall.go`    | the `ufw.Firewall` cover, `ufw.AddRule`, `ufwdocker.UfwDocker`                                                                 |
| `secrets.go`     | age helpers (`seal`/`unseal`) and `secretFile`, a `File.Content` node helper                                                   |
| `www.go`         | `rsync.Sync(...).Notify` plus a custom `NewEffect` (nginx reload)                                                              |
| `traefik.go`     | `Dir` + `File`/`TemplateData` compose + `docker.Compose` + `ufw.AddRule` route rules                                           |
| `app.go`         | `git.Repo` + a data node combining the revision with site config + `docker.Compose.BuildImages`                                |
| `wireguard.go`   | a `Data` node (`ipv6`) + `TemplateData` + `ufw.AddRule` set + a port-forward task                                              |
| `deploy.go`      | `deploy-local`: build for the target arch, upload with `Remote.Rsync`, run `-local`                                            |
| `vagrant.go`     | `vagrant ssh-config` resolution, the Vagrant remote middleware, and `vm-up`/`vm-shell`/`vm-down`                               |
| `e2e.go`         | `e2e` modes and idempotence/site assertions                                                                                    |
| `files/`, `srv/` | host config (embedded) and per-service compose/config (templates) deployed with `File`/`TemplateData`/`rsync`                  |

`S` in `main.go` is a short alias for `input.Static`. The module examples use it
to keep call sites short.

## Usage

Requires a host reachable over SSH and resolvable the way gabric resolves hosts
(`~/.ssh/config`, keys, ssh-agent, `known_hosts`). Pass `-ssh-host` explicitly.

```bash
./gab -ssh-host deploy@server hello
./gab -ssh-host deploy@server -dry-run all   # dry run: probe, never apply
./gab -ssh-host deploy@server all            # converge the whole host
./gab -ssh-host deploy@server wg-ui          # SSH-forward the VPN UI
```

Tasks: `all`, `bootstrap`, `user`, `docker`, `firewall`, `www`, `traefik`, `app`,
`wireguard`, `seal`, `unseal`, `ssh`, `forward`, `wg-ui`, `hello`, `deploy-local`,
`e2e`, `vm-up`, `vm-shell`, `vm-down`.

`user.go` creates the `deploy` login account, adds it to the `docker` group
(installing docker first via `ApplyNeeds(dockerNode)`), and installs the
throwaway key committed in `files/deploy_authorized_keys`; replace that key (or
drop the task) before converging a real host.

## HTTPS

TLS is **off by default**. Traefik serves plain HTTP on `:80` and routes by
`Host` header, so the example converges a host with no public DNS (a VM or a
dynamically addressed box). Turn HTTPS on with ACME when you have real DNS:

```bash
./gab -ssh-host deploy@server -tls \
  -domain example.net -acme-email admin@example.net all
```

`-domain` defaults to `example.net` and `-acme-email` to `admin@<domain>`. The
`site` data node in `config.go` feeds the traefik/nginx/app compose templates,
which add the `websecure` entrypoint, the HTTP→HTTPS redirect, the ACME
resolver and the `:443` port only when `-tls` is set.

## Secrets

Each secret is a file resource node for one service, like a template. The app
example decrypts `secrets/app.env.age` in memory (the identity stays on the
operator or local machine) into `/srv/app/.env` with mode `0600`:

```go
var appEnv = secretFile("/srv/app/.env", secretsDir+"/app.env.age")
```

The node belongs to the service group, and the service's Compose file reads it
with `env_file: .env`. To add a secret to another service, declare another
`secretFile(target, source)` node in that service and add it to the group.

The `seal` and `unseal` tasks manage the encrypted source files:

```bash
./gab unseal     # secrets/*.age -> secrets.plain/ (for editing)
./gab seal       # secrets.plain/ -> secrets/*.age (for committing)
```

Secrets are applied with their service, for example `./gab app`, or with
`./gab all`.

`secrets/recipients.txt` lists the age recipients; `secrets/app.env.age` is a
sample encrypted secret and `secrets/sample-identity.txt` its throwaway
identity, so the example works out of the box. For real use, replace both and
keep the identity out of the repo (`-age-identity /path/to/keys.txt`, or the
default `~/.config/age/keys.txt`).

The age plumbing (seal/unseal, recipients, identity discovery) is intentionally
left in the example rather than `modules`: it is deployment policy, not a
generic reconcile primitive. The reconcile part is `secretFile`, which is just
`modules.File(S(target)).Content(decrypt).Mode(modules.FileMode(0o600))`.

## Local mode

`-local` runs the graph on the machine executing the binary. It selects
`gabric.LocalRemoteTarget`, so modules keep remote-style `/bin/sh` and coreutils
commands, remote working directories, and `WithRemoteSudo`:

```bash
./gab -local all
./gab -local -dry-run all
```

Use `LocalTarget` for native, unprivileged local modules, and
`LocalRemoteTarget` for local sudo.

`deploy-local` builds the binary for the target architecture, uploads the whole
directory with `Remote.Rsync`, and runs `./bin/homelab` with the exact arguments
you pass as its task argument. It adds no flags and forwards no site config;
include `-local` and any application flags yourself.

```bash
./gab "deploy-local:-local all"
./gab "deploy-local:-local -dry-run all"
./gab "deploy-local:-local docker"

# Explicit site config for the local run:
./gab "deploy-local:-local -domain example.net -tls all"

# With an explicit SSH host:
./gab -ssh-host deploy@server "deploy-local:-local all"
```

Quote the whole task argument when it contains spaces. Inner flags must come
before the task name. `deploy-local` with no argument is an error.

The upload carries `secrets/`, `srv/`, and the built `bin/homelab`; it excludes
`.git`, `.vagrant`, `secrets.plain`, and `secrets/keys.txt` (never upload a real
age identity). The binary is built with `CGO_ENABLED=0` for
`linux/<target arch>`. `deploy-local` ensures `rsync` on Debian/Ubuntu targets
first with the reconcile `pkg.Package` node (via `sudo -n`), because the upload happens
before the graph's bootstrap. Passwordless sudo is required.

Local mode decrypts the staged `secrets/` on the target. The committed sample
identity is the default; if `-age-identity` points at an operator-only path,
stage that file in the VM too or use the sample. The bootstrap installs `age`
before decrypting the app env, so `all` works on a fresh VM. Running an inner
task that needs `age` without bootstrap (for example
`deploy-local:-local app`) requires `age` to be installed first.

Runtime inputs that are not embedded (`secrets/` and `srv/www/`) travel with the
directory. Custom data/effect nodes follow the selected target only when they
use `gabric.TargetOps`; direct `Local`/`Remote` are explicit escapes.

## Vagrant VM and e2e

`Vagrantfile` defines the Ubuntu VM. These tasks are local-only (they need
Vagrant and a provider) and are not run in CI.

```bash
./gab vm-up       # boot the VM and log its SSH target
./gab vm-shell    # boot if needed and open a Gabric SSH shell
./gab vm-down     # destroy the VM
```

`vm-shell` reads the VM target from `vagrant ssh-config` through a task
middleware and scopes permissive host-key checking and the Vagrant identity to
that VM. `vm-up`/`vm-shell`/`vm-down` reject `-dry-run`.

`e2e` runs the Vagrant workflow:

```bash
./gab e2e                                      # default: both modes
./gab -e2e-fresh -e2e-mode remote e2e          # install from scratch over SSH
./gab -e2e-fresh -e2e-mode local e2e           # install from scratch locally
./gab -e2e-mode both e2e                       # cross-transport idempotence
./gab -e2e-destroy e2e                         # destroy when done
```

`remote` converges over SSH, `local` builds/uploads/runs `-local`, and `both`
runs remote first, then local on the converged VM, then another remote dry run.
The e2e asserts no failures, no dry-run changes, and zero changes when local
runs are expected to be idempotent; it also checks both sites through traefik
and that UFW is active. It leaves the VM running unless `-e2e-destroy` is set.

Run these from `examples/homelab`. The e2e is local-only and not part of CI.

### E2E notes

- `checkSite` uses plain HTTP, so the e2e assumes TLS is off. With `-tls`,
  traefik redirects HTTP to HTTPS; extend the check to HTTPS (`curl -k`) or keep
  TLS off for the e2e.
- A dry run on a completely fresh VM skips package installs, so tools such as
  `age` and `rsync` must already exist. The e2e always runs a real convergence
  before its dry run.
- The e2e task rejects `-dry-run`; its local run passes `-local all` (or
  `-local -dry-run all`) explicitly. For a manual local dry run, use
  `./gab "deploy-local:-local -dry-run all"`.
- `deploy-local` does not forward outer flags. Local/both e2e use the default
  site config; outer `-domain`, `-tls`, `-acme-email`, `-ipv6`, and
  `-age-identity` apply only to remote mode.
- `all` may restart `sshd` while hardening it. The e2e reuses the Gabric SSH
  transport; if the session drops, the failure is surfaced rather than retried.

Requires Go, `age`, `rsync`, Vagrant plus a provider, and passwordless sudo in
the VM. `docker compose --dry-run` (used by the `Compose` probe) needs Compose
v2.20 or newer, which the docker.com `docker-compose-plugin` provides.
Use `-ssh-insecure` for manual SSH runs against throwaway hosts whose keys are
not in `known_hosts`; the VM tasks configure this only for their own VM.

## Next

- [reconcile overview](../../docs/reconcile/README.md)
- [architecture](../../docs/reconcile/architecture.md)
- [modules reference](../../docs/reconcile/modules.md)
- [cookbook](../../docs/reconcile/cookbook.md)
