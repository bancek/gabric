package gabrictest

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/logger"
)

func TestCommand(t *testing.T) {
	g := New(t)

	got := g.Command("echo", "hi")
	want := Command{Name: "echo", Args: []string{"hi"}, Dir: "/"}

	if !got.Equal(want) {
		t.Errorf("Command() = %v, want %v", got, want)
	}
}

func TestContextDefaults(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	if !logger.HasLogger(ctx) {
		t.Error("Context() has no logger")
	}
	if got := gabric.GetLocalWorkingDir(ctx); got != "/" {
		t.Errorf("local working dir = %q, want %q", got, "/")
	}
	if got := gabric.GetRemoteWorkingDir(ctx); got != "/" {
		t.Errorf("remote working dir = %q, want %q", got, "/")
	}
	if !gabric.HasLocalBackend(ctx) {
		t.Error("Context() has no local backend")
	}
	if !gabric.HasRemoteBackend(ctx) {
		t.Error("Context() has no remote backend")
	}
}

func TestWithContextPreservesExisting(t *testing.T) {
	g := New(t)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	var (
		localPut  bool
		remotePut bool
	)

	localBackend := gabric.LocalBackendFunc{
		PutFunc: func(context.Context, gabric.PutRequest) error {
			localPut = true
			return nil
		},
	}
	remoteBackend := gabric.RemoteBackendFunc{
		PutFunc: func(context.Context, gabric.PutRequest) error {
			remotePut = true
			return nil
		},
	}

	ctx := context.Background()
	ctx = logger.NewLoggerContext(ctx, log)
	ctx = gabric.NewLocalWorkingDirContext(ctx, "/local")
	ctx = gabric.NewRemoteWorkingDirContext(ctx, "/remote")
	ctx = gabric.NewLocalBackendContext(ctx, localBackend)
	ctx = gabric.NewRemoteBackendContext(ctx, remoteBackend)

	got := g.WithContext(ctx)

	if gabric.Logger(got) != log {
		t.Error("WithContext() replaced the logger")
	}
	if got := gabric.GetLocalWorkingDir(got); got != "/local" {
		t.Errorf("local working dir = %q, want %q", got, "/local")
	}
	if got := gabric.GetRemoteWorkingDir(got); got != "/remote" {
		t.Errorf("remote working dir = %q, want %q", got, "/remote")
	}

	if err := gabric.Local(got).Put("/x").String("x").RunErr(); err != nil {
		t.Fatalf("local Put() error: %v", err)
	}
	if !localPut {
		t.Error("WithContext() replaced the local backend")
	}

	if err := gabric.Remote(got).Put("/x").String("x").RunErr(); err != nil {
		t.Fatalf("remote Put() error: %v", err)
	}
	if !remotePut {
		t.Error("WithContext() replaced the remote backend")
	}
}

func TestSidesAreIsolated(t *testing.T) {
	g := New(t)

	g.Remote().WriteFile("/srv/remote", []byte("remote"), 0o644)

	if _, err := g.Local().ReadFile("/srv/remote"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("local ReadFile() error = %v, want ErrNotExist", err)
	}

	g.Local().WriteFile("/srv/local", []byte("local"), 0o644)

	if _, err := g.Remote().ReadFile("/srv/local"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("remote ReadFile() error = %v, want ErrNotExist", err)
	}
}

func TestContextRoutesToBothBackends(t *testing.T) {
	g := New(t)
	ctx := g.Context()

	if err := gabric.Remote(ctx).Put("/srv/remote").EnsureParent().String("remote").RunErr(); err != nil {
		t.Fatalf("remote Put() error: %v", err)
	}
	if err := gabric.Local(ctx).Put("/srv/local").EnsureParent().String("local").RunErr(); err != nil {
		t.Fatalf("local Put() error: %v", err)
	}

	if got, err := g.Remote().ReadFile("/srv/remote"); err != nil || string(got) != "remote" {
		t.Errorf("remote ReadFile() = %q, %v; want %q, nil", got, err, "remote")
	}
	if got, err := g.Local().ReadFile("/srv/local"); err != nil || string(got) != "local" {
		t.Errorf("local ReadFile() = %q, %v; want %q, nil", got, err, "local")
	}
}

func TestWithRemoteExecError(t *testing.T) {
	sentinel := errors.New("transport")
	g := New(t)
	ctx := g.WithRemoteExecError(sentinel)

	if err := gabric.Remote(ctx).Exec("true").RunErr(); !errors.Is(err, sentinel) {
		t.Errorf("remote Exec() error = %v, want %v", err, sentinel)
	}
}
