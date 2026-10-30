package gabricdocker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
)

func TestInstallFlags(t *testing.T) {
	g := gabric.New()
	Install(g)

	g.Stderr = &bytes.Buffer{}

	if err := g.ParseFlags([]string{"-docker-container", "app"}); err != nil {
		t.Fatalf("ParseFlags() error: %v", err)
	}

	var reqs []gabric.ExecRequest

	base := gabric.NewLocalBackendContext(testContext(t), recordingBackend(&reqs))

	run := middleware.Apply(g.BuiltinMiddlewares(), func(ctx context.Context) error {
		container, err := GetContainer(ctx)
		if err != nil {
			return err
		}
		if container != "app" {
			t.Errorf("GetContainer() = %q, want %q", container, "app")
		}

		if !gabric.HasRemoteRunner(ctx) {
			t.Error("Docker runner was not installed")
		}
		if !gabric.HasRsyncRsh(ctx) {
			t.Error("Docker rsync policy was not installed")
		}
		if _, err := gabric.GetRsyncRsh(ctx); !errors.Is(err, gabric.ErrNoRsyncRsh) {
			t.Errorf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
		}

		return gabric.Remote(ctx).Exec("echo", "hi").Capture().RunErr()
	})

	if err := run(base); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(reqs))
	}
	if want := []string{"docker", "exec", "-i", "app", "/bin/sh", "-c", "/bin/sh -c 'echo hi'"}; !slices.Equal(reqs[0].Cmd, want) {
		t.Errorf("Cmd = %v, want %v", reqs[0].Cmd, want)
	}
}

func TestInstallDoublePanics(t *testing.T) {
	g := gabric.New()
	Install(g)

	defer func() {
		if recover() == nil {
			t.Fatal("Install() twice did not panic")
		}
	}()

	Install(g)
}

func TestInstallEmptyContainer(t *testing.T) {
	g := gabric.New()
	Install(g)

	g.Stderr = &bytes.Buffer{}

	if err := g.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags() error: %v", err)
	}

	run := middleware.Apply(g.BuiltinMiddlewares(), func(ctx context.Context) error {
		if HasContainer(ctx) {
			t.Error("HasContainer() = true, want false")
		}
		if !gabric.HasRemoteRunner(ctx) {
			t.Error("Docker runner was not installed for an empty container")
		}

		err := gabric.Remote(ctx).Exec("true").RunErr()
		if !errors.Is(err, ErrNoContainer) {
			t.Errorf("Exec() error = %v, want ErrNoContainer", err)
		}

		return nil
	})

	if err := run(testContext(t)); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestWithDocker(t *testing.T) {
	if err := WithDocker(testContext(t), func(ctx context.Context) error {
		if !gabric.HasRemoteRunner(ctx) {
			t.Error("HasRemoteRunner() = false, want true")
		}
		if !gabric.HasRsyncRsh(ctx) {
			t.Error("HasRsyncRsh() = false, want true")
		}

		return nil
	}); err != nil {
		t.Fatalf("WithDocker() error: %v", err)
	}
}

func TestWithContainerOnlySetsContainer(t *testing.T) {
	if err := WithContainer("app")(testContext(t), func(ctx context.Context) error {
		container, err := GetContainer(ctx)
		if err != nil {
			return err
		}
		if container != "app" {
			t.Errorf("GetContainer() = %q, want %q", container, "app")
		}
		if gabric.HasRemoteRunner(ctx) {
			t.Error("HasRemoteRunner() = true, want false")
		}

		return nil
	}); err != nil {
		t.Fatalf("WithContainer() error: %v", err)
	}
}

func TestWithDockerKeepsExistingRunner(t *testing.T) {
	var runnerCalled bool

	runner := func(context.Context, string, io.Reader, io.Writer, io.Writer) error {
		runnerCalled = true
		return nil
	}
	wantRsh := gabric.RsyncRsh{HostSpec: "custom", Rsh: "custom-rsh"}

	ctx := testContext(t)
	ctx = gabric.NewRemoteRunnerContext(ctx, runner)
	ctx = gabric.NewRsyncRshContext(ctx, func(context.Context) (gabric.RsyncRsh, error) {
		return wantRsh, nil
	})

	if err := WithDocker(ctx, func(ctx context.Context) error {
		return WithContainer("app")(ctx, func(ctx context.Context) error {
			container, err := GetContainer(ctx)
			if err != nil {
				return err
			}
			if container != "app" {
				t.Errorf("GetContainer() = %q, want %q", container, "app")
			}
			if !gabric.HasRemoteRunner(ctx) {
				t.Error("HasRemoteRunner() = false, want true")
			}

			if err := gabric.GetRemoteRunner(ctx)(ctx, "", nil, nil, nil); err != nil {
				return err
			}
			if !runnerCalled {
				t.Error("existing runner was replaced")
			}

			rsh, err := gabric.GetRsyncRsh(ctx)
			if err != nil {
				return err
			}
			if rsh != wantRsh {
				t.Errorf("GetRsyncRsh() = %#v, want %#v", rsh, wantRsh)
			}

			return nil
		})
	}); err != nil {
		t.Fatalf("WithDocker() error: %v", err)
	}
}
