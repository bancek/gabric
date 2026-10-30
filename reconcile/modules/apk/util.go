package apk

import "strings"

// versionFrom returns the version of name in apk command output.
func versionFrom(out, name string) string {
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if v, ok := strings.CutPrefix(fields[0], name+"-"); ok {
			return v
		}
	}

	return ""
}
