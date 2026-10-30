package systemd_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

func TestSystemdE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()
		name := "cron"
		disabled := systemd.Service(input.Static(name)).Enabled(input.False).Build().Node()
		start := systemd.Service(input.Static(name)).Started().Build().Node()

		if _, err := g.RunReportRef(g.Context(), disabled); err != nil {
			t.Fatalf("disable %s: %v", name, err)
		}
		assertServiceState(t, g, name, false, false)

		enabled, err := g.RunNodeErr(g.Context(), systemd.EnabledData(input.Static(name)))
		if err != nil {
			t.Fatalf("EnabledData(): %v", err)
		}
		if enabled {
			t.Error("EnabledData() = true, want false")
		}

		active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static(name)))
		if err != nil {
			t.Fatalf("ActiveData(): %v", err)
		}
		if active {
			t.Error("ActiveData() = true, want false")
		}

		moduletest.AssertDryRun(t, g, start)
		moduletest.AssertConverges(t, g, start)
		assertServiceState(t, g, name, true, true)

		enabled, err = g.RunNodeErr(g.Context(), systemd.EnabledData(input.Static(name)))
		if err != nil {
			t.Fatalf("EnabledData(): %v", err)
		}
		if !enabled {
			t.Error("EnabledData() = false, want true")
		}

		active, err = g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static(name)))
		if err != nil {
			t.Fatalf("ActiveData(): %v", err)
		}
		if !active {
			t.Error("ActiveData() = false, want true")
		}

		restartTopic := reconcile.NewTopic("systemd-restart-e2e")
		_ = systemd.RestartEffect(input.Static(name), restartTopic)
		before := invocationID(t, g, name)
		notify(t, g, restartTopic)
		after := invocationID(t, g, name)
		if before == "" || after == "" || before == after {
			t.Fatalf("restart invocation ID: before=%q after=%q", before, after)
		}

		runEmpty(t, g)
		if got := invocationID(t, g, name); got != after {
			t.Errorf("restart effect ran again: invocation ID = %q, want %q", got, after)
		}

		if err := reconcile.Run(dryrun.With(g.Context()), func(ctx context.Context) error {
			return restartTopic.Notify(ctx)
		}); err != nil {
			t.Fatalf("dry restart effect: %v", err)
		}
		if got := invocationID(t, g, name); got != after {
			t.Errorf("dry-run restart changed invocation ID to %q, want %q", got, after)
		}

		tryTopic := reconcile.NewTopic("systemd-try-restart-e2e")
		_ = systemd.TryRestartEffect(input.Static(name), tryTopic)
		before = invocationID(t, g, name)
		notify(t, g, tryTopic)
		after = invocationID(t, g, name)
		if before == after {
			t.Errorf("try-restart invocation ID did not change: %q", after)
		}

		stopTopic := reconcile.NewTopic("systemd-stop-e2e")
		_ = systemd.StopEffect(input.Static(name), stopTopic)
		notify(t, g, stopTopic)
		assertServiceState(t, g, name, true, false)

		startTopic := reconcile.NewTopic("systemd-start-e2e")
		_ = systemd.StartEffect(input.Static(name), startTopic)
		notify(t, g, startTopic)
		assertServiceState(t, g, name, true, true)

		daemonTopic := reconcile.NewTopic("systemd-daemon-reload-e2e")
		_ = systemd.DaemonReloadEffect(daemonTopic)
		notify(t, g, daemonTopic)
		runEmpty(t, g)
	})
}

func notify(t testing.TB, g *moduletest.Target, topic *reconcile.Topic) {
	t.Helper()

	if err := reconcile.Run(g.Context(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}
}

func runEmpty(t testing.TB, g *moduletest.Target) {
	t.Helper()

	if err := reconcile.Run(g.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("empty run: %v", err)
	}
}

func invocationID(t testing.TB, g *moduletest.Target, name string) string {
	t.Helper()

	return systemctlOutput(t, g, "show", "-p", "InvocationID", "--value", name)
}

func assertServiceState(t testing.TB, g *moduletest.Target, name string, wantEnabled, wantActive bool) {
	t.Helper()

	enabled, err := g.RunNodeErr(g.Context(), systemd.EnabledData(input.Static(name)))
	if err != nil {
		t.Fatalf("EnabledData(%s): %v", name, err)
	}
	if enabled != wantEnabled {
		t.Errorf("%s enabled = %v, want %v", name, enabled, wantEnabled)
	}

	active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static(name)))
	if err != nil {
		t.Fatalf("ActiveData(%s): %v", name, err)
	}
	if active != wantActive {
		t.Errorf("%s active = %v, want %v", name, active, wantActive)
	}
}

func systemctlOutput(t testing.TB, g *moduletest.Target, args ...string) string {
	t.Helper()

	e := g.Ops().Exec(append([]string{"systemctl"}, args...)...).CaptureStdout()
	_ = e.RunErr()

	return strings.TrimSpace(e.StdoutStr())
}
