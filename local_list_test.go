package gabric

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLocalList(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	entries := Local(ctx).List(dir)

	want := []ListEntry{
		{Name: ".hidden"},
		{Name: "b.txt"},
		{Name: "sub", IsDir: true},
	}

	if !slices.Equal(entries, want) {
		t.Errorf("List() = %+v, want %+v", entries, want)
	}
}

func TestLocalListErr(t *testing.T) {
	ctx := testContext(t)

	if _, err := Local(ctx).ListErr(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("ListErr(missing) error = nil, want error")
	}
}

func TestLocalListPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("List(missing) did not panic")
		}
	}()

	ctx := testContext(t)
	Local(ctx).List(filepath.Join(t.TempDir(), "missing"))
}
