//go:build unix

package gabric

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalWithWorkingDirErr(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	sentinel := errors.New("boom")

	err := Local(ctx).WithWorkingDirErr(dir, func(ctx context.Context) error {
		if got := GetLocalWorkingDir(ctx); got != dir {
			t.Errorf("working dir = %q, want %q", got, dir)
		}

		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("WithWorkingDirErr() error = %v, want %v", err, sentinel)
	}
}

func TestLocalWithWorkingDir(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	var out string

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		out = Local(ctx).Exec("pwd").CaptureStdout().Run().StdoutStr()
	})

	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error: %v", dir, err)
	}

	gotDir, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error: %v", out, err)
	}

	if gotDir != wantDir {
		t.Errorf("pwd = %q, want %q", gotDir, wantDir)
	}
}
