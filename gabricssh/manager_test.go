package gabricssh

import (
	"context"
	"testing"

	"github.com/bancek/gabric/sshremote"
)

func TestGetManager(t *testing.T) {
	manager := sshremote.NewRemoteManager()
	defer manager.Close()

	ctx := NewManagerContext(context.Background(), manager)

	if GetManager(ctx) != manager {
		t.Error("GetManager() did not return the manager from context")
	}
}

func TestGetManagerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("GetManager() did not panic")
		}
	}()

	GetManager(context.Background())
}

func TestGetRemote(t *testing.T) {
	manager := sshremote.NewRemoteManager()
	defer manager.Close()

	config := sshremote.NewConfig().WithAgentSocket("agent.sock")
	host := sshremote.NewHost("example.com")

	ctx := NewConfigContext(context.Background(), config)
	ctx = NewManagerContext(ctx, manager)
	ctx = NewHostContext(ctx, host)

	r := GetRemote(ctx)
	if r == nil {
		t.Fatal("GetRemote() = nil, want remote")
	}

	if GetRemote(ctx) != r {
		t.Error("GetRemote() did not return the cached remote")
	}
}

func TestGetRemotePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("GetRemote() did not panic")
		}
	}()

	GetRemote(context.Background())
}

func TestWithManager(t *testing.T) {
	ctx := testContext(t)

	var got context.Context

	if err := WithManager(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("WithManager() error: %v", err)
	}

	if !HasManager(got) {
		t.Error("manager not added")
	}
}

func TestWithManagerKeepsExisting(t *testing.T) {
	manager := sshremote.NewRemoteManager()
	defer manager.Close()

	ctx := NewManagerContext(testContext(t), manager)

	var got context.Context

	if err := WithManager(ctx, func(ctx context.Context) error {
		got = ctx
		return nil
	}); err != nil {
		t.Fatalf("WithManager() error: %v", err)
	}

	if GetManager(got) != manager {
		t.Error("existing manager was replaced")
	}
}
