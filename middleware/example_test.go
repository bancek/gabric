package middleware_test

import (
	"context"
	"fmt"

	"github.com/bancek/gabric/middleware"
)

func ExampleApply() {
	task := middleware.Apply(
		[]middleware.Middleware{
			func(ctx context.Context, next middleware.Next) error {
				fmt.Println("before")
				err := next(ctx)
				fmt.Println("after")
				return err
			},
		},
		func(ctx context.Context) error {
			fmt.Println("task")
			return nil
		},
	)

	_ = task(context.Background())
	// Output:
	// before
	// task
	// after
}
