package gabric

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGetOpsTerminals(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, path string, f func(io.Reader) error) error {
		if path != "/etc/app/config" {
			t.Errorf("path = %q, want %q", path, "/etc/app/config")
		}

		return f(strings.NewReader("hello"))
	}

	if got := newGet(ctx, run, "/etc/app/config").Bytes(); string(got) != "hello" {
		t.Errorf("Bytes() = %q, want %q", string(got), "hello")
	}

	if got := newGet(ctx, run, "/etc/app/config").String(); got != "hello" {
		t.Errorf("String() = %q, want %q", got, "hello")
	}

	var b strings.Builder

	newGet(ctx, run, "/etc/app/config").Copy(&b)

	if b.String() != "hello" {
		t.Errorf("Copy() = %q, want %q", b.String(), "hello")
	}

	var got string

	newGet(ctx, run, "/etc/app/config").WithReader(func(r io.Reader) error {
		data, err := io.ReadAll(r)
		got = string(data)

		return err
	})

	if got != "hello" {
		t.Errorf("WithReader() = %q, want %q", got, "hello")
	}
}

func TestGetOpsErrors(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")

	run := func(_ context.Context, path string, f func(io.Reader) error) error {
		return boom
	}

	if _, err := newGet(ctx, run, "/p").BytesErr(); !errors.Is(err, boom) {
		t.Errorf("BytesErr() = %v, want %v", err, boom)
	}

	if _, err := newGet(ctx, run, "/p").StringErr(); !errors.Is(err, boom) {
		t.Errorf("StringErr() = %v, want %v", err, boom)
	}

	if err := newGet(ctx, run, "/p").CopyErr(io.Discard); !errors.Is(err, boom) {
		t.Errorf("CopyErr() = %v, want %v", err, boom)
	}

	if err := newGet(ctx, run, "/p").WithReaderErr(func(io.Reader) error { return nil }); !errors.Is(err, boom) {
		t.Errorf("WithReaderErr() = %v, want %v", err, boom)
	}
}

func TestGetOpsCallbackError(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("callback")

	run := func(_ context.Context, path string, f func(io.Reader) error) error {
		return f(strings.NewReader("hello"))
	}

	if err := newGet(ctx, run, "/p").WithReaderErr(func(io.Reader) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("WithReaderErr() = %v, want %v", err, boom)
	}
}

func TestGetOpsPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, path string, f func(io.Reader) error) error {
		return errors.New("boom")
	}

	tests := []struct {
		name string
		call func()
	}{
		{"Bytes", func() { _ = newGet(ctx, run, "/p").Bytes() }},
		{"String", func() { _ = newGet(ctx, run, "/p").String() }},
		{"Copy", func() { newGet(ctx, run, "/p").Copy(io.Discard) }},
		{"WithReader", func() { newGet(ctx, run, "/p").WithReader(func(io.Reader) error { return nil }) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s() did not panic", tt.name)
				}
			}()

			tt.call()
		})
	}
}
