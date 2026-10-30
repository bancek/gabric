//go:build !unix

package gabric

import (
	"errors"
	"testing"
)

func TestRsyncRunnerRequiresNativeTransport(t *testing.T) {
	ctx := NewRemoteRunnerContext(testContext(t), localRemoteRunner)

	err := Remote(ctx).Rsync().Local("src").Remote("dst").Upload().RunErr()
	if !errors.Is(err, errRsyncTransportUnsupported) {
		t.Fatalf("RunErr() error = %v, want errRsyncTransportUnsupported", err)
	}
}
