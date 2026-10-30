package linux_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestSysctlE2E(t *testing.T) {
	moduletest.ForEachLinuxSysctlTarget(t, func(g *moduletest.Target) {
		t := g.T()
		key := "net.ipv4.ip_forward"
		value := "1"
		procPath := "/proc/sys/net/ipv4/ip_forward"
		dropinPath := "/etc/sysctl.d/99-reconcile-net-ipv4-ip_forward.conf"
		content := []byte("net.ipv4.ip_forward = 1\n")

		node := linux.Sysctl(input.Static(key), input.Static(value)).Node()
		moduletest.AssertDryRun(t, g, node)
		moduletest.AssertConverges(t, g, node)

		got, err := g.Ops().Get(procPath).StringErr()
		if err != nil {
			t.Fatalf("read %s: %v", procPath, err)
		}
		if strings.TrimSpace(got) != value {
			t.Errorf("%s = %q, want %q", procPath, strings.TrimSpace(got), value)
		}
		moduletest.AssertFile(t, g, dropinPath, content)
	})
}
