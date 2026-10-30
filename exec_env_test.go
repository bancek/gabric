package gabric

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

func TestNewExecEnv(t *testing.T) {
	env := NewExecEnv(Env("B", "1"), Env("A", "2"), Env("B", "3"), EnvVar{})

	if got, want := env.Environ(), []string{"B=3", "A=2"}; !slices.Equal(got, want) {
		t.Errorf("Environ() = %v, want %v", got, want)
	}
	if got, ok := env.Get("B"); !ok || got != "3" {
		t.Errorf("Get(B) = %q, %v, want 3, true", got, ok)
	}
	if _, ok := env.Get("BAD"); ok {
		t.Error("Get(BAD) = true, want false")
	}
}

func TestParseExecEnv(t *testing.T) {
	env := ParseExecEnv("B=1", "A=2", "B=3", "BAD", "=bad")

	if got, want := env.Environ(), []string{"B=3", "A=2"}; !slices.Equal(got, want) {
		t.Errorf("Environ() = %v, want %v", got, want)
	}
}

func TestExecEnvWith(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), Env("B", "2")).
		With(Env("A", "3")).
		With(Env("C", "4"))

	if got, want := env.Environ(), []string{"A=3", "B=2", "C=4"}; !slices.Equal(got, want) {
		t.Errorf("Environ() = %v, want %v", got, want)
	}
}

func TestExecEnvSensitive(t *testing.T) {
	env := NewExecEnv(Env("TOKEN", "old")).
		MarkSensitive("TOKEN").
		With(Env("TOKEN", "new"))

	if !env.IsSensitive("TOKEN") {
		t.Error("IsSensitive(TOKEN) = false, want true")
	}
	if got, _ := env.Get("TOKEN"); got != "new" {
		t.Errorf("Get(TOKEN) = %q, want new", got)
	}
	if got, want := env.String(), "TOKEN=<sensitive>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if !slices.Contains(env.Environ(), "TOKEN=new") {
		t.Errorf("Environ() = %v, want it to contain the real value", env.Environ())
	}
}

func TestExecEnvMergeSensitive(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "base")).
		Merge(NewExecEnv(Env("A", "2"), Env("TOKEN", "override"), Env("B", "3")))

	if !env.IsSensitive("TOKEN") {
		t.Error("IsSensitive(TOKEN) = false, want sticky sensitivity")
	}
	if got, want := env.String(), "A=2 TOKEN=<sensitive> B=3"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestExecEnvWithBatch(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "old"), Env("B", "2")).
		With(
			Env("C", "3"),
			Env("A", "4"),
			Env("TOKEN", "new"),
			Env("C", "5"),
			EnvVar{},
			Env("", "ignored"),
		)

	if got, want := env.Environ(), []string{"A=4", "TOKEN=new", "B=2", "C=5"}; !slices.Equal(got, want) {
		t.Errorf("Environ() = %v, want %v", got, want)
	}
	if !env.IsSensitive("TOKEN") {
		t.Error("IsSensitive(TOKEN) = false, want sticky sensitivity")
	}
	if got, want := env.String(), "A=4 TOKEN=<sensitive> B=2 C=5"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestExecEnvWithBatchSensitivitySticky(t *testing.T) {
	env := NewExecEnv().With(SensitiveEnv("TOKEN", "secret"), Env("TOKEN", "override"))

	if !env.IsSensitive("TOKEN") {
		t.Error("IsSensitive(TOKEN) = false, want true")
	}
	if got, want := env.String(), "TOKEN=<sensitive>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestExecEnvWithBatchIsImmutable(t *testing.T) {
	base := NewExecEnv(Env("A", "1"), Env("B", "2"))
	next := base.With(Env("A", "3"), Env("C", "4"), Env("B", "5"))

	if got, want := base.Environ(), []string{"A=1", "B=2"}; !slices.Equal(got, want) {
		t.Errorf("base Environ() = %v, want %v", got, want)
	}
	if got, want := next.Environ(), []string{"A=3", "B=5", "C=4"}; !slices.Equal(got, want) {
		t.Errorf("next Environ() = %v, want %v", got, want)
	}
}

