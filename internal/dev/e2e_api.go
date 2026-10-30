package main

import (
	"context"

	"github.com/bancek/gabric"
)

var E2EAPIDemo = gabric.Task("e2e-api-demo", func(ctx context.Context) {
	gabric.Local(ctx).
		Exec("go", "test", "./internal/apidemo2e", "-count=1").
		WithEnv(gabric.Env("GABRIC_E2E", "1")).
		Run()
})
