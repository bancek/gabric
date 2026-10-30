package gabrictest

import (
	"context"
	"io"
	"io/fs"
	"slices"
	"sync"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/shelltest"
	"mvdan.cc/sh/v3/interp"
)

// RsyncHandler returns the result of a recorded rsync transfer.
type RsyncHandler func(req gabric.RsyncRequest) Result

// Side is one sandbox, local or remote.
type Side interface {
	Handle(name string, h Handler)
	HandleRsync(h RsyncHandler)

	Commands() []Command
	AssertCommands(want ...Command)
	AssertExecuted(name string, args ...string) Command
	AssertNotExecuted(name string, args ...string)

	Calls() []Call
	AssertCalls(want ...Call)
	ResetCommands()
	ResetCalls()

	WriteFile(path string, data []byte, mode fs.FileMode)
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
	MkdirAll(path string, mode fs.FileMode)
	SetOwner(path, user, group string)

	AssertPutCalled(want gabric.PutRequest)
	AssertGetCalled(want gabric.GetRequest)
	AssertMkdirCalled(want gabric.MkdirRequest)
	AssertRemoveCalled(want gabric.RemoveRequest)
	AssertExistsCalled(want gabric.ExistsRequest)
	AssertStatCalled(want gabric.StatRequest)
	AssertListCalled(want gabric.ListRequest)
	AssertRsyncCalled(want gabric.RsyncRequest)
}

type side struct {
	t     testing.TB
	core  *shelltest.Core
	calls *callLog

	rsyncMu      sync.RWMutex
	rsyncHandler RsyncHandler
}

func newSide(t testing.TB) *side {
	return &side{
		t:     t,
		core:  shelltest.New(),
		calls: &callLog{},
	}
}

func (s *side) Handle(name string, h Handler) {
	s.core.Register(name, shelltest.Handler{
		Run: func(ctx context.Context, cmd shelltest.Command) error {
			result := h(cmd)
			hc := interp.HandlerCtx(ctx)

			if _, err := io.WriteString(hc.Stdout, result.Stdout); err != nil {
				return err
			}
			if _, err := io.WriteString(hc.Stderr, result.Stderr); err != nil {
				return err
			}

			if result.ExitCode != 0 {
				return interp.ExitStatus(result.ExitCode)
			}

			return nil
		},
	})
}

// HandleRsync registers a handler for recorded rsync transfers.
func (s *side) HandleRsync(h RsyncHandler) {
	s.rsyncMu.Lock()
	defer s.rsyncMu.Unlock()

	s.rsyncHandler = h
}

func (s *side) handleRsync(req gabric.RsyncRequest) (Result, bool) {
	s.rsyncMu.RLock()
	defer s.rsyncMu.RUnlock()

	if s.rsyncHandler == nil {
		return Result{}, false
	}

	return s.rsyncHandler(req), true
}

func (s *side) Commands() []Command {
	return s.core.Commands()
}

func (s *side) AssertCommands(want ...Command) {
	s.t.Helper()

	if got := s.core.Commands(); !slices.EqualFunc(got, want, Command.Equal) {
		s.t.Errorf("commands = %v, want %v", got, want)
	}
}

func (s *side) Calls() []Call {
	return s.calls.snapshot()
}

func (s *side) AssertCalls(want ...Call) {
	s.t.Helper()

	if got := s.Calls(); !slices.EqualFunc(got, want, Call.Equal) {
		s.t.Errorf("calls = %v, want %v", got, want)
	}
}

// ResetCommands clears recorded commands without changing handlers, calls, or
// files.
func (s *side) ResetCommands() {
	s.core.Reset()
}

// ResetCalls clears recorded backend calls without changing handlers, commands,
// or files.
func (s *side) ResetCalls() {
	s.calls.reset()
}

func (s *side) AssertExecuted(name string, args ...string) Command {
	s.t.Helper()

	for _, cmd := range s.core.Commands() {
		if cmd.Name == name && slices.Equal(cmd.Args, args) {
			return cmd
		}
	}

	s.t.Errorf("command not executed: %s %v; recorded: %v", name, args, s.core.Commands())

	return Command{}
}

func (s *side) AssertNotExecuted(name string, args ...string) {
	s.t.Helper()

	for _, cmd := range s.core.Commands() {
		if cmd.Name == name && slices.Equal(cmd.Args, args) {
			s.t.Errorf("command executed: %s %v; recorded: %v", name, args, s.core.Commands())
			return
		}
	}
}

func (s *side) WriteFile(path string, data []byte, mode fs.FileMode) {
	s.t.Helper()

	path = virtualPath(path)

	if err := s.core.FS().WriteFile(path, data, mode); err != nil {
		s.t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}

func (s *side) ReadFile(path string) ([]byte, error) {
	return s.core.FS().ReadFile(virtualPath(path))
}

func (s *side) ReadDir(path string) ([]fs.DirEntry, error) {
	return s.core.FS().ReadDir(virtualPath(path))
}

func (s *side) Stat(path string) (fs.FileInfo, error) {
	return s.core.FS().Stat(virtualPath(path))
}

func (s *side) MkdirAll(path string, mode fs.FileMode) {
	s.t.Helper()

	path = virtualPath(path)

	if err := s.core.FS().MkdirAll(path, mode); err != nil {
		s.t.Fatalf("MkdirAll(%q) error: %v", path, err)
	}
}

func (s *side) SetOwner(path, user, group string) {
	s.t.Helper()

	path = virtualPath(path)

	if err := s.core.FS().SetOwner(path, user, group); err != nil {
		s.t.Fatalf("SetOwner(%q) error: %v", path, err)
	}
}

func (s *side) AssertPutCalled(want gabric.PutRequest) {
	s.t.Helper()

	wantCall := Call{Put: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("put call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertGetCalled(want gabric.GetRequest) {
	s.t.Helper()

	wantCall := Call{Get: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("get call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertMkdirCalled(want gabric.MkdirRequest) {
	s.t.Helper()

	wantCall := Call{Mkdir: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("mkdir call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertRemoveCalled(want gabric.RemoveRequest) {
	s.t.Helper()

	wantCall := Call{Remove: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("remove call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertExistsCalled(want gabric.ExistsRequest) {
	s.t.Helper()

	wantCall := Call{Exists: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("exists call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertStatCalled(want gabric.StatRequest) {
	s.t.Helper()

	wantCall := Call{Stat: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("stat call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertListCalled(want gabric.ListRequest) {
	s.t.Helper()

	wantCall := Call{List: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("list call not found: %+v; recorded: %+v", want, calls)
}

func (s *side) AssertRsyncCalled(want gabric.RsyncRequest) {
	s.t.Helper()

	wantCall := Call{Rsync: &want}
	calls := s.calls.snapshot()

	for _, call := range calls {
		if call.Equal(wantCall) {
			return
		}
	}

	s.t.Errorf("rsync call not found: %+v; recorded: %+v", want, calls)
}
