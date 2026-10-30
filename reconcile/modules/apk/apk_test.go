package apk

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestPackageInstalled(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("info") {
			return gabrictest.Result{}
		}

		return gabrictest.Result{ExitCode: 2}
	})

	gt.RunNode(gt.Context(), Package(input.Static("curl")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("apk", "info", "-e", "curl"))
	gt.Remote().AssertCalls()
}

func TestPackageDrift(t *testing.T) {
	gt := gabrictest.New(t)
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
	gt.Remote().AssertCalls()
}

func TestPackageLatestUpToDate(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("info"):
			return gabrictest.Result{}
		case cmd.HasArg("list"):
			return gabrictest.Result{Stdout: "curl-8.14.1-r2 aarch64 {curl} (curl) [installed]\n"}
		case cmd.HasArg("search"):
			return gabrictest.Result{Stdout: "curl-8.14.1-r2\n"}
		}

		return gabrictest.Result{ExitCode: 2}
	})

	gt.RunNode(gt.Context(), Package(input.Static("curl")).Latest().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("apk", "info", "-e", "curl"),
		gt.Command("apk", "list", "-I", "curl"),
		gt.Command("apk", "search", "-x", "curl"),
	)
	gt.Remote().AssertCalls()
}

func TestPackageLatestDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("info"):
			return gabrictest.Result{}
		case cmd.HasArg("list"):
			return gabrictest.Result{Stdout: "curl-8.14.1-r2 aarch64 {curl} (curl) [installed]\n"}
		case cmd.HasArg("search"):
			return gabrictest.Result{Stdout: "curl-8.14.2-r0\n"}
		}

		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Package(input.Static("curl")).Latest().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("apk", "info", "-e", "curl"),
		gt.Command("apk", "list", "-I", "curl"),
		gt.Command("apk", "search", "-x", "curl"),
		gt.Command("apk", "add", "-u", "curl"),
	)
	gt.Remote().AssertCalls()
}
