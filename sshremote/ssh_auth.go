package sshremote

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ErrAgentUnavailable reports that the configured ssh agent could not be
// dialed or listed. Callers can retry with an empty agent socket to
// authenticate with private keys instead.
var ErrAgentUnavailable = errors.New("ssh agent unavailable")

// HostSSHAuth returns the ssh authentication method for hc, combining the ssh
// agent and the private keys. When the agent is unavailable it logs the failure
// and retries without the agent. Advanced: most callers should use
// NewRemoteClient.
func HostSSHAuth(hc HostConfig, logger *slog.Logger) (ssh.AuthMethod, error) {
	signers, err := GetSSHSigners(hc)
	if errors.Is(err, ErrAgentUnavailable) {
		logger.With("error", err).
			Warn("ssh agent unavailable, falling back to private keys")

		hc = hc.WithConfig(hc.Config().WithAgentSocket(""))

		signers, err = GetSSHSigners(hc)
	}
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signers...), nil
}

// withAgentConn dials the ssh agent, calls f with a connected client, and
// closes the connection. Agent connections are not kept open between calls.
//
// It is a free function rather than a method because Go does not allow type
// parameters on methods.
func withAgentConn[T any](socketPath string, f func(agent.Agent) (T, error)) (T, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("could not connect to ssh agent: %w", err)
	}
	defer conn.Close()

	return f(agent.NewClient(conn))
}

// agentClient talks to the ssh agent over SSH_AUTH_SOCK, opening a connection
// per operation so that no long-lived agent connection is held.
type agentClient struct {
	socketPath string
}

func (c *agentClient) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return withAgentConn(c.socketPath, func(a agent.Agent) (*ssh.Signature, error) {
		return a.Sign(key, data)
	})
}

func (c *agentClient) Signers() ([]ssh.Signer, error) {
	return withAgentConn(c.socketPath, func(a agent.Agent) ([]ssh.Signer, error) {
		agentSigners, err := a.Signers()
		if err != nil {
			return nil, fmt.Errorf("could not get ssh agent signers: %w", err)
		}

		signers := make([]ssh.Signer, 0, len(agentSigners))

		for _, signer := range agentSigners {
			signers = append(signers, &agentSigner{agent: c, key: signer.PublicKey()})
		}

		return signers, nil
	})
}

// agentSigner is an ssh.Signer for a single key held by the ssh agent. Signing
// goes through agentClient, which opens an agent connection per Sign call.
type agentSigner struct {
	agent *agentClient
	key   ssh.PublicKey
}

func (s *agentSigner) PublicKey() ssh.PublicKey {
	return s.key
}

func (s *agentSigner) Sign(_ io.Reader, data []byte) (*ssh.Signature, error) {
	return s.agent.Sign(s.key, data)
}

// GetSSHSigners returns the ssh signers for hc from the ssh agent and the
// effective private keys. It returns ErrAgentUnavailable when the configured
// agent cannot be used. Passphrase protected keys are skipped when an ssh agent
// is available. Advanced: most callers should use NewRemoteClient.
func GetSSHSigners(hc HostConfig) ([]ssh.Signer, error) {
	var signers []ssh.Signer

	hasAgent := false

	if socketPath := hc.Config().AgentSocket(); socketPath != "" {
		agentSigners, err := (&agentClient{socketPath: socketPath}).Signers()
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrAgentUnavailable, socketPath, err)
		}

		signers = append(signers, agentSigners...)

		hasAgent = true
	}

	signers = append(signers, hc.Signers()...)

	for _, path := range hc.PrivateKeysPaths() {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			if _, ok := errors.AsType[*ssh.PassphraseMissingError](err); ok && hasAgent {
				// ignore this key, ssh agent will be used
			} else {
				return nil, err
			}
		} else {
			signers = append(signers, signer)
		}
	}

	return signers, nil
}
