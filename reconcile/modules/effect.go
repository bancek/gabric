package modules

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

// ExecEffectBuilder builds an effect that runs a command when any subscribed
// topic is notified.
type ExecEffectBuilder struct {
	name     string
	topics   []*reconcile.Topic
	cmdInput input.Input[[]string]
}

// ExecEffect starts an effect builder. Chain Topics and Exec, then Effect.
func ExecEffect(name string) *ExecEffectBuilder {
	return &ExecEffectBuilder{name: name}
}

// Topics subscribes the effect to topics. It returns the builder.
func (b *ExecEffectBuilder) Topics(topics ...*reconcile.Topic) *ExecEffectBuilder {
	b.topics = append(b.topics, topics...)
	return b
}

// Exec sets the command to run. It returns the builder.
func (b *ExecEffectBuilder) Exec(cmdInput input.Input[[]string]) *ExecEffectBuilder {
	b.cmdInput = cmdInput
	return b
}

// Effect builds the effect. It panics if no topics or no command were set.
func (b *ExecEffectBuilder) Effect() *reconcile.Effect {
	if len(b.topics) == 0 {
		panic("modules: effect " + b.name + ": no topics set (use Topics)")
	}

	if b.cmdInput == nil {
		panic("modules: effect " + b.name + ": no command set (use Exec)")
	}

	cmdInput := b.cmdInput

	return reconcile.NewEffect(b.name, b.topics, func(ctx context.Context) error {
		cmd, err := input.Value(ctx, cmdInput)
		if err != nil {
			return err
		}
		if len(cmd) == 0 {
			return nil
		}

		return gabric.TargetOps(ctx).Exec(cmd...).RunErr()
	})
}
