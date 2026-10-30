package main

import (
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/apt"
	"github.com/bancek/gabric/reconcile/modules/linux"
	"github.com/bancek/gabric/reconcile/modules/pkg"
	"github.com/bancek/gabric/reconcile/modules/service"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

// sshTopic is notified when the sshd hardening file changes.
var sshTopic = reconcile.NewTopic("ssh")

// Restart ssh once, only when something notified sshTopic.
var _ = systemd.RestartEffect(S("ssh"), sshTopic)

// apt-get update is a node; package nodes read it lazily with ApplyNeeds, so it
// runs only when a package drifts.
var aptGetUpdate = apt.Update().Build().Node()

// rsyncPkg is also used by deploy-local before the bootstrap has run.
var rsyncPkg = pkg.Package(S("rsync")).Build().ApplyNeeds(aptGetUpdate).Node()

var bootstrapPackages = []reconcile.NodeRef{
	pkg.Package(S("ufw")).Build().ApplyNeeds(aptGetUpdate).Node(),
	pkg.Package(S("fail2ban")).Build().ApplyNeeds(aptGetUpdate).Node(),
	pkg.Package(S("unattended-upgrades")).Build().ApplyNeeds(aptGetUpdate).Node(),
	pkg.Package(S("ca-certificates")).Build().ApplyNeeds(aptGetUpdate).Node(),
	pkg.Package(S("curl")).Build().ApplyNeeds(aptGetUpdate).Node(),
	rsyncPkg,
	pkg.Package(S("git")).Build().ApplyNeeds(aptGetUpdate).Node(),
	pkg.Package(S("age")).Build().ApplyNeeds(aptGetUpdate).Node(),
}

// sshdHardeningConf is a File resource: it probes the remote file and rewrites
// it only on drift, notifying sshTopic when it changes.
var sshdHardeningConf = modules.File(S("/etc/ssh/sshd_config.d/99-hardening.conf")).
	Content(S([]byte(sshdHardening))).
	Mode(modules.FileMode(0o644)).
	Build().Notify(sshTopic).Node()

var autoRebootConf = modules.File(S("/etc/apt/apt.conf.d/51-auto-reboot")).
	Content(S([]byte(autoReboot))).
	Mode(modules.FileMode(0o644)).
	Build().Node()

var ipForward = linux.Sysctl(S("net.ipv4.ip_forward"), S("1")).Node()
var srcValidMark = linux.Sysctl(S("net.ipv4.conf.all.src_valid_mark"), S("1")).Node()

var unattendedUpgradesUnit = service.Service(S("unattended-upgrades")).Build().Node()
var fail2banUnit = service.Service(S("fail2ban")).Started().Build().Node()

var bootstrapNode = reconcile.Group("bootstrap").
	Contains(bootstrapPackages...).
	Contains(sshdHardeningConf, autoRebootConf).
	Contains(ipForward, srcValidMark).
	Contains(unattendedUpgradesUnit, fail2banUnit).
	Node()

var Bootstrap = base.NodeTask(bootstrapNode)
