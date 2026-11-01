package systemd

import (
	"context"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestEffects(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*reconcile.Topic) *reconcile.Effect
		want string
	}{
		{"restart", func(tp *reconcile.Topic) *reconcile.Effect { return RestartEffect(input.Static("docker"), tp) }, "restart"},
		{"reload", func(tp *reconcile.Topic) *reconcile.Effect { return ReloadEffect(input.Static("docker"), tp) }, "reload"},
		{"try-restart", func(tp *reconcile.Topic) *reconcile.Effect { return TryRestartEffect(input.Static("docker"), tp) }, "try-restart"},
		{"start", func(tp *reconcile.Topic) *reconcile.Effect { return StartEffect(input.Static("docker"), tp) }, "start"},
		{"stop", func(tp *reconcile.Topic) *reconcile.Effect { return StopEffect(input.Static("docker"), tp) }, "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			registerSystemctlHandler(gt, "", "")

			topic := reconcile.NewTopic("t")
			_ = tt.fn(topic)

			if err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
				return topic.Notify(ctx)
			}); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			gt.Remote().AssertCommands(gt.Command("systemctl", tt.want, "docker"))
			gt.Remote().AssertCalls()
		})
	}
}

func TestDaemonReloadEffect(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "", "")

	topic := reconcile.NewTopic("t")
	_ = DaemonReloadEffect(topic)

	if err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	gt.Remote().AssertCommands(gt.Command("systemctl", "daemon-reload"))
	gt.Remote().AssertCalls()
}

func TestUserEffects(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*reconcile.Topic) *reconcile.Effect
		verb string
	}{
		{"restart", func(tp *reconcile.Topic) *reconcile.Effect { return UserRestartEffect(input.Static("docker"), tp) }, "restart"},
		{"reload", func(tp *reconcile.Topic) *reconcile.Effect { return UserReloadEffect(input.Static("docker"), tp) }, "reload"},
		{"try-restart", func(tp *reconcile.Topic) *reconcile.Effect { return UserTryRestartEffect(input.Static("docker"), tp) }, "try-restart"},
		{"start", func(tp *reconcile.Topic) *reconcile.Effect { return UserStartEffect(input.Static("docker"), tp) }, "start"},
		{"stop", func(tp *reconcile.Topic) *reconcile.Effect { return UserStopEffect(input.Static("docker"), tp) }, "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			registerSystemctlHandler(gt, "", "")

			topic := reconcile.NewTopic("t")
			_ = tt.fn(topic)

			if err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
				return topic.Notify(ctx)
			}); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			gt.Remote().AssertCommands(gt.Command("systemctl", "--user", tt.verb, "docker"))
			gt.Remote().AssertCalls()
		})
	}
}

func TestUserDaemonReloadEffect(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "", "")

	topic := reconcile.NewTopic("t")
	_ = UserDaemonReloadEffect(topic)

	if err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	gt.Remote().AssertCommands(gt.Command("systemctl", "--user", "daemon-reload"))
	gt.Remote().AssertCalls()
}

func TestUserEffectNames(t *testing.T) {
	topic := reconcile.NewTopic("t")

	if got, want := UserRestartEffect(input.Static("app.service"), topic).Name(), "user-restart:app.service"; got != want {
		t.Errorf("UserRestartEffect() name = %q, want %q", got, want)
	}
	if got, want := UserDaemonReloadEffect(topic).Name(), "user-daemon-reload"; got != want {
		t.Errorf("UserDaemonReloadEffect() name = %q, want %q", got, want)
	}
}
