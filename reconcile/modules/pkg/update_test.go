package pkg

import (
	"strconv"
	"testing"
	"time"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestUpdateApt(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().Build().Node())

	gt.Remote().AssertCommands(gt.Command("apt-get", "update"))
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateApk(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=alpine\n"), 0o644)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().Build().Node())

	gt.Remote().AssertCommands(gt.Command("apk", "update"))
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateFresh(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran with a fresh cache")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(gt.Command("stat", "-c", "%Y", "/var/lib/apt/periodic/update-success-stamp"))
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateStale(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("stat", "-c", "%Y", "/var/lib/apt/periodic/update-success-stamp"),
		gt.Command("stat", "-c", "%Y", "/var/lib/apt/lists/partial"),
		gt.Command("apt-get", "update"),
	)
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateDryRun(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nID_LIKE=debian\n"), 0o644)
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran during a dry run")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(
		dryrun.With(reconcile.WithReportCollector(gt.Context(), collector)),
		Update().Build().Node(),
	)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(getCall("/etc/os-release"))
	assertUpdateReport(t, collector, reconcile.ReportWouldChange)
}

func TestUpdateUnsupported(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=fedora\n"), 0o644)

	if _, err := gt.RunNodeErr(gt.Context(), Update().Build().Node()); err == nil {
		t.Error("Update() error = nil, want error")
	}
}

func assertUpdateReport(t *testing.T, collector *reconcile.ReportCollector, status reconcile.ReportStatus) {
	t.Helper()

	results := collector.Results()
	if len(results) != 1 || results[0].Name != "pkg.update" || results[0].Status != status {
		t.Fatalf("report = %#v, want one %s pkg.update", results, status)
	}
}
