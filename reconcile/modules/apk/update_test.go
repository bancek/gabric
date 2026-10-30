package apk

import (
	"strconv"
	"testing"
	"time"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestUpdate(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().Build().Node())

	gt.Remote().AssertCommands(gt.Command("apk", "update"))
	gt.Remote().AssertCalls()
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateFresh(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		now := strconv.FormatInt(time.Now().Unix(), 10)
		return gabrictest.Result{Stdout: now + "\n" + now + "\n"}
	})
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		t.Error("apk ran with a fresh cache")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(gt.Command("sh", "-c", apkIndexStat))
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateStale(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Unix(), 10) + "\n" + strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10) + "\n"}
	})
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("sh", "-c", apkIndexStat),
		gt.Command("apk", "update"),
	)
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateMissingCache(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("sh", "-c", apkIndexStat),
		gt.Command("apk", "update"),
	)
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateDryRunFresh(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		now := strconv.FormatInt(time.Now().Unix(), 10)
		return gabrictest.Result{Stdout: now + "\n" + now + "\n"}
	})
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		t.Error("apk ran during a dry run")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(
		dryrun.With(reconcile.WithReportCollector(gt.Context(), collector)),
		Update().MaxAge(input.Static(time.Hour)).Build().Node(),
	)

	gt.Remote().AssertCommands(gt.Command("sh", "-c", apkIndexStat))
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateDryRunStale(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)}
	})
	gt.Remote().Handle("apk", func(gabrictest.Command) gabrictest.Result {
		t.Error("apk ran during a dry run")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(
		dryrun.With(reconcile.WithReportCollector(gt.Context(), collector)),
		Update().MaxAge(input.Static(time.Hour)).Build().Node(),
	)

	gt.Remote().AssertCommands(gt.Command("sh", "-c", apkIndexStat))
	assertUpdateReport(t, collector, reconcile.ReportWouldChange)
}

func TestUpdateStatError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), Update().MaxAge(input.Static(time.Hour)).Build().Node()); err == nil {
		t.Error("Update() error = nil, want error")
	}
}

func TestUpdateParseError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "not-a-number\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), Update().MaxAge(input.Static(time.Hour)).Build().Node()); err == nil {
		t.Error("Update() error = nil, want error")
	}
}

func assertUpdateReport(t *testing.T, collector *reconcile.ReportCollector, status reconcile.ReportStatus) {
	t.Helper()

	results := collector.Results()
	if len(results) != 1 || results[0].Name != "apk.update" || results[0].Status != status {
		t.Fatalf("report = %#v, want one %s apk.update", results, status)
	}
}
