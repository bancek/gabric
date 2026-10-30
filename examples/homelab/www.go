package main

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/rsync"
)

var wwwTopic = reconcile.NewTopic("www")

// wwwComposeFile renders the nginx compose (traefik router labels depend on the
// site config).
var wwwComposeFile = modules.File(S("/srv/www/compose.yaml")).
	Content(modules.TemplateData(
		S("/srv/www/compose.yaml"),
		S(wwwComposeTemplate),
		site,
	)).
	Mode(modules.FileMode(0o644)).
	Build().
	Node()

// wwwNode deploys the nginx config, static files and compose file, then brings
// the project up. The config rsync notifies wwwTopic when it changes.
var wwwNode = reconcile.Group("www").
	Contains(
		modules.Dir(S("/srv/www")).Build().Node(),
		rsync.Sync(S("./srv/www/conf.d/")).Remote(S("/srv/www/conf.d")).Build().Notify(wwwTopic).Node(),
		wwwComposeFile,
		rsync.Sync(S("./srv/www/www/")).Remote(S("/srv/www/www")).Build().Node(),
		docker.Compose(S("/srv/www/compose.yaml")).Build().ApplyNeeds(wwwComposeFile).Node(),
	).
	Node()

var Www = base.NodeTask(wwwNode)

// Reload nginx when the config changed (no-op if the container is not running).
var _ = reconcile.NewEffect("www", []*reconcile.Topic{wwwTopic}, func(ctx context.Context) error {
	running, err := composeServiceRunning(ctx, "/srv/www", "www")
	if err != nil {
		return err
	}
	if !running {
		return nil
	}

	return gabric.TargetOps(ctx).Exec("docker", "compose", "-f", "/srv/www/compose.yaml",
		"exec", "-T", "www", "nginx", "-s", "reload").RunErr()
})

func composeServiceRunning(ctx context.Context, dir, service string) (bool, error) {
	e := gabric.TargetOps(ctx).Exec("docker", "compose", "-f", dir+"/compose.yaml", "ps", "-q", service).Capture()
	if err := e.RunErr(); err != nil {
		return false, err
	}

	return strings.TrimSpace(e.StdoutStr()) != "", nil
}
