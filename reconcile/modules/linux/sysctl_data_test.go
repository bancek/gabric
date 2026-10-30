package linux

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestSysctlSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte(sysctlTestValue), 0o644)
	gt.Remote().WriteFile(sysctlTestDropinPath, []byte(sysctlTestContent), 0o644)

	if !gt.RunNode(gt.Context(), SysctlSatisfiedData(input.Static(sysctlTestKey), input.Static(sysctlTestValue))) {
		t.Error("SysctlSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(
		gt.Command("cat", sysctlTestProcPath),
		gt.Command("cat", sysctlTestDropinPath),
	)
}

func TestSysctlSatisfiedDataRuntimeDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte("0"), 0o644)

	if gt.RunNode(gt.Context(), SysctlSatisfiedData(input.Static(sysctlTestKey), input.Static(sysctlTestValue))) {
		t.Error("SysctlSatisfiedData() = true, want false")
	}

	gt.Remote().AssertCommands(gt.Command("cat", sysctlTestProcPath))
}

func TestSysctlSatisfiedDataDropinMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte(sysctlTestValue), 0o644)

	if gt.RunNode(gt.Context(), SysctlSatisfiedData(input.Static(sysctlTestKey), input.Static(sysctlTestValue))) {
		t.Error("SysctlSatisfiedData() = true, want false")
	}

	gt.Remote().AssertCommands(
		gt.Command("cat", sysctlTestProcPath),
		gt.Command("cat", sysctlTestDropinPath),
	)
}

func TestSysctlRuntimeData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(sysctlTestProcPath, []byte("1\n"), 0o644)

	got, err := gt.RunNodeErr(gt.Context(), SysctlRuntimeData(input.Static(sysctlTestKey)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "1" {
		t.Errorf("SysctlRuntimeData() = %q, want %q", got, "1")
	}

	gt.Remote().AssertCommands(gt.Command("cat", sysctlTestProcPath))
}

func TestSysctlRuntimeDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	got, err := gt.RunNodeErr(gt.Context(), SysctlRuntimeData(input.Static(sysctlTestKey)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("SysctlRuntimeData() = %q, want empty", got)
	}

	gt.Remote().AssertCommands(gt.Command("cat", sysctlTestProcPath))
}

func TestSysctlDropinData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(sysctlTestDropinPath, []byte(sysctlTestContent), 0o644)

	got, err := gt.RunNodeErr(gt.Context(), SysctlDropinData(input.Static(sysctlTestKey)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != sysctlTestContent {
		t.Errorf("SysctlDropinData() = %q, want %q", got, sysctlTestContent)
	}

	gt.Remote().AssertCommands(gt.Command("cat", sysctlTestDropinPath))
}

func TestSysctlDropinDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	got, err := gt.RunNodeErr(gt.Context(), SysctlDropinData(input.Static(sysctlTestKey)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("SysctlDropinData() = %q, want empty", got)
	}

	gt.Remote().AssertCommands(gt.Command("cat", sysctlTestDropinPath))
}

func TestSysctlDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("cat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), SysctlRuntimeData(input.Static(sysctlTestKey))); err == nil {
		t.Error("SysctlRuntimeData() error = nil, want error")
	}
	if _, err := gt.RunNodeErr(gt.Context(), SysctlDropinData(input.Static(sysctlTestKey))); err == nil {
		t.Error("SysctlDropinData() error = nil, want error")
	}
}

func TestSysctlDataTransportErrors(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	t.Run("SysctlRuntimeData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, SysctlRuntimeData(input.Static(sysctlTestKey))); !errors.Is(err, sentinel) {
			t.Errorf("SysctlRuntimeData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("SysctlDropinData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, SysctlDropinData(input.Static(sysctlTestKey))); !errors.Is(err, sentinel) {
			t.Errorf("SysctlDropinData() error = %v, want %v", err, sentinel)
		}
	})
}
