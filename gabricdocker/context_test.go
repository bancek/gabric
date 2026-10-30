package gabricdocker

import (
	"errors"
	"testing"
)

func TestContainerContext(t *testing.T) {
	ctx := NewContainerContext(testContext(t), "app")

	container, err := GetContainer(ctx)
	if err != nil {
		t.Fatalf("GetContainer() error: %v", err)
	}
	if container != "app" {
		t.Errorf("GetContainer() = %q, want %q", container, "app")
	}
	if !HasContainer(ctx) {
		t.Error("HasContainer() = false, want true")
	}
}

func TestContainerContextEmpty(t *testing.T) {
	ctx := NewContainerContext(testContext(t), "")

	if _, err := GetContainer(ctx); !errors.Is(err, ErrNoContainer) {
		t.Fatalf("GetContainer() error = %v, want ErrNoContainer", err)
	}
	if HasContainer(ctx) {
		t.Error("HasContainer() = true, want false")
	}
}

func TestContainerContextUnset(t *testing.T) {
	if _, err := GetContainer(testContext(t)); !errors.Is(err, ErrNoContainer) {
		t.Fatalf("GetContainer() error = %v, want ErrNoContainer", err)
	}
	if HasContainer(testContext(t)) {
		t.Error("HasContainer() = true, want false")
	}
}
