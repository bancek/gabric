package linux

import (
	"context"
	"slices"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// OSRelease is the parsed /etc/os-release data.
type OSRelease struct {
	ID              string
	IDLike          []string
	Name            string
	PrettyName      string
	VersionID       string
	VersionCodename string
}

// IsLike reports whether the distribution ID or one of its ID_LIKE values
// matches any of ids.
func (r OSRelease) IsLike(ids ...string) bool {
	for _, id := range ids {
		if r.ID == id || slices.Contains(r.IDLike, id) {
			return true
		}
	}

	return false
}

// OSReleaseData reads /etc/os-release as a data node (dry-run safe, memoized
// once per run). A missing file yields the zero value; other read errors
// propagate.
func OSReleaseData() *reconcile.Node[OSRelease] {
	return reconcile.Data("linux.os-release", func(ctx context.Context) (OSRelease, error) {
		content, err := modules.FileContentData(input.Static("/etc/os-release")).Value(ctx)
		if err != nil {
			return OSRelease{}, err
		}

		return parseOSRelease(content), nil
	}).Node()
}

func parseOSRelease(content string) OSRelease {
	fields := map[string]string{}

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		fields[strings.TrimSpace(key)] = unquoteOSRelease(strings.TrimSpace(value))
	}

	return OSRelease{
		ID:              fields["ID"],
		IDLike:          strings.Fields(fields["ID_LIKE"]),
		Name:            fields["NAME"],
		PrettyName:      fields["PRETTY_NAME"],
		VersionID:       fields["VERSION_ID"],
		VersionCodename: fields["VERSION_CODENAME"],
	}
}

func unquoteOSRelease(value string) string {
	words, err := shellquote.Split(value)
	if err != nil || len(words) == 0 {
		return value
	}

	return strings.Join(words, " ")
}
