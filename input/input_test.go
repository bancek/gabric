package input_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

var _ input.Input[string] = (*reconcile.Node[string])(nil)

func TestStatic(t *testing.T) {
	in := input.Static("value")

	if got, want := in.Name(), "value"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	got, err := in.Value(context.Background())
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if got != "value" {
		t.Errorf("Value() = %q, want value", got)
	}
}

func TestList(t *testing.T) {
	in := input.List("a", "b")

	if got, want := in.Name(), "a,b"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	got, err := in.Value(context.Background())
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if want := []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("Value() = %v, want %v", got, want)
	}
}

func TestTrueFalse(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   input.Input[bool]
		want bool
	}{
		{"true", input.True, true},
		{"false", input.False, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Value(context.Background())
			if err != nil {
				t.Fatalf("Value() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlag(t *testing.T) {
	if got, err := input.Flag().Value(context.Background()); err != nil || !got {
		t.Errorf("Flag() = %v, %v; want true, nil", got, err)
	}
	if got, err := input.Flag(input.False).Value(context.Background()); err != nil || got {
		t.Errorf("Flag(False) = %v, %v; want false, nil", got, err)
	}
}

func TestFlagPanics(t *testing.T) {
	tests := []struct {
		name  string
		flags []input.Input[bool]
	}{
		{"nil", []input.Input[bool]{nil}},
		{"more than one", []input.Input[bool]{input.True, input.False}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic for %s", tt.name)
				}
			}()

			input.Flag(tt.flags...)
		})
	}
}

func TestValue(t *testing.T) {
	got, err := input.Value(context.Background(), input.Static("value"))
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if got != "value" {
		t.Errorf("Value() = %q, want value", got)
	}
}

func TestValueNil(t *testing.T) {
	var in input.Input[string]

	got, err := input.Value(context.Background(), in)
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if got != "" {
		t.Errorf("Value() = %q, want empty", got)
	}
}

func TestValueOrNil(t *testing.T) {
	var in input.Input[string]

	got, err := input.ValueOr(context.Background(), in, "fallback")
	if err != nil {
		t.Fatalf("ValueOr() error: %v", err)
	}
	if got != "fallback" {
		t.Errorf("ValueOr() = %q, want fallback", got)
	}
}

func TestWithName(t *testing.T) {
	in := input.WithName(input.Static("value"), "custom")

	if got, want := in.Name(), "custom"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	got, err := in.Value(context.Background())
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if got != "value" {
		t.Errorf("Value() = %q, want value", got)
	}
}

func TestWithNameNilPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for a nil input")
		}
	}()

	input.WithName[string](nil, "custom")
}

func TestWithNameEmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for an empty name")
		}
	}()

	input.WithName(input.Static("value"), "")
}

func TestMap(t *testing.T) {
	in := input.Map(input.Static(2), func(_ context.Context, v int) (string, error) {
		return strconv.Itoa(v * 2), nil
	})

	if got, want := in.Name(), "int:map"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	got, err := in.Value(context.Background())
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if got != "4" {
		t.Errorf("Value() = %q, want 4", got)
	}
}

func TestValueNode(t *testing.T) {
	node := reconcile.Data("node", func(context.Context) (string, error) {
		return "node", nil
	}).Node()

	err := reconcile.Run(context.Background(), func(ctx context.Context) error {
		got, err := input.Value(ctx, node)
		if err != nil {
			return err
		}
		if got != "node" {
			t.Errorf("Value() = %q, want node", got)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestValueNodeError(t *testing.T) {
	sentinel := errors.New("boom")
	node := reconcile.Data("node", func(context.Context) (string, error) {
		return "", sentinel
	}).Node()

	err := reconcile.Run(context.Background(), func(ctx context.Context) error {
		_, err := input.Value(ctx, node)
		return err
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("Value() error = %v, want %v", err, sentinel)
	}
}
