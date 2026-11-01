package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// LingerBuilder builds a node that ensures a user's lingering state.
type LingerBuilder struct {
	userInput    input.Input[string]
	enabledInput input.Input[bool]
}

// Linger ensures lingering is enabled for the user.
func Linger(userInput input.Input[string]) *LingerBuilder {
	return &LingerBuilder{userInput: userInput}
}

// Enabled sets whether lingering is enabled.
func (b *LingerBuilder) Enabled(flag ...input.Input[bool]) *LingerBuilder {
	b.enabledInput = input.Flag(flag...)
	return b
}

// Build hands off to the reconcile node builder.
func (b *LingerBuilder) Build() *reconcile.ResourceBuilder[bool] {
	if b.userInput == nil {
		panic("systemd: Linger: nil user")
	}

	userInput, enabledInput := b.userInput, b.enabledInput

	return reconcile.ResourceBool("systemd.linger:"+userInput.Name(),
		func(ctx context.Context) (bool, error) {
			enabled, err := input.ValueOr(ctx, enabledInput, true)
			if err != nil {
				return false, err
			}

			actual, err := LingerData(userInput).Value(ctx)
			if err != nil {
				return false, err
			}

			return actual == enabled, nil
		},
		func(ctx context.Context) error {
			user, err := userInput.Value(ctx)
			if err != nil {
				return err
			}
			enabled, err := input.ValueOr(ctx, enabledInput, true)
			if err != nil {
				return err
			}

			if enabled {
				return gabric.TargetOps(ctx).Exec("loginctl", "enable-linger", user).RunErr()
			}

			return gabric.TargetOps(ctx).Exec("loginctl", "disable-linger", user).RunErr()
		})
}

// LingerData reports whether lingering is enabled for the user, as a data node
// (dry-run safe, memoized once per run). A user with no session and no
// lingering is false.
func LingerData(userInput input.Input[string]) *reconcile.Node[bool] {
	if userInput == nil {
		panic("systemd: LingerData: nil user")
	}

	return reconcile.Data("systemd.linger-data:"+userInput.Name(), func(ctx context.Context) (bool, error) {
		user, err := userInput.Value(ctx)
		if err != nil {
			return false, err
		}

		e := gabric.TargetOps(ctx).Exec("loginctl", "show-user", user, "-p", "Linger").Capture()
		err = e.RunErr()
		out := strings.TrimSpace(e.StdoutStr())
		if err != nil {
			if out == "" && modules.IsExitStatus(err, 1) {
				return false, nil
			}

			return false, err
		}

		switch out {
		case "Linger=yes":
			return true, nil
		case "Linger=no":
			return false, nil
		default:
			return false, fmt.Errorf("systemd: linger %s: unexpected loginctl output %q", user, out)
		}
	}).Node()
}
