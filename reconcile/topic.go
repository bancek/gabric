package reconcile

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// Topic is a valueless change signal. Nodes Notify it when they change; effects
// subscribe to it.
//
// mu guards effects, which may be appended by NewEffect concurrently with a
// Flush reading them.
type Topic struct {
	name    string
	mu      sync.RWMutex
	effects []*Effect
}

// NewTopic creates a topic with the given name.
func NewTopic(name string) *Topic {
	return &Topic{name: name}
}

// Name returns the topic name.
func (t *Topic) Name() string {
	return t.name
}

// effects returns a snapshot of the topic's effects.
func (t *Topic) snapshotEffects() []*Effect {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return slices.Clone(t.effects)
}

// subscribe registers an effect with the topic.
func (t *Topic) subscribe(e *Effect) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.effects = append(t.effects, e)
}

// replaceEffect replaces old with next in the topic's subscriptions. It is a
// no-op when old is not subscribed.
func (t *Topic) replaceEffect(old, next *Effect) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, e := range t.effects {
		if e == old {
			t.effects[i] = next
		}
	}
}

// Notify marks the topic dirty for the current run. Repeated notifications
// before a flush collapse into a single one. It is safe to call Notify
// concurrently from different node evaluations. It returns ErrNoRun outside a
// Run.
func (t *Topic) Notify(ctx context.Context) error {
	rs, ok := runFrom(ctx)
	if !ok {
		return fmt.Errorf("reconcile: %w", ErrNoRun)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.dirty[t] {
		return nil
	}

	rs.dirty[t] = true
	rs.dirtyOrder = append(rs.dirtyOrder, t)

	return nil
}
