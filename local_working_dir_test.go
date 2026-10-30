package gabric

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetLocalWorkingDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}

	if got := GetLocalWorkingDir(context.Background()); got != wd {
		t.Errorf("GetLocalWorkingDir() = %q, want %q", got, wd)
	}

	ctx := NewLocalWorkingDirContext(context.Background(), "/srv/app")

	if got := GetLocalWorkingDir(ctx); got != "/srv/app" {
		t.Errorf("GetLocalWorkingDir() = %q, want %q", got, "/srv/app")
	}
}

func TestResolveLocalPath(t *testing.T) {
	dir := t.TempDir()
	ctx := NewLocalWorkingDirContext(context.Background(), dir)
	abs := filepath.Join(t.TempDir(), "file")

	if got := ResolveLocalPath(ctx, abs); got != abs {
		t.Errorf("ResolveLocalPath(abs) = %q, want %q", got, abs)
	}

	want := filepath.Join(dir, "config")

	if got := ResolveLocalPath(ctx, "config"); got != want {
		t.Errorf("ResolveLocalPath(rel) = %q, want %q", got, want)
	}
}

func TestHasLocalWorkingDir(t *testing.T) {
	if HasLocalWorkingDir(context.Background()) {
		t.Error("HasLocalWorkingDir() = true without a working dir, want false")
	}

	ctx := NewLocalWorkingDirContext(context.Background(), "/srv/app")
	if !HasLocalWorkingDir(ctx) {
		t.Error("HasLocalWorkingDir() = false with a working dir, want true")
	}
}
