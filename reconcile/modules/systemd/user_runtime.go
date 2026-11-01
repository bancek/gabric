package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

// UserRuntimeDir reads the user's runtime directory with
// loginctl show-user <user> -p RuntimePath.
func UserRuntimeDir(userInput input.Input[string]) *reconcile.Node[string] {
	if userInput == nil {
		panic("systemd: UserRuntimeDir: nil user")
	}

	return reconcile.Data("systemd.user-runtime-dir:"+userInput.Name(), func(ctx context.Context) (string, error) {
		user, err := userInput.Value(ctx)
		if err != nil {
			return "", err
		}

		e := gabric.TargetOps(ctx).Exec("loginctl", "show-user", user, "-p", "RuntimePath").Capture()
		if err := e.RunErr(); err != nil {
			return "", err
		}

		const prefix = "RuntimePath="

		out := strings.TrimSpace(e.StdoutStr())
		if !strings.HasPrefix(out, prefix) {
			return "", fmt.Errorf("systemd: user %s: unexpected loginctl output %q", user, out)
		}

		path := strings.TrimPrefix(out, prefix)
		if path == "" {
			return "", fmt.Errorf("systemd: user %s: empty runtime path", user)
		}

		return path, nil
	}).Node()
}

// WithUserRuntimeDir returns middleware that sets XDG_RUNTIME_DIR to the
// user's runtime directory.
func WithUserRuntimeDir(userInput input.Input[string]) middleware.Middleware {
	dir := UserRuntimeDir(userInput)

	return func(ctx context.Context, next middleware.Next) error {
		path, err := dir.Value(ctx)
		if err != nil {
			return err
		}

		ctx = gabric.AppendTargetOpsExecEnvContext(ctx, gabric.Env("XDG_RUNTIME_DIR", path))

		return next(ctx)
	}
}
