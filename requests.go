package gabric

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"slices"
)

// ExecRequest is a command request shared by the local and remote backends.
// Dir is the resolved working directory. Empty means the backend default: the
// process working directory for local commands and the login directory for
// remote commands.
type ExecRequest struct {
	Cmd []string
	Dir string
	// Env is the resolved exec environment for the command: the context's
	// exec environment with per-command entries applied.
	Env        ExecEnv
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	ExtraFiles []*os.File
	AfterStart func()
}

// PutRequest is a file write request shared by the local and remote backends.
type PutRequest struct {
	Path         string
	Content      io.Reader
	Mode         fs.FileMode
	User         string
	Group        string
	EnsureParent bool
}

// Equal reports whether r and other are the same request. Content is compared
// only when both sides are *bytes.Buffer; other reader types compare unequal.
func (r PutRequest) Equal(other PutRequest) bool {
	if r.Path != other.Path ||
		r.Mode != other.Mode ||
		r.User != other.User ||
		r.Group != other.Group ||
		r.EnsureParent != other.EnsureParent {
		return false
	}

	if r.Content == nil || other.Content == nil {
		return r.Content == nil && other.Content == nil
	}

	rBuffer, rOK := r.Content.(*bytes.Buffer)
	otherBuffer, otherOK := other.Content.(*bytes.Buffer)
	if !rOK || !otherOK {
		return false
	}

	return bytes.Equal(rBuffer.Bytes(), otherBuffer.Bytes())
}

// GetRequest is a file read request shared by the local and remote backends.
type GetRequest struct {
	Path string
}

// Equal reports whether r and other are the same request.
func (r GetRequest) Equal(other GetRequest) bool {
	return r == other
}

// ExistsRequest is a path probe shared by the local and remote backends.
type ExistsRequest struct {
	Path string
	Kind FileKind
}

// Equal reports whether r and other are the same request.
func (r ExistsRequest) Equal(other ExistsRequest) bool {
	return r == other
}

// StatRequest is a file metadata request shared by the local and remote
// backends.
type StatRequest struct {
	Path string
}

// Equal reports whether r and other are the same request.
func (r StatRequest) Equal(other StatRequest) bool {
	return r == other
}

// MkdirRequest is a directory creation request shared by the local and remote
// backends.
type MkdirRequest struct {
	Path        string
	Recursive   bool
	Mode        fs.FileMode
	User, Group string
}

// Equal reports whether r and other are the same request.
func (r MkdirRequest) Equal(other MkdirRequest) bool {
	return r == other
}

// RemoveRequest is a path removal request shared by the local and remote
// backends.
type RemoveRequest struct {
	Path      string
	Recursive bool
	Force     bool
}

// Equal reports whether r and other are the same request.
func (r RemoveRequest) Equal(other RemoveRequest) bool {
	return r == other
}

// ListEntry is one directory entry returned by List.
type ListEntry struct {
	Name  string
	IsDir bool
}

// ListRequest is a directory listing request shared by the local and remote
// backends.
type ListRequest struct {
	Path string
}

// Equal reports whether r and other are the same request.
func (r ListRequest) Equal(other ListRequest) bool {
	return r == other
}

// RsyncDirection is the direction of an rsync transfer.
type RsyncDirection int

const (
	RsyncDirectionUnset RsyncDirection = iota
	RsyncDirectionUpload
	RsyncDirectionDownload
)

// RsyncRequest is a transfer request for an rsync backend.
type RsyncRequest struct {
	LocalPath  string
	RemotePath string
	Direction  RsyncDirection
	Delete     bool
	Excludes   []string
	Flags      []string
	Sudo       *Sudo
}

// Equal reports whether r and other are the same request.
func (r RsyncRequest) Equal(other RsyncRequest) bool {
	return r.LocalPath == other.LocalPath &&
		r.RemotePath == other.RemotePath &&
		r.Direction == other.Direction &&
		r.Delete == other.Delete &&
		slices.Equal(r.Excludes, other.Excludes) &&
		slices.Equal(r.Flags, other.Flags) &&
		r.Sudo.Equal(other.Sudo)
}

// Clone returns a copy of r with its slices copied.
func (r RsyncRequest) Clone() RsyncRequest {
	r.Excludes = slices.Clone(r.Excludes)
	r.Flags = slices.Clone(r.Flags)

	if r.Sudo != nil {
		sudo := *r.Sudo
		sudo.Flags = slices.Clone(sudo.Flags)
		r.Sudo = &sudo
	}

	return r
}
