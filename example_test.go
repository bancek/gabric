package gabric_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/logger"
)

func exampleContext() context.Context {
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return logger.NewLoggerContext(ctx, log)
}

func ExampleLocal() {
	ctx := exampleContext()

	gabric.Local(ctx).Exec("go", "version").Stdout(io.Discard).Run()
}

func ExampleTask() {
	ctx := exampleContext()

	g := gabric.New()

	hello := g.Task("hello", func(ctx context.Context) {
		fmt.Println("hello")
	})

	hello(ctx)
	// Output: hello
}
