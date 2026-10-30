package reconcile

import (
	"context"
	"slices"
)

// GroupBuilder builds a group: a container that reads its members in order. A
// group produces no value, is never memoized, and does not take part in the
// dependency/invalidation graph; reading it runs its members every time (the
// members are memoized value nodes). To reset a group, start a new Run.
//
// A GroupBuilder is immutable: every method returns a new builder.
type GroupBuilder struct {
	name    string
	members []func(ctx context.Context) error
}

// Group starts a group builder.
func Group(name string) *GroupBuilder {
	return &GroupBuilder{name: name}
}

func (b *GroupBuilder) clone() *GroupBuilder {
	c := *b
	c.members = slices.Clone(b.members)

	return &c
}

// Contains adds the nodes to run, in order, each time the group is read. It
// accepts nodes and types embedding one, such as group covers. It returns a new
// builder.
func (b *GroupBuilder) Contains(refs ...NodeRef) *GroupBuilder {
	next := b.clone()

	for _, ref := range refs {
		next.members = append(next.members, ref.Evaluate)
	}

	return next
}

// Node builds the group node. The builder's configuration is snapshotted, so
// later changes to the builder do not affect the returned node.
func (b *GroupBuilder) Node() *Node[Void] {
	name := b.name
	members := slices.Clone(b.members)

	n := NewNode(name, func(ctx context.Context) (Void, error) {
		for _, member := range members {
			if err := member(ctx); err != nil {
				return Void{}, err
			}
		}

		return Void{}, nil
	})
	n.group = true

	return n
}
