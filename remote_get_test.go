package gabric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestRemoteGet(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	path := "/srv/app/file.txt"

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if got := Remote(ctx).Get(path).String(); got != "hello" {
		t.Errorf("String() = %q, want %q", got, "hello")
	}

	if got := string(Remote(ctx).Get(path).Bytes()); got != "hello" {
		t.Errorf("Bytes() = %q, want %q", got, "hello")
	}

	var b bytes.Buffer

	Remote(ctx).Get(path).Copy(&b)

	if b.String() != "hello" {
		t.Errorf("Copy() = %q, want %q", b.String(), "hello")
	}

	var got string

	Remote(ctx).Get(path).WithReader(func(r io.Reader) error {
		data, err := io.ReadAll(r)
		got = string(data)

		return err
	})

	if got != "hello" {
		t.Errorf("WithReader() = %q, want %q", got, "hello")
	}

	asserts.AssertExecuted("cat", "--", path)
}

func TestRemoteGetErr(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	path := "/srv/app/file.txt"

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := Remote(ctx).Get(path).BytesErr()
	if err != nil {
		t.Fatalf("BytesErr() error: %v", err)
	}

	if string(got) != "hello" {
		t.Errorf("BytesErr() = %q, want %q", string(got), "hello")
	}
}

func TestRemoteGetStopsOnEarlyReturn(t *testing.T) {
	ctx := NewRemoteRunnerContext(testContext(t), func(ctx context.Context, cmd string, _ io.Reader, stdout, _ io.Writer) error {
		if strings.Contains(cmd, "cat") {
			io.WriteString(stdout, "partial")
			// Stop producing output without exiting, to prove the client does
			// not block waiting for the command.
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})

	done := make(chan error, 1)

	go func() {
		done <- Remote(ctx).Get("/whatever").WithReaderErr(func(r io.Reader) error {
			_, err := io.ReadFull(r, make([]byte, 3))
			return err
		})
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Get did not return after the reader stopped consuming")
	}
}

func TestRemoteGetErrMissing(t *testing.T) {
	_, _, ctx := localRemoteContext(t)

	path := "/srv/missing.txt"

	_, err := Remote(ctx).Get(path).BytesErr()
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("BytesErr(missing) error = %v, want ErrNotExist", err)
	}
}
