package gabric

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestRemoteExecUsesBackend(t *testing.T) {
	var got ExecRequest

	backend := RemoteBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req
			_, err := io.WriteString(req.Stdout, "out")
			return err
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	out := Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()

	if out != "out" {
		t.Errorf("stdout = %q, want %q", out, "out")
	}

	if !slices.Equal(got.Cmd, []string{"echo", "hi"}) {
		t.Errorf("Cmd = %v, want [echo hi]", got.Cmd)
	}
	if got.Dir != "/srv/app" {
		t.Errorf("Dir = %q, want %q", got.Dir, "/srv/app")
	}
}

func TestRemotePutUsesBackend(t *testing.T) {
	var (
		got     PutRequest
		content []byte
	)

	backend := RemoteBackendFunc{
		PutFunc: func(_ context.Context, req PutRequest) error {
			got = req

			var err error
			content, err = io.ReadAll(req.Content)

			return err
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)

	err := Remote(ctx).Put("/srv/x").
		String("hello").
		Mode(0o600).
		User("root").
		Group("root").
		EnsureParent().
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got.Path != "/srv/x" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/x")
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

func TestRemotePutResolvesWorkingDir(t *testing.T) {
	var got PutRequest

	backend := RemoteBackendFunc{
		PutFunc: func(_ context.Context, req PutRequest) error {
			got = req
			return nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	if err := Remote(ctx).Put("config").String("x").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got.Path != "/srv/app/config" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/config")
	}
}

func TestRemoteGetUsesBackend(t *testing.T) {
	var got GetRequest

	backend := RemoteBackendFunc{
		GetFunc: func(_ context.Context, req GetRequest, f func(io.Reader) error) error {
			got = req
			return f(strings.NewReader("out"))
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)

	out := Remote(ctx).Get("/srv/x").String()

	if out != "out" {
		t.Errorf("String() = %q, want %q", out, "out")
	}

	if got.Path != "/srv/x" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/x")
	}
}

func TestRemoteGetResolvesWorkingDir(t *testing.T) {
	var got GetRequest

	backend := RemoteBackendFunc{
		GetFunc: func(_ context.Context, req GetRequest, f func(io.Reader) error) error {
			got = req
			return f(strings.NewReader(""))
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	if got := Remote(ctx).Get("config").String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}

	if got.Path != "/srv/app/config" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/config")
	}
}

func TestRemoteExistsUsesBackend(t *testing.T) {
	var got ExistsRequest

	backend := RemoteBackendFunc{
		ExistsFunc: func(_ context.Context, req ExistsRequest) (bool, error) {
			got = req
			return true, nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	if !Remote(ctx).Exists("config").File().Run() {
		t.Error("Exists() = false, want true")
	}

	if got.Path != "/srv/app/config" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/config")
	}
	if got.Kind != FileKindFile {
		t.Errorf("Kind = %v, want %v", got.Kind, FileKindFile)
	}
}

func TestRemoteStatUsesBackend(t *testing.T) {
	var got StatRequest

	backend := RemoteBackendFunc{
		StatFunc: func(_ context.Context, req StatRequest) (FileStat, error) {
			got = req
			return FileStat{Mode: 0o640, User: "root", Group: "root"}, nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	stat := Remote(ctx).Stat("config").Run()
	if stat.Mode.Perm() != 0o640 {
		t.Errorf("Mode.Perm() = %v, want %v", stat.Mode.Perm(), os.FileMode(0o640))
	}

	if got.Path != "/srv/app/config" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/config")
	}
}

func TestRemoteMkdirUsesBackend(t *testing.T) {
	var got MkdirRequest

	backend := RemoteBackendFunc{
		MkdirFunc: func(_ context.Context, req MkdirRequest) error {
			got = req
			return nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	if err := Remote(ctx).Mkdir("cache").
		Recursive().
		Mode(0o750).
		User("root").
		Group("root").
		RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got.Path != "/srv/app/cache" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/cache")
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

func TestRemoteRemoveUsesBackend(t *testing.T) {
	var got RemoveRequest

	backend := RemoteBackendFunc{
		RemoveFunc: func(_ context.Context, req RemoveRequest) error {
			got = req
			return nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	if err := Remote(ctx).Remove("cache").
		Recursive().
		Force().
		RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if got.Path != "/srv/app/cache" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/cache")
	}
	if !got.Recursive {
		t.Error("Recursive = false, want true")
	}
	if !got.Force {
		t.Error("Force = false, want true")
	}
}

func TestRemoteRsyncUsesBackend(t *testing.T) {
	var got RsyncRequest

	backend := RemoteBackendFunc{
		RsyncFunc: func(_ context.Context, req RsyncRequest, stdout, stderr io.Writer) error {
			got = req

			if _, err := io.WriteString(stdout, "out"); err != nil {
				return err
			}

			_, err := io.WriteString(stderr, "err")

			return err
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteSudoContext(ctx, &Sudo{})

	out := Remote(ctx).Rsync().
		Local("./app").
		Remote("/home/app").
		Upload().
		Delete().
		Exclude("**/.env").
		AddFlags("--info").
		Capture().
		Run()

	if out.StdoutStr() != "out" {
		t.Errorf("StdoutStr() = %q, want %q", out.StdoutStr(), "out")
	}
	if out.StderrStr() != "err" {
		t.Errorf("StderrStr() = %q, want %q", out.StderrStr(), "err")
	}

	want := RsyncRequest{
		LocalPath:  "./app",
		RemotePath: "/home/app",
		Direction:  RsyncDirectionUpload,
		Delete:     true,
		Excludes:   []string{"**/.env"},
		Flags:      []string{"-pthrvz", "--info"},
		Sudo:       &Sudo{},
	}

	if !got.Equal(want) {
		t.Errorf("RsyncRequest = %+v, want %+v", got, want)
	}
}

func TestGetRemoteBackendDefault(t *testing.T) {
	if GetRemoteBackend(context.Background()) == nil {
		t.Error("GetRemoteBackend() = nil, want default backend")
	}
}

func TestHasRemoteBackend(t *testing.T) {
	if HasRemoteBackend(context.Background()) {
		t.Error("HasRemoteBackend() = true without a backend, want false")
	}

	ctx := NewRemoteBackendContext(context.Background(), RemoteBackendFunc{})
	if !HasRemoteBackend(ctx) {
		t.Error("HasRemoteBackend() = false with a backend, want true")
	}
}

func TestRemoteBackendFuncUnsupported(t *testing.T) {
	backend := RemoteBackendFunc{}

	if err := backend.Exec(context.Background(), ExecRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Exec() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if err := backend.Put(context.Background(), PutRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Put() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if err := backend.Get(context.Background(), GetRequest{}, nil); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Get() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if _, err := backend.Exists(context.Background(), ExistsRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Exists() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if _, err := backend.Stat(context.Background(), StatRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Stat() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if err := backend.Mkdir(context.Background(), MkdirRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Mkdir() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if err := backend.Remove(context.Background(), RemoveRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Remove() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if _, err := backend.List(context.Background(), ListRequest{}); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("List() error = %v, want ErrRemoteBackendUnsupported", err)
	}

	if err := backend.Rsync(context.Background(), RsyncRequest{}, nil, nil); !errors.Is(err, ErrRemoteBackendUnsupported) {
		t.Errorf("Rsync() error = %v, want ErrRemoteBackendUnsupported", err)
	}
}

func TestRemoteListUsesBackend(t *testing.T) {
	var got ListRequest

	backend := RemoteBackendFunc{
		ListFunc: func(_ context.Context, req ListRequest) ([]ListEntry, error) {
			got = req

			return []ListEntry{{Name: "b"}, {Name: "a", IsDir: true}}, nil
		},
	}

	ctx := NewRemoteBackendContext(testContext(t), backend)
	ctx = NewRemoteWorkingDirContext(ctx, "/srv/app")

	entries := Remote(ctx).List("sub")

	if got.Path != "/srv/app/sub" {
		t.Errorf("Path = %q, want %q", got.Path, "/srv/app/sub")
	}

	want := []ListEntry{{Name: "a", IsDir: true}, {Name: "b"}}
	if !slices.Equal(entries, want) {
		t.Errorf("List() = %+v, want %+v", entries, want)
	}
}
