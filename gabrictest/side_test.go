package gabrictest

import (
	"io/fs"
	"testing"
)

func TestSideFileHelpers(t *testing.T) {
	g := New(t)

	g.Local().MkdirAll("/srv/app", 0o755)
	g.Local().WriteFile("/srv/app/config", []byte("hello"), 0o640)

	content, err := g.Local().ReadFile("/srv/app/config")
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("ReadFile() = %q, want %q", content, "hello")
	}

	info, err := g.Local().Stat("/srv/app/config")
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o640); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}

	entries, err := g.Local().ReadDir("/srv/app")
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config" {
		t.Errorf("ReadDir() = %v, want [config]", entries)
	}
}
