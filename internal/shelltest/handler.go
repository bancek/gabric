package shelltest

import (
	"context"
)

// Handler runs an external command.
type Handler struct {
	Run         func(ctx context.Context, cmd Command) error
	Transparent bool
}

// Register adds a handler for name. It replaces an existing handler.
func (c *Core) Register(name string, handler Handler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()

	c.handlers[name] = handler
}

func (c *Core) handler(name string) (Handler, bool) {
	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()

	handler, ok := c.handlers[name]

	return handler, ok
}
