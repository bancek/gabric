package sshremote

import (
	"sync"
)

// Default connection pooling settings.
const (
	DefaultIdleConnections = 5
	DefaultMaxConnections  = 5
)

// RemoteManager creates and caches Remotes by HostConfig identity.
type RemoteManager struct {
	remotes      map[hostConfigIdentity]*Remote
	remotesMutex sync.Mutex
}

// NewRemoteManager returns an empty remote manager.
func NewRemoteManager() *RemoteManager {
	return &RemoteManager{
		remotes: map[hostConfigIdentity]*Remote{},
	}
}

// Close closes all managed remotes.
func (m *RemoteManager) Close() {
	m.remotesMutex.Lock()
	defer m.remotesMutex.Unlock()

	for _, remote := range m.remotes {
		remote.Close()
	}

	m.remotes = map[hostConfigIdentity]*Remote{}
}

// GetRemote returns the Remote for hc, creating it if needed.
func (m *RemoteManager) GetRemote(hc HostConfig) *Remote {
	key := hc.identity()

	m.remotesMutex.Lock()
	defer m.remotesMutex.Unlock()

	remote, ok := m.remotes[key]
	if ok {
		return remote
	}

	remote = NewRemote(hc)

	m.remotes[key] = remote

	return remote
}
