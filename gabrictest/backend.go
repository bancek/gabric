package gabrictest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"github.com/bancek/gabric"
)

var errLocalExecUnsupported = errors.New("gabrictest: local exec with ExtraFiles or AfterStart is not supported")

type localBackend struct {
	side *side
}

type remoteBackend struct {
	side *side
}

func (b localBackend) Exec(ctx context.Context, req gabric.ExecRequest) error {
	if len(req.ExtraFiles) != 0 || req.AfterStart != nil {
		return errLocalExecUnsupported
	}

	return run(ctx, b.side, virtualPath(req.Dir), req)
}

func (b localBackend) Put(ctx context.Context, req gabric.PutRequest) error {
	req.Path = virtualPath(req.Path)
	return put(b.side, req)
}

func (b localBackend) Get(ctx context.Context, req gabric.GetRequest, f func(io.Reader) error) error {
	req.Path = virtualPath(req.Path)
	return get(b.side, req, f)
}

func (b localBackend) Exists(ctx context.Context, req gabric.ExistsRequest) (bool, error) {
	req.Path = virtualPath(req.Path)
	return exists(b.side, req)
}

func (b localBackend) Stat(_ context.Context, req gabric.StatRequest) (gabric.FileStat, error) {
	req.Path = virtualPath(req.Path)
	return stat(b.side, req)
}

func (b localBackend) Mkdir(ctx context.Context, req gabric.MkdirRequest) error {
	req.Path = virtualPath(req.Path)
	return mkdir(b.side, req)
}

func (b localBackend) Remove(ctx context.Context, req gabric.RemoveRequest) error {
	req.Path = virtualPath(req.Path)
	return remove(b.side, req)
}

func (b localBackend) List(ctx context.Context, req gabric.ListRequest) ([]gabric.ListEntry, error) {
	req.Path = virtualPath(req.Path)
	return list(b.side, req)
}

func (b remoteBackend) Exec(ctx context.Context, req gabric.ExecRequest) error {
	req.Cmd = gabric.GetRemoteSudo(ctx).WrapCommand(req.Cmd)

	return run(ctx, b.side, req.Dir, req)
}

func (b remoteBackend) Put(ctx context.Context, req gabric.PutRequest) error {
	return put(b.side, req)
}

func (b remoteBackend) Get(ctx context.Context, req gabric.GetRequest, f func(io.Reader) error) error {
	return get(b.side, req, f)
}

func (b remoteBackend) Exists(ctx context.Context, req gabric.ExistsRequest) (bool, error) {
	return exists(b.side, req)
}

func (b remoteBackend) Stat(_ context.Context, req gabric.StatRequest) (gabric.FileStat, error) {
	return stat(b.side, req)
}

func (b remoteBackend) Mkdir(ctx context.Context, req gabric.MkdirRequest) error {
	return mkdir(b.side, req)
}

func (b remoteBackend) Remove(ctx context.Context, req gabric.RemoveRequest) error {
	return remove(b.side, req)
}

func (b remoteBackend) List(ctx context.Context, req gabric.ListRequest) ([]gabric.ListEntry, error) {
	return list(b.side, req)
}

func (b remoteBackend) Rsync(ctx context.Context, req gabric.RsyncRequest, stdout, stderr io.Writer) error {
	b.side.calls.recordRsync(req)

	result, ok := b.side.handleRsync(req)
	if !ok {
		return nil
	}

	if stdout != nil {
		if _, err := io.WriteString(stdout, result.Stdout); err != nil {
			return err
		}
	}

	if stderr != nil {
		if _, err := io.WriteString(stderr, result.Stderr); err != nil {
			return err
		}
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("rsync: exit status %d", result.ExitCode)
	}

	return nil
}

