package gabricssh

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
)

func TestInstallFlags(t *testing.T) {
	g := gabric.New()
	Install(g)

	g.Stderr = &bytes.Buffer{}

	err := g.ParseFlags([]string{
		"-ssh-host", "deploy@example.com:2222",
		"-ssh-identity", "key1",
		"-ssh-identity", "key2",
		"-ssh-identity", "key3",
	})
	if err != nil {
		t.Fatalf("ParseFlags() error: %v", err)
	}

	run := middleware.Apply(g.BuiltinMiddlewares(), func(ctx context.Context) error {
		host := GetHost(ctx)
		if got, want := host.Hostname(), "example.com"; got != want {
			t.Errorf("Hostname = %q, want %q", got, want)
		}
		if got, want := host.Port(), 2222; got != want {
			t.Errorf("Port = %d, want %d", got, want)
		}
		if got, want := host.User(), "deploy"; got != want {
			t.Errorf("User = %q, want %q", got, want)
		}

		keys := GetConfig(ctx).PrivateKeysPaths()
		if !slices.Contains(keys, "key1") || !slices.Contains(keys, "key2") || !slices.Contains(keys, "key3") {
			t.Errorf("PrivateKeysPaths = %v, want key1, key2 and key3", keys)
		}

		if !gabric.HasRemoteRunner(ctx) {
			t.Error("SSH runner was not installed")
		}
		if !gabric.HasRsyncRsh(ctx) {
			t.Error("SSH rsync resolver was not installed")
		}
		if !HasManager(ctx) {
			t.Error("SSH manager was not installed")
		}

		return nil
	})

	if err := run(testContext(t)); err != nil {
		t.Fatalf("run error: %v", err)
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

func TestInstallDoesNotRegisterTasks(t *testing.T) {
	g := gabric.New()
	Install(g)

	if _, err := g.ParseTasks([]string{"ssh"}); err == nil {
		t.Error("ParseTasks(ssh) = nil error, want unknown task")
	}
}
