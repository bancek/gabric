// Package sshtest provides an in-process SSH server for tests. It supports
// command execution through a pluggable handler and direct-tcpip channels (used
// for jump hosts).
package sshtest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// DefaultUser is the username accepted by the test server.
const DefaultUser = "gabric"

// ExecHandler handles an exec request. It writes output to stdout/stderr and
// returns the exit status.
type ExecHandler func(cmd string, stdin io.Reader, stdout, stderr io.Writer) int

type options struct {
	execHandler   ExecHandler
	shellHandler  ExecHandler
	authorizedKey ssh.PublicKey
	noClientAuth  bool
	directTCPIP   bool
}

// Option configures a Server.
type Option func(*options)

// WithExecHandler sets the exec handler. The default drains stdin and returns 0.
func WithExecHandler(handler ExecHandler) Option {
	return func(o *options) { o.execHandler = handler }
}

// WithShellHandler sets the handler for interactive "shell" requests. It
// defaults to the exec handler invoked with /bin/sh.
func WithShellHandler(handler ExecHandler) Option {
	return func(o *options) { o.shellHandler = handler }
}

// WithAuthorizedKey sets the public key accepted for authentication. By default
// the server authorizes the client key returned by ClientKeyPath.
func WithAuthorizedKey(key ssh.PublicKey) Option {
	return func(o *options) { o.authorizedKey = key }
}

// WithNoClientAuth disables authentication.
func WithNoClientAuth() Option {
	return func(o *options) { o.noClientAuth = true }
}

// WithDirectTCPIP enables direct-tcpip channels (disabled by default).
func WithDirectTCPIP() Option {
	return func(o *options) { o.directTCPIP = true }
}

// Server is an in-process SSH server.
type Server struct {
	listener net.Listener
	config   *ssh.ServerConfig

	user          string
	hostKey       ssh.PublicKey
	clientKeyPath string
	directTCPIP   bool
	execHandler   ExecHandler
	shellHandler  ExecHandler

	ptyTerm       string
	ptyWidth      int
	ptyHeight     int
	lastWinWidth  int
	lastWinHeight int

	serveWg sync.WaitGroup
	connWg  sync.WaitGroup

	connCount      atomic.Int64
	sessionCount   atomic.Int64
	rejectSessions atomic.Bool
	sessionBlock   atomic.Pointer[chan struct{}]

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
}

// Start starts an SSH server on 127.0.0.1 with an ephemeral port.
func Start(opts ...Option) (*Server, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		return nil, err
	}

	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		return nil, err
	}

	authorizedKey := o.authorizedKey
	if authorizedKey == nil && !o.noClientAuth {
		authorizedKey = clientSigner.PublicKey()
	}

	config := &ssh.ServerConfig{
		NoClientAuth: o.noClientAuth,
	}
	if authorizedKey != nil {
		config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized key")
		}
	}
	config.AddHostKey(hostSigner)

	keyBlock, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		return nil, err
	}

	keyDir, err := os.MkdirTemp("", "sshtest")
	if err != nil {
		return nil, err
	}

	clientKeyPath := filepath.Join(keyDir, "id_ed25519")
	if err := os.WriteFile(clientKeyPath, pem.EncodeToMemory(keyBlock), 0600); err != nil {
		os.RemoveAll(keyDir)
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(keyDir)
		return nil, err
	}

	execHandler := o.execHandler
	if execHandler == nil {
		execHandler = func(_ string, stdin io.Reader, _, _ io.Writer) int {
			io.Copy(io.Discard, stdin)
			return 0
		}
	}

	shellHandler := o.shellHandler
	if shellHandler == nil {
		shellHandler = func(_ string, stdin io.Reader, stdout, stderr io.Writer) int {
			return execHandler("/bin/sh", stdin, stdout, stderr)
		}
	}

	s := &Server{
		listener:      listener,
		config:        config,
		user:          DefaultUser,
		hostKey:       hostSigner.PublicKey(),
		clientKeyPath: clientKeyPath,
		directTCPIP:   o.directTCPIP,
		execHandler:   execHandler,
		shellHandler:  shellHandler,
		conns:         map[net.Conn]struct{}{},
	}

	s.serveWg.Add(1)
	go s.serve()

	return s, nil
}

// Addr returns the listener address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Port returns the listener port.
func (s *Server) Port() int {
	_, portStr, _ := net.SplitHostPort(s.Addr())
	port, _ := strconv.Atoi(portStr)
	return port
}

// User returns the username accepted by the server.
func (s *Server) User() string {
	return s.user
}

// HostKey returns the server host key.
func (s *Server) HostKey() ssh.PublicKey {
	return s.hostKey
}

// KnownHosts returns a known_hosts line for the server.
func (s *Server) KnownHosts() string {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port()))
	return knownhosts.Line([]string{addr}, s.hostKey)
}

// ClientKeyPath returns the path to a private key authorized by the server.
func (s *Server) ClientKeyPath() string {
	return s.clientKeyPath
}

// ConnCount returns the number of TCP connections accepted so far.
func (s *Server) ConnCount() int {
	return int(s.connCount.Load())
}

// SessionCount returns the number of session channel opens received so far,
// including rejected opens.
func (s *Server) SessionCount() int {
	return int(s.sessionCount.Load())
}

// RejectSessions controls whether new session channels are rejected.
func (s *Server) RejectSessions(reject bool) {
	s.rejectSessions.Store(reject)
}

