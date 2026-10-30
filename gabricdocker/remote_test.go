package gabricdocker

import (
	"errors"
	"slices"
	"testing"

	"github.com/bancek/gabric"
)

func TestRemote(t *testing.T) {
	var reqs []gabric.ExecRequest

	ctx := gabric.NewLocalBackendContext(testContext(t), recordingBackend(&reqs))

	err := Remote(ctx, "app").Exec("echo", "hi").Capture().RunErr()
	if err != nil {
		t.Fatalf("Exec() error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(reqs))
	}
	if want := []string{"docker", "exec", "-i", "app", "/bin/sh", "-c", "/bin/sh -c 'echo hi'"}; !slices.Equal(reqs[0].Cmd, want) {
		t.Errorf("Cmd = %v, want %v", reqs[0].Cmd, want)
	}
}

func TestRemoteEmptyContainer(t *testing.T) {
	err := Remote(testContext(t), "").Exec("true").RunErr()
	if !errors.Is(err, ErrNoContainer) {
		t.Fatalf("Exec() error = %v, want ErrNoContainer", err)
	}
}

func TestTarget(t *testing.T) {
	var reqs []gabric.ExecRequest

	ctx := gabric.NewLocalBackendContext(testContext(t), recordingBackend(&reqs))
	ctx = gabric.NewTargetContext(ctx, Target("app"))

	err := gabric.TargetOps(ctx).Exec("echo", "hi").Capture().RunErr()
	if err != nil {
		t.Fatalf("Exec() error: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(reqs))
	}
	if reqs[0].Cmd[3] != "app" {
		t.Errorf("container = %q, want app", reqs[0].Cmd[3])
	}
}
