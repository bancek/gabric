package reconcile

import (
	"context"
	"github.com/bancek/gabric/dryrun"
	"testing"
)

type testGroupCover struct {
	*Node[Void]
}

func TestGroupContainsCover(t *testing.T) {
	evals := 0

	member := Data("member", func(ctx context.Context) (Void, error) {
		evals++
		return Void{}, nil
	}).Node()
	cover := &testGroupCover{Node: member}

	group := Group("g").Contains(cover).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := group.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals != 1 {
		t.Errorf("member evals = %d, want 1", evals)
	}
}

func TestGroupReads(t *testing.T) {
	evals := map[string]int{}

	a := Data("a", func(ctx context.Context) (Void, error) { evals["a"]++; return Void{}, nil }).Node()
	b := Data("b", func(ctx context.Context) (Void, error) { evals["b"]++; return Void{}, nil }).Node()

	group := Group("g").Contains(a, b).Node()

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		_, err := group.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals["a"] != 1 || evals["b"] != 1 {
		t.Errorf("evals = %v, want a=1 b=1 (in dry run)", evals)
	}
}

// TestGroupNotMemoized checks that a group runs on every read; its nodes are
// memoized value nodes.
func TestGroupNotMemoized(t *testing.T) {
	evals := map[string]int{}

	a := NewNode("a", func(ctx context.Context) (int, error) { evals["a"]++; return 1, nil })
	b := NewNode("b", func(ctx context.Context) (int, error) { evals["b"]++; return 2, nil })

	group := Group("g").Contains(a, b).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := group.Value(ctx); err != nil {
			return err
		}

		_, err := group.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals["a"] != 1 || evals["b"] != 1 {
		t.Errorf("evals = %v, want a=1 b=1 (members are memoized)", evals)
	}
}

// TestGroupInvalidateDoesNotClearMembers checks that a group is not an owner:
// invalidating it does not clear its nodes.
func TestGroupInvalidateDoesNotClearMembers(t *testing.T) {
	evals := 0

	member := NewNode("member", func(ctx context.Context) (int, error) { evals++; return 1, nil })

	group := Group("g").Contains(member).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := group.Value(ctx); err != nil {
			return err
		}

		if err := group.Invalidate(ctx); err != nil {
			return err
		}

		_, err := group.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals != 1 {
		t.Errorf("member evaluated %d times, want 1 (the group does not own it)", evals)
	}
}

// TestInvalidateMemberRereadByGroup checks that invalidating a node makes the
// next group read re-evaluate it (the group is not memoized).
func TestInvalidateMemberRereadByGroup(t *testing.T) {
	evals := 0

	member := NewNode("member", func(ctx context.Context) (int, error) { evals++; return 1, nil })

	group := Group("g").Contains(member).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := group.Value(ctx); err != nil {
			return err
		}

		if err := member.Invalidate(ctx); err != nil {
			return err
		}

		_, err := group.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals != 2 {
		t.Errorf("member evaluated %d times, want 2", evals)
	}
}
