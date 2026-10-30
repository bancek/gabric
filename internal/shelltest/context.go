package shelltest

import "context"

type state struct {
	sudo *Sudo
	env  []string
}

type stateKeyType int

var stateKey stateKeyType

func withSudo(ctx context.Context, sudo *Sudo) context.Context {
	next := &state{sudo: sudo}

	if current, ok := ctx.Value(stateKey).(*state); ok {
		*next = *current
		next.sudo = sudo
	}

	return context.WithValue(ctx, stateKey, next)
}

func sudoFromContext(ctx context.Context) *Sudo {
	current, ok := ctx.Value(stateKey).(*state)
	if !ok {
		return nil
	}

	return current.sudo
}

func withEnv(ctx context.Context, env []string) context.Context {
	next := &state{env: env}

	if current, ok := ctx.Value(stateKey).(*state); ok {
		*next = *current
		next.env = env
	}

	return context.WithValue(ctx, stateKey, next)
}

func envFromContext(ctx context.Context) []string {
	current, ok := ctx.Value(stateKey).(*state)
	if !ok {
		return nil
	}

	return current.env
}
