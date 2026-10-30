// Package modules provides the base Reconcile modules for files, directories,
// templates, commands, effects, and file/data reads.
//
// Domain-specific modules live in subpackages:
//
//   - linux: users, authorized keys, sysctl
//   - ufw: the UFW firewall
//   - ufwdocker: the ufw-docker integration
//   - systemd: systemd services
//   - apt: Debian/Ubuntu packages
//   - git: git repositories
//   - docker: Docker networks and Compose
//   - httpfile: HTTP file downloads
//   - github: pinned GitHub raw downloads
//   - rsync: rsync uploads
//
// A module is a builder that configures the resource and then hands off to the
// reconcile node builder with Build, on which Notify, CheckNeeds/ApplyNeeds,
// ApplyInvalidates and Node are called:
//
//	modules.File("/etc/ssh/sshd_config.d/99-hardening.conf").
//		Content(hardening).Mode(0o644).Build().Notify(sshTopic).Node()
//
// Modules operate on the target from gabric.TargetOps. gabric.Main sets a
// remote target by default. Direct Reconcile use must set one explicitly with
// gabric.NewTargetContext.
package modules
