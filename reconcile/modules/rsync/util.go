package rsync

import "strings"

// rsyncHasChanges reports whether rsync's itemized (-i) output contains a
// transfer or creation. Metadata-only lines (".f...p.....") are ignored.
func rsyncHasChanges(out string) bool {
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}

		switch line[0] {
		case '>', '<', 'c', 'h', '*':
			return true
		}
	}

	return false
}
