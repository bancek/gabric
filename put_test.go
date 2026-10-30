package gabric

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPutOpsContentLastOneWins(t *testing.T) {
	ctx := testContext(t)

	var got putCall

	run := func(_ context.Context, call putCall) error {
		got = call
		return nil
	}

	err := newPut(ctx, run, "/etc/app/config").
		String("first").
		Bytes([]byte("second")).
		Reader(strings.NewReader("third")).
		User("deploy").
		Group("deploy").
		Mode(0o600).
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if got.path != "/etc/app/config" {
		t.Errorf("path = %q, want %q", got.path, "/etc/app/config")
	}

	if got.user != "deploy" || got.group != "deploy" {
		t.Errorf("owner = %q:%q, want deploy:deploy", got.user, got.group)
	}

	if got.mode != 0o600 {
		t.Errorf("mode = %v, want %v", got.mode, 0o600)
	}

	b, err := io.ReadAll(got.reader)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}

	if string(b) != "third" {
		t.Errorf("content = %q, want %q", string(b), "third")
	}
}

func TestPutOpsEnsureParent(t *testing.T) {
	ctx := testContext(t)

	var got putCall

	run := func(_ context.Context, call putCall) error {
		got = call
		return nil
	}

	if err := newPut(ctx, run, "/a/b/c").EnsureParent().String("x").RunErr(); err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if !got.ensureParent {
		t.Error("ensureParent = false, want true")
	}
}

func TestPutOpsBytesNilWritesEmpty(t *testing.T) {
	ctx := testContext(t)

	var got putCall

	run := func(_ context.Context, call putCall) error {
		got = call
		return nil
	}

	if err := newPut(ctx, run, "/p").Bytes(nil).RunErr(); err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if !got.hasContent {
		t.Error("hasContent = false, want true")
	}

	b, _ := io.ReadAll(got.reader)
	if len(b) != 0 {
		t.Errorf("content = %q, want empty", string(b))
	}
}

func TestPutOpsNoContent(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call putCall) error {
		t.Error("run was called without content")
		return nil
	}

	p := newPut(ctx, run, "/p").Mode(0o644)

	if err := p.RunErr(); err == nil {
		t.Error("RunErr() = nil, want error")
	}
}

func TestPutOpsNilReader(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call putCall) error {
		t.Error("run was called with a nil reader")
		return nil
	}

	if err := newPut(ctx, run, "/p").Reader(nil).RunErr(); err == nil {
		t.Error("RunErr(nil reader) = nil, want error")
	}
}

func TestPutOpsRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call putCall) error {
		return errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	newPut(ctx, run, "/p").String("x").Run()
}
