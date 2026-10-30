package main

import (
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/linux"
)

// deployUser is the login account. The committed authorized key is a throwaway
// example: replace it (or skip this task) when converging a real host.
// ApplyNeeds installs docker (and so creates the docker group) only when the
// account drifts.
var deployUser = linux.User(S("deploy")).
	Shell(S("/bin/bash")).
	Groups(input.List("docker")).
	Build().
	ApplyNeeds(dockerNode).
	Node()

// deployKeys reads deployUser first, so the account exists before its home is
// used.
var deployKeys = linux.SSHAuthorizedKeys(S("deploy"), input.List(deployAuthorizedKeysFile))

var userNode = reconcile.Group("user").
	Contains(deployUser, deployKeys).
	Node()

var User = base.NodeTask(userNode)
