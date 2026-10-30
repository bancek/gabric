package github

import (
	"bytes"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
)

const (
	githubRawDest = "/srv/ufw-docker"
	githubRawRef  = "/srv/ufw-docker.ref"
	githubRawURL  = "https://raw.githubusercontent.com/chaifeng/ufw-docker/abc123/ufw-docker"
)

func TestRawFileDownloads(t *testing.T) {
	gt := gabrictest.New(t)
	registerCurlDownload(gt, githubRawDest+".tmp", []byte("script"))

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		Dest(input.Static(githubRawDest)).Mode(modules.FileMode(0o755)).Build()

	gt.RunNode(gt.Context(), script.Node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", githubRawURL, "-o", githubRawDest+".tmp"),
		gt.Command("mv", githubRawDest+".tmp", githubRawDest),
		gt.Command("chmod", "755", githubRawDest),
		gt.Command("sha256sum", githubRawRef),
	)
	gt.Remote().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: githubRawRef}},
		httpFileMkdir("/srv"),
		existsCall(githubRawRef, gabric.FileKindAny),
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         githubRawRef,
			Content:      bytes.NewBufferString("abc123"),
			Mode:         0o644,
			EnsureParent: true,
		}},
	)
}

func TestRawFileNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(githubRawDest, []byte("script"), 0o755)
	gt.Remote().WriteFile(githubRawRef, []byte("abc123"), 0o644)

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		Dest(input.Static(githubRawDest)).Mode(modules.FileMode(0o755)).Build()

	gt.RunNode(gt.Context(), script.Node)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: githubRawRef}},
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: githubRawDest,
			Kind: gabric.FileKindFile,
		}},
	)
}

func TestRawFileDestMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(githubRawRef, []byte("abc123"), 0o644)
	registerCurlDownload(gt, githubRawDest+".tmp", []byte("script"))

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		Dest(input.Static(githubRawDest)).Mode(modules.FileMode(0o755)).Build()

	gt.RunNode(gt.Context(), script.Node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", githubRawURL, "-o", githubRawDest+".tmp"),
		gt.Command("mv", githubRawDest+".tmp", githubRawDest),
		gt.Command("chmod", "755", githubRawDest),
		gt.Command("sha256sum", githubRawRef),
	)
	gt.Remote().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: githubRawRef}},
		existsCall(githubRawDest, gabric.FileKindFile),
		httpFileMkdir("/srv"),
		statCall(githubRawRef),
	)
}

func TestRawFileRefFileAndOwner(t *testing.T) {
	refFile := "/srv/custom.ref"
	gt := gabrictest.New(t)
	registerCurlDownload(gt, githubRawDest+".tmp", []byte("script"))

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		Dest(input.Static(githubRawDest)).
		RefFile(input.Static(refFile)).
		Mode(modules.FileMode(0o755)).
		User(input.Static("root")).
		Group(input.Static("root")).
		Build()

	gt.RunNode(gt.Context(), script.Node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", githubRawURL, "-o", githubRawDest+".tmp"),
		gt.Command("mv", githubRawDest+".tmp", githubRawDest),
		gt.Command("chown", "root:root", githubRawDest),
		gt.Command("chmod", "755", githubRawDest),
		gt.Command("sha256sum", refFile),
	)
	gt.Remote().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: refFile}},
		httpFileMkdir("/srv"),
		existsCall(refFile, gabric.FileKindAny),
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         refFile,
			Content:      bytes.NewBufferString("abc123"),
			Mode:         0o644,
			EnsureParent: true,
		}},
	)
}

func TestRawFileDryRun(t *testing.T) {
	gt := gabrictest.New(t)

	ctx := dryrun.With(gt.Context())

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		Dest(input.Static(githubRawDest)).Mode(modules.FileMode(0o755)).Build()

	gt.RunNode(ctx, script.Node)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Get: &gabric.GetRequest{Path: githubRawRef}})
}

func TestRawFileBaseURL(t *testing.T) {
	gt := gabrictest.New(t)
	dest := "/srv/custom"
	registerCurlDownload(gt, dest+".tmp", []byte("script"))

	script := RawFile(input.Static("chaifeng"), input.Static("ufw-docker"), input.Static("ufw-docker"), input.Static("abc123")).
		BaseURL(input.Static("http://example.test/raw")).
		Dest(input.Static(dest)).
		Build()

	gt.RunNode(gt.Context(), script.Node)

	gt.Remote().AssertExecuted("curl", "-fsSL",
		"http://example.test/raw/chaifeng/ufw-docker/abc123/ufw-docker",
		"-o", dest+".tmp")
}
