package gabric

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type fakeWriteCloser struct {
	bytes.Buffer
	closed   bool
	closeErr error
}

func (f *fakeWriteCloser) Close() error {
	f.closed = true
	return f.closeErr
}

type fakeReadCloser struct {
	*bytes.Reader
	closed   bool
	closeErr error
}

func (f *fakeReadCloser) Close() error {
	f.closed = true
	return f.closeErr
}

func TestCopyAndCloseWriter(t *testing.T) {
	w := &fakeWriteCloser{}

	if err := copyAndCloseWriter(w, strings.NewReader("data")); err != nil {
		t.Fatalf("copyAndCloseWriter() error: %v", err)
	}

	if !w.closed {
		t.Error("writer was not closed")
	}

	if got := w.String(); got != "data" {
		t.Errorf("written = %q, want %q", got, "data")
	}
}

func TestCopyAndCloseWriterCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	w := &fakeWriteCloser{closeErr: closeErr}

	if err := copyAndCloseWriter(w, strings.NewReader("data")); !errors.Is(err, closeErr) {
		t.Errorf("copyAndCloseWriter() error = %v, want %v", err, closeErr)
	}

	if !w.closed {
		t.Error("writer was not closed")
	}
}

func TestCopyAndCloseReader(t *testing.T) {
	r := &fakeReadCloser{Reader: bytes.NewReader([]byte("data"))}

	var buf bytes.Buffer

	if err := copyAndCloseReader(r, &buf); err != nil {
		t.Fatalf("copyAndCloseReader() error: %v", err)
	}

	if !r.closed {
		t.Error("reader was not closed")
	}

	if got := buf.String(); got != "data" {
		t.Errorf("copied = %q, want %q", got, "data")
	}
}

func TestCopyAndCloseReaderCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	r := &fakeReadCloser{Reader: bytes.NewReader([]byte("data")), closeErr: closeErr}

	var buf bytes.Buffer

	if err := copyAndCloseReader(r, &buf); !errors.Is(err, closeErr) {
		t.Errorf("copyAndCloseReader() error = %v, want %v", err, closeErr)
	}

	if !r.closed {
		t.Error("reader was not closed")
	}
}
