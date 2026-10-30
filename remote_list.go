package gabric

import (
	"context"
	"strings"
)

// List returns the entries of a remote directory, sorted by name, and panics on
// failure.
func (o *RemoteOps) List(p string) []ListEntry {
	entries, err := o.ListErr(p)
	if err != nil {
		panic(err)
	}

	return entries
}

// ListErr returns the entries of a remote directory, sorted by name.
func (o *RemoteOps) ListErr(p string) ([]ListEntry, error) {
	entries, err := GetRemoteBackend(o.ctx).List(
		withoutRemoteWorkingDir(o.ctx),
		ListRequest{Path: ResolveRemotePath(o.ctx, p)},
	)
	if err != nil {
		return nil, err
	}

	sortListEntries(entries)

	return entries, nil
}

func (realRemoteBackend) List(ctx context.Context, req ListRequest) ([]ListEntry, error) {
	p := req.Path
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}

	e := Remote(ctx).Exec("ls", "-1Ap", p).Capture()
	if err := e.RunErr(); err != nil {
		return nil, err
	}

	lines := strings.Split(e.StdoutStr(), "\n")
	entries := make([]ListEntry, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		isDir := strings.HasSuffix(line, "/")
		if isDir {
			line = strings.TrimSuffix(line, "/")
		}

		entries = append(entries, ListEntry{
			Name:  line,
			IsDir: isDir,
		})
	}

	return entries, nil
}
