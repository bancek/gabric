package systemd

import (
	"context"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// Effect builds an effect that runs `systemctl verb name` when any topic is
// notified. The effect is named "verb:name".
func Effect(nameInput input.Input[string], verb string, topics ...*reconcile.Topic) *reconcile.Effect {
	if nameInput == nil {
		panic("systemd: Effect: nil name")
	}

	cmd := input.Map(nameInput, func(ctx context.Context, name string) ([]string, error) {
		return []string{"systemctl", verb, name}, nil
	})

	return modules.ExecEffect(verb + ":" + nameInput.Name()).Topics(topics...).Exec(cmd).Effect()
}

// RestartEffect runs `systemctl restart name`.
func RestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "restart", topics...)
}

// ReloadEffect runs `systemctl reload name`.
func ReloadEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "reload", topics...)
}

// TryRestartEffect runs `systemctl try-restart name`.
func TryRestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "try-restart", topics...)
}

// StartEffect runs `systemctl start name`.
func StartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "start", topics...)
}

// StopEffect runs `systemctl stop name`.
func StopEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "stop", topics...)
}

// DaemonReloadEffect runs `systemctl daemon-reload`.
func DaemonReloadEffect(topics ...*reconcile.Topic) *reconcile.Effect {
	return modules.ExecEffect("daemon-reload").
		Topics(topics...).
		Exec(input.List("systemctl", "daemon-reload")).
		Effect()
}
