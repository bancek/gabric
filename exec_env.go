package gabric

import (
	"log/slog"
	"slices"
	"strings"
)

const sensitivePlaceholder = "<sensitive>"

// EnvVar is one opaque environment entry. Its value is hidden so that
// formatting an entry with %v, %+v or %#v never exposes it.
type EnvVar struct {
	data *envVarData
}

type envVarData struct {
	key       string
	value     string
	sensitive bool
}

// Env returns a normal environment entry.
func Env(key string, value string) EnvVar {
	return EnvVar{data: &envVarData{key: key, value: value}}
}

// SensitiveEnv returns an environment entry whose value is redacted by Gabric
// logging and formatting.
func SensitiveEnv(key string, value string) EnvVar {
	return EnvVar{data: &envVarData{key: key, value: value, sensitive: true}}
}

// LogValue returns a redacted slog representation of the entry.
func (v EnvVar) LogValue() slog.Value {
	if v.data == nil {
		return slog.GroupValue()
	}

	return slog.GroupValue(slog.String(v.data.key, v.data.displayValue()))
}

// ExecEnv is an immutable, ordered set of environment entries. The zero value
// is empty.
type ExecEnv struct {
	data *execEnvData
}

type execEnvData struct {
	entries []envVarData
}

// NewExecEnv returns an environment from vars. Nil entries are ignored.
func NewExecEnv(vars ...EnvVar) ExecEnv {
	return ExecEnv{}.With(vars...)
}

// ParseExecEnv parses KEY=VALUE entries as normal variables. Malformed entries
// are ignored; duplicate keys keep their first position and last value.
func ParseExecEnv(entries ...string) ExecEnv {
	vars := make([]EnvVar, 0, len(entries))

	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}

		vars = append(vars, Env(key, value))
	}

	return NewExecEnv(vars...)
}

// With returns a copy with vars applied. Sensitivity is sticky: marking a key
// sensitive in any layer keeps it redacted after later overrides.
func (e ExecEnv) With(vars ...EnvVar) ExecEnv {
	entries := make([]envVarData, 0, len(vars))

	for _, v := range vars {
		if v.data == nil {
			continue
		}

		entries = append(entries, *v.data)
	}

	return e.withEntries(entries)
}

// Merge returns a copy of e with other applied.
func (e ExecEnv) Merge(other ExecEnv) ExecEnv {
	return e.withEntries(other.entries())
}

// MarkSensitive marks key sensitive. It is a no-op when key is not present.
func (e ExecEnv) MarkSensitive(key string) ExecEnv {
	entries := e.entries()

	i := slices.IndexFunc(entries, func(entry envVarData) bool {
		return entry.key == key
	})
	if i < 0 {
		return e
	}

	clone := slices.Clone(entries)
	clone[i].sensitive = true

	return ExecEnv{data: &execEnvData{entries: clone}}
}

// Get returns the real value for key.
func (e ExecEnv) Get(key string) (string, bool) {
	for _, entry := range e.entries() {
		if entry.key == key {
			return entry.value, true
		}
	}

	return "", false
}

// IsSensitive reports whether key is marked sensitive.
func (e ExecEnv) IsSensitive(key string) bool {
	for _, entry := range e.entries() {
		if entry.key == key {
			return entry.sensitive
		}
	}

	return false
}

// Len returns the number of entries.
func (e ExecEnv) Len() int {
	return len(e.entries())
}

// Vars returns the environment entries. The returned slice is a copy; the
// entries themselves are immutable.
func (e ExecEnv) Vars() []EnvVar {
	entries := e.entries()
	out := make([]EnvVar, 0, len(entries))

	for _, entry := range entries {
		data := entry
		out = append(out, EnvVar{data: &data})
	}

	return out
}

// Environ returns the real KEY=VALUE entries for execution. It includes
// sensitive values; do not log it.
func (e ExecEnv) Environ() []string {
	entries := e.entries()
	out := make([]string, 0, len(entries))

	for _, entry := range entries {
		out = append(out, entry.key+"="+entry.value)
	}

	return out
}

// EnvironLog returns the KEY=VALUE entries with sensitive values replaced by
// the redaction marker. It is safe to log.
func (e ExecEnv) EnvironLog() []string {
	entries := e.entries()
	out := make([]string, 0, len(entries))

	for _, entry := range entries {
		out = append(out, entry.key+"="+entry.displayValue())
	}

	return out
}

// Equal reports whether e and other have the same entries.
func (e ExecEnv) Equal(other ExecEnv) bool {
	return slices.Equal(e.entries(), other.entries())
}

// String returns a redacted view of the environment.
func (e ExecEnv) String() string {
	entries := e.entries()
	parts := make([]string, 0, len(entries))

	for _, entry := range entries {
		parts = append(parts, entry.key+"="+entry.displayValue())
	}

	return strings.Join(parts, " ")
}

// LogValue returns a redacted slog representation of the environment.
func (e ExecEnv) LogValue() slog.Value {
	entries := e.entries()
	attrs := make([]slog.Attr, 0, len(entries))

	for _, entry := range entries {
		attrs = append(attrs, slog.String(entry.key, entry.displayValue()))
	}

	return slog.GroupValue(attrs...)
}

func (e ExecEnv) entries() []envVarData {
	if e.data == nil {
		return nil
	}

	return e.data.entries
}

func (e ExecEnv) withEntries(vars []envVarData) ExecEnv {
	if len(vars) == 0 {
		return e
	}

	entries := e.entries()
	index := make(map[string]int, len(entries)+len(vars))
	out := make([]envVarData, 0, len(entries)+len(vars))

	for _, entry := range entries {
		index[entry.key] = len(out)
		out = append(out, entry)
	}

	for _, entry := range vars {
		if entry.key == "" {
			continue
		}

		if i, ok := index[entry.key]; ok {
			out[i].value = entry.value
			out[i].sensitive = out[i].sensitive || entry.sensitive
			continue
		}

		index[entry.key] = len(out)
		out = append(out, entry)
	}

	return ExecEnv{data: &execEnvData{entries: out}}
}

func (v envVarData) displayValue() string {
	if v.sensitive {
		return sensitivePlaceholder
	}

	return v.value
}

func mergeEnv(old []string, new []string) []string {
	out := make([]string, 0, len(old)+len(new))
	index := make(map[string]int, len(old)+len(new))

	out = mergeEnvEntries(out, index, old)
	out = mergeEnvEntries(out, index, new)

	return out
}

func mergeEnvEntries(out []string, index map[string]int, entries []string) []string {
	for _, entry := range entries {
		key, ok := envEntryKey(entry)
		if !ok {
			continue
		}

		if i, ok := index[key]; ok {
			out[i] = entry
			continue
		}

		index[key] = len(out)
		out = append(out, entry)
	}

	return out
}

// envEntryKey returns the key of a KEY=VALUE entry. A key may begin with "=":
// Windows exposes per-drive working directories as entries like =C:=C:\dir, so
// the separator search starts after a single leading "=".
func envEntryKey(entry string) (string, bool) {
	start := 0
	if strings.HasPrefix(entry, "=") {
		start = 1
	}

	key, _, ok := strings.Cut(entry[start:], "=")
	if !ok {
		return "", false
	}

	if start == 1 {
		key = "=" + key
	}

	if key == "" {
		return "", false
	}

	return key, true
}
