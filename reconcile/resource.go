package reconcile

import (
	"context"
	"slices"
)

// ResourceBuilder builds a resource node: an optional drift probe (check) plus a
// mutation (apply) run only on drift, inside Do (so a dry run skips the
// mutation).
//
// A ResourceBuilder is immutable: every method returns a new builder.
type ResourceBuilder[T any] struct {
	name             string
	check            func(ctx context.Context) (bool, error)
	apply            func(ctx context.Context) (T, error)
	checkNeeds       []func(ctx context.Context) error
	applyNeeds       []func(ctx context.Context) error
	applyInvalidates []*node
	notify           []*Topic
}

// Resource starts a resource node. check is the optional drift probe (nil means
// always apply); apply is required and its result is the node's value.
func Resource[T any](name string, check func(ctx context.Context) (bool, error), apply func(ctx context.Context) (T, error)) *ResourceBuilder[T] {
	if apply == nil {
		panic("reconcile: resource " + name + ": Apply is required")
	}

	return &ResourceBuilder[T]{name: name, check: check, apply: apply}
}

// ResourceBool is a resource whose value reports whether it changed: apply runs
// (yielding true) only on drift, and a satisfied resource yields false. It
// is shorthand for Resource(name, check, func(ctx) (bool, error) { ... }).
func ResourceBool(name string, check func(ctx context.Context) (bool, error), apply func(ctx context.Context) error) *ResourceBuilder[bool] {
	return Resource(name, check, func(ctx context.Context) (bool, error) {
		if err := apply(ctx); err != nil {
			return false, err
		}

		return true, nil
	})
}

func (b *ResourceBuilder[T]) clone() *ResourceBuilder[T] {
	c := *b
	c.checkNeeds = slices.Clone(b.checkNeeds)
	c.applyNeeds = slices.Clone(b.applyNeeds)
	c.applyInvalidates = slices.Clone(b.applyInvalidates)
	c.notify = slices.Clone(b.notify)

	return &c
}

// CheckNeeds adds dependencies read before Check (unconditional). Use it when the
// probe itself needs the dependency in place. It accepts nodes and group covers.
// It returns a new builder.
func (b *ResourceBuilder[T]) CheckNeeds(refs ...NodeRef) *ResourceBuilder[T] {
	next := b.clone()

	for _, ref := range refs {
		next.checkNeeds = append(next.checkNeeds, ref.Evaluate)
	}

	return next
}

// ApplyNeeds adds dependencies read in the Apply path (lazily: only when the
// resource drifts, inside Do, so a dry run skips them). It accepts nodes and
// group covers. It returns a new builder.
func (b *ResourceBuilder[T]) ApplyNeeds(refs ...NodeRef) *ResourceBuilder[T] {
	next := b.clone()

	for _, ref := range refs {
		next.applyNeeds = append(next.applyNeeds, ref.Evaluate)
	}

	return next
}

// ApplyInvalidates records nodes cleared after this resource's Apply runs (inside
// Do, before Notify), so reads that alias the state it mutates re-evaluate on
// their next Value. A resource must not invalidate a node it reads, directly or
// indirectly: the engine rejects that with ErrConcurrentUse. It returns a new
// builder.
func (b *ResourceBuilder[T]) ApplyInvalidates(refs ...NodeRef) *ResourceBuilder[T] {
	next := b.clone()

	for _, ref := range refs {
		next.applyInvalidates = append(next.applyInvalidates, ref.nodeRef())
	}

	return next
}

// Notify adds topics marked dirty when the resource changes. It returns a new
// builder.
func (b *ResourceBuilder[T]) Notify(topics ...*Topic) *ResourceBuilder[T] {
	next := b.clone()
	next.notify = append(next.notify, topics...)

	return next
}

func (b *ResourceBuilder[T]) spec() resourceSpec[T] {
	return resourceSpec[T]{
		check:            b.check,
		apply:            b.apply,
		checkNeeds:       slices.Clone(b.checkNeeds),
		applyNeeds:       slices.Clone(b.applyNeeds),
		applyInvalidates: slices.Clone(b.applyInvalidates),
		notify:           slices.Clone(b.notify),
	}
}

// Node builds the node. The builder's configuration is snapshotted, so later
// changes do not affect the node.
func (b *ResourceBuilder[T]) Node() *Node[T] {
	name := b.name
	spec := b.spec()

	return NewNode(name, func(ctx context.Context) (T, error) {
		var zero T

		ok, err := spec.doCheck(ctx)
		if err != nil {
			reportCurrent(ctx, ReportFailed, err)
			return zero, err
		}
		if ok {
			reportCurrent(ctx, ReportOK, nil)
			return zero, nil
		}

		var out T

		err = Do(ctx, func() error {
			var applyErr error
			out, applyErr = spec.doApply(ctx)
			return applyErr
		})
		if err != nil {
			reportCurrent(ctx, ReportFailed, err)
			return out, err
		}

		if rs, ok := runFrom(ctx); ok && rs.dryRun {
			reportCurrent(ctx, ReportWouldChange, nil)
		} else {
			reportCurrent(ctx, ReportChanged, nil)
		}

		return out, nil
	})
}

type resourceSpec[T any] struct {
	check            func(ctx context.Context) (bool, error)
	apply            func(ctx context.Context) (T, error)
	checkNeeds       []func(ctx context.Context) error
	applyNeeds       []func(ctx context.Context) error
	applyInvalidates []*node
	notify           []*Topic
}

func (s resourceSpec[T]) doCheck(ctx context.Context) (bool, error) {
	for _, need := range s.checkNeeds {
		if err := need(ctx); err != nil {
			return false, err
		}
	}

	if s.check == nil {
		return false, nil
	}

	return s.check(ctx)
}

func (s resourceSpec[T]) doApply(ctx context.Context) (T, error) {
	var zero T

	for _, need := range s.applyNeeds {
		if err := need(ctx); err != nil {
			return zero, err
		}
	}

	out, err := s.apply(ctx)
	if err != nil {
		return zero, err
	}

	for _, ref := range s.applyInvalidates {
		if err := invalidate(ctx, ref); err != nil {
			return out, err
		}
	}

	for _, topic := range s.notify {
		if err := topic.Notify(ctx); err != nil {
			return out, err
		}
	}

	return out, nil
}