func TestExecEnvMergeBatchIsImmutable(t *testing.T) {
	base := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "base"), Env("B", "2"))
	other := NewExecEnv(Env("TOKEN", "override"), Env("C", "3"), Env("A", "4"))
	merged := base.Merge(other)

	if got, want := merged.Environ(), []string{"A=4", "TOKEN=override", "B=2", "C=3"}; !slices.Equal(got, want) {
		t.Errorf("merged Environ() = %v, want %v", got, want)
	}
	if !merged.IsSensitive("TOKEN") {
		t.Error("IsSensitive(TOKEN) = false, want sticky sensitivity")
	}
	if got, want := base.Environ(), []string{"A=1", "TOKEN=base", "B=2"}; !slices.Equal(got, want) {
		t.Errorf("base Environ() = %v, want %v", got, want)
	}
}

func TestExecEnvWithMany(t *testing.T) {
	const n = 1000

	vars := makeEnvVars(n)
	env := NewExecEnv(vars...)

	if got := env.Len(); got != n {
		t.Fatalf("Len() = %d, want %d", got, n)
	}

	updates := make([]EnvVar, 0, n)
	for i := range n {
		updates = append(updates, Env(fmt.Sprintf("KEY_%d", i), "updated"))
	}

	env = env.With(updates...)

	if got := env.Len(); got != n {
		t.Errorf("Len() after With() = %d, want %d", got, n)
	}
	if got, ok := env.Get("KEY_0"); !ok || got != "updated" {
		t.Errorf("Get(KEY_0) = %q, %v, want updated, true", got, ok)
	}
	if got, ok := env.Get(fmt.Sprintf("KEY_%d", n-1)); !ok || got != "updated" {
		t.Errorf("Get(KEY_%d) = %q, %v, want updated, true", n-1, got, ok)
	}
}

func TestParseExecEnvDuplicateOrder(t *testing.T) {
	env := ParseExecEnv("B=1", "A=2", "B=3", "C=4", "A=5")

	if got, want := env.Environ(), []string{"B=3", "A=5", "C=4"}; !slices.Equal(got, want) {
		t.Errorf("Environ() = %v, want %v", got, want)
	}
}

func TestExecEnvMarkSensitiveMissing(t *testing.T) {
	env := NewExecEnv(Env("A", "1"))
	if !env.MarkSensitive("MISSING").Equal(env) {
		t.Error("MarkSensitive(MISSING) changed the environment")
	}
}

func TestExecEnvVars(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))

	vars := env.Vars()
	if got := NewExecEnv(vars...); !got.Equal(env) {
		t.Errorf("NewExecEnv(env.Vars()...) = %v, want %v", got, env)
	}

	vars[0] = Env("A", "2")
	if got, _ := env.Get("A"); got != "1" {
		t.Errorf("Get(A) after mutating Vars() = %q, want 1", got)
	}
}

func TestExecEnvEnvironLog(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))

	if got, want := env.EnvironLog(), []string{"A=1", "TOKEN=<sensitive>"}; !slices.Equal(got, want) {
		t.Errorf("EnvironLog() = %v, want %v", got, want)
	}

	entries := env.EnvironLog()
	entries[0] = "A=2"

	if got, _ := env.Get("A"); got != "1" {
		t.Errorf("Get(A) after mutating EnvironLog() = %q, want 1", got)
	}
}

func TestExecEnvEnvironIsCopy(t *testing.T) {
	env := NewExecEnv(Env("A", "1"))

	entries := env.Environ()
	entries[0] = "A=2"

	if got, _ := env.Get("A"); got != "1" {
		t.Errorf("Get(A) = %q, want 1", got)
	}
}

func TestExecEnvEqual(t *testing.T) {
	a := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))
	b := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))

	if !a.Equal(b) {
		t.Error("Equal() = false, want true")
	}

	differentValue := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "other"))
	if a.Equal(differentValue) {
		t.Error("Equal() = true with a different value, want false")
	}

	differentSensitivity := NewExecEnv(Env("A", "1"), Env("TOKEN", "secret"))
	if a.Equal(differentSensitivity) {
		t.Error("Equal() = true with different sensitivity, want false")
	}
}

