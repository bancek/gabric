package main

import (
	"context"

	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/git"
)

// appRepo clones and updates the application source. Here it is traefik/whoami,
// a tiny HTTP echo server that reports the request and the container that
// served it, and a stand-in for your own app. LocalRevision is the checked-out
// commit.
var appRepo = git.Repo(
	S("https://github.com/traefik/whoami.git"),
	S("/srv/app/app"),
).Build()

// appComposeData combines the checked-out revision with the site config. The
// revision lands in a service label, so a new upstream commit changes Compose's
// config hash and `up` recreates the container.
var appComposeData = reconcile.Data("app-compose-data", func(ctx context.Context) (any, error) {
	rev, err := appRepo.LocalRevision.Value(ctx)
	if err != nil {
		return nil, err
	}

	s, err := site.Value(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"Revision": rev,
		"Domain":   s.Domain,
		"TLS":      s.TLS,
	}, nil
}).Node()

var appComposeFile = modules.File(S("/srv/app/compose.yaml")).
	Content(modules.TemplateData(
		S("/srv/app/compose.yaml"),
		S(appComposeTemplate),
		appComposeData,
	)).
	Mode(modules.FileMode(0o644)).
	Build().
	Node()

// appEnv decrypts the app's env file before Compose reads it.
var appEnv = secretFile("/srv/app/.env", secretsDir+"/app.env.age")

// appCompose needs the reconciled env file (env_file) before it can start.
var appCompose = docker.Compose(S("/srv/app/compose.yaml")).
	BuildImages().
	Build().
	ApplyNeeds(appComposeFile).
	CheckNeeds(appEnv).
	Node()

var appNode = reconcile.Group("app").
	Contains(modules.Dir(S("/srv/app")).Build().Node()).
	Contains(appRepo.Node).
	Contains(appEnv).
	Contains(appComposeFile, appCompose).
	Node()

var App = base.NodeTask(appNode)
