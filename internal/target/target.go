package target

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"

	"github.com/C-X1an/faultbraid/internal/raftadapter"
	raft "go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
)

type result struct {
	hash  [32]byte
	value int64
}
type request struct {
	id      string
	write   bool
	hash    [32]byte
	context string
	index   uint64
	ready   bool
}
type Target struct {
	cfg                   Config
	rn                    *raft.RawNode
	election              *raftadapter.ElectionRandom
	started, stopped      bool
	value                 int64
	applied, readSequence uint64
	dedup                 map[string]result
	pending               map[string]*request
	order                 []string
}

func New(cfg Config) (*Target, error) {
	if cfg.ID < 1 || cfg.ID > 3 || cfg.Store == nil || cfg.Entropy == nil {
		return nil, fmt.Errorf("target: ID 1..3, store and entropy required")
	}
	if cfg.LogLimitBytes == 0 {
		cfg.LogLimitBytes = DefaultLogLimitBytes
	}
	if cfg.LogLimitBytes > maxLogLimitBytes {
		return nil, ErrLimit
	}
	if cfg.Variant == "" {
		cfg.Variant = Safe
	}
	if cfg.Variant != Safe && cfg.Variant != UnsafeLocalRead {
		return nil, fmt.Errorf("target: unsupported variant")
	}
	return &Target{cfg: cfg, dedup: map[string]result{}, pending: map[string]*request{}}, nil
}
func (t *Target) ID() uint64 { return t.cfg.ID }
func (t *Target) Leader() uint64 {
	if t.rn == nil {
		return 0
	}
	return t.rn.BasicStatus().Lead
}
func (t *Target) Value() int64    { return t.value }
func (t *Target) Applied() uint64 { return t.applied }
func (t *Target) Status() Status {
	s := Status{ID: t.cfg.ID, Applied: t.applied, Value: t.value, Started: t.started, Stopped: t.stopped, Variant: t.cfg.Variant, Pending: len(t.pending)}
	if t.rn != nil {
		r := t.rn.BasicStatus()
		s.Leader = r.Lead
		s.Term = r.GetTerm()
		s.Commit = r.GetCommit()
	}
	return s
}
func (t *Target) Step(event Event) ([]Effect, error) {
	var effects []Effect
	if t.stopped {
		return nil, ErrStopped
	}
	if _, ok := event.(Start); ok {
		if t.started {
			return nil, fmt.Errorf("target: already started")
		}
		if err := t.start(); err != nil {
			return t.fail(effects, err)
		}
	} else {
		if !t.started {
			return nil, fmt.Errorf("target: Start required")
		}
		// Preserve headroom for Raft's term increments, including PreVote's
		// prospective next term; fail explicitly instead of uint64 wraparound.
		if t.rn.BasicStatus().GetTerm() >= math.MaxUint64-2 {
			return t.fail(effects, ErrLimit)
		}
		var err error
		switch e := event.(type) {
		case Tick:
			t.rn.Tick()
		case Campaign:
			err = t.rn.Campaign()
		case Message:
			var m *pb.Message
			m, err = t.decodeMessage(e.Data)
			if err == nil {
				err = t.rn.Step(m)
			}
		case Write:
			effects, err = t.write(e)
		case Read:
			effects, err = t.read(e)
		default:
			err = fmt.Errorf("target: unsupported event")
		}
		if err != nil {
			if errors.Is(err, ErrInjectedCrash) {
				return t.fail(effects, err)
			}
			// Rejected input has no admitted client operation. Persistence/entropy
			// failures are handled by drain and stop all subsequent mutation.
			return append(effects, Effect{Kind: FailEffect, Code: errorCode(err)}), err
		}
	}
	if err := t.election.Err(); err != nil {
		return t.fail(effects, err)
	}
	if err := t.drain(&effects); err != nil {
		return t.fail(effects, err)
	}
	return effects, nil
}
func (t *Target) fail(effects []Effect, err error) ([]Effect, error) {
	t.stopped = true
	if errors.Is(err, ErrInjectedCrash) {
		// Recover discards staged state in both stores. It does not roll back a
		// completed fsync, released message or client response.
		_, recoverErr := t.cfg.Store.Recover()
		if recoverErr != nil {
			err = errors.Join(err, recoverErr)
		}
	}
	return append(effects, Effect{Kind: FailEffect, Code: errorCode(err)}), err
}
func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrInjectedCrash):
		return "INJECTED_CRASH"
	case errors.Is(err, ErrLimit):
		return "LIMIT_EXCEEDED"
	case errors.Is(err, ErrCorrupt):
		return "CORRUPT_STATE"
	case errors.Is(err, ErrUnsupportedSnapshot):
		return "UNSUPPORTED_SNAPSHOT"
	}
	return "TARGET_ERROR"
}
func (t *Target) start() error {
	state, err := t.cfg.Store.Recover()
	if err != nil {
		return err
	}
	if err = validateState(&state, t.cfg.LogLimitBytes); err != nil {
		return err
	}
	if len(state.Entries) > 0 && state.HardState.GetCommit() < 3 {
		return ErrCorrupt
	}
	for _, e := range state.Entries {
		if e.GetIndex() > state.HardState.GetCommit() {
			break
		}
		if e.GetType() == pb.EntryNormal && len(e.Data) > 0 {
			if err = t.applyCommand(e.Data); err != nil {
				return err
			}
		}
		t.applied = e.GetIndex()
	}
	t.election, err = raftadapter.NewElectionRandom(t.cfg.Entropy, "raft-election")
	if err != nil {
		return err
	}
	cfg := &raft.Config{
		ID: t.cfg.ID, Storage: t.cfg.Store.Storage(), Applied: t.applied,
		ElectionTick: 10, HeartbeatTick: 1, ElectionRandom: t.election.Func(),
		PreVote: true, CheckQuorum: true, ReadOnlyOption: raft.ReadOnlySafe,
		DisableProposalForwarding: true, MaxInflightMsgs: 64,
		MaxSizePerMsg:             MaxFrameBytes - 1024,
		MaxCommittedSizePerReady:  MaxFrameBytes - 1024,
		MaxUncommittedEntriesSize: t.cfg.LogLimitBytes,
		Logger:                    &raft.DefaultLogger{Logger: log.New(io.Discard, "", 0)},
	}
	t.rn, err = raft.NewRawNode(cfg)
	if err != nil {
		return err
	}
	if err = t.election.Err(); err != nil {
		return err
	}
	if len(state.Entries) == 0 {
		if err = t.rn.Bootstrap([]raft.Peer{{ID: 1}, {ID: 2}, {ID: 3}}); err != nil {
			return err
		}
	}
	t.started = true
	return nil
}
func (t *Target) barrier(b Barrier, effects *[]Effect) error {
	*effects = append(*effects, Effect{Kind: BarrierEffect, Barrier: b, Index: t.applied})
	if t.cfg.Barrier != nil {
		if err := t.cfg.Barrier(b); err != nil {
			t.stopped = true
			return err
		}
	}
	return nil
}
func (t *Target) respond(id string, value int64, effects *[]Effect) error {
	if err := t.barrier(AfterApplyBeforeResponse, effects); err != nil {
		return err
	}
	*effects = append(*effects, Effect{Kind: ReplyEffect, RequestID: id, Value: value, Index: t.applied})
	return t.barrier(AfterResponse, effects)
}
func (t *Target) write(e Write) ([]Effect, error) {
	if !validID(e.RequestID) {
		return nil, fmt.Errorf("target: invalid request ID")
	}
	data := encodeCommand(e.RequestID, e.Value)
	hash := sha256.Sum256(data)
	if r, ok := t.dedup[e.RequestID]; ok {
		if r.hash != hash {
			return nil, fmt.Errorf("target: request ID payload conflict")
		}
		var effects []Effect
		err := t.respond(e.RequestID, r.value, &effects)
		return effects, err
	}
	existing := false
	if p, ok := t.pending[e.RequestID]; ok {
		if !p.write || p.hash != hash {
			return nil, fmt.Errorf("target: request ID payload conflict")
		}
		existing = true
	}
	if t.rn.BasicStatus().RaftState != raft.StateLeader {
		if existing {
			return nil, nil
		} // The earlier admission remains pending.
		return nil, raft.ErrProposalDropped
	}
	if !existing && (len(t.pending) >= MaxRequests || len(t.dedup) >= MaxRequests) {
		return nil, ErrLimit
	}
	// Bound admission before Propose as well as validating the actual Ready.
	last, err := t.cfg.Store.Storage().LastIndex()
	if err != nil {
		return nil, err
	}
	entries, err := t.cfg.Store.Storage().Entries(1, last+1, math.MaxUint64)
	if err != nil {
		return nil, err
	}
	if logBytes(entries)+uint64(len(data))+32 > t.cfg.LogLimitBytes {
		return nil, ErrLimit
	}
	if err = t.rn.Propose(data); err != nil {
		return nil, err
	}
	if !existing {
		t.pending[e.RequestID] = &request{id: e.RequestID, write: true, hash: hash}
		t.order = append(t.order, e.RequestID)
	}
	return nil, nil
}
func (t *Target) read(e Read) ([]Effect, error) {
	if !validID(e.RequestID) {
		return nil, fmt.Errorf("target: invalid request ID")
	}
	if _, ok := t.dedup[e.RequestID]; ok {
		return nil, fmt.Errorf("target: request ID payload conflict")
	}
	existing := false
	if p, ok := t.pending[e.RequestID]; ok {
		if p.write {
			return nil, fmt.Errorf("target: request ID payload conflict")
		}
		existing = true
	}
	if t.rn.BasicStatus().RaftState != raft.StateLeader {
		if existing {
			return nil, nil
		}
		return nil, raft.ErrProposalDropped
	}
	if t.cfg.Variant == UnsafeLocalRead {
		var effects []Effect
		err := t.respond(e.RequestID, t.value, &effects)
		return effects, err
	}
	if !existing && len(t.pending) >= MaxRequests || t.readSequence == math.MaxUint64 {
		return nil, ErrLimit
	}
	t.readSequence++
	// A restarted node cannot admit reads until elected in a newer durable
	// term. Node+term+counter therefore cannot reuse a stale read context.
	context := make([]byte, 24)
	binary.BigEndian.PutUint64(context, t.cfg.ID)
	binary.BigEndian.PutUint64(context[8:], t.rn.BasicStatus().GetTerm())
	binary.BigEndian.PutUint64(context[16:], t.readSequence)
	p := &request{id: e.RequestID, context: string(context)}
	t.pending[e.RequestID] = p
	if !existing {
		t.order = append(t.order, e.RequestID)
	}
	t.rn.ReadIndex(context)
	return nil, nil
}
func (t *Target) applyCommand(data []byte) error {
	c, err := decodeCommand(data)
	if err != nil {
		return err
	}
	if r, ok := t.dedup[c.id]; ok {
		if r.hash != c.hash {
			return fmt.Errorf("%w: replicated request ID conflict", ErrCorrupt)
		}
		return nil
	}
	if len(t.dedup) >= MaxRequests {
		return ErrLimit
	}
	t.value = c.value
	t.dedup[c.id] = result{hash: c.hash, value: c.value}
	return nil
}
func (t *Target) drain(effects *[]Effect) error {
	for t.rn.HasReady() {
		rd := t.rn.Ready()
		if rd.Snapshot != nil {
			return ErrUnsupportedSnapshot
		}
		sends := make([]Effect, 0, len(rd.Messages))
		for _, m := range rd.Messages {
			if m.GetType() == pb.MsgSnap {
				return ErrUnsupportedSnapshot
			}
			data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
			if err != nil {
				return err
			}
			if len(data) > MaxFrameBytes {
				return ErrLimit
			}
			sends = append(sends, Effect{Kind: SendEffect, To: m.GetTo(), Data: data})
		}
		if err := t.barrier(BeforeAppend, effects); err != nil {
			return err
		}
		if err := t.cfg.Store.Stage(Batch{Entries: rd.Entries, HardState: rd.HardState}); err != nil {
			return err
		}
		if err := t.barrier(AfterStageBeforeFlush, effects); err != nil {
			return err
		}
		if err := t.cfg.Store.Flush(); err != nil {
			return err
		}
		*effects = append(*effects, Effect{Kind: PersistEffect, Index: rd.HardState.GetCommit()})
		if err := t.barrier(AfterFlushBeforeSend, effects); err != nil {
			return err
		}
		*effects = append(*effects, sends...)
		if err := t.barrier(AfterSendBeforeApply, effects); err != nil {
			return err
		}
		for _, e := range rd.CommittedEntries {
			if e.GetIndex() != t.applied+1 {
				return ErrCorrupt
			}
			switch e.GetType() {
			case pb.EntryNormal:
				if len(e.Data) > 0 {
					if err := t.applyCommand(e.Data); err != nil {
						return err
					}
				}
			case pb.EntryConfChange:
				if e.GetIndex() > 3 {
					return ErrCorrupt
				}
				var cc pb.ConfChange
				if proto.Unmarshal(e.Data, &cc) != nil {
					return ErrCorrupt
				}
				t.rn.ApplyConfChange(&cc)
			default:
				return ErrCorrupt
			}
			t.applied = e.GetIndex()
		}
		for _, rs := range rd.ReadStates {
			// Lookups affect only membership, never output iteration order.
			for _, id := range t.order {
				p := t.pending[id]
				if !p.write && p.context == string(rs.RequestCtx) {
					p.index = rs.Index
					p.ready = true
					break
				}
			}
		}
		remaining := make([]string, 0, len(t.order))
		for _, id := range t.order {
			p := t.pending[id]
			value := t.value
			complete := false
			if p.write {
				r, ok := t.dedup[id]
				if ok {
					if r.hash != p.hash {
						return ErrCorrupt
					}
					value = r.value
					complete = true
				}
			} else {
				complete = p.ready && t.applied >= p.index
			}
			if complete {
				if err := t.respond(id, value, effects); err != nil {
					return err
				}
				delete(t.pending, id)
			} else {
				remaining = append(remaining, id)
			}
		}
		t.order = remaining
		t.rn.Advance(rd)
		if err := t.election.Err(); err != nil {
			return err
		}
	}
	return nil
}
func (t *Target) decodeMessage(data []byte) (*pb.Message, error) {
	if len(data) == 0 || len(data) > MaxFrameBytes {
		return nil, ErrLimit
	}
	m := &pb.Message{}
	if err := (proto.UnmarshalOptions{RecursionLimit: 8}).Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("target: malformed raft frame")
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	if err != nil || !bytes.Equal(canonical, data) || len(m.Responses) != 0 || len(m.ProtoReflect().GetUnknown()) != 0 || m.GetFrom() < 1 || m.GetFrom() > 3 || m.GetFrom() == t.cfg.ID || m.GetTo() != t.cfg.ID || m.Type == nil {
		return nil, fmt.Errorf("target: invalid raft frame")
	}
	if m.Snapshot != nil || m.GetType() == pb.MsgSnap {
		return nil, ErrUnsupportedSnapshot
	}
	if m.GetTerm() == 0 || m.GetTerm() >= math.MaxUint64-1 || m.GetLogTerm() > m.GetTerm() || m.GetIndex() > t.cfg.LogLimitBytes/4+3 || m.GetCommit() > t.cfg.LogLimitBytes/4+3 || m.GetRejectHint() > t.cfg.LogLimitBytes/4+3 {
		return nil, ErrCorrupt
	}
	last, err := t.cfg.Store.Storage().LastIndex()
	if err != nil {
		return nil, err
	}
	if m.GetTerm() >= t.rn.BasicStatus().GetTerm() && (m.GetType() == pb.MsgHeartbeat && m.GetCommit() > last || m.GetType() == pb.MsgAppResp && !m.GetReject() && m.GetIndex() > last) {
		return nil, ErrCorrupt
	}
	switch m.GetType() {
	case pb.MsgApp, pb.MsgAppResp, pb.MsgVote, pb.MsgVoteResp, pb.MsgPreVote, pb.MsgPreVoteResp, pb.MsgHeartbeat, pb.MsgHeartbeatResp:
	default:
		return nil, fmt.Errorf("target: unsupported network message")
	}
	for i, e := range m.Entries {
		if m.GetType() != pb.MsgApp || e == nil || len(e.ProtoReflect().GetUnknown()) != 0 || e.GetIndex() != m.GetIndex()+uint64(i)+1 || e.GetTerm() == 0 || e.GetTerm() > m.GetTerm() {
			return nil, ErrCorrupt
		}
		if m.GetTerm() >= t.rn.BasicStatus().GetTerm() && e.GetIndex() <= t.rn.BasicStatus().GetCommit() {
			committed, err := t.cfg.Store.Storage().Entries(e.GetIndex(), e.GetIndex()+1, math.MaxUint64)
			if err != nil || len(committed) != 1 || !proto.Equal(e, committed[0]) {
				return nil, ErrCorrupt
			}
		}
		switch e.GetType() {
		case pb.EntryNormal:
			if e.GetIndex() <= 3 {
				return nil, ErrCorrupt
			}
			if len(e.Data) > 0 {
				if _, err := decodeCommand(e.Data); err != nil {
					return nil, err
				}
			}
		case pb.EntryConfChange:
			var cc pb.ConfChange
			if e.GetIndex() < 1 || e.GetIndex() > 3 || proto.Unmarshal(e.Data, &cc) != nil || cc.GetNodeId() != e.GetIndex() || cc.GetType() != pb.ConfChangeAddNode || len(cc.Context) != 0 || len(cc.ProtoReflect().GetUnknown()) != 0 {
				return nil, ErrCorrupt
			}
		default:
			return nil, ErrCorrupt
		}
	}
	return m, nil
}
