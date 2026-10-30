package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/reconcile"
)

var (
	e2eModeFlag = gabric.Default.FlagSet.String(
		"e2e-mode", "both", "remote, local, or both")
	e2eFreshFlag = gabric.Default.FlagSet.Bool(
		"e2e-fresh", false, "destroy and recreate the Vagrant VM")
	e2eDestroyFlag = gabric.Default.FlagSet.Bool(
		"e2e-destroy", false, "destroy the Vagrant VM when done")
)

// E2E converges the Vagrant VM in remote mode, local mode, or both.
var E2E = gabric.Task("e2e", func(ctx context.Context) {
	if dryrun.Enabled(ctx) {
		panic("e2e: do not run with -dry-run")
	}

	switch *e2eModeFlag {
	case "remote", "local", "both":
	default:
		panic(fmt.Sprintf("e2e: unknown mode %q", *e2eModeFlag))
	}

	hostConfig, err := vagrantUp(ctx, *e2eFreshFlag)
	gabric.Must(err)

	if *e2eDestroyFlag {
		defer func() {
			if err := vagrantDestroy(ctx); err != nil {
				gabric.Logger(ctx).Warn("vagrant destroy failed", "error", err)
			}
		}()
	}

	err = withVagrantHostConfig(ctx, hostConfig, func(ctx context.Context) error {
		ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)

		var modeErr error

		switch *e2eModeFlag {
		case "remote":
			modeErr = e2eRemote(ctx)
		case "local":
			modeErr = e2eLocal(ctx, false)
		case "both":
			if modeErr = e2eRemote(ctx); modeErr != nil {
				break
			}
			if modeErr = e2eLocal(ctx, true); modeErr != nil {
				break
			}
			modeErr = e2eRemoteDry(ctx)
		}
		if modeErr != nil {
			return modeErr
		}

		return checkSite(ctx)
	})
	gabric.Must(err)
})

func runRemoteAll(ctx context.Context, dry bool) ([]reconcile.ReportResult, error) {
	collector := &reconcile.ReportCollector{}
	ctx = reconcile.WithReportCollector(ctx, collector)

	if dry {
		ctx = dryrun.With(ctx)
	}

	calls, err := gabric.Default.ParseTasks([]string{"all"})
	if err != nil {
		return nil, err
	}
	if err := gabric.Default.Run(ctx, calls); err != nil {
		return nil, err
	}

	return collector.Results(), nil
}

func assertReport(results []reconcile.ReportResult, dry bool) error {
	if len(results) == 0 {
		return fmt.Errorf("e2e: reconcile report is empty")
	}

	for _, r := range results {
		switch r.Status {
		case reconcile.ReportFailed:
			if r.Err != nil {
				return fmt.Errorf("e2e: %s failed: %w", r.Name, r.Err)
			}
			return fmt.Errorf("e2e: %s failed", r.Name)
		case reconcile.ReportWouldChange:
			if dry {
				return fmt.Errorf("e2e: %s would change after convergence", r.Name)
			}
			return fmt.Errorf("e2e: %s unexpectedly reported a dry-run change", r.Name)
		}
	}

	return nil
}

func e2eRemote(ctx context.Context) error {
	results, err := runRemoteAll(ctx, false)
	if err != nil {
		return err
	}
	if err := assertReport(results, false); err != nil {
		return err
	}

	return e2eRemoteDry(ctx)
}

func e2eRemoteDry(ctx context.Context) error {
	results, err := runRemoteAll(ctx, true)
	if err != nil {
		return err
	}

	return assertReport(results, true)
}

func e2eLocal(ctx context.Context, expectNoChanges bool) error {
	localDir, err := filepath.Abs(*homelabDirFlag)
	if err != nil {
		return err
	}

	remoteDir := *remoteDirFlag
	if remoteDir == "" {
		remoteDir, err = defaultRemoteDir(ctx)
		if err != nil {
			return err
		}
	}

	realOut, err := deployLocal(ctx, deployOptions{
		localDir:  localDir,
		remoteDir: remoteDir,
		gabArgs:   []string{"-local", "all"},
		capture:   true,
	})
	logCaptured("local run", realOut)
	if err != nil {
		return err
	}
	if expectNoChanges {
		if err := assertSummary(realOut); err != nil {
			return err
		}
	}

	dryOut, err := deployLocal(ctx, deployOptions{
		localDir:  localDir,
		remoteDir: remoteDir,
		gabArgs:   []string{"-local", "-dry-run", "all"},
		capture:   true,
	})
	logCaptured("local dry run", dryOut)
	if err != nil {
		return err
	}

	return assertSummary(dryOut)
}

func logCaptured(label, out string) {
	if out == "" {
		return
	}

	fmt.Fprintf(os.Stderr, "==> %s\n", label)
	fmt.Fprint(os.Stderr, out)

	if !strings.HasSuffix(out, "\n") {
		fmt.Fprintln(os.Stderr)
	}
}

var reportSummaryRE = regexp.MustCompile(
	`(\d+) ok, (\d+) changed, (\d+) would-change, (\d+) failed`,
)

func assertSummary(out string) error {
	matches := reportSummaryRE.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return fmt.Errorf("e2e: no reconcile summary in inner output")
	}

	m := matches[len(matches)-1]

	ok, _ := strconv.Atoi(m[1])
	changed, _ := strconv.Atoi(m[2])
	wouldChange, _ := strconv.Atoi(m[3])
	failed, _ := strconv.Atoi(m[4])

	if ok == 0 {
		return fmt.Errorf("e2e: empty reconcile summary: %s", m[0])
	}
	if changed != 0 || wouldChange != 0 || failed != 0 {
		return fmt.Errorf("e2e: not idempotent: %s", m[0])
	}

	return nil
}

func checkSite(ctx context.Context) error {
	domain := *domainFlag

	// checkSite assumes TLS is off; with -tls, traefik redirects HTTP to HTTPS.

	if err := checkHTTP(ctx, domain, "homelab"); err != nil {
		return err
	}
	if err := checkHTTP(ctx, "app."+domain, "Hostname"); err != nil {
		return err
	}

	sudoCtx := gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{
		Flags: []string{"-n"},
	})

	return gabric.Remote(sudoCtx).
		Exec("sh", "-c", `ufw status | grep -q 'Status: active'`).
		RunErr()
}

func checkHTTP(ctx context.Context, host, want string) error {
	e := gabric.Remote(ctx).
		Exec("curl", "-fsS", "-H", "Host: "+host, "http://127.0.0.1/").
		CaptureStdout()
	if err := e.RunErr(); err != nil {
		return fmt.Errorf("e2e: http check for %s: %w", host, err)
	}

	if !strings.Contains(e.StdoutStr(), want) {
		return fmt.Errorf("e2e: http check for %s: response does not contain %q", host, want)
	}

	return nil
}