// BlockSessions blocks new session channels until the returned function is
// called. The returned function is idempotent.
func (s *Server) BlockSessions() func() {
	unblock := make(chan struct{})
	s.sessionBlock.Store(&unblock)

	var once sync.Once

	return func() {
		once.Do(func() {
			s.sessionBlock.Store(nil)
			close(unblock)
		})
	}
}

// PTYTerm returns the terminal type from the last pty-req, if any.
func (s *Server) PTYTerm() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ptyTerm
}

// PTYSize returns the width and height from the last pty-req, if any.
func (s *Server) PTYSize() (width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ptyWidth, s.ptyHeight
}

// LastWindowSize returns the width and height from the last window-change, if
// any.
func (s *Server) LastWindowSize() (width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.lastWinWidth, s.lastWinHeight
}

// CloseConnections closes currently accepted client connections without
// stopping the listener. It returns the number of connections closed.
func (s *Server) CloseConnections() int {
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}

	return len(conns)
}

// Close stops the server.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	err := s.listener.Close()

	s.serveWg.Wait()

	s.mu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.conns = nil
	s.mu.Unlock()

	s.connWg.Wait()

	os.RemoveAll(filepath.Dir(s.clientKeyPath))

	return err
}

func (s *Server) trackConn(conn net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if add {
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
}

func (s *Server) serve() {
	defer s.serveWg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.connCount.Add(1)
		s.trackConn(conn, true)

		s.connWg.Go(func() {
			defer s.trackConn(conn, false)
			s.handleConn(conn)
		})
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	var wg sync.WaitGroup

	for newChan := range chans {
		wg.Go(func() {
			s.handleChannel(newChan)
		})
	}

	wg.Wait()
}

func (s *Server) handleChannel(newChan ssh.NewChannel) {
	switch newChan.ChannelType() {
	case "session":
		s.sessionCount.Add(1)

		if s.rejectSessions.Load() {
			newChan.Reject(ssh.Prohibited, "sessions disabled")
			return
		}

		if block := s.sessionBlock.Load(); block != nil {
			<-*block
		}

		ch, reqs, err := newChan.Accept()
		if err != nil {
			return
		}
		s.handleSession(ch, reqs)
	case "direct-tcpip":
		if !s.directTCPIP {
			newChan.Reject(ssh.Prohibited, "direct-tcpip disabled")
			return
		}
		s.handleDirectTCPIP(newChan)
	default:
		newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
	}
}

type execMsg struct {
	Command string
}

type ptyRequestMsg struct {
	Term    string
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
	Modes   string
}

type windowChangeMsg struct {
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
}

type exitStatusMsg struct {
	Status uint32
}

func (s *Server) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()

	for req := range reqs {
		switch req.Type {
		case "pty-req":
			var msg ptyRequestMsg
			if err := ssh.Unmarshal(req.Payload, &msg); err != nil {
				req.Reply(false, nil)
				continue
			}

			s.mu.Lock()
			s.ptyTerm = msg.Term
			s.ptyWidth = int(msg.Columns)
			s.ptyHeight = int(msg.Rows)
			s.mu.Unlock()

			req.Reply(true, nil)
		case "window-change":
			var msg windowChangeMsg
			if err := ssh.Unmarshal(req.Payload, &msg); err != nil {
				req.Reply(false, nil)
				continue
			}

			s.mu.Lock()
			s.lastWinWidth = int(msg.Columns)
			s.lastWinHeight = int(msg.Rows)
			s.mu.Unlock()

			req.Reply(false, nil)
		case "shell":
			req.Reply(true, nil)

			done := make(chan int, 1)

			go func() {
				done <- s.shellHandler("", ch, ch, ch.Stderr())
			}()

			for {
				select {
				case status := <-done:
					ch.SendRequest("exit-status", false, ssh.Marshal(exitStatusMsg{Status: uint32(status)}))
					ch.Close()

					return
				case req, ok := <-reqs:
					if !ok {
						<-done
						return
					}

					switch req.Type {
					case "window-change":
						var msg windowChangeMsg
						if err := ssh.Unmarshal(req.Payload, &msg); err != nil {
							req.Reply(false, nil)
							continue
						}

						s.mu.Lock()
						s.lastWinWidth = int(msg.Columns)
						s.lastWinHeight = int(msg.Rows)
						s.mu.Unlock()

						req.Reply(false, nil)
					default:
						req.Reply(false, nil)
					}
				}
			}
		case "exec":
			var msg execMsg
			if err := ssh.Unmarshal(req.Payload, &msg); err != nil {
				req.Reply(false, nil)
				continue
			}

			req.Reply(true, nil)

			status := s.execHandler(msg.Command, ch, ch, ch.Stderr())

			ch.SendRequest("exit-status", false, ssh.Marshal(exitStatusMsg{Status: uint32(status)}))
			ch.Close()

			return
		default:
			req.Reply(false, nil)
		}
	}
}

type directTCPIPMsg struct {
	Host       string
	Port       uint32
	OriginHost string
	OriginPort uint32
}

func (s *Server) handleDirectTCPIP(newChan ssh.NewChannel) {
	var msg directTCPIPMsg
	if err := ssh.Unmarshal(newChan.ExtraData(), &msg); err != nil {
		newChan.Reject(ssh.ConnectionFailed, "invalid direct-tcpip payload")
		return
	}

	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	defer ch.Close()

	go ssh.DiscardRequests(reqs)

	dst, err := net.Dial("tcp", net.JoinHostPort(msg.Host, strconv.Itoa(int(msg.Port))))
	if err != nil {
		return
	}
	defer dst.Close()

	// Close both sides as soon as either direction ends so the other copy
	// is unblocked.
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(dst, ch)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(ch, dst)
		done <- struct{}{}
	}()

	<-done
}
