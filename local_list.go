package gabric

import (
	"context"
	"os"
)

// List returns the entries of a local directory, sorted by name, and panics on
// failure.
func (o *LocalOps) List(p string) []ListEntry {
	entries, err := o.ListErr(p)
	if err != nil {
		panic(err)
	}

	return entries
}

// ListErr returns the entries of a local directory, sorted by name.
func (o *LocalOps) ListErr(p string) ([]ListEntry, error) {
	entries, err := GetLocalBackend(o.ctx).List(o.ctx, ListRequest{Path: ResolveLocalPath(o.ctx, p)})
	if err != nil {
		return nil, err
	}

	sortListEntries(entries)

	return entries, nil
}

func (realLocalBackend) List(_ context.Context, req ListRequest) ([]ListEntry, error) {
	dirEntries, err := os.ReadDir(req.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]ListEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		entries = append(entries, ListEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}

	return entries, nil
}
