package gabrictest

import (
	"sync"

	"github.com/bancek/gabric"
)

type callLog struct {
	mu    sync.Mutex
	calls []Call
}

func (l *callLog) recordPut(req gabric.PutRequest) {
	l.record(Call{Put: &req})
}

func (l *callLog) recordGet(req gabric.GetRequest) {
	l.record(Call{Get: &req})
}

func (l *callLog) recordMkdir(req gabric.MkdirRequest) {
	l.record(Call{Mkdir: &req})
}

func (l *callLog) recordRemove(req gabric.RemoveRequest) {
	l.record(Call{Remove: &req})
}

func (l *callLog) recordExists(req gabric.ExistsRequest) {
	l.record(Call{Exists: &req})
}

func (l *callLog) recordStat(req gabric.StatRequest) {
	l.record(Call{Stat: &req})
}

func (l *callLog) recordList(req gabric.ListRequest) {
	l.record(Call{List: &req})
}

func (l *callLog) recordRsync(req gabric.RsyncRequest) {
	req = req.Clone()

	l.record(Call{Rsync: &req})
}

func (l *callLog) record(call Call) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls = append(l.calls, call)
}

func (l *callLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls = nil
}

func (l *callLog) snapshot() []Call {
	l.mu.Lock()
	defer l.mu.Unlock()

	calls := make([]Call, len(l.calls))
	for i, call := range l.calls {
		calls[i] = call.Clone()
	}

	return calls
}
