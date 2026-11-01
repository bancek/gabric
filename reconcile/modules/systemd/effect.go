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
	return effect(nameInput, verb, false, topics...)
}

// UserEffect builds an effect that runs `systemctl --user verb name` when any
// topic is notified. The effect is named "user-verb:name".
func UserEffect(nameInput input.Input[string], verb string, topics ...*reconcile.Topic) *reconcile.Effect {
	return effect(nameInput, verb, true, topics...)
}

func effect(nameInput input.Input[string], verb string, user bool, topics ...*reconcile.Topic) *reconcile.Effect {
	if nameInput == nil {
		panic("systemd: Effect: nil name")
	}

	cmd := input.Map(nameInput, func(_ context.Context, name string) ([]string, error) {
		return systemctlArgs(user, verb, name), nil
	})

	effectName := verb + ":" + nameInput.Name()
	if user {
		effectName = "user-" + effectName
	}

	return modules.ExecEffect(effectName).Topics(topics...).Exec(cmd).Effect()
}

// RestartEffect runs `systemctl restart name`.
func RestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "restart", topics...)
}

// UserRestartEffect runs `systemctl --user restart name`.
func UserRestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return UserEffect(nameInput, "restart", topics...)
}

// ReloadEffect runs `systemctl reload name`.
func ReloadEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "reload", topics...)
}

// UserReloadEffect runs `systemctl --user reload name`.
func UserReloadEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return UserEffect(nameInput, "reload", topics...)
}

// TryRestartEffect runs `systemctl try-restart name`.
func TryRestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "try-restart", topics...)
}

// UserTryRestartEffect runs `systemctl --user try-restart name`.
func UserTryRestartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return UserEffect(nameInput, "try-restart", topics...)
}

// StartEffect runs `systemctl start name`.
func StartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "start", topics...)
}

// UserStartEffect runs `systemctl --user start name`.
func UserStartEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return UserEffect(nameInput, "start", topics...)
}

// StopEffect runs `systemctl stop name`.
func StopEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return Effect(nameInput, "stop", topics...)
}

// UserStopEffect runs `systemctl --user stop name`.
func UserStopEffect(nameInput input.Input[string], topics ...*reconcile.Topic) *reconcile.Effect {
	return UserEffect(nameInput, "stop", topics...)
}

// DaemonReloadEffect runs `systemctl daemon-reload`.
func DaemonReloadEffect(topics ...*reconcile.Topic) *reconcile.Effect {
	return modules.ExecEffect("daemon-reload").
		Topics(topics...).
		Exec(input.List("systemctl", "daemon-reload")).
		Effect()
}

// UserDaemonReloadEffect runs `systemctl --user daemon-reload`.
func UserDaemonReloadEffect(topics ...*reconcile.Topic) *reconcile.Effect {
	return modules.ExecEffect("user-daemon-reload").
		Topics(topics...).
		Exec(input.List(systemctlArgs(true, "daemon-reload")...)).
		Effect()
}
