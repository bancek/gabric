package apt

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestPackageInstalled(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "install ok installed"}
	})

	gt.RunNode(gt.Context(), Package(input.Static("ufw")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"))
	gt.Remote().AssertCalls()
}

func TestPackageDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Package(input.Static("ufw")).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"),
		gt.Command("apt-get", "install", "-y", "ufw").WithEnv("DEBIAN_FRONTEND=noninteractive"),
	)
	gt.Remote().AssertCalls()
}

func TestPackageLatestUpToDate(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("dpkg-query", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("-f=${Status}"):
			return gabrictest.Result{Stdout: "install ok installed"}
		case cmd.HasArg("-f=${Version}"):
			return gabrictest.Result{Stdout: "1.2.3"}
		}

		return gabrictest.Result{}
	})
	gt.Remote().Handle("apt-cache", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "ufw:\n  Candidate: 1.2.3\n"}
	})

	gt.RunNode(gt.Context(), Package(input.Static("ufw")).Latest().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"),
		gt.Command("dpkg-query", "-W", "-f=${Version}", "ufw"),
		gt.Command("apt-cache", "policy", "ufw"),
	)
	gt.Remote().AssertCalls()
}