func run(ctx context.Context, s *side, dir string, req gabric.ExecRequest) error {
	stdout := req.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	stderr := req.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	code, err := s.core.RunArgsEnv(ctx, dir, req.Cmd, req.Env.Environ(), req.Stdin, stdout, stderr)
	if err != nil {
		return err
	}

	if code != 0 {
		return &gabric.ExecExitError{Status: code}
	}

	return nil
}

func put(s *side, req gabric.PutRequest) error {
	content, err := io.ReadAll(req.Content)
	if err != nil {
		return err
	}

	req.Content = bytes.NewBuffer(content)
	s.calls.recordPut(req)

	if dir := parentDir(req.Path); dir != "" {
		if req.EnsureParent {
			if err := s.core.FS().MkdirAll(dir, 0o777); err != nil {
				return err
			}
		} else if _, err := s.core.FS().Stat(dir); err != nil {
			return err
		}
	}

	mode := fs.FileMode(0o666)
	if info, err := s.core.FS().Stat(req.Path); err == nil {
		mode = info.Mode().Perm()
	}

	if req.Mode != 0 {
		mode = req.Mode
	}

	return s.core.FS().WriteFile(req.Path, content, mode)
}

func get(s *side, req gabric.GetRequest, f func(io.Reader) error) error {
	s.calls.recordGet(req)

	content, err := s.core.FS().ReadFile(req.Path)
	if err != nil {
		return err
	}

	return f(bytes.NewReader(content))
}

func exists(s *side, req gabric.ExistsRequest) (bool, error) {
	s.calls.recordExists(req)

	var (
		info fs.FileInfo
		err  error
	)

	if req.Kind == gabric.FileKindLink {
		info, err = s.core.FS().Lstat(req.Path)
	} else {
		info, err = s.core.FS().Stat(req.Path)
	}

	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	switch req.Kind {
	case gabric.FileKindDir:
		return info.IsDir(), nil
	case gabric.FileKindFile:
		return info.Mode().IsRegular(), nil
	case gabric.FileKindLink:
		return info.Mode()&fs.ModeSymlink != 0, nil
	default:
		return true, nil
	}
}

func stat(s *side, req gabric.StatRequest) (gabric.FileStat, error) {
	s.calls.recordStat(req)

	info, err := s.core.FS().Stat(req.Path)
	if err != nil {
		return gabric.FileStat{}, err
	}

	user, group, err := s.core.FS().Owner(req.Path)
	if err != nil {
		return gabric.FileStat{}, err
	}

	return gabric.FileStat{Mode: info.Mode(), User: user, Group: group}, nil
}

func mkdir(s *side, req gabric.MkdirRequest) error {
	s.calls.recordMkdir(req)

	mode := req.Mode
	if mode == 0 {
		mode = 0o777
	}

	if info, err := s.core.FS().Stat(req.Path); err == nil && info.IsDir() {
		// already exists
	} else if req.Recursive {
		if err := s.core.FS().MkdirAll(req.Path, mode); err != nil {
			return err
		}
	} else if err := s.core.FS().Mkdir(req.Path, mode); err != nil {
		return err
	}

	if req.Mode != 0 {
		return s.core.FS().Chmod(req.Path, req.Mode)
	}

	return nil
}

func remove(s *side, req gabric.RemoveRequest) error {
	s.calls.recordRemove(req)

	if req.Recursive {
		if !req.Force {
			if _, err := s.core.FS().Lstat(req.Path); err != nil {
				return err
			}
		}

		return s.core.FS().RemoveAll(req.Path)
	}

	err := s.core.FS().Remove(req.Path)
	if err != nil {
		if req.Force && errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	return nil
}

func list(s *side, req gabric.ListRequest) ([]gabric.ListEntry, error) {
	s.calls.recordList(req)

	dirEntries, err := s.core.FS().ReadDir(req.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]gabric.ListEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		entries = append(entries, gabric.ListEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}

	return entries, nil
}

func parentDir(p string) string {
	dir := path.Dir(p)

	if dir == "" || dir == "." || dir == "/" {
		return ""
	}

	return dir
}
