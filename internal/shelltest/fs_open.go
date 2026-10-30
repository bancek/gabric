package shelltest

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"time"
)

var errUnsupportedOpen = errors.New("unsupported open flags")

func (f *FS) openAt(cwd, name string, flag int, perm fs.FileMode) (io.ReadWriteCloser, error) {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
		if _, _, err := f.lookupLocked(remotePath, false); err == nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
		}
	}

	n, resolved, err := f.lookupLocked(remotePath, true)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) || flag&os.O_CREATE == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}

		parentPath := path.Dir(resolved)
		base := path.Base(resolved)

		if base == "/" || base == "." {
			return nil, &fs.PathError{Op: "open", Path: name, Err: errIsDir}
		}

		parent, err := f.nodeLocked(parentPath)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}

		if !parent.mode.IsDir() {
			return nil, &fs.PathError{Op: "open", Path: name, Err: errNotDir}
		}

		mode := perm.Perm()
		if mode == 0 {
			mode = 0o666
		}

		n = &node{mode: mode, modTime: time.Now()}
		parent.children[base] = n
	} else if n.mode.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: errIsDir}
	}

	switch {
	case flag&os.O_RDWR != 0:
		return nil, &fs.PathError{Op: "open", Path: name, Err: errUnsupportedOpen}
	case flag&os.O_WRONLY != 0:
		if flag&os.O_TRUNC != 0 {
			n.data = nil
			n.modTime = time.Now()
		}

		return &writeFileHandle{
			fs:     f,
			node:   n,
			append: flag&os.O_APPEND != 0,
		}, nil
	default:
		return &readFileHandle{reader: bytes.NewReader(slices.Clone(n.data))}, nil
	}
}

type readFileHandle struct {
	reader *bytes.Reader
}

func (h *readFileHandle) Read(p []byte) (int, error) {
	return h.reader.Read(p)
}

func (h *readFileHandle) Write([]byte) (int, error) {
	return 0, errors.New("read-only file")
}

func (h *readFileHandle) Close() error {
	return nil
}

type writeFileHandle struct {
	fs     *FS
	node   *node
	append bool
	buf    bytes.Buffer
}

func (h *writeFileHandle) Write(p []byte) (int, error) {
	return h.buf.Write(p)
}

func (h *writeFileHandle) Read([]byte) (int, error) {
	return 0, errors.New("write-only file")
}

func (h *writeFileHandle) Close() error {
	h.fs.mu.Lock()
	defer h.fs.mu.Unlock()

	if h.append {
		h.node.data = append(h.node.data, h.buf.Bytes()...)
	} else {
		h.node.data = slices.Clone(h.buf.Bytes())
	}

	h.node.modTime = time.Now()

	return nil
}

type devNull struct{}

func (devNull) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (devNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (devNull) Close() error {
	return nil
}
