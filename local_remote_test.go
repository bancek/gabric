package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestNewLocalRemoteContext(t *testing.T) {
	ctx := NewLocalRemoteContext(testContext(t))

	if !HasRemoteRunner(ctx) {
		t.Error("HasRemoteRunner() = false, want true")
	}
	if !HasRsyncRsh(ctx) {
		t.Error("HasRsyncRsh() = false, want true")
	}
	if _, err := GetRsyncRsh(ctx); !errors.Is(err, ErrNoRsyncRsh) {
		t.Fatalf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
	}
}

func TestLocalRemoteNoRsyncRsh(t *testing.T) {
	ops := LocalRemote(testContext(t))

	if _, err := GetRsyncRsh(ops.ctx); !errors.Is(err, ErrNoRsyncRsh) {
		t.Fatalf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
	}
}

func TestLocalRemoteOverridesRsyncRsh(t *testing.T) {
	ctx := NewRsyncRshContext(testContext(t), func(context.Context) (RsyncRsh, error) {
		return RsyncRsh{HostSpec: "example.com", Rsh: "custom"}, nil
	})
	ops := LocalRemote(ctx)

	if _, err := GetRsyncRsh(ops.ctx); !errors.Is(err, ErrNoRsyncRsh) {
		t.Fatalf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
	}
}
