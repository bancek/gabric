package ufw

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func registerUFWStatusHandler(gt *gabrictest.Test, out string, code int) {
	gt.Remote().Handle("ufw", func(cmd gabrictest.Command) gabrictest.Result {
		if slices.Equal(cmd.Args, []string{"status", "verbose"}) {
			return gabrictest.Result{Stdout: out, ExitCode: code}
		}

		return gabrictest.Result{}
	})
}

func TestParseUFWStatus(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want StatusInfo
	}{
		{
			name: "active",
			in:   "Status: active\nLogging: on (low)\nDefault: deny (incoming), allow (outgoing), disabled (routed)\n",
			want: StatusInfo{Active: true, Incoming: "deny", Outgoing: "allow", Routed: "disabled"},
		},
		{
			name: "inactive",
			in:   "Status: inactive\n",
			want: StatusInfo{},
		},
		{
			name: "no routed",
			in:   "Status: active\nDefault: allow (incoming), deny (outgoing)\n",
			want: StatusInfo{Active: true, Incoming: "allow", Outgoing: "deny"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseUFWStatus(tc.in); got != tc.want {
				t.Errorf("parseUFWStatus() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStatusData(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: deny (incoming), allow (outgoing)\n", 0)

	got := gt.RunNode(gt.Context(), StatusData())

	want := StatusInfo{Active: true, Incoming: "deny", Outgoing: "allow"}
	if got != want {
		t.Errorf("StatusData() = %+v, want %+v", got, want)
	}

	gt.Remote().AssertCommands(gt.Command("ufw", "status", "verbose"))
	gt.Remote().AssertCalls()
}

func TestStatusDataError(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "", 1)

	if _, err := gt.RunNodeErr(gt.Context(), StatusData()); err == nil {
		t.Error("StatusData() error = nil, want error")
	}

	gt.Remote().AssertCommands(gt.Command("ufw", "status", "verbose"))
	gt.Remote().AssertCalls()
}

func TestDefaultIncomingNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: deny (incoming), allow (outgoing)\n", 0)

	node := DefaultIncoming(StatusData(), input.Static("deny")).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("ufw", "status", "verbose"))
	gt.Remote().AssertCalls()
}

func TestDefaultIncomingDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: allow (incoming), allow (outgoing)\n", 0)

	node := DefaultIncoming(StatusData(), input.Static("deny")).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "default", "deny", "incoming"),
	)
	gt.Remote().AssertCalls()
}

func TestDefaultOutgoingNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: deny (incoming), allow (outgoing)\n", 0)

	node := DefaultOutgoing(StatusData(), input.Static("allow")).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("ufw", "status", "verbose"))
	gt.Remote().AssertCalls()
}

func TestDefaultOutgoingDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: deny (incoming), deny (outgoing)\n", 0)

	node := DefaultOutgoing(StatusData(), input.Static("allow")).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "default", "allow", "outgoing"),
	)
	gt.Remote().AssertCalls()
}

func TestEnabledNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\n", 0)

	node := Enabled(StatusData(), input.True).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("ufw", "status", "verbose"))
	gt.Remote().AssertCalls()
}

func TestEnabledDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: inactive\n", 0)

	node := Enabled(StatusData(), input.True).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "--force", "enable"),
	)
	gt.Remote().AssertCalls()
}

func TestEnabledDisable(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\n", 0)

	node := Enabled(StatusData(), input.False).Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "--force", "disable"),
	)
	gt.Remote().AssertCalls()
}

func TestFirewallBuildPanics(t *testing.T) {
	assertPanics(t, "no enabled", func() { Firewall().Build() })
}

func TestFirewallDefaultPanics(t *testing.T) {
	status := StatusData()

	assertPanics(t, "empty incoming policy", func() { DefaultIncoming(status, nil) })
	assertPanics(t, "empty outgoing policy", func() { DefaultOutgoing(status, nil) })
}

func TestFirewallCoverOrder(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: inactive\n", 0)

	ufw := Firewall().
		Rule(Rule().Port(input.Static("22")).Proto(input.Static("tcp"))).
		Incoming(input.Static("deny")).Outgoing(input.Static("allow")).
		Enabled().
		Build()

	gt.RunNode(gt.Context(), ufw.Node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "allow", "22/tcp"),
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "default", "deny", "incoming"),
		gt.Command("ufw", "default", "allow", "outgoing"),
		gt.Command("ufw", "--force", "enable"),
		gt.Command("ufw", "status", "verbose"),
	)
	gt.Remote().AssertCalls()
}

func TestFirewallCoverPartialDefaults(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: allow (incoming), allow (outgoing)\n", 0)

	ufw := Firewall().Incoming(input.Static("deny")).Outgoing(input.Static("allow")).Enabled().Build()
	gt.RunNode(gt.Context(), ufw.Node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "default", "deny", "incoming"),
		gt.Command("ufw", "status", "verbose"),
	)
	gt.Remote().AssertCalls()
}

func TestFirewallCoverDryRun(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: inactive\n", 0)

	ctx := dryrun.With(gt.Context())

	ufw := Firewall().
		Rule(Rule().Port(input.Static("22")).Proto(input.Static("tcp"))).
		Incoming(input.Static("deny")).Outgoing(input.Static("allow")).
		Enabled().
		Build()

	gt.RunNode(ctx, ufw.Node)

	gt.Remote().AssertCommands(
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "status", "verbose"),
	)
	gt.Remote().AssertCalls()
}

func TestFirewallAddRuleInvalidatesStatus(t *testing.T) {
	gt := gabrictest.New(t)
	registerUFWStatusHandler(gt, "Status: active\nDefault: deny (incoming), allow (outgoing)\n", 0)

	ufw := Firewall().Incoming(input.Static("deny")).Outgoing(input.Static("allow")).Enabled().Build()
	rule := ufw.AddRule(Rule().Port(input.Static("22")).Proto(input.Static("tcp")))

	err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
		if _, err := ufw.Value(ctx); err != nil {
			return err
		}

		if _, err := rule.Value(ctx); err != nil {
			return err
		}

		_, err := ufw.Status.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "show", "added"),
		gt.Command("ufw", "allow", "22/tcp"),
		gt.Command("ufw", "status", "verbose"),
	)
	gt.Remote().AssertCalls()
}

func TestFirewallCoverRefreshesStatus(t *testing.T) {
	gt := gabrictest.New(t)

	var (
		mu     sync.Mutex
		active bool
	)

	gt.Remote().Handle("ufw", func(cmd gabrictest.Command) gabrictest.Result {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case slices.Equal(cmd.Args, []string{"--force", "enable"}):
			active = true
			return gabrictest.Result{}
		case slices.Equal(cmd.Args, []string{"status", "verbose"}):
			if active {
				return gabrictest.Result{Stdout: "Status: active\n"}
			}

			return gabrictest.Result{Stdout: "Status: inactive\n"}
		}

		return gabrictest.Result{}
	})

	ufw := Firewall().Enabled().Build()

	var after StatusInfo

	err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
		if _, err := ufw.Status.Value(ctx); err != nil {
			return err
		}

		if _, err := ufw.Value(ctx); err != nil {
			return err
		}

		var err error
		after, err = ufw.Status.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !after.Active {
		t.Errorf("ufw.Status after the cover = %+v, want Active (invalidated and re-probed)", after)
	}

	gt.Remote().AssertCommands(
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "status", "verbose"),
		gt.Command("ufw", "--force", "enable"),
		gt.Command("ufw", "status", "verbose"),
	)
	gt.Remote().AssertCalls()
}
