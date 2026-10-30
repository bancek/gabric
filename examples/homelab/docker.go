package main

import (
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

var dockerTopic = reconcile.NewTopic("docker")

var _ = systemd.RestartEffect(S("docker"), dockerTopic)

var dockerEngine = docker.DockerEngine().Build()

var dockerDaemonConf = modules.File(S("/etc/docker/daemon.json")).
	Content(S([]byte(dockerDaemon))).
	Mode(modules.FileMode(0o644)).
	Build().Notify(dockerTopic).Node()

var webNetwork = docker.Network(S("web")).Node()

var dockerNode = reconcile.Group("docker").
	Contains(dockerEngine.Node).
	Contains(dockerDaemonConf, webNetwork).
	Node()

var Docker = base.NodeTask(dockerNode)
