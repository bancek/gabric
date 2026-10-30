package apk

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("info") {
			return gabrictest.Result{}
		}

		return gabrictest.Result{ExitCode: 2}
	})

	if !gt.RunNode(gt.Context(), SatisfiedData(input.Static("curl"), input.False)) {
		t.Error("SatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("apk", "info", "-e", "curl"))
}

func TestSatisfiedDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), SatisfiedData(input.Static("curl"), input.False)) {
		t.Error("SatisfiedData() = true, want false")
	}
}

func TestSatisfiedDataLatest(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"current", "8.14.1-r2", true},
		{"drift", "8.14.2-r0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
				switch {
				case cmd.HasArg("info"):
					return gabrictest.Result{}
				case cmd.HasArg("list"):
					return gabrictest.Result{Stdout: "curl-8.14.1-r2 aarch64 {curl} (curl) [installed]\n"}
				case cmd.HasArg("search"):
					return gabrictest.Result{Stdout: "curl-" + tt.candidate + "\n"}
				}

				return gabrictest.Result{ExitCode: 2}
			})

			if got := gt.RunNode(gt.Context(), SatisfiedData(input.Static("curl"), input.True)); got != tt.want {
				t.Errorf("SatisfiedData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands(
				gt.Command("apk", "info", "-e", "curl"),
				gt.Command("apk", "list", "-I", "curl"),
				gt.Command("apk", "search", "-x", "curl"),
			)
		})
	}
}

func TestInstalledData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	if !gt.RunNode(gt.Context(), InstalledData(input.Static("curl"))) {
		t.Error("InstalledData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("apk", "info", "-e", "curl"))
}

func TestInstalledDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), InstalledData(input.Static("curl"))) {
		t.Error("InstalledData() = true, want false")
	}
}

func TestInstalledDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), InstalledData(input.Static("curl"))); err == nil {
		t.Error("InstalledData() error = nil, want error")
	}
}

func TestVersionData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "curl-8.14.1-r2 aarch64 {curl} (curl) [installed]\n"}
	})

	got, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("curl")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "8.14.1-r2" {
		t.Errorf("VersionData() = %q, want %q", got, "8.14.1-r2")
	}

	gt.Remote().AssertCommands(gt.Command("apk", "list", "-I", "curl"))
}

func TestVersionDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	got, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("curl")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("VersionData() = %q, want empty", got)
	}
}

func TestVersionDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("curl"))); err == nil {
		t.Error("VersionData() error = nil, want error")
	}
}

func TestCandidateData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "curl-8.14.1-r2\n"}
	})

	got, err := gt.RunNodeErr(gt.Context(), CandidateData(input.Static("curl")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "8.14.1-r2" {
		t.Errorf("CandidateData() = %q, want %q", got, "8.14.1-r2")
	}

	gt.Remote().AssertCommands(gt.Command("apk", "search", "-x", "curl"))
}

func TestCandidateDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	got, err := gt.RunNodeErr(gt.Context(), CandidateData(input.Static("curl")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("CandidateData() = %q, want empty", got)
	}
}

func TestCandidateDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), CandidateData(input.Static("curl"))); err == nil {
		t.Error("CandidateData() error = nil, want error")
	}
}

func TestPackageDataTransportErrors(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	t.Run("InstalledData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, InstalledData(input.Static("curl"))); !errors.Is(err, sentinel) {
			t.Errorf("InstalledData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("VersionData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, VersionData(input.Static("curl"))); !errors.Is(err, sentinel) {
			t.Errorf("VersionData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("CandidateData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, CandidateData(input.Static("curl"))); !errors.Is(err, sentinel) {
			t.Errorf("CandidateData() error = %v, want %v", err, sentinel)
		}
	})
}
