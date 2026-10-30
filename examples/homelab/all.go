package main

import (
	"github.com/bancek/gabric/reconcile"
)

// allNode converges the whole host. Each service is its own group; reading it
// evaluates (and memoizes) every part, and the shared run makes nodes evaluated
// by several services run only once.
var allNode = reconcile.Group("all").
	Contains(
		bootstrapNode,
		userNode,
		dockerNode,
		firewallNode,
		wwwNode,
		traefikNode,
		appNode,
		wireguardNode,
	).
	Node()

var All = base.NodeTask(allNode)
