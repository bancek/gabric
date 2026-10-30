package sshremote

import "strings"

// StrictHostKeyChecking controls host key verification. The values match the
// ssh_config StrictHostKeyChecking setting. The zero value is
// StrictHostKeyCheckingYes.
type StrictHostKeyChecking string

const (
	// StrictHostKeyCheckingYes verifies host keys against the known_hosts files
	// and rejects unknown hosts.
	StrictHostKeyCheckingYes StrictHostKeyChecking = "yes"
	// StrictHostKeyCheckingAsk verifies host keys like
	// StrictHostKeyCheckingYes. gabric does not prompt for unknown host keys,
	// so ask never accepts one implicitly.
	StrictHostKeyCheckingAsk StrictHostKeyChecking = "ask"
	// StrictHostKeyCheckingAcceptNew verifies host keys against the known_hosts
	// files, appends unknown keys to the first known_hosts file and rejects
	// changed keys.
	StrictHostKeyCheckingAcceptNew StrictHostKeyChecking = "accept-new"
	// StrictHostKeyCheckingNo skips host key verification.
	StrictHostKeyCheckingNo StrictHostKeyChecking = "no"
)

func (p StrictHostKeyChecking) verifyKnownHosts() bool {
	return p != StrictHostKeyCheckingNo
}

func (p StrictHostKeyChecking) acceptNew() bool {
	return p == StrictHostKeyCheckingAcceptNew
}

// parseStrictHostKeyChecking maps an ssh_config StrictHostKeyChecking value to
// a StrictHostKeyChecking. Unrecognized values fall back to strict verification.
func parseStrictHostKeyChecking(value string) StrictHostKeyChecking {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "no", "off", "false":
		return StrictHostKeyCheckingNo
	case "accept-new":
		return StrictHostKeyCheckingAcceptNew
	case "ask":
		return StrictHostKeyCheckingAsk
	default:
		return StrictHostKeyCheckingYes
	}
}
