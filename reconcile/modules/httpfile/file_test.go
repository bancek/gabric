package httpfile

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
)

const (
	httpFilePath       = "/srv/app.bin"
	httpFileDir        = "/srv"
	httpFileParentPath = "/srv/sub/app.bin"
	httpFileParentDir  = "/srv/sub"
)

func TestFileResourceName(t *testing.T) {
	file := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath))

	if got, want := file.Build().Node().Name(), "httpfile.file:"+httpFilePath; got != want {
		t.Errorf("default name = %q, want %q", got, want)
	}

	got := File(input.Static("https://example.com/app.bin")).
		Path(input.WithName(input.Static(httpFilePath), "app.bin")).
		Build().Node().Name()
	if want := "httpfile.file:app.bin"; got != want {
		t.Errorf("WithName() = %q, want %q", got, want)
	}
}

func TestSatisfiedData(t *testing.T) {
	content := []byte("hello")
	sha := contentHash(content)

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(httpFilePath, content, 0o644)

	if !gt.RunNode(gt.Context(), SatisfiedData(input.Static(httpFilePath), input.Static(sha), modules.FileMode(0o644))) {
		t.Error("SatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", httpFilePath))
	gt.Remote().AssertCalls(statCall(httpFilePath))
}

func TestFileMatches(t *testing.T) {
	content := []byte("hello")
	sha := contentHash(content)

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(httpFilePath, content, 0o644)

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", httpFilePath))
	gt.Remote().AssertCalls(statCall(httpFilePath))
}

func TestFileMissing(t *testing.T) {
	content := []byte("hello")
	sha := contentHash(content)

	gt := gabrictest.New(t)
	registerCurlDownload(gt, httpFilePath+".tmp", content)

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("sha256sum", httpFilePath),
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
		gt.Command("mv", httpFilePath+".tmp", httpFilePath),
		gt.Command("chmod", "644", httpFilePath),
	)
	gt.Remote().AssertCalls(
		existsCall(httpFilePath, gabric.FileKindAny),
		httpFileMkdir(httpFileDir),
	)
}

func TestFileHashDrift(t *testing.T) {
	sha := contentHash([]byte("hello"))

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(httpFilePath, []byte("other"), 0o644)
	registerCurlDownload(gt, httpFilePath+".tmp", []byte("hello"))

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("sha256sum", httpFilePath),
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
		gt.Command("mv", httpFilePath+".tmp", httpFilePath),
		gt.Command("chmod", "644", httpFilePath),
	)
	gt.Remote().AssertCalls(
		httpFileMkdir(httpFileDir),
	)
}

func TestFileModeDrift(t *testing.T) {
	content := []byte("hello")
	sha := contentHash(content)

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(httpFilePath, content, 0o600)
	registerCurlDownload(gt, httpFilePath+".tmp", content)

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("sha256sum", httpFilePath),
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
		gt.Command("mv", httpFilePath+".tmp", httpFilePath),
		gt.Command("chmod", "644", httpFilePath),
	)
	gt.Remote().AssertCalls(
		statCall(httpFilePath),
		httpFileMkdir(httpFileDir),
	)
}

func TestFileNoSHA256NoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(httpFilePath, []byte("deadbeef"), 0o644)

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(
		existsCall(httpFilePath, gabric.FileKindFile),
		statCall(httpFilePath),
	)
}

func TestFileParent(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("curl", func(gabrictest.Command) gabrictest.Result {
		if _, err := gt.Remote().Stat(httpFileParentDir); err != nil {
			t.Errorf("curl ran before the parent was created: %v", err)
		}

		gt.Remote().WriteFile(httpFileParentPath+".tmp", []byte("hello"), 0o644)

		return gabrictest.Result{}
	})

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFileParentPath)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFileParentPath+".tmp"),
		gt.Command("mv", httpFileParentPath+".tmp", httpFileParentPath),
	)
	gt.Remote().AssertCalls(
		existsCall(httpFileParentPath, gabric.FileKindFile),
		httpFileMkdir(httpFileParentDir),
	)
}

func TestFileChown(t *testing.T) {
	gt := gabrictest.New(t)
	registerCurlDownload(gt, httpFilePath+".tmp", []byte("hello"))

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).User(input.Static("root")).Group(input.Static("root")).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
		gt.Command("mv", httpFilePath+".tmp", httpFilePath),
		gt.Command("chown", "root:root", httpFilePath),
	)
	gt.Remote().AssertCalls(
		existsCall(httpFilePath, gabric.FileKindFile),
		httpFileMkdir(httpFileDir),
	)
}

func TestFileDryRun(t *testing.T) {
	gt := gabrictest.New(t)

	ctx := dryrun.With(gt.Context())
	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).Build().Node()
	gt.RunNode(ctx, node)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(existsCall(httpFilePath, gabric.FileKindFile))
}

func TestFileForceWrite(t *testing.T) {
	gt := gabrictest.New(t)
	registerCurlDownload(gt, httpFilePath+".tmp", []byte("hello"))

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Mode(modules.FileMode(0o644)).ForceWrite().Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
		gt.Command("mv", httpFilePath+".tmp", httpFilePath),
		gt.Command("chmod", "644", httpFilePath),
	)
	gt.Remote().AssertCalls(
		httpFileMkdir(httpFileDir),
	)
}

func TestFileForceWriteSHA256Error(t *testing.T) {
	gt := gabrictest.New(t)
	node := File(input.Static("https://example.com/app.bin")).
		Path(input.Static("/tmp/x")).
		ForceWrite().
		SHA256(input.Static("abc")).
		Build().
		Node()

	if _, err := gt.RunNodeErr(gt.Context(), node); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestFileError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("curl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	node := File(input.Static("https://example.com/app.bin")).Path(input.Static(httpFilePath)).Build().Node()

	if _, err := gt.RunNodeErr(gt.Context(), node); err == nil {
		t.Error("node error = nil, want error")
	}

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL", "https://example.com/app.bin", "-o", httpFilePath+".tmp"),
	)
	gt.Remote().AssertCalls(
		existsCall(httpFilePath, gabric.FileKindFile),
		httpFileMkdir(httpFileDir),
	)
}

func TestFileBuildPanics(t *testing.T) {
	assertPanics(t, "nil url", func() { File(nil).Path(input.Static("/tmp/x")).Build() })
	assertPanics(t, "nil path", func() { File(input.Static("https://example.com/x")).Build() })
}
