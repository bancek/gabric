package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/examples/apidemo"
	"github.com/bancek/gabric/gabricdocker"
)

var Api = apidemo.Register(gabric.Default)

func main() {
	gabricdocker.InstallDefault()
	gabric.Main()
}
