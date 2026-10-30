package linux

import (
	"bytes"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

const (
	sysctlTestKey        = "net.ipv4.ip_forward"
	sysctlTestValue      = "1"
	sysctlTestProcPath   = "/proc/sys/net/ipv4/ip_forward"
	sysctlTestDropinPath = "/etc/sysctl.d/99-reconcile-net-ipv4-ip_forward.conf"
	sysctlTestContent    = "net.ipv4.ip_forward = 1\n"
)

func registerSysctlHandler(gt *gabrictest.Test) {
	gt.Remote().Handle("sysctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
}

func TestSysctlMatches(t *testing.T) {
	gt := gabrictest.New(t)
	registerSysctlHandler(gt)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte(sysctlTestValue), 0o644)
	gt.Remote().WriteFile(sysctlTestDropinPath, []byte(sysctlTestContent), 0o644)

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, Sysctl(input.Static(sysctlTestKey), input.Static(sysctlTestValue)).Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "cat", sysctlTestProcPath),
		sudoCommand(gt, "cat", sysctlTestDropinPath),
	)
	gt.Remote().AssertCalls()
}

func TestSysctlDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerSysctlHandler(gt)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte("0"), 0o644)

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, Sysctl(input.Static(sysctlTestKey), input.Static(sysctlTestValue)).Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "cat", sysctlTestProcPath),
		sudoCommand(gt, "sysctl", "-p", sysctlTestDropinPath),
	)
	gt.Remote().AssertCalls(gabrictest.Call{Put: &gabric.PutRequest{
		Path:         sysctlTestDropinPath,
		Content:      bytes.NewBufferString(sysctlTestContent),
		EnsureParent: true,
	}})
}

func TestSysctlMissingPersistence(t *testing.T) {
	gt := gabrictest.New(t)
	registerSysctlHandler(gt)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte(sysctlTestValue), 0o644)

	ctx := gabric.NewRemoteSudoContext(gt.Context(), &gabric.Sudo{})
	gt.RunNode(ctx, Sysctl(input.Static(sysctlTestKey), input.Static(sysctlTestValue)).Node())

	gt.Remote().AssertCommands(
		sudoCommand(gt, "cat", sysctlTestProcPath),
		sudoCommand(gt, "cat", sysctlTestDropinPath),
		sudoCommand(gt, "sysctl", "-p", sysctlTestDropinPath),
	)
	gt.Remote().AssertCalls(gabrictest.Call{Put: &gabric.PutRequest{
		Path:         sysctlTestDropinPath,
		Content:      bytes.NewBufferString(sysctlTestContent),
		EnsureParent: true,
	}})
}
