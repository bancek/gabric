// Package middleware defines the context middleware shape shared by gabric
// and reconcile. It has no dependencies beyond the standard library.
package middleware

import (
	"context"
	"slices"
)

// Next is the function a middleware wraps. It is also the shape of a task and
// of a dispatched run.
type Next = func(ctx context.Context) error

// Middleware wraps a Next. It may call next zero or more times, with the same
// or a derived context, and return its error. Returning nil without calling
// next short-circuits.
type Middleware = func(ctx context.Context, next Next) error

// Apply applies middlewares to next, outermost first.
//
// Example:
//
//	func Middleware1(ctx context.Context, next middleware.Next) error {
//		fmt.Println("Middleware 1")
//		return next(ctx)
//	}
//
//	func Middleware2(ctx context.Context, next middleware.Next) error {
//		fmt.Println("Middleware 2")
//		return next(ctx)
//	}
//
//	var run = middleware.Apply(
//		[]middleware.Middleware{Middleware1, Middleware2},
//		func(ctx context.Context) error {
//			fmt.Println("RUN")
//			return nil
//		},
//	)
func Apply(middlewares []Middleware, next Next) Next {
	for _, middleware := range slices.Backward(middlewares) {
		prev := next
		next = func(ctx context.Context) error {
			return middleware(ctx, prev)
		}
	}

	return next
}
