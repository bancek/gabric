package gabric

import (
	"slices"
	"testing"
)

func TestSortListEntries(t *testing.T) {
	entries := []ListEntry{
		{Name: "b"},
		{Name: "a", IsDir: true},
		{Name: ".hidden"},
	}

	sortListEntries(entries)

	want := []ListEntry{
		{Name: ".hidden"},
		{Name: "a", IsDir: true},
		{Name: "b"},
	}

	if !slices.Equal(entries, want) {
		t.Errorf("sortListEntries() = %+v, want %+v", entries, want)
	}
}
