package linux_test

import (
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestSSHAuthorizedKeysE2E(t *testing.T) {
	moduletest.ForEachLinuxTarget(t, func(g *moduletest.Target) {
		t := g.T()
		name := "modtest-keys"
		key := "ssh-ed25519 AAAAtest modtest@example"
		content := []byte(key + "\n")

		user := linux.User(input.Static(name)).Build().Node()
		moduletest.AssertConverges(t, g, user)

		keys := linux.SSHAuthorizedKeys(input.Static(name), input.List(key))
		moduletest.AssertDryRun(t, g, keys)
		moduletest.AssertConverges(t, g, keys)

		moduletest.AssertFile(t, g, "/home/"+name+"/.ssh/authorized_keys", content)
	})
}
