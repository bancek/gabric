package gabric

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kballard/go-shellquote"
)

// rsyncPlaceholderHost is the destination host used with the native transport.
// The sidecar drops the host; only the remote path matters.
const rsyncPlaceholderHost = "gabric-remote"

type rsyncOptions struct {
	remoteSpec string
	sudo       *Sudo
	localPath  string
	remotePath string
	direction  RsyncDirection
	delete     bool
	excludes   []string
	flags      []string
	rsh        string
}

// RsyncOps builds an rsync transfer between the local machine and the remote
// side from context.
type RsyncOps struct {
	ctx context.Context
	rsyncOptions

	captureStdout bool
	captureStderr bool
	stdout        []byte
	stderr        []byte
}

// Rsync returns a builder for an rsync transfer between the local machine and
// the remote side from the RemoteOps context.
func (o *RemoteOps) Rsync() *RsyncOps {
	return &RsyncOps{
		ctx:          o.ctx,
		rsyncOptions: rsyncOptions{flags: []string{"-pthrvz"}},
	}
}

// Local sets the local side path of the transfer.
func (r *RsyncOps) Local(path string) *RsyncOps {
	r.localPath = path
	return r
}

// Remote sets the remote side path of the transfer.
func (r *RsyncOps) Remote(path string) *RsyncOps {
	r.remotePath = path
	return r
}

// Upload transfers from the local side to the remote side.
func (r *RsyncOps) Upload() *RsyncOps {
	r.direction = RsyncDirectionUpload
	return r
}

// Download transfers from the remote side to the local side.
func (r *RsyncOps) Download() *RsyncOps {
	r.direction = RsyncDirectionDownload
	return r
}

// Delete passes --delete to rsync, removing files that no longer exist on the
// source.
func (r *RsyncOps) Delete() *RsyncOps {
	r.delete = true
	return r
}

// Exclude adds rsync --exclude patterns. Excluded files are not transferred and,
// with Delete, are not deleted.
func (r *RsyncOps) Exclude(patterns ...string) *RsyncOps {
	r.excludes = append(r.excludes, patterns...)
	return r
}

// AddFlags appends rsync flags on top of the existing set (the default -pthrvz).
func (r *RsyncOps) AddFlags(flags ...string) *RsyncOps {
	r.flags = append(r.flags, flags...)
	return r
}

// SetFlags replaces the rsync flag set. Call with no arguments to run rsync
// without extra flags.
func (r *RsyncOps) SetFlags(flags ...string) *RsyncOps {
	r.flags = flags
	return r
}

// Capture captures the local rsync process' stdout and stderr.
func (r *RsyncOps) Capture() *RsyncOps {
	r.captureStdout = true
	r.captureStderr = true
	return r
}

// CaptureStdout captures the local rsync process' stdout. Stderr is still
// inherited from the process.
func (r *RsyncOps) CaptureStdout() *RsyncOps {
	r.captureStdout = true
	return r
}

// CaptureStderr captures the local rsync process' stderr. Stdout is still
// inherited from the process.
func (r *RsyncOps) CaptureStderr() *RsyncOps {
	r.captureStderr = true
	return r
}

// Stdout returns the captured local rsync stdout.
func (r *RsyncOps) Stdout() []byte {
	return r.stdout
}

// StdoutStr returns the captured local rsync stdout as a string.
func (r *RsyncOps) StdoutStr() string {
	return string(r.stdout)
}

// Stderr returns the captured local rsync stderr.
func (r *RsyncOps) Stderr() []byte {
	return r.stderr
}

// StderrStr returns the captured local rsync stderr as a string.
func (r *RsyncOps) StderrStr() string {
	return string(r.stderr)
}

func (r *RsyncOps) request() RsyncRequest {
	sudo := GetRemoteSudo(r.ctx)

	req := RsyncRequest{
		LocalPath:  r.localPath,
		RemotePath: r.remotePath,
		Direction:  r.direction,
		Delete:     r.delete,
		Excludes:   r.excludes,
		Flags:      r.flags,
		Sudo:       sudo,
	}

	return req.Clone()
}

func runLocalRsync(ctx context.Context, cmd []string, stdout, stderr io.Writer, extraFiles []*os.File, afterStart func()) error {
	e := Local(ctx).Exec(cmd...)

	if stdout != nil {
		e = e.Stdout(stdout)
	}

	if stderr != nil {
		e = e.Stderr(stderr)
	}

	e.extraFiles = extraFiles
	e.afterStart = afterStart

	return e.RunErr()
}

// RsyncRsh is the fallback used when the native rsync transport is unavailable.
type RsyncRsh struct {
	HostSpec string // user@host, IPv6-bracketed
	Rsh      string // --rsh value
}

// RsyncRshResolver resolves the fallback lazily, when the native transport is
// unavailable.
type RsyncRshResolver func(context.Context) (RsyncRsh, error)

// ErrNoRsyncRsh reports that the current transport has no rsync rsh fallback.
var ErrNoRsyncRsh = errors.New("gabric: no rsync rsh fallback")

