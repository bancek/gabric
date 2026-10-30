package sshremote

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// KnownHostsCallback returns an ssh host key callback that strictly verifies
// the host key against knownHostsPaths. Nonexistent files are skipped. Unknown
// or mismatched keys are rejected.
func KnownHostsCallback(knownHostsPaths ...string) (ssh.HostKeyCallback, error) {
	existingPaths := make([]string, 0, len(knownHostsPaths))

	for _, knownHostsPath := range knownHostsPaths {
		if knownHostsPath == "" {
			continue
		}

		if _, err := os.Stat(knownHostsPath); err == nil {
			existingPaths = append(existingPaths, knownHostsPath)
		}
	}

	return knownhosts.New(existingPaths...)
}

// KnownHostsAcceptNewCallback returns an ssh host key callback that verifies
// known keys against knownHostsPaths and appends an unknown key to the first
// file. Changed, revoked or otherwise failing keys are rejected. At least one
// known_hosts path is required.
func KnownHostsAcceptNewCallback(knownHostsPaths ...string) (ssh.HostKeyCallback, error) {
	var knownHostsPath string

	for _, path := range knownHostsPaths {
		if path != "" {
			knownHostsPath = path
			break
		}
	}

	if knownHostsPath == "" {
		return nil, errors.New("accept-new requires a known_hosts file")
	}

	// Validate the files eagerly; the callback re-reads them so a key appended
	// by an earlier call is verified next time.
	if _, err := KnownHostsCallback(knownHostsPaths...); err != nil {
		return nil, err
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		verify, err := KnownHostsCallback(knownHostsPaths...)
		if err != nil {
			return err
		}

		err = verify(hostname, remote, key)
		if err != nil {
			if !IsUnknownHost(err) {
				return err
			}
		} else {
			return nil
		}

		if err := KnownHostsAppend(knownHostsPath, KnownHostsLine(hostname, key)); err != nil {
			return fmt.Errorf("failed to append host key to %s: %w", knownHostsPath, err)
		}

		return nil
	}, nil
}

// KnownHostsLine formats a known_hosts entry for hostname and key.
func KnownHostsLine(hostname string, key ssh.PublicKey) string {
	return knownhosts.Line([]string{hostname}, key) + "\n"
}

// KnownHostsAppend appends line to the known_hosts file. New files are created
// with 0600 permissions.
func KnownHostsAppend(knownHostsPath string, line string) (err error) {
	f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err := f.WriteString(line); err != nil {
		return err
	}

	return f.Sync()
}

// IsUnknownHost reports whether err is a known_hosts "key is unknown" error, as
// opposed to a key mismatch, a revoked key, or an unrelated error. It is the
// condition under which trust-on-first-use may safely add the key.
func IsUnknownHost(err error) bool {
	keyErr, ok := errors.AsType[*knownhosts.KeyError](err)

	return ok && len(keyErr.Want) == 0
}
