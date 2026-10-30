// Package input provides lazily resolved values for module builders.
package input

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Input is a lazily read value. A *reconcile.Node[T] satisfies it. Static values
// use Static, so builders can accept both a constant and a value read during a
// run.
type Input[T any] interface {
	Value(ctx context.Context) (T, error)
	Name() string
}

type staticInput[T any] struct {
	value T
	name  string
}

func (s staticInput[T]) Value(context.Context) (T, error) {
	return s.value, nil
}

func (s staticInput[T]) Name() string {
	return s.name
}

// Static returns an input for a constant value. It does not create a run node:
// it has no memoization, dependency edge or invalidation.
func Static[T any](value T) Input[T] {
	return staticInput[T]{value: value, name: staticName(value)}
}

// List returns a static input for a list of values.
func List[T any](values ...T) Input[[]T] {
	return Static(values)
}

// Value returns in's value, or the zero value when in is nil.
func Value[T any](ctx context.Context, in Input[T]) (T, error) {
	var zero T

	if in == nil {
		return zero, nil
	}

	return in.Value(ctx)
}

// ValueOr returns in's value, or fallback when in is nil.
func ValueOr[T any](ctx context.Context, in Input[T], fallback T) (T, error) {
	if in == nil {
		return fallback, nil
	}

	return in.Value(ctx)
}

type mappedInput[T, U any] struct {
	in   Input[T]
	name string
	fn   func(ctx context.Context, value T) (U, error)
}

func (m mappedInput[T, U]) Value(ctx context.Context) (U, error) {
	var zero U

	v, err := m.in.Value(ctx)
	if err != nil {
		return zero, err
	}

	return m.fn(ctx, v)
}

func (m mappedInput[T, U]) Name() string {
	return m.name
}

// WithName returns an input that reads in but reports name from Name. It
// changes only the name, not the value or the input's identity.
func WithName[T any](in Input[T], name string) Input[T] {
	if in == nil {
		panic("input: WithName: nil input")
	}
	if name == "" {
		panic("input: WithName: empty name")
	}

	return namedInput[T]{in: in, name: name}
}

type namedInput[T any] struct {
	in   Input[T]
	name string
}

func (n namedInput[T]) Value(ctx context.Context) (T, error) {
	return n.in.Value(ctx)
}

func (n namedInput[T]) Name() string {
	return n.name
}

// Map returns an input whose value is fn applied to in's value. The result is
// not memoized; a node input is still evaluated once per run, so the transform
// runs again only when the mapped input is read again.
func Map[T, U any](in Input[T], fn func(ctx context.Context, value T) (U, error)) Input[U] {
	if in == nil {
		panic("input: Map: nil input")
	}

	return mappedInput[T, U]{
		in:   in,
		name: in.Name() + ":map",
		fn:   fn,
	}
}

// Flag returns the single bool input passed to a behavior flag, or True when it
// was called with no argument. It panics when given more than one input or a
// nil input.
func Flag(flags ...Input[bool]) Input[bool] {
	switch len(flags) {
	case 0:
		return True
	case 1:
		if flags[0] == nil {
			panic("input: nil bool input")
		}

		return flags[0]
	default:
		panic("input: expected at most one bool input")
	}
}

// True and False are static bool inputs for behavior flags.
var (
	True  = Static(true)
	False = Static(false)
)

func staticName[T any](value T) string {
	switch v := any(value).(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case []byte:
		return "bytes"
	case []string:
		return truncateName(strings.Join(v, ","), 64)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%T", v)
	}
}

func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}

	return name[:maxLen-3] + "..."
}
