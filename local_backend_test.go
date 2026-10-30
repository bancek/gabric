package gabric

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLocalExecUsesBackend(t *testing.T) {
	var (
		got     ExecRequest
		content []byte
	)

	backend := LocalBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req

			var err error
			content, err = io.ReadAll(req.Stdin)
			if err != nil {
				return err
			}

			_, err = io.WriteString(req.Stdout, "out")
			if err != nil {
				return err
			}

			_, err = io.WriteString(req.Stderr, "err")

			return err
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	e := Local(ctx).Exec("cat").Stdin(strings.NewReader("input")).Capture().Run()

	if out := e.StdoutStr(); out != "out" {
		t.Errorf("StdoutStr() = %q, want %q", out, "out")
	}
	if stderr := e.StderrStr(); stderr != "err" {
		t.Errorf("StderrStr() = %q, want %q", stderr, "err")
	}
	if !slices.Equal(got.Cmd, []string{"cat"}) {
		t.Errorf("Cmd = %v, want [cat]", got.Cmd)
	}
	if want := filepath.Join("/srv/app", "."); got.Dir != want {
		t.Errorf("Dir = %q, want %q", got.Dir, want)
	}
	if string(content) != "input" {
		t.Errorf("Stdin = %q, want %q", content, "input")
	}
}

func TestLocalExecPassesExtraFilesAndAfterStart(t *testing.T) {
	extra, err := os.CreateTemp(t.TempDir(), "extra")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	defer extra.Close()

	var (
		got      ExecRequest
		afterRun bool
	)

	backend := LocalBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req

			if req.AfterStart != nil {
				req.AfterStart()
			}

			return nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)

	e := Local(ctx).Exec("rsync")
	e.extraFiles = []*os.File{extra}
	e.afterStart = func() { afterRun = true }

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if len(got.ExtraFiles) != 1 || got.ExtraFiles[0] != extra {
		t.Errorf("ExtraFiles = %v, want the configured file", got.ExtraFiles)
	}
	if !afterRun {
		t.Error("AfterStart was not passed to the backend")
	}
}

