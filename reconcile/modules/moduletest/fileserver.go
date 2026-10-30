package moduletest

import (
	"path"
	"testing"
	"time"

	"github.com/bancek/gabric/gabrice2e"
	"github.com/bancek/gabric/internal/testutils"
)

// FileServer serves a file tree from a Docker container.
type FileServer struct {
	*Target

	URL string
	Dir string
}

// NewFileServer starts a file server on network and waits until client can
// reach it. Files are written below Dir with Put.
func NewFileServer(t *testing.T, network *gabrice2e.Network, client *Target) *FileServer {
	t.Helper()
	Require(t)

	if client == nil {
		panic("moduletest: nil file server client")
	}

	target := Docker(t, HTTP,
		gabrice2e.WithContainerNetwork(network),
		gabrice2e.WithContainerCommand("httpd", "-f", "-p", "8080", "-h", "/www"),
	)

	server := &FileServer{
		Target: target,
		URL:    "http://" + target.Container().Name() + ":8080",
		Dir:    "/www",
	}

	testutils.Eventually(t, 10*time.Second, func() bool {
		return client.Ops().Exec("curl", "-s", "-o", "/dev/null", server.URL).RunErr() == nil
	})

	return server
}

// Put writes content at name below the served directory.
func (s *FileServer) Put(name string, content []byte) {
	t := s.T()
	t.Helper()

	err := s.Ops().Put(path.Join(s.Dir, name)).
		EnsureParent().
		Bytes(content).
		Mode(0o644).
		RunErr()
	if err != nil {
		t.Fatalf("moduletest: put %s on file server: %v", name, err)
	}
}
