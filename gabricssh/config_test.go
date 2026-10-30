package gabricssh

import (
	"context"
	"testing"

	"github.com/bancek/gabric/sshremote"
)

func TestConfigContext(t *testing.T) {
	config := sshremote.NewConfig().WithAgentSocket("agent.sock")

	ctx := NewConfigContext(context.Background(), config)

	if GetConfig(ctx) != config {
		t.Error("GetConfig() did not return the config from context")
	}
}

func TestGetConfigPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("GetConfig() did not panic")
		}
	}()

	GetConfig(context.Background())
}

func TestResolveHost(t *testing.T) {
	ctx := NewConfigContext(context.Background(), sshremote.NewConfig())

	host, err := ResolveHost(ctx, "deploy@example.com")
	if err != nil {
		t.Fatalf("ResolveHost() error: %v", err)
	}

	if host.Hostname() != "example.com" {
		t.Errorf("Hostname = %q, want %q", host.Hostname(), "example.com")
	}

	if host.User() != "deploy" {
		t.Errorf("User = %q, want %q", host.User(), "deploy")
	}
}
