package logger

import (
	"context"
	"io"
	"log/slog"
)

type contextKey[T any] string

const loggerKey contextKey[*slog.Logger] = "logger.logger"

func contextValue[T any](ctx context.Context, key contextKey[T]) (T, bool) {
	value, ok := ctx.Value(key).(T)
	return value, ok
}

func withContextValue[T any](ctx context.Context, key contextKey[T], value T) context.Context {
	return context.WithValue(ctx, key, value)
}

// NewLoggerContext returns a context carrying logger.
func NewLoggerContext(ctx context.Context, logger *slog.Logger) context.Context {
	return withContextValue(ctx, loggerKey, logger)
}

// LoggerFromContext returns the logger from ctx, falling back to the default
// logger (slog.Default) when none is set.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := contextValue(ctx, loggerKey)
	if !ok || logger == nil {
		return slog.Default()
	}
	return logger
}

// HasLogger reports whether ctx carries a logger.
func HasLogger(ctx context.Context) bool {
	logger, ok := contextValue(ctx, loggerKey)

	return ok && logger != nil
}

// NewTextHandler returns a slog text handler that keeps the last value for
// duplicate attribute keys, matching the previous logrus field semantics (e.g.
// nested task and host fields).
func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &dedupHandler{inner: slog.NewTextHandler(w, opts)}
}

// prefixedAttr is an unresolved attribute together with the group prefix it was
// added under. Resolution and flattening are deferred to Handle, so values that
// implement slog.LogValuer (for example a lazily-resolved host) are evaluated
// when the record is logged rather than when With is called.
type prefixedAttr struct {
	prefix string
	attr   slog.Attr
}

// dedupHandler wraps a slog.Handler and keeps the last value for each
// attribute key. slog appends attributes on With, while logrus overwrote map
// fields; this restores the last-wins behaviour.
//
// Groups are flattened to dotted keys.
type dedupHandler struct {
	inner  slog.Handler
	prefix string
	attrs  []prefixedAttr
}

func (h *dedupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *dedupHandler) Handle(ctx context.Context, r slog.Record) error {
	base := make([]slog.Attr, 0, len(h.attrs))
	for _, pa := range h.attrs {
		base = append(base, flatten(pa.prefix, pa.attr)...)
	}

	var attrs []slog.Attr

	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, flatten(h.prefix, a)...)
		return true
	})

	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	out.AddAttrs(mergeAttrs(base, attrs)...)

	return h.inner.Handle(ctx, out)
}

func (h *dedupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &dedupHandler{
		inner:  h.inner,
		prefix: h.prefix,
		attrs:  append([]prefixedAttr(nil), h.attrs...),
	}

	for _, a := range attrs {
		next.attrs = append(next.attrs, prefixedAttr{prefix: h.prefix, attr: a})
	}

	return next
}

func (h *dedupHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	return &dedupHandler{
		inner:  h.inner,
		prefix: h.prefix + name + ".",
		attrs:  h.attrs,
	}
}

func flatten(prefix string, attr slog.Attr) []slog.Attr {
	attr.Value = attr.Value.Resolve()

	if attr.Value.Kind() == slog.KindGroup {
		groupPrefix := prefix
		if attr.Key != "" {
			groupPrefix += attr.Key + "."
		}

		var out []slog.Attr
		for _, a := range attr.Value.Group() {
			out = append(out, flatten(groupPrefix, a)...)
		}
		return out
	}

	return []slog.Attr{{Key: prefix + attr.Key, Value: attr.Value}}
}

// mergeAttrs merges attribute lists, keeping the position of the first
// occurrence of a key but the value of the last.
func mergeAttrs(groups ...[]slog.Attr) []slog.Attr {
	index := map[string]int{}
	var out []slog.Attr

	for _, g := range groups {
		for _, a := range g {
			if i, ok := index[a.Key]; ok {
				out[i] = a
				continue
			}

			index[a.Key] = len(out)
			out = append(out, a)
		}
	}

	return out
}
