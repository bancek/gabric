package apt

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "install ok installed"}
	})

	if !gt.RunNode(gt.Context(), SatisfiedData(input.Static("ufw"), input.False)) {
		t.Error("SatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"))
}

func TestSatisfiedDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), SatisfiedData(input.Static("ufw"), input.False)) {
		t.Error("SatisfiedData() = true, want false")
	}
}

func TestSatisfiedDataLatest(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"current", "1.2.3", true},
		{"drift", "1.2.4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("dpkg-query", func(cmd gabrictest.Command) gabrictest.Result {
				if cmd.HasArg("-f=${Version}") {
					return gabrictest.Result{Stdout: "1.2.3\n"}
				}

				return gabrictest.Result{Stdout: "install ok installed"}
			})
			gt.Remote().Handle("apt-cache", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: "ufw:\n  Candidate: " + tt.candidate + "\n"}
			})

			if got := gt.RunNode(gt.Context(), SatisfiedData(input.Static("ufw"), input.True)); got != tt.want {
				t.Errorf("SatisfiedData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands(
				gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"),
				gt.Command("dpkg-query", "-W", "-f=${Version}", "ufw"),
				gt.Command("apt-cache", "policy", "ufw"),
			)
		})
	}
}

func TestInstalledData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "install ok installed"}
	})

	if !gt.RunNode(gt.Context(), InstalledData(input.Static("ufw"))) {
		t.Error("InstalledData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"))
}

func TestInstalledDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), InstalledData(input.Static("ufw"))) {
		t.Error("InstalledData() = true, want false")
	}
}

func TestVersionData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "1.2.3\n"}
	})

	got, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("ufw")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("VersionData() = %q, want %q", got, "1.2.3")
	}

	gt.Remote().AssertCommands(gt.Command("dpkg-query", "-W", "-f=${Version}", "ufw"))
}

func TestVersionDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	got, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("ufw")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("VersionData() = %q, want empty", got)
	}
}

func TestCandidateData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apt-cache", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "ufw:\n  Installed: 1.2.2\n  Candidate: 1.2.3\n"}
	})

	got, err := gt.RunNodeErr(gt.Context(), CandidateData(input.Static("ufw")))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("CandidateData() = %q, want %q", got, "1.2.3")
	}

	gt.Remote().AssertCommands(gt.Command("apt-cache", "policy", "ufw"))
}

func TestCandidateDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apt-cache", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), CandidateData(input.Static("ufw"))); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestPackageDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), InstalledData(input.Static("ufw"))); err == nil {
		t.Error("InstalledData() error = nil, want error")
	}
	if _, err := gt.RunNodeErr(gt.Context(), VersionData(input.Static("ufw"))); err == nil {
		t.Error("VersionData() error = nil, want error")
	}
}

func TestPackageDataTransportErrors(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	t.Run("InstalledData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, InstalledData(input.Static("ufw"))); !errors.Is(err, sentinel) {
			t.Errorf("InstalledData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("VersionData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, VersionData(input.Static("ufw"))); !errors.Is(err, sentinel) {
			t.Errorf("VersionData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("CandidateData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, CandidateData(input.Static("ufw"))); !errors.Is(err, sentinel) {
			t.Errorf("CandidateData() error = %v, want %v", err, sentinel)
		}
	})
}