func TestLocalPutUsesBackend(t *testing.T) {
	var (
		got     PutRequest
		content []byte
	)

	backend := LocalBackendFunc{
		PutFunc: func(_ context.Context, req PutRequest) error {
			got = req

			var err error
			content, err = io.ReadAll(req.Content)

			return err
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	err := Local(ctx).Put("config").
		String("hello").
		Mode(0o600).
		User("root").
		Group("root").
		EnsureParent().
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	wantPath := filepath.Join("/srv/app", "config")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
	if string(content) != "hello" {
		t.Errorf("Content = %q, want %q", content, "hello")
	}
	if got.Mode != os.FileMode(0o600) {
		t.Errorf("Mode = %v, want %v", got.Mode, os.FileMode(0o600))
	}
	if got.User != "root" || got.Group != "root" {
		t.Errorf("owner = %q:%q, want root:root", got.User, got.Group)
	}
	if !got.EnsureParent {
		t.Error("EnsureParent = false, want true")
	}
}

func TestLocalGetUsesBackend(t *testing.T) {
	var got GetRequest

	backend := LocalBackendFunc{
		GetFunc: func(_ context.Context, req GetRequest, f func(io.Reader) error) error {
			got = req
			return f(strings.NewReader("out"))
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	out := Local(ctx).Get("config").String()

	if out != "out" {
		t.Errorf("String() = %q, want %q", out, "out")
	}

	wantPath := filepath.Join("/srv/app", "config")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
}

func TestLocalExistsUsesBackend(t *testing.T) {
	var got ExistsRequest

	backend := LocalBackendFunc{
		ExistsFunc: func(_ context.Context, req ExistsRequest) (bool, error) {
			got = req
			return true, nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	if !Local(ctx).Exists("config").File().Run() {
		t.Error("Exists() = false, want true")
	}

	wantPath := filepath.Join("/srv/app", "config")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
	if got.Kind != FileKindFile {
		t.Errorf("Kind = %v, want %v", got.Kind, FileKindFile)
	}
}

func TestLocalStatUsesBackend(t *testing.T) {
	var got StatRequest

	backend := LocalBackendFunc{
		StatFunc: func(_ context.Context, req StatRequest) (FileStat, error) {
			got = req
			return FileStat{Mode: 0o640, User: "root", Group: "root"}, nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	stat := Local(ctx).Stat("config").Run()
	if stat.Mode.Perm() != 0o640 {
		t.Errorf("Mode.Perm() = %v, want %v", stat.Mode.Perm(), os.FileMode(0o640))
	}

	wantPath := filepath.Join("/srv/app", "config")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
}

func TestLocalMkdirUsesBackend(t *testing.T) {
	var got MkdirRequest

	backend := LocalBackendFunc{
		MkdirFunc: func(_ context.Context, req MkdirRequest) error {
			got = req
			return nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	if err := Local(ctx).Mkdir("cache").
		Recursive().
		Mode(0o750).
		User("root").
		Group("root").
		RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	wantPath := filepath.Join("/srv/app", "cache")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
	if !got.Recursive {
		t.Error("Recursive = false, want true")
	}
	if got.Mode != os.FileMode(0o750) {
		t.Errorf("Mode = %v, want %v", got.Mode, os.FileMode(0o750))
	}
	if got.User != "root" || got.Group != "root" {
		t.Errorf("owner = %q:%q, want root:root", got.User, got.Group)
	}
}

func TestLocalRemoveUsesBackend(t *testing.T) {
	var got RemoveRequest

	backend := LocalBackendFunc{
		RemoveFunc: func(_ context.Context, req RemoveRequest) error {
			got = req
			return nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	if err := Local(ctx).Remove("cache").
		Recursive().
		Force().
		RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	wantPath := filepath.Join("/srv/app", "cache")
	if got.Path != wantPath {
		t.Errorf("Path = %q, want %q", got.Path, wantPath)
	}
	if !got.Recursive {
		t.Error("Recursive = false, want true")
	}
	if !got.Force {
		t.Error("Force = false, want true")
	}
}

func TestGetLocalBackendDefault(t *testing.T) {
	if GetLocalBackend(context.Background()) == nil {
		t.Error("GetLocalBackend() = nil, want default backend")
	}
}

func TestHasLocalBackend(t *testing.T) {
	if HasLocalBackend(context.Background()) {
		t.Error("HasLocalBackend() = true without a backend, want false")
	}

	ctx := NewLocalBackendContext(context.Background(), LocalBackendFunc{})
	if !HasLocalBackend(ctx) {
		t.Error("HasLocalBackend() = false with a backend, want true")
	}
}

func TestLocalBackendFuncUnsupported(t *testing.T) {
	backend := LocalBackendFunc{}

	if err := backend.Exec(context.Background(), ExecRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Exec() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if err := backend.Put(context.Background(), PutRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Put() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if err := backend.Get(context.Background(), GetRequest{}, nil); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Get() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if _, err := backend.Exists(context.Background(), ExistsRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Exists() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if _, err := backend.Stat(context.Background(), StatRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Stat() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if err := backend.Mkdir(context.Background(), MkdirRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Mkdir() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if err := backend.Remove(context.Background(), RemoveRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("Remove() error = %v, want ErrLocalBackendUnsupported", err)
	}

	if _, err := backend.List(context.Background(), ListRequest{}); !errors.Is(err, ErrLocalBackendUnsupported) {
		t.Errorf("List() error = %v, want ErrLocalBackendUnsupported", err)
	}
}

func TestLocalListUsesBackend(t *testing.T) {
	var got ListRequest

	backend := LocalBackendFunc{
		ListFunc: func(_ context.Context, req ListRequest) ([]ListEntry, error) {
			got = req

			return []ListEntry{{Name: "b"}, {Name: "a", IsDir: true}}, nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewLocalWorkingDirContext(ctx, "/srv/app")

	entries := Local(ctx).List("sub")

	if want := filepath.Join("/srv/app", "sub"); got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}

	want := []ListEntry{{Name: "a", IsDir: true}, {Name: "b"}}
	if !slices.Equal(entries, want) {
		t.Errorf("List() = %+v, want %+v", entries, want)
	}
}
