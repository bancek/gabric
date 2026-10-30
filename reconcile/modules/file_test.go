package modules

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const filePath = "/srv/app.conf"

func filePutCall(content string, mode fs.FileMode) gabrictest.Call {
	return gabrictest.Call{Put: &gabric.PutRequest{
		Path:         filePath,
		Content:      bytes.NewBufferString(content),
		Mode:         mode,
		EnsureParent: true,
	}}
}

func TestFileResourceName(t *testing.T) {
	content := input.Static([]byte("hello"))

	if got, want := File(input.Static(filePath)).Content(content).Build().Node().Name(), "modules.file:"+filePath; got != want {
		t.Errorf("default name = %q, want %q", got, want)
	}

	got := File(input.WithName(input.Static(filePath), "app.conf")).Content(content).Build().Node().Name()
	if want := "modules.file:app.conf"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestFileSatisfiedData(t *testing.T) {
	content := []byte("hello")

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(filePath, content, 0o644)

	contentNode := reconcile.Data("content", func(context.Context) ([]byte, error) {
		return content, nil
	}).Node()

	if !gt.RunNode(gt.Context(), FileSatisfiedData(input.Static(filePath), contentNode, FileMode(0o644), input.False)) {
		t.Error("FileSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(statCall(filePath))
}

func TestFileMatches(t *testing.T) {
	content := []byte("hello")

	gt := gabrictest.New(t)
	gt.Remote().WriteFile(filePath, content, 0o644)

	node := File(input.Static(filePath)).Content(input.Static(content)).Mode(FileMode(0o644)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(statCall(filePath))
}

func TestFileDrift(t *testing.T) {
	content := []byte("hello")

	gt := gabrictest.New(t)

	node := File(input.Static(filePath)).Content(input.Static(content)).Mode(FileMode(0o644)).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		filePutCall("hello", 0o644),
	)
}

func TestFileLocalTarget(t *testing.T) {
	content := []byte("hello")

	gt := gabrictest.New(t)
	gt.Local().WriteFile(filePath, content, 0o644)

	ctx := gabric.NewTargetContext(gt.Context(), gabric.LocalTarget)
	node := File(input.Static(filePath)).Content(input.Static(content)).Mode(FileMode(0o644)).Build().Node()
	gt.RunNode(ctx, node)

	gt.Local().AssertCommands()
	gt.Local().AssertCalls(getCall(filePath), statCall(filePath))
	gt.Remote().AssertCalls()
}

func TestFileLocalTargetContentOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.conf")
	ctx := gabric.NewTargetContext(t.Context(), gabric.LocalTarget)
	node := File(input.Static(path)).Content(input.Static([]byte("hello\n"))).Build().Node()

	changed, err := reconcile.RunNode(ctx, node)
	if err != nil {
		t.Fatalf("first RunNode() error: %v", err)
	}
	if !changed {
		t.Error("first RunNode() changed = false, want true")
	}

	changed, err = reconcile.RunNode(ctx, node)
	if err != nil {
		t.Fatalf("second RunNode() error: %v", err)
	}
	if changed {
		t.Error("second RunNode() changed = true, want false")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if want := []byte("hello\n"); !bytes.Equal(got, want) {
		t.Errorf("file content = %q, want %q", got, want)
	}

	changedNode := File(input.Static(path)).Content(input.Static([]byte("goodbye\n"))).Build().Node()

	changed, err = reconcile.RunNode(ctx, changedNode)
	if err != nil {
		t.Fatalf("changed RunNode() error: %v", err)
	}
	if !changed {
		t.Error("changed RunNode() changed = false, want true")
	}

	changed, err = reconcile.RunNode(ctx, changedNode)
	if err != nil {
		t.Fatalf("settled RunNode() error: %v", err)
	}
	if changed {
		t.Error("settled RunNode() changed = true, want false")
	}
}

func TestFileNotify(t *testing.T) {
	gt := gabrictest.New(t)

	topic := reconcile.NewTopic("t")
	notified := false

	reconcile.NewEffect("e", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		notified = true
		return nil
	})

	node := File(input.Static(filePath)).Content(input.Static([]byte("hello"))).Build().Notify(topic).Node()
	gt.RunNode(gt.Context(), node)

	if !notified {
		t.Error("change did not notify / flush the effect")
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		filePutCall("hello", 0),
	)
}

func TestFileDryRun(t *testing.T) {
	gt := gabrictest.New(t)

	ctx := dryrun.With(gt.Context())
	node := File(input.Static(filePath)).Content(input.Static([]byte("hello"))).Build().Node()
	gt.RunNode(ctx, node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(existsCall(filePath, gabric.FileKindAny))
}

func TestFileContentNodeMemoized(t *testing.T) {
	calls := 0
	content := []byte("hello")

	gt := gabrictest.New(t)

	contentNode := reconcile.Data("file-content", func(ctx context.Context) ([]byte, error) {
		calls++
		return content, nil
	}).Node()
	node := File(input.Static(filePath)).Content(contentNode).Build().Node()

	gt.RunNode(gt.Context(), node)

	if calls != 1 {
		t.Errorf("content node calls = %d, want 1 (memoized per run)", calls)
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		filePutCall("hello", 0),
	)
}

func TestFileContentNode(t *testing.T) {
	content := []byte("hello")

	gt := gabrictest.New(t)
	contentNode := reconcile.Data("shared-content", func(ctx context.Context) ([]byte, error) {
		return content, nil
	}).Node()

	node := File(input.Static(filePath)).Content(contentNode).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		filePutCall("hello", 0),
	)
}

func TestFileContentLastWins(t *testing.T) {
	gt := gabrictest.New(t)

	node := File(input.Static(filePath)).
		Content(input.Static([]byte("func"))).
		Content(input.Static([]byte("static"))).
		Build().Node()

	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		filePutCall("static", 0),
	)
}

func TestFileCreateOnly(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(filePath, []byte("old"), 0o644)

	node := File(input.Static(filePath)).Content(input.Static([]byte("new"))).CreateOnly().Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(existsCall(filePath, gabric.FileKindFile))
}

func TestFileOwner(t *testing.T) {
	gt := gabrictest.New(t)

	node := File(input.Static(filePath)).Content(input.Static([]byte("hello"))).User(input.Static("root")).Group(input.Static("root")).Build().Node()
	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(gt.Command("sha256sum", filePath))
	gt.Remote().AssertCalls(
		existsCall(filePath, gabric.FileKindAny),
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         filePath,
			Content:      bytes.NewBufferString("hello"),
			User:         "root",
			Group:        "root",
			EnsureParent: true,
		}},
	)
}
