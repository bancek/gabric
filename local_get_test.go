package gabric

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalGet(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if got := Local(ctx).Get("file.txt").String(); got != "hello" {
			t.Errorf("String() = %q, want %q", got, "hello")
		}

		if got := string(Local(ctx).Get("file.txt").Bytes()); got != "hello" {
			t.Errorf("Bytes() = %q, want %q", got, "hello")
		}

		var b bytes.Buffer

		Local(ctx).Get("file.txt").Copy(&b)

		if b.String() != "hello" {
			t.Errorf("Copy() = %q, want %q", b.String(), "hello")
		}

		var got string

		Local(ctx).Get("file.txt").WithReader(func(r io.Reader) error {
			data, err := io.ReadAll(r)
			got = string(data)

			return err
		})

		if got != "hello" {
			t.Errorf("WithReader() = %q, want %q", got, "hello")
		}
	})
}

func TestLocalGetErr(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		var b bytes.Buffer

		if err := Local(ctx).Get("file.txt").CopyErr(&b); err != nil {
			t.Fatalf("CopyErr() error: %v", err)
		}

		if b.String() != "hello" {
			t.Errorf("CopyErr() = %q, want %q", b.String(), "hello")
		}

		got, err := Local(ctx).Get("file.txt").BytesErr()
		if err != nil {
			t.Fatalf("BytesErr() error: %v", err)
		}

		if string(got) != "hello" {
			t.Errorf("BytesErr() = %q, want %q", string(got), "hello")
		}
	})
}

func TestLocalGetErrMissing(t *testing.T) {
	ctx := testContext(t)
	dir := t.TempDir()

	Local(ctx).WithWorkingDir(dir, func(ctx context.Context) {
		if _, err := Local(ctx).Get("missing.txt").BytesErr(); err == nil {
			t.Error("BytesErr(missing) = nil, want error")
		}
	})
}
