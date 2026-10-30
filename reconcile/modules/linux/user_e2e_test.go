package linux_test

import (
	"path"
	"slices"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestUserE2E(t *testing.T) {
	moduletest.ForEachLinuxTarget(t, func(g *moduletest.Target) {
		t := g.T()

		name := "modtest-user"
		node := linux.User(input.Static(name)).Groups(input.List("users")).Build().Node()

		moduletest.AssertDryRun(t, g, node)
		moduletest.AssertConverges(t, g, node)

		passwd, err := g.RunNodeErr(g.Context(), linux.UserPasswdData(input.Static(name)))
		if err != nil {
			t.Fatalf("read user %s: %v", name, err)
		}
		if !passwd.Exists {
			t.Fatalf("user %s does not exist", name)
		}
		if want := path.Join("/home", name); passwd.Home != want {
			t.Errorf("home = %q, want %q", passwd.Home, want)
		}
		if passwd.Shell != "/bin/bash" {
			t.Errorf("shell = %q, want /bin/bash", passwd.Shell)
		}

		groups, err := g.RunNodeErr(g.Context(), linux.UserGroupsData(input.Static(name)))
		if err != nil {
			t.Fatalf("read groups for %s: %v", name, err)
		}
		if !slices.Contains(groups, "users") {
			t.Errorf("groups = %v, want users", groups)
		}

		assertHomeExists(t, g, passwd.Home, true)

		customName := "modtest-custom"
		customHome := g.Path("custom-home")
		custom := linux.User(input.Static(customName)).
			Shell(input.Static("/bin/sh")).
			Home(input.Static(customHome)).
			Build().
			Node()

		moduletest.AssertDryRun(t, g, custom)
		moduletest.AssertConverges(t, g, custom)

		customPasswd, err := g.RunNodeErr(g.Context(), linux.UserPasswdData(input.Static(customName)))
		if err != nil {
			t.Fatalf("read user %s: %v", customName, err)
		}
		if customPasswd.Home != customHome {
			t.Errorf("custom home = %q, want %q", customPasswd.Home, customHome)
		}
		if customPasswd.Shell != "/bin/sh" {
			t.Errorf("custom shell = %q, want /bin/sh", customPasswd.Shell)
		}
		assertHomeExists(t, g, customHome, true)

		systemName := "modtest-system"
		system := linux.User(input.Static(systemName)).System().Build().Node()
		moduletest.AssertConverges(t, g, system)

		systemPasswd, err := g.RunNodeErr(g.Context(), linux.UserPasswdData(input.Static(systemName)))
		if err != nil {
			t.Fatalf("read user %s: %v", systemName, err)
		}
		if !systemPasswd.Exists {
			t.Fatalf("system user %s does not exist", systemName)
		}
		assertHomeExists(t, g, systemPasswd.Home, false)

		noHomeName := "modtest-nohome"
		noHome := linux.User(input.Static(noHomeName)).CreateHome(input.False).Build().Node()
		moduletest.AssertConverges(t, g, noHome)

		noHomePasswd, err := g.RunNodeErr(g.Context(), linux.UserPasswdData(input.Static(noHomeName)))
		if err != nil {
			t.Fatalf("read user %s: %v", noHomeName, err)
		}
		if !noHomePasswd.Exists {
			t.Fatalf("user %s does not exist", noHomeName)
		}
		assertHomeExists(t, g, noHomePasswd.Home, false)
	})
}

func assertHomeExists(t testing.TB, g *moduletest.Target, home string, want bool) {
	t.Helper()

	if home == "" {
		if want {
			t.Error("home is empty, want an existing directory")
		}
		return
	}

	got, err := g.Ops().Exists(home).Dir().RunErr()
	if err != nil {
		t.Fatalf("check home %s: %v", home, err)
	}
	if got != want {
		t.Errorf("home %s exists = %v, want %v", home, got, want)
	}
}