func TestExecEnvString(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))

	if got, want := env.String(), "A=1 TOKEN=<sensitive>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestExecEnvFormatIsOpaque(t *testing.T) {
	entry := SensitiveEnv("TOKEN", "secret")
	env := NewExecEnv(entry)

	for _, format := range []string{"%v", "%+v", "%#v"} {
		if got := fmt.Sprintf(format, entry); strings.Contains(got, "secret") {
			t.Errorf("fmt.Sprintf(%q, entry) = %q, want no secret", format, got)
		}
		if got := fmt.Sprintf(format, env); strings.Contains(got, "secret") {
			t.Errorf("fmt.Sprintf(%q, env) = %q, want no secret", format, got)
		}
	}
}

func TestExecEnvLogValue(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		log := slog.New(slog.NewTextHandler(&buf, nil))

		log.Info("env", "env", env)

		out := buf.String()
		if strings.Contains(out, "secret") {
			t.Errorf("log output = %q, want no secret", out)
		}
		if !strings.Contains(out, "A=1") || !strings.Contains(out, "sensitive") {
			t.Errorf("log output = %q, want non-sensitive value and a redacted marker", out)
		}
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		log := slog.New(slog.NewJSONHandler(&buf, nil))

		log.Info("env", "env", env)

		out := buf.String()
		if strings.Contains(out, "secret") {
			t.Errorf("log output = %q, want no secret", out)
		}
		if !strings.Contains(out, `"A":"1"`) || !strings.Contains(out, "sensitive") {
			t.Errorf("log output = %q, want non-sensitive value and a redacted marker", out)
		}
	})
}

func TestExecEnvLogValueNested(t *testing.T) {
	env := NewExecEnv(Env("A", "1"), SensitiveEnv("TOKEN", "secret"))
	payload := struct{ Env ExecEnv }{Env: env}

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	log.Info("nested", "payload", payload)

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output = %q, want no secret", out)
	}
	if !strings.Contains(out, `"Env":{}`) {
		t.Errorf("log output = %q, want the opaque env field to render as {}", out)
	}
}

func TestEnvVarLogValue(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	log.Info("entry", "entry", SensitiveEnv("TOKEN", "secret"))

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output = %q, want no secret", out)
	}
	if !strings.Contains(out, "sensitive") {
		t.Errorf("log output = %q, want a redacted marker", out)
	}
}

func TestMergeEnv(t *testing.T) {
	got := mergeEnv(
		[]string{"A=1", "BAD", "B=2", "A=3"},
		[]string{"B=4", "C=5", "=bad"},
	)
	want := []string{"A=3", "B=4", "C=5"}

	if !slices.Equal(got, want) {
		t.Errorf("mergeEnv() = %v, want %v", got, want)
	}
}

func TestMergeEnvEqualsPrefixedKey(t *testing.T) {
	got := mergeEnv(
		[]string{"=C:=C:\\old", "A=1"},
		[]string{"=C:=C:\\new", "B=2"},
	)
	want := []string{"=C:=C:\\new", "A=1", "B=2"}

	if !slices.Equal(got, want) {
		t.Errorf("mergeEnv() = %v, want %v", got, want)
	}
}

func makeEnvVars(n int) []EnvVar {
	vars := make([]EnvVar, 0, n)

	for i := range n {
		vars = append(vars, Env(fmt.Sprintf("KEY_%d", i), "value"))
	}

	return vars
}

func BenchmarkExecEnvWithMany(b *testing.B) {
	vars := makeEnvVars(1000)

	for b.Loop() {
		NewExecEnv(vars...)
	}
}

func BenchmarkExecEnvMergeMany(b *testing.B) {
	base := NewExecEnv(makeEnvVars(1000)...)
	other := NewExecEnv(makeEnvVars(1000)...)

	for b.Loop() {
		base.Merge(other)
	}
}

func BenchmarkParseExecEnvMany(b *testing.B) {
	entries := make([]string, 0, 1000)

	for i := range 1000 {
		entries = append(entries, fmt.Sprintf("KEY_%d=value", i))
	}

	for b.Loop() {
		ParseExecEnv(entries...)
	}
}