// NoRsyncRsh is the resolver for transports that have no rsh fallback.
func NoRsyncRsh(context.Context) (RsyncRsh, error) {
	return RsyncRsh{}, ErrNoRsyncRsh
}

const rsyncRshKey contextKey[RsyncRshResolver] = "gabric.rsyncRsh"

// NewRsyncRshContext returns a context carrying an rsync rsh resolver.
func NewRsyncRshContext(ctx context.Context, resolve RsyncRshResolver) context.Context {
	if resolve == nil {
		panic("gabric: nil rsync rsh resolver")
	}

	return withContextValue(ctx, rsyncRshKey, resolve)
}

// HasRsyncRsh reports whether ctx carries an rsync rsh resolver.
func HasRsyncRsh(ctx context.Context) bool {
	resolve, ok := contextValue(ctx, rsyncRshKey)

	return ok && resolve != nil
}

// GetRsyncRsh returns the rsync rsh fallback from ctx.
func GetRsyncRsh(ctx context.Context) (RsyncRsh, error) {
	resolve, ok := contextValue(ctx, rsyncRshKey)
	if !ok || resolve == nil {
		return RsyncRsh{}, ErrNoRsyncRsh
	}

	return resolve(ctx)
}

func (realRemoteBackend) Rsync(ctx context.Context, req RsyncRequest, stdout, stderr io.Writer) error {
	GetRemoteRunner(ctx)

	opts := rsyncOptions{
		remoteSpec: rsyncPlaceholderHost,
		sudo:       req.Sudo,
		localPath:  req.LocalPath,
		remotePath: req.RemotePath,
		direction:  req.Direction,
		delete:     req.Delete,
		excludes:   req.Excludes,
		flags:      req.Flags,
	}

	transport, err := newRsyncTransport()
	if err != nil {
		rsh, rshErr := GetRsyncRsh(ctx)
		if rshErr != nil {
			Logger(ctx).
				With("error", err, "fallback_error", rshErr).
				Debug("Native rsync transport unavailable")

			return errors.Join(
				fmt.Errorf("rsync requires the native transport: %w", err),
				rshErr,
			)
		}

		Logger(ctx).
			With("error", err, "host", rsh.HostSpec, "rsh", rsh.Rsh).
			Debug("Using rsync rsh fallback")

		opts.remoteSpec = rsh.HostSpec
		opts.rsh = rsh.Rsh

		return runLocalRsync(ctx, buildRsyncArgs(opts), stdout, stderr, nil, nil)
	}
	defer transport.Close()

	opts.rsh = transport.Rsh()

	cmd := buildRsyncArgs(opts)

	remoteErrCh := make(chan error, 1)

	go func() {
		remoteErrCh <- transport.Serve(ctx)
	}()

	rsyncErr := runLocalRsync(ctx, cmd, stdout, stderr, transport.ChildFiles(), transport.CloseChildFiles)

	transport.CloseArgs()

	remoteErr := <-remoteErrCh

	if _, ok := errors.AsType[*ExecExitError](remoteErr); ok {
		return remoteErr
	}

	if rsyncErr != nil {
		return rsyncErr
	}

	return remoteErr
}

// Run runs the transfer and panics on failure.
func (r *RsyncOps) Run() *RsyncOps {
	if err := r.RunErr(); err != nil {
		panic(err)
	}
	return r
}

// RunErr runs the transfer.
func (r *RsyncOps) RunErr() error {
	if r.direction == RsyncDirectionUnset {
		return errors.New("rsync direction not set: call Upload() or Download()")
	}

	if r.localPath == "" {
		return errors.New("rsync local path not set")
	}

	if r.remotePath == "" {
		return errors.New("rsync remote path not set")
	}

	var (
		stdoutBuf bytes.Buffer
		stderrBuf bytes.Buffer
	)

	var stdout, stderr io.Writer
	if r.captureStdout {
		stdout = &stdoutBuf
	}
	if r.captureStderr {
		stderr = &stderrBuf
	}

	err := GetRemoteBackend(r.ctx).Rsync(r.ctx, r.request(), stdout, stderr)

	r.stdout = stdoutBuf.Bytes()
	r.stderr = stderrBuf.Bytes()

	return err
}

func buildRsyncArgs(opts rsyncOptions) []string {
	cmd := []string{"rsync"}

	if opts.delete {
		cmd = append(cmd, "--delete")
	}

	for _, pattern := range opts.excludes {
		cmd = append(cmd, "--exclude="+pattern)
	}

	cmd = append(cmd, opts.flags...)

	if opts.sudo != nil {
		cmd = append(cmd, "--rsync-path="+shellquote.Join(append(opts.sudo.Prefix(), "rsync")...))
	}

	cmd = append(cmd, "--rsh="+opts.rsh)

	remote := opts.remoteSpec + ":" + opts.remotePath

	if opts.direction == RsyncDirectionUpload {
		cmd = append(cmd, opts.localPath, remote)
	} else {
		cmd = append(cmd, remote, opts.localPath)
	}

	return cmd
}
