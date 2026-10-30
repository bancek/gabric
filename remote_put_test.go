package gabric

import "testing"

func TestRemotePut(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	path := "/srv/app/file.txt"

	if err := Remote(ctx).Put(path).String("hello").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	b, err := core.FS().ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}

	asserts.AssertExecuted("dd", "bs=1048576", "of="+path)
}

func TestRemotePutEnsureParent(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	path := "/srv/app/a/b/file.txt"

	if err := Remote(ctx).Put(path).EnsureParent().String("hello").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	b, err := core.FS().ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "hello" {
		t.Errorf("content = %q, want %q", string(b), "hello")
	}

	asserts.AssertExecuted("mkdir", "-p", "/srv/app/a/b")
	asserts.AssertExecuted("dd", "bs=1048576", "of="+path)
}

func TestRemotePutMissingDir(t *testing.T) {
	_, _, ctx := localRemoteContext(t)

	path := "/srv/missing/file.txt"

	if err := Remote(ctx).Put(path).String("x").RunErr(); err == nil {
		t.Error("RunErr(missing dir) = nil, want error")
	}
}

func TestRemotePutOwnerMode(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	path := "/srv/app/config"

	err := Remote(ctx).Put(path).
		Bytes([]byte("secret")).
		User("root").
		Group("root").
		Mode(0o600).
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	b, err := core.FS().ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if string(b) != "secret" {
		t.Errorf("content = %q, want %q", string(b), "secret")
	}

	info, err := core.FS().Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want %v", got, 0o600)
	}

	asserts.AssertExecuted("dd", "bs=1048576", "of="+path)
	asserts.AssertExecuted("chown", "root:root", path)
	asserts.AssertExecuted("chmod", "600", path)
}

func TestRemotePutNoContent(t *testing.T) {
	ctx := testContext(t)

	if err := Remote(ctx).Put("/tmp/whatever").RunErr(); err == nil {
		t.Error("RunErr(no content) = nil, want error")
	}
}
