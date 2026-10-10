// Package target integrates instrumented etcd/raft with an explicitly durable
// store and a fixed three-voter integer register. Calls have one serialized owner.
package target

import (
	"errors"

	"github.com/C-X1an/faultbraid/internal/raftadapter"
	raft "go.etcd.io/raft/v3"
	pb "go.etcd.io/raft/v3/raftpb"
)

const (
	DefaultLogLimitBytes uint64 = 1 << 20
	MaxFrameBytes               = 64 << 10
	MaxRequests                 = 4096
)

var (
	ErrInjectedCrash       = errors.New("target: injected crash")
	ErrLimit               = errors.New("target: resource limit exceeded")
	ErrCorrupt             = errors.New("target: corrupt durable state")
	ErrStopped             = errors.New("target: stopped; restart required")
	ErrUnsupportedSnapshot = errors.New("target: snapshots are unsupported")
)

type Variant string

const (
	Safe            Variant = "raft-register-v1"
	UnsafeLocalRead Variant = "unsafe-local-read"
)

type Barrier string

const (
	BeforeAppend             Barrier = "before-append"
	AfterStageBeforeFlush    Barrier = "after-stage-before-flush"
	AfterFlushBeforeSend     Barrier = "after-flush-before-send"
	AfterSendBeforeApply     Barrier = "after-send-before-apply"
	AfterApplyBeforeResponse Barrier = "after-apply-before-response"
	AfterResponse            Barrier = "after-response"
)

type Config struct {
	ID            uint64
	Store         Store
	Entropy       raftadapter.Entropy
	LogLimitBytes uint64
	Barrier       func(Barrier) error
	Variant       Variant
}
type Event interface{ targetEvent() }
type Start struct{}
type Tick struct{}
type Campaign struct{}
type Message struct{ Data []byte }
type Write struct {
	RequestID string
	Value     int64
}
type Read struct{ RequestID string }

func (Start) targetEvent()    {}
func (Tick) targetEvent()     {}
func (Campaign) targetEvent() {}
func (Message) targetEvent()  {}
func (Write) targetEvent()    {}
func (Read) targetEvent()     {}

type EffectKind string

const (
	SendEffect    EffectKind = "send"
	ReplyEffect   EffectKind = "reply"
	BarrierEffect EffectKind = "barrier"
	PersistEffect EffectKind = "persist"
	FailEffect    EffectKind = "fail"
)

// Data is an immutable copied protobuf frame. A reply is a completed operation;
// absence of a reply means pending unless an explicit rejection is returned.
type Effect struct {
	Kind      EffectKind
	To        uint64
	Data      []byte
	RequestID string
	Value     int64
	Barrier   Barrier
	Code      string
	Index     uint64
}
type Batch struct {
	Entries   []*pb.Entry
	HardState *pb.HardState
	Snapshot  *pb.Snapshot
}
type State struct {
	Entries   []*pb.Entry
	HardState *pb.HardState
	ConfState *pb.ConfState
}

// Storage exposes only the flushed view. Recover discards staged data and
// reconstructs fixed membership from the durable committed bootstrap prefix.
type Store interface {
	Stage(Batch) error
	Flush() error
	Recover() (State, error)
	Storage() raft.Storage
}
type Status struct {
	ID      uint64
	Leader  uint64
	Term    uint64
	Commit  uint64
	Applied uint64
	Value   int64
	Started bool
	Stopped bool
	Variant Variant
	Pending int
}
