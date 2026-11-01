package systemd_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/internal/testutils"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

func waitForLogind(t testing.TB, g *moduletest.Target) {
	t.Helper()

	testutils.Eventually(t, 30*time.Second, func() bool {
		active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static("systemd-logind")))
		return err == nil && active
	})
}

func waitForUserManager(t testing.TB, g *moduletest.Target, uid string) {
	t.Helper()

	testutils.Eventually(t, 30*time.Second, func() bool {
		active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static("user@"+uid+".service")))
		return err == nil && active
	})
}

func ensureUser(t testing.TB, g *moduletest.Target, user string) string {
	t.Helper()

	moduletest.AssertConverges(t, g, linux.User(input.Static(user)).Build().Node())

	passwd, err := g.RunNodeErr(g.Context(), linux.UserPasswdData(input.Static(user)))
	if err != nil {
		t.Fatalf("UserPasswdData(%s): %v", user, err)
	}
	if !passwd.Exists {
		t.Fatalf("user %s does not exist", user)
	}

	return passwd.UID
}

func userInvocationIDNode(unit string, withUser []middleware.Middleware) *reconcile.Node[string] {
	return reconcile.Data("systemd-user-invocation-id:"+unit, func(ctx context.Context) (string, error) {
		e := gabric.TargetOps(ctx).
			Exec("systemctl", "--user", "show", "-p", "InvocationID", "--value", unit).
			CaptureStdout()
		if err := e.RunErr(); err != nil {
			return "", err
		}

		return strings.TrimSpace(e.StdoutStr()), nil
	}).Node().With(withUser...)
}
