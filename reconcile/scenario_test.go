package reconcile

import (
	"context"
	"testing"
)

// TestUfwScenario exercises the full model: two nodes that both Notify the same
// topic, and a reload effect subscribed to it.
func TestUfwScenario(t *testing.T) {
	ufw := NewTopic("ufw")
	reloads := 0

	NewEffect("reload-ufw", []*Topic{ufw}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	ufwInstalled := false
	ufwDockerInstalled := false

	installUfw := NewNode("install-ufw", func(ctx context.Context) (string, error) {
		if !ufwInstalled {
			if err := Do(ctx, func() error {
				ufwInstalled = true
				return nil
			}); err != nil {
				return "", err
			}

			if err := ufw.Notify(ctx); err != nil {
				return "", err
			}
		}

		return "ufw:1.0", nil
	})

	installUfwDocker := NewNode("install-ufw-docker", func(ctx context.Context) (string, error) {
		if !ufwDockerInstalled {
			if err := Do(ctx, func() error {
				ufwDockerInstalled = true
				return nil
			}); err != nil {
				return "", err
			}

			if err := ufw.Notify(ctx); err != nil {
				return "", err
			}
		}

		return "ufw-docker:1.0", nil
	})

	run := func() {
		if err := Run(context.Background(), func(ctx context.Context) error {
			if _, err := installUfw.Value(ctx); err != nil {
				return err
			}

			_, err := installUfwDocker.Value(ctx)

			return err
		}); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	}

	// First run: both nodes change, the effect runs once.
	run()
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}

	// Second run: nothing drifts, no notification, no reload.
	run()
	if reloads != 1 {
		t.Fatalf("reloads after no-op run = %d, want 1", reloads)
	}

	// Force a reload by notifying the topic again.
	if err := Run(context.Background(), func(ctx context.Context) error {
		return ufw.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reloads != 2 {
		t.Fatalf("reloads after force = %d, want 2", reloads)
	}
}

// TestUfwFlushThenNotifyAgain covers a topic that is flushed mid-run and then
// notified again later in the same run: the effect runs once per notification,
// not once per run.
func TestUfwFlushThenNotifyAgain(t *testing.T) {
	ufw := NewTopic("ufw")
	reloads := 0

	NewEffect("reload-ufw", []*Topic{ufw}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	installUfw := NewNode("install-ufw", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, ufw.Notify(ctx)
	})

	installPlugin := NewNode("install-plugin", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, ufw.Notify(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := installUfw.Value(ctx); err != nil {
			return err
		}

		// Flush early: the effect runs now.
		if err := Flush(ctx); err != nil {
			return err
		}

		if reloads != 1 {
			t.Errorf("reloads after early flush = %d, want 1", reloads)
		}

		// Notify the same topic again, later in the same run.
		_, err := installPlugin.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reloads != 2 {
		t.Fatalf("reloads after notifying again = %d, want 2", reloads)
	}
}

// TestIdempotentRerun asserts the invariant: rerunning the graph changes
// nothing once converged.
func TestIdempotentRerun(t *testing.T) {
	ufw := NewTopic("ufw")
	applies := 0
	reloads := 0

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	installed := false

	install := NewNode("install", func(ctx context.Context) (struct{}, error) {
		if installed {
			return struct{}{}, nil
		}

		if err := Do(ctx, func() error {
			applies++
			installed = true
			return nil
		}); err != nil {
			return struct{}{}, err
		}

		if err := ufw.Notify(ctx); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})

	run := func() {
		if err := Run(context.Background(), func(ctx context.Context) error {
			_, err := install.Value(ctx)
			return err
		}); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	}

	run()
	if applies != 1 || reloads != 1 {
		t.Fatalf("first run: applies=%d reloads=%d, want 1 and 1", applies, reloads)
	}

	run()
	if applies != 1 || reloads != 1 {
		t.Fatalf("second run changed state: applies=%d reloads=%d, want 1 and 1", applies, reloads)
	}
}
