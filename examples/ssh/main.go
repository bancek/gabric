package main

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

var Ssh = gabricssh.RegisterSshTask(gabric.Default)

var Forward = gabricssh.RegisterForwardTask(gabric.Default)

var HelloLocal = gabric.Task("hello_local", func(ctx context.Context) {
	gabric.Local(ctx).Exec("echo", "Hello local").Run()
})

var HelloRemote = gabric.Task("hello_remote", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("uname", "-a").Run()
})

var Hello = gabric.Task("hello", func(ctx context.Context) {
	HelloLocal(ctx)
	HelloRemote(ctx)
})

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
