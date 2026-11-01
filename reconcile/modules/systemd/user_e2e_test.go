package systemd_test

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

const (
	userE2EUnit        = "gabric-e2e.service"
	userE2EUnitContent = `[Unit]
Description=Gabric e2e user service

[Service]
Type=simple
ExecStart=/bin/sleep 3600

[Install]
WantedBy=default.target
`
)

func TestUserServiceE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()
		user := "gabric-e2e"
		unitPath := "/etc/systemd/user/" + userE2EUnit
		uid := ensureUser(t, g, user)

		waitForLogind(t, g)

		moduletest.AssertConverges(t, g, systemd.Linger(input.Static(user)).Build().Node())
		waitForUserManager(t, g, uid)

		runtimeDir, err := g.RunNodeErr(g.Context(), systemd.UserRuntimeDir(input.Static(user)))
		if err != nil {
			t.Fatalf("UserRuntimeDir(): %v", err)
		}
		if !g.Ops().Exists(runtimeDir).Dir().Run() {
			t.Fatalf("runtime directory %s does not exist", runtimeDir)
		}

		if err := g.Ops().Put(unitPath).String(userE2EUnitContent).Mode(0o644).RunErr(); err != nil {
			t.Fatalf("write %s: %v", unitPath, err)
		}

		withUser := []middleware.Middleware{
			systemd.WithUserRuntimeDir(input.Static(user)),
			gabric.WithRemoteSudo(&gabric.Sudo{User: user}),
		}

		reloadTopic := reconcile.NewTopic("systemd-user-daemon-reload-e2e")
		_ = systemd.UserDaemonReloadEffect(reloadTopic).With(withUser...)
		notify(t, g, reloadTopic)

		service := systemd.Service(input.Static(userE2EUnit)).User().Started().Build().Node().With(withUser...)
		moduletest.AssertDryRun(t, g, service)
		moduletest.AssertConverges(t, g, service)
		assertUserServiceState(t, g, userE2EUnit, withUser, true, true)

		invocationIDNode := userInvocationIDNode(userE2EUnit, withUser)

		invocationID := func() string {
			return g.RunNode(g.Context(), invocationIDNode)
		}

		restartTopic := reconcile.NewTopic("systemd-user-restart-e2e")
		_ = systemd.UserRestartEffect(input.Static(userE2EUnit), restartTopic).With(withUser...)
		before := invocationID()
		notify(t, g, restartTopic)
		after := invocationID()
		if before == "" || after == "" || before == after {
			t.Fatalf("restart invocation ID: before=%q after=%q", before, after)
		}

		tryTopic := reconcile.NewTopic("systemd-user-try-restart-e2e")
		_ = systemd.UserTryRestartEffect(input.Static(userE2EUnit), tryTopic).With(withUser...)
		before = invocationID()
		notify(t, g, tryTopic)
		after = invocationID()
		if before == after {
			t.Errorf("try-restart invocation ID did not change: %q", after)
		}

		stopTopic := reconcile.NewTopic("systemd-user-stop-e2e")
		_ = systemd.UserStopEffect(input.Static(userE2EUnit), stopTopic).With(withUser...)
		notify(t, g, stopTopic)
		assertUserServiceState(t, g, userE2EUnit, withUser, true, false)

		startTopic := reconcile.NewTopic("systemd-user-start-e2e")
		_ = systemd.UserStartEffect(input.Static(userE2EUnit), startTopic).With(withUser...)
		notify(t, g, startTopic)
		assertUserServiceState(t, g, userE2EUnit, withUser, true, true)

		disabled := systemd.Service(input.Static(userE2EUnit)).User().Enabled(input.False).Build().Node().With(withUser...)
		if _, err := g.RunReportRef(g.Context(), disabled); err != nil {
			t.Fatalf("disable %s: %v", userE2EUnit, err)
		}
		assertUserServiceState(t, g, userE2EUnit, withUser, false, false)
	})
}

func assertUserServiceState(t testing.TB, g *moduletest.Target, unit string, withUser []middleware.Middleware, wantEnabled, wantActive bool) {
	t.Helper()

	enabled, err := g.RunNodeErr(g.Context(), systemd.UserEnabledData(input.Static(unit)).With(withUser...))
	if err != nil {
		t.Fatalf("UserEnabledData(%s): %v", unit, err)
	}
	if enabled != wantEnabled {
		t.Errorf("%s enabled = %v, want %v", unit, enabled, wantEnabled)
	}

	active, err := g.RunNodeErr(g.Context(), systemd.UserActiveData(input.Static(unit)).With(withUser...))
	if err != nil {
		t.Fatalf("UserActiveData(%s): %v", unit, err)
	}
	if active != wantActive {
		t.Errorf("%s active = %v, want %v", unit, active, wantActive)
	}
}
