package target

import (
	"fmt"
	"math"

	raft "go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
)

// MemoryStore models completed durable flushes, not physical disk persistence.
// Its lifetime must outlive the Target across simulated process restarts.
type MemoryStore struct {
	limit   uint64
	durable State
	staged  *State
	batch   Batch
	view    *storageView
}

func NewMemoryStore(limit uint64) *MemoryStore {
	if limit == 0 {
		limit = DefaultLogLimitBytes
	}
	s := &MemoryStore{limit: limit, durable: State{ConfState: &pb.ConfState{}}}
	s.view = &storageView{s: s}
	return s
}
func cloneEntry(e *pb.Entry) *pb.Entry { return proto.Clone(e).(*pb.Entry) }
func cloneHard(h *pb.HardState) *pb.HardState {
	if h == nil {
		return nil
	}
	return proto.Clone(h).(*pb.HardState)
}
func cloneState(s State) State {
	out := State{HardState: cloneHard(s.HardState), ConfState: &pb.ConfState{}}
	if s.ConfState != nil {
		out.ConfState = proto.Clone(s.ConfState).(*pb.ConfState)
	}
	for _, e := range s.Entries {
		out.Entries = append(out.Entries, cloneEntry(e))
	}
	return out
}
func logBytes(es []*pb.Entry) uint64 {
	var n uint64
	for _, e := range es {
		n += uint64(proto.Size(e))
	}
	return n
}
func (s *MemoryStore) Stage(b Batch) error {
	if s.staged != nil {
		return fmt.Errorf("target: batch already staged")
	}
	next, err := mergeBatch(s.durable, b, s.limit)
	if err != nil {
		return err
	}
	s.staged = &next
	s.batch = Batch{HardState: cloneHard(b.HardState)}
	for _, e := range b.Entries {
		s.batch.Entries = append(s.batch.Entries, cloneEntry(e))
	}
	return nil
}
func (s *MemoryStore) Flush() error {
	if s.staged == nil {
		return fmt.Errorf("target: no staged batch")
	}
	s.durable = *s.staged
	s.staged = nil
	s.batch = Batch{}
	return nil
}
func (s *MemoryStore) Recover() (State, error) {
	s.staged = nil
	s.batch = Batch{}
	if err := validateState(&s.durable, s.limit); err != nil {
		return State{}, err
	}
	return cloneState(s.durable), nil
}
func (s *MemoryStore) Storage() raft.Storage { return s.view }
func mergeBatch(old State, b Batch, limit uint64) (State, error) {
	if b.Snapshot != nil {
		return State{}, ErrUnsupportedSnapshot
	}
	if limit > maxLogLimitBytes || uint64(len(b.Entries)) > limit/4+3 || logBytes(b.Entries) > limit {
		return State{}, ErrLimit
	}
	next := cloneState(old)
	if len(b.Entries) > 0 {
		first := b.Entries[0]
		if first == nil || first.GetIndex() == 0 {
			return State{}, ErrCorrupt
		}
		start := first.GetIndex()
		if start > uint64(len(next.Entries))+1 {
			return State{}, ErrCorrupt
		}
		for i, e := range b.Entries {
			if e == nil || e.GetIndex() != start+uint64(i) {
				return State{}, ErrCorrupt
			}
			if e.GetIndex() <= old.HardState.GetCommit() {
				if !proto.Equal(e, next.Entries[e.GetIndex()-1]) {
					return State{}, ErrCorrupt
				}
			}
		}
		next.Entries = next.Entries[:start-1]
		for _, e := range b.Entries {
			next.Entries = append(next.Entries, cloneEntry(e))
		}
	}
	if b.HardState != nil {
		if b.HardState.GetTerm() < old.HardState.GetTerm() || b.HardState.GetCommit() < old.HardState.GetCommit() {
			return State{}, ErrCorrupt
		}
		if b.HardState.GetTerm() == old.HardState.GetTerm() && old.HardState.GetVote() != 0 && b.HardState.GetVote() != old.HardState.GetVote() {
			return State{}, ErrCorrupt
		}
		next.HardState = cloneHard(b.HardState)
	}
	if err := validateState(&next, limit); err != nil {
		return State{}, err
	}
	return next, nil
}
func validateState(s *State, limit uint64) error {
	if s.HardState.GetTerm() >= math.MaxUint64-1 {
		return ErrLimit
	}
	if logBytes(s.Entries) > limit {
		return ErrLimit
	}
	if s.HardState.GetCommit() > uint64(len(s.Entries)) || s.HardState.GetVote() > 3 {
		return ErrCorrupt
	}
	s.ConfState = &pb.ConfState{}
	var previous uint64
	for i, e := range s.Entries {
		if e == nil || e.GetIndex() != uint64(i+1) || e.GetTerm() == 0 || e.GetTerm() < previous || e.GetTerm() > s.HardState.GetTerm() {
			return ErrCorrupt
		}
		previous = e.GetTerm()
		if len(e.ProtoReflect().GetUnknown()) != 0 {
			return ErrCorrupt
		}
		if i < 3 {
			var cc pb.ConfChange
			if e.GetType() != pb.EntryConfChange || proto.Unmarshal(e.Data, &cc) != nil || cc.GetType() != pb.ConfChangeAddNode || cc.GetNodeId() != uint64(i+1) || len(cc.GetContext()) != 0 || len(cc.ProtoReflect().GetUnknown()) != 0 {
				return ErrCorrupt
			}
			if e.GetIndex() <= s.HardState.GetCommit() {
				s.ConfState.Voters = append(s.ConfState.Voters, cc.GetNodeId())
			}
		} else {
			if e.GetType() != pb.EntryNormal {
				return fmt.Errorf("%w: dynamic membership", ErrCorrupt)
			}
			if len(e.Data) > 0 {
				if _, err := decodeCommand(e.Data); err != nil {
					return err
				}
			}
		}
	}
	if s.HardState != nil && len(s.HardState.ProtoReflect().GetUnknown()) != 0 {
		return ErrCorrupt
	}
	return nil
}

// storageView only reads the validated flushed state; protobuf objects are
// copied so callers cannot mutate durable data through raft.Storage.
type storageView struct{ s *MemoryStore }

func (v *storageView) InitialState() (*pb.HardState, *pb.ConfState, error) {
	s := cloneState(v.s.durable)
	return s.HardState, s.ConfState, nil
}
func (v *storageView) Entries(lo, hi, maxSize uint64) ([]*pb.Entry, error) {
	if lo < 1 {
		return nil, raft.ErrCompacted
	}
	if hi < lo || hi > uint64(len(v.s.durable.Entries))+1 {
		return nil, raft.ErrUnavailable
	}
	var out []*pb.Entry
	var size uint64
	for _, e := range v.s.durable.Entries[lo-1 : hi-1] {
		n := uint64(proto.Size(e))
		if len(out) > 0 && (size > math.MaxUint64-n || size+n > maxSize) {
			break
		}
		out = append(out, cloneEntry(e))
		size += n
	}
	return out, nil
}
func (v *storageView) Term(i uint64) (uint64, error) {
	if i == 0 {
		return 0, nil
	}
	if i > uint64(len(v.s.durable.Entries)) {
		return 0, raft.ErrUnavailable
	}
	return v.s.durable.Entries[i-1].GetTerm(), nil
}
func (v *storageView) LastIndex() (uint64, error)      { return uint64(len(v.s.durable.Entries)), nil }
func (v *storageView) FirstIndex() (uint64, error)     { return 1, nil }
func (v *storageView) Snapshot() (*pb.Snapshot, error) { return nil, ErrUnsupportedSnapshot }
