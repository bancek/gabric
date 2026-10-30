package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/examples/apidemo"
	"github.com/bancek/gabric/gabricssh"
)

var Api = apidemo.Register(gabric.Default)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
