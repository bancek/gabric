package pkg

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestPackage(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
	gt.Remote().Handle("dpkg-query", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "install ok installed"}
	})

	gt.RunNode(gt.Context(), Package(input.Static("ufw")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("dpkg-query", "-W", "-f=${Status}", "ufw"))
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
}

func TestPackageDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
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
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
}

func TestPackageAlpine(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=alpine\n"), 0o644)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Package(input.Static("curl")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("apk", "info", "-e", "curl"))
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
}

func TestPackageAlpineDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=alpine\n"), 0o644)
	gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("info") {
			return gabrictest.Result{ExitCode: 1}
		}

		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Package(input.Static("curl")).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("apk", "info", "-e", "curl"),
		gt.Command("apk", "add", "curl"),
	)
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
}

func TestPackageUnsupported(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=fedora\n"), 0o644)

	if _, err := gt.RunNodeErr(gt.Context(), Package(input.Static("ufw")).Build().Node()); err == nil {
		t.Error("Package() error = nil, want error")
	}
}

func getCall(path string) gabrictest.Call {
	return gabrictest.Call{Get: &gabric.GetRequest{Path: path}}
}
