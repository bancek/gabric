package main

import _ "embed"

// Host config files are embedded into the binary at build time.

//go:embed files/sshd_hardening.conf
var sshdHardening string

//go:embed files/auto_reboot.conf
var autoReboot string

//go:embed files/docker_daemon.json
var dockerDaemon string

//go:embed files/deploy_authorized_keys
var deployAuthorizedKeysFile string

// Service templates are embedded so package initialization does not read the
// working directory.

//go:embed srv/app/compose.yaml.tpl
var appComposeTemplate string

//go:embed srv/traefik/compose.yaml.tpl
var traefikComposeTemplate string

//go:embed srv/www/compose.yaml.tpl
var wwwComposeTemplate string

//go:embed srv/wireguard/compose.yaml.tpl
var wireguardComposeTemplate string
