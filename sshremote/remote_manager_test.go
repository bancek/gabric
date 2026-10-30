package sshremote_test

import (
	"testing"
	"time"

	"github.com/bancek/gabric/sshremote"
)

func TestRemoteManagerHostIdentity(t *testing.T) {
	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)

	config := sshremote.NewConfig()

	first := sshremote.NewHost("example.com").WithPort(0).WithUser("deploy")
	equivalent := sshremote.NewHost("example.com").WithPort(22).WithUser("deploy")

	if manager.GetRemote(sshremote.NewHostConfig(first, config)) != manager.GetRemote(sshremote.NewHostConfig(equivalent, config)) {
		t.Error("simple hosts with the same identity returned different Remotes")
	}

	differentUser := sshremote.NewHost("example.com").WithUser("other")
	if manager.GetRemote(sshremote.NewHostConfig(first, config)) == manager.GetRemote(sshremote.NewHostConfig(differentUser, config)) {
		t.Error("simple hosts with different users returned the same Remote")
	}

	withKeys := sshremote.NewHost("example.com").
		WithUser("deploy").
		WithPrivateKeysPaths([]string{"/key"})
	withKeysClone := withKeys.Clone()

	if manager.GetRemote(sshremote.NewHostConfig(withKeys, config)) != manager.GetRemote(sshremote.NewHostConfig(withKeys, config)) {
		t.Error("the same non-simple host pointer returned different Remotes")
	}
	if manager.GetRemote(sshremote.NewHostConfig(withKeys, config)) == manager.GetRemote(sshremote.NewHostConfig(withKeysClone, config)) {
		t.Error("non-simple host clones returned the same Remote")
	}
}

func TestRemoteManagerRemoteConfigIdentity(t *testing.T) {
	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)

	host := sshremote.NewHost("example.com")
	config := sshremote.NewConfig()

	first := manager.GetRemote(sshremote.NewHostConfig(host, config))
	if manager.GetRemote(sshremote.NewHostConfig(host, config)) != first {
		t.Error("the same config pointer returned a different Remote")
	}

	clone := config.Clone()
	if manager.GetRemote(sshremote.NewHostConfig(host, clone)) == first {
		t.Error("a cloned config returned the same Remote")
	}

	changed := config.WithConnectionTimeout(time.Minute)
	if manager.GetRemote(sshremote.NewHostConfig(host, changed)) == first {
		t.Error("a changed config returned the same Remote")
	}
}

func TestRemoteManagerCache(t *testing.T) {
	srv := startServer(t)

	remoteManager := sshremote.NewRemoteManager()
	defer remoteManager.Close()

	host := testHost(srv)
	config := testConfig()

	first := remoteManager.GetRemote(sshremote.NewHostConfig(host, config))
	second := remoteManager.GetRemote(sshremote.NewHostConfig(host, config))

	if first != second {
		t.Error("GetRemote() did not return the same cached remote")
	}
}
