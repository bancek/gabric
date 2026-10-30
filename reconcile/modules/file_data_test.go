package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

const fileDataPath = "/srv/revision"

func TestFileContentData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileDataPath, []byte("abc123\n"), 0o644)

	got, err := gt.RunNodeErr(gt.Context(), FileContentData(input.Static(fileDataPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "abc123\n" {
		t.Errorf("FileContentData() = %q, want %q", got, "abc123\n")
	}

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Get: &gabric.GetRequest{Path: fileDataPath}})
}

func TestFileContentDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	got, err := gt.RunNodeErr(gt.Context(), FileContentData(input.Static(fileDataPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("FileContentData() = %q, want empty", got)
	}

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Get: &gabric.GetRequest{Path: fileDataPath}})
}

func TestFileExistsData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileDataPath, []byte("x"), 0o644)

	got := gt.RunNode(gt.Context(), FileExistsData(input.Static(fileDataPath)))
	if !got {
		t.Error("FileExistsData() = false, want true")
	}

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: fileDataPath,
		Kind: gabric.FileKindFile,
	}})
}

func TestFileExistsDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	got := gt.RunNode(gt.Context(), FileExistsData(input.Static(fileDataPath)))
	if got {
		t.Error("FileExistsData() = true, want false")
	}

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: fileDataPath,
		Kind: gabric.FileKindFile,
	}})
}

func TestPathExistsData(t *testing.T) {
	tests := []struct {
		name  string
		setup func(gt *gabrictest.Test)
		want  bool
	}{
		{"file", func(gt *gabrictest.Test) {
			gt.Remote().WriteFile(fileDataPath, []byte("x"), 0o644)
		}, true},
		{"dir", func(gt *gabrictest.Test) {
			gt.Remote().MkdirAll(fileDataPath, 0o755)
		}, true},
		{"missing", func(*gabrictest.Test) {}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			tt.setup(gt)

			if got := gt.RunNode(gt.Context(), PathExistsData(input.Static(fileDataPath))); got != tt.want {
				t.Errorf("PathExistsData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands()
			gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
				Path: fileDataPath,
				Kind: gabric.FileKindAny,
			}})
		})
	}
}

func TestDirExistsData(t *testing.T) {
	tests := []struct {
		name  string
		setup func(gt *gabrictest.Test)
		want  bool
	}{
		{"dir", func(gt *gabrictest.Test) {
			gt.Remote().MkdirAll(fileDataPath, 0o755)
		}, true},
		{"file", func(gt *gabrictest.Test) {
			gt.Remote().WriteFile(fileDataPath, []byte("x"), 0o644)
		}, false},
		{"missing", func(*gabrictest.Test) {}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			tt.setup(gt)

			if got := gt.RunNode(gt.Context(), DirExistsData(input.Static(fileDataPath))); got != tt.want {
				t.Errorf("DirExistsData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands()
			gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
				Path: fileDataPath,
				Kind: gabric.FileKindDir,
			}})
		})
	}
}

func TestFileHashData(t *testing.T) {
	gt := gabrictest.New(t)
	content := []byte("hello")
	gt.Remote().WriteFile(fileDataPath, content, 0o644)

	got, err := gt.RunNodeErr(gt.Context(), FileHashData(input.Static(fileDataPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if want := contentHash(content); got != want {
		t.Errorf("FileHashData() = %q, want %q", got, want)
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", fileDataPath))
	gt.Remote().AssertCalls()
}

func TestFileHashDataMissing(t *testing.T) {
	gt := gabrictest.New(t)

	got, err := gt.RunNodeErr(gt.Context(), FileHashData(input.Static(fileDataPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "" {
		t.Errorf("FileHashData() = %q, want empty", got)
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", fileDataPath))
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: fileDataPath,
		Kind: gabric.FileKindAny,
	}})
}

func TestFileHashDataDirectory(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().MkdirAll(fileDataPath, 0o755)

	if _, err := gt.RunNodeErr(gt.Context(), FileHashData(input.Static(fileDataPath))); err == nil {
		t.Error("node error = nil, want error")
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", fileDataPath))
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: fileDataPath,
		Kind: gabric.FileKindAny,
	}})
}

func TestFileHashDataShasumFallback(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(fileDataPath, []byte("abc"), 0o644)
	gt.Remote().Handle("sha256sum", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 127}
	})
	gt.Remote().Handle("shasum", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "abc123  " + fileDataPath + "\n"}
	})

	got, err := gt.RunNodeErr(gt.Context(), FileHashData(input.Static(fileDataPath)))
	if err != nil {
		t.Fatalf("node error: %v", err)
	}
	if got != "abc123" {
		t.Errorf("FileHashData() = %q, want abc123", got)
	}

	gt.Remote().AssertCommands(
		gt.Command("sha256sum", fileDataPath),
		gt.Command("shasum", "-a", "256", fileDataPath),
	)
}

func TestFileHashDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sha256sum", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), FileHashData(input.Static(fileDataPath))); err == nil {
		t.Error("FileHashData() error = nil, want error")
	}

	gt.Remote().AssertCommands(gt.Command("sha256sum", fileDataPath))
	gt.Remote().AssertCalls()
}

func TestFileHashDataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, FileHashData(input.Static(fileDataPath))); !errors.Is(err, sentinel) {
		t.Errorf("FileHashData() error = %v, want %v", err, sentinel)
	}
}

func TestFileHashMatchesData(t *testing.T) {
	gt := gabrictest.New(t)
	ctx := gt.Context()

	hash := reconcile.Data("hash", func(context.Context) (string, error) {
		return "abc", nil
	}).Node()

	if !gt.RunNode(ctx, FileHashMatchesData(hash, input.Static("abc"))) {
		t.Error("FileHashMatchesData(abc) = false, want true")
	}
	if gt.RunNode(ctx, FileHashMatchesData(hash, input.Static("def"))) {
		t.Error("FileHashMatchesData(def) = true, want false")
	}

	empty := reconcile.Data("empty", func(context.Context) (string, error) {
		return "", nil
	}).Node()

	if gt.RunNode(ctx, FileHashMatchesData(empty, input.Static(""))) {
		t.Error("FileHashMatchesData(missing) = true, want false")
	}
}

func TestFileContentMatchesData(t *testing.T) {
	gt := gabrictest.New(t)
	ctx := gt.Context()

	content := []byte("hello")
	hash := reconcile.Data("hash", func(context.Context) (string, error) {
		return contentHash(content), nil
	}).Node()

	if !gt.RunNode(ctx, FileContentMatchesData(hash, input.Static(content))) {
		t.Error("FileContentMatchesData(match) = false, want true")
	}
	if gt.RunNode(ctx, FileContentMatchesData(hash, input.Static([]byte("other")))) {
		t.Error("FileContentMatchesData(drift) = true, want false")
	}

	empty := reconcile.Data("empty", func(context.Context) (string, error) {
		return "", nil
	}).Node()

	if gt.RunNode(ctx, FileContentMatchesData(empty, input.Static(content))) {
		t.Error("FileContentMatchesData(missing) = true, want false")
	}
}

func TestContentHash(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", nil, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", []byte("abc"), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentHash(tt.in); got != tt.want {
				t.Errorf("contentHash(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
