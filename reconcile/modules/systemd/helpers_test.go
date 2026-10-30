package systemd

import "github.com/bancek/gabric/gabrictest"

func registerSystemctlHandler(gt *gabrictest.Test, enabled, active string) {
	gt.Remote().Handle("systemctl", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("is-enabled"):
			return gabrictest.Result{Stdout: enabled}
		case cmd.HasArg("is-active"):
			return gabrictest.Result{Stdout: active}
		}

		return gabrictest.Result{}
	})
}
