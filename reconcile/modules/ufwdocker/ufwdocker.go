package ufwdocker

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/github"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

// The upstream ufw-docker repository and script path.
const (
	ufwDockerOwner = "chaifeng"
	ufwDockerRepo  = "ufw-docker"
	ufwDockerFile  = "ufw-docker"
)

// Paths, overridable in tests.
var (
	ufwDockerScriptPath  = "/usr/local/bin/ufw-docker"
	ufwDockerServicePath = "/etc/systemd/system/ufw-docker.service"
	ufwDockerAfterRules  = "/etc/ufw/after.rules"
)

const ufwDockerMarker = "BEGIN UFW AND DOCKER"

// Topic is notified when ufw-docker (re)installs its rules or service.
var Topic = reconcile.NewTopic("ufw")

// RestartEffect restarts ufw on Topic.
func RestartEffect() *reconcile.Effect {
	return systemd.RestartEffect(input.Static("ufw"), Topic)
}

// InstalledData reports whether the ufw-docker block is present in
// /etc/ufw/after.rules, as a data node (dry-run safe, memoized once per run).
// A grep "no match" exit is false; other errors propagate.
func InstalledData() *reconcile.Node[bool] {
	return reconcile.Data("ufwdocker.installed", func(ctx context.Context) (bool, error) {
		_, err := modules.CommandOutputData(input.List("grep", "-q", ufwDockerMarker, ufwDockerAfterRules)).Value(ctx)
		if err != nil {
			if !modules.IsExitStatus(err, 1) {
				return false, err
			}

			return false, nil
		}

		return true, nil
	}).Node()
}

// Install ensures `ufw-docker install` has run (its block is present in
// /etc/ufw/after.rules). It reads script first (the node that installs the
// ufw-docker script) and notifies Topic.
func Install(script *reconcile.Node[reconcile.Void]) *reconcile.ResourceBuilder[bool] {
	return reconcile.ResourceBool("ufwdocker.install",
		func(ctx context.Context) (bool, error) {
			return InstalledData().Value(ctx)
		},
		func(ctx context.Context) error {
			return gabric.TargetOps(ctx).Exec("ufw-docker", "install").RunErr()
		}).
		CheckNeeds(script).
		Notify(Topic)
}

// InstallService ensures `ufw-docker install-service` has run (its systemd unit
// exists). It reads script first and notifies Topic.
func InstallService(script *reconcile.Node[reconcile.Void]) *reconcile.ResourceBuilder[bool] {
	return reconcile.ResourceBool("ufwdocker.install-service",
		func(ctx context.Context) (bool, error) {
			return modules.FileExistsData(input.Static(ufwDockerServicePath)).Value(ctx)
		},
		func(ctx context.Context) error {
			return gabric.TargetOps(ctx).Exec("ufw-docker", "install-service").RunErr()
		}).
		CheckNeeds(script).
		Notify(Topic)
}

// UfwDockerGroup is the ufw-docker installation cover: the script (downloaded
// at a pinned ref) plus the install and install-service steps.
type UfwDockerGroup struct {
	*reconcile.Node[reconcile.Void]

	Script         *github.RawFileGroup
	Install        *reconcile.Node[bool]
	InstallService *reconcile.Node[bool]
}

// UfwDockerBuilder configures the ufw-docker installation cover.
type UfwDockerBuilder struct {
	refInput     input.Input[string]
	baseURLInput input.Input[string]
}

// UfwDocker starts a ufw-docker installation builder.
func UfwDocker(refInput input.Input[string]) *UfwDockerBuilder {
	return &UfwDockerBuilder{refInput: refInput}
}

// BaseURL overrides the GitHub raw file base URL used for the script.
func (b *UfwDockerBuilder) BaseURL(baseURLInput input.Input[string]) *UfwDockerBuilder {
	b.baseURLInput = baseURLInput
	return b
}

// Build groups the ufw-docker install: it downloads the script at the pinned
// ref, then runs `ufw-docker install` and `install-service`. Reading the cover
// runs them in that order; the install steps read the script cover first, so a
// download that succeeded but whose install failed retries install on the next
// run.
func (b *UfwDockerBuilder) Build() *UfwDockerGroup {
	if b.refInput == nil {
		panic("ufwdocker: UfwDocker: nil ref")
	}

	rawFile := github.RawFile(
		input.Static(ufwDockerOwner),
		input.Static(ufwDockerRepo),
		input.Static(ufwDockerFile),
		b.refInput,
	)
	if b.baseURLInput != nil {
		rawFile = rawFile.BaseURL(b.baseURLInput)
	}

	script := rawFile.
		Dest(input.Static(ufwDockerScriptPath)).
		Mode(modules.FileMode(0o755)).
		Build()

	install := Install(script.Node).Node()
	installService := InstallService(script.Node).Node()

	node := reconcile.Group("ufwdocker.ufw-docker").
		Contains(script.Node).
		Contains(install, installService).
		Node()

	return &UfwDockerGroup{
		Node:           node,
		Script:         script,
		Install:        install,
		InstallService: installService,
	}
}
