package ufw

import "strings"

// fieldAfter returns the text after the first line starting with prefix.
func fieldAfter(s, prefix string) string {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(after)
		}
	}

	return ""
}
