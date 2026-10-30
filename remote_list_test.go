package gabric

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/bancek/gabric/internal/shelltest"
	"mvdan.cc/sh/v3/interp"
)

func TestRemoteListShell(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	core.Register("ls", shelltest.Handler{
		Run: func(ctx context.Context, _ shelltest.Command) error {
			_, err := io.WriteString(interp.HandlerCtx(ctx).Stdout, "b.txt\nadir/\n.hidden\n")

			return err
		},
	})

	entries := Remote(ctx).List("/srv/app")

	asserts.AssertExecuted("ls", "-1Ap", "/srv/app/")

	want := []ListEntry{
		{Name: ".hidden"},
		{Name: "adir", IsDir: true},
		{Name: "b.txt"},
	}

	if !slices.Equal(entries, want) {
		t.Errorf("List() = %+v, want %+v", entries, want)
	}
}

func TestRemoteListErr(t *testing.T) {
	core, asserts, ctx := localRemoteContext(t)

	core.Register("ls", shelltest.Handler{
		Run: func(context.Context, shelltest.Command) error {
			return interp.ExitStatus(1)
		},
	})

	if _, err := Remote(ctx).ListErr("/srv/app"); err == nil {
		t.Error("ListErr() error = nil, want error")
	}

	asserts.AssertExecuted("ls", "-1Ap", "/srv/app/")
}

func TestRemoteListPanics(t *testing.T) {
	core, _, ctx := localRemoteContext(t)

	core.Register("ls", shelltest.Handler{
		Run: func(context.Context, shelltest.Command) error {
			return interp.ExitStatus(1)
		},
	})

	defer func() {
		if recover() == nil {
			t.Error("List() did not panic")
		}
	}()

	Remote(ctx).List("/srv/app")
}
