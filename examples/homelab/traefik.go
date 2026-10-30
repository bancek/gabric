package main

import (
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/ufw"
)

// traefikComposeFile renders the traefik compose: HTTPS (ACME) is added only
// when the site config has TLS enabled.
var traefikComposeFile = modules.File(S("/srv/traefik/compose.yaml")).
	Content(modules.TemplateData(
		S("/srv/traefik/compose.yaml"),
		S(traefikComposeTemplate),
		site,
	)).
	Mode(modules.FileMode(0o644)).
	Build().
	Node()

var traefikNode = reconcile.Group("traefik").
	Contains(
		modules.Dir(S("/srv/traefik")).Build().Node(),
		traefikComposeFile,
		docker.Compose(S("/srv/traefik/compose.yaml")).Build().ApplyNeeds(traefikComposeFile).Node(),
	).
	Contains(
		// traefik publishes 80/443; ufw-docker gates published ports.
		ufwFirewall.AddRule(ufw.Rule().Route().Port(S("80")).Proto(S("tcp"))),
		ufwFirewall.AddRule(ufw.Rule().Route().Port(S("443")).Proto(S("tcp"))),
	).
	Node()

var Traefik = base.NodeTask(traefikNode)
