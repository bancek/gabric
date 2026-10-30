package modules

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestCommandOutputData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("cmdout", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "out\n"}
	})

	out, err := gt.RunNodeErr(gt.Context(), CommandOutputData(input.List("cmdout", "hi")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if out != "out\n" {
		t.Errorf("CommandOutputData() = %q, want %q", out, "out\n")
	}

	gt.Remote().AssertCommands(gt.Command("cmdout", "hi"))
	gt.Remote().AssertCalls()
}

func TestCommandStderrData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("cmderr", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stderr: "progress\n"}
	})

	out, err := gt.RunNodeErr(gt.Context(), CommandStderrData(input.List("cmderr", "compose", "ps")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if out != "progress\n" {
		t.Errorf("CommandStderrData() = %q, want %q", out, "progress\n")
	}

	gt.Remote().AssertCommands(gt.Command("cmderr", "compose", "ps"))
	gt.Remote().AssertCalls()
}

func TestCommandOutputDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("failcmd", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), CommandOutputData(input.List("failcmd"))); err == nil {
		t.Error("node error = nil, want error")
	}
}
