package gabric

import "sort"

func sortListEntries(entries []ListEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
}
