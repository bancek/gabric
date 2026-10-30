package main

import (
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/ufw"
	"github.com/bancek/gabric/reconcile/modules/ufwdocker"
)

// ufwDockerRef pins ufw-docker to a git commit (tags can move). This is the
// master revision that includes the `install-service` subcommand.
const ufwDockerRef = "020a8699f95592561f254d8d4ad1bb40d401dfc7"

// ufwFirewall is the ufwFirewall cover: allow ssh before default-deny + enable. The cover
// applies rules first, then the default policies, then enables the ufwFirewall.
var ufwFirewall = ufw.Firewall().
	Rule(ufw.Rule().Port(S("22")).Proto(S("tcp"))).
	Incoming(S("deny")).
	Outgoing(S("allow")).
	Enabled().
	Build()

var _ = ufwdocker.RestartEffect()

// ufwDocker installs ufw-docker (script + install + install-service). Docker
// bypasses UFW, so this stub gates published container ports via route rules.
var ufwDocker = ufwdocker.UfwDocker(S(ufwDockerRef)).Build()

var firewallNode = reconcile.Group("firewall").
	Contains(ufwFirewall.Node).
	Contains(ufwDocker.Node).
	Node()

var Firewall = base.NodeTask(firewallNode)
