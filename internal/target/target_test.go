package target

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
	"math"
	"reflect"
	"testing"
)

type delivery struct {
	from, to uint64
	data     []byte
}
type cluster struct {
	t       *testing.T
	nodes   [4]*Target
	stores  [4]*MemoryStore
	entropy [4]*raftadapter.Recorder
	queue   []delivery
	replies []Effect
	block   func(uint64, uint64) bool
}

func newCluster(t *testing.T, variant Variant) *cluster {
	t.Helper()
	c := &cluster{t: t}
	for id := uint64(1); id <= 3; id++ {
		c.stores[id] = NewMemoryStore(0)
		c.entropy[id], _ = raftadapter.NewRecorder(id, 100+id)
		c.nodes[id], _ = New(Config{ID: id, Store: c.stores[id], Entropy: c.entropy[id], Variant: variant})
		c.step(id, Start{})
	}
	c.step(1, Campaign{})
	c.pump()
	if c.nodes[1].Leader() != 1 {
		t.Fatalf("leader not elected: %+v", c.nodes[1].Status())
	}
	return c
}
func (c *cluster) collect(from uint64, effects []Effect) {
	for _, e := range effects {
		switch e.Kind {
		case SendEffect:
			c.queue = append(c.queue, delivery{from, e.To, append([]byte(nil), e.Data...)})
		case ReplyEffect:
			c.replies = append(c.replies, e)
		}
	}
}
func (c *cluster) step(id uint64, event Event) []Effect {
	c.t.Helper()
	effects, err := c.nodes[id].Step(event)
	c.collect(id, effects)
	if err != nil {
		c.t.Fatalf("node %d event %T: %v", id, event, err)
	}
	return effects
}
func (c *cluster) pump() {
	c.t.Helper()
	for count := 0; len(c.queue) > 0; count++ {
		if count >= 10000 {
			c.t.Fatal("message pump exceeded bound")
		}
		d := c.queue[0]
		c.queue = c.queue[1:]
		if c.block != nil && c.block(d.from, d.to) || c.nodes[d.to].stopped {
			continue
		}
		c.step(d.to, Message{Data: d.data})
	}
}
func (c *cluster) reply(id string) (Effect, bool) {
	for _, r := range c.replies {
		if r.RequestID == id {
			return r, true
		}
	}
	return Effect{}, false
}
func (c *cluster) write(id uint64, request string, value int64) {
	c.t.Helper()
	c.step(id, Write{RequestID: request, Value: value})
	c.pump()
	r, ok := c.reply(request)
	if !ok || r.Value != value {
		c.t.Fatalf("write %q missing or wrong reply: %+v", request, c.replies)
	}
}
func (c *cluster) majorityLeader() uint64 {
	c.t.Helper()
	for i := 0; i < 40; i++ {
		c.step(2, Tick{})
		c.step(3, Tick{})
		c.pump()
		for id := uint64(2); id <= 3; id++ {
			if c.nodes[id].Leader() == id && c.nodes[id].rn.BasicStatus().RaftState.String() == "StateLeader" {
				return id
			}
		}
	}
	c.t.Fatal("majority did not elect within declared 40-tick schedule")
	return 0
}

func TestMajorityWriteReadAndRecoveredDedup(t *testing.T) {
	c := newCluster(t, Safe)
	c.write(1, "w1", 7)
	c.write(1, "w2", 9)
	c.step(1, Read{RequestID: "r1"})
	if _, ok := c.reply("r1"); ok {
		t.Fatal("read replied before quorum messages")
	}
	c.pump()
	if r, ok := c.reply("r1"); !ok || r.Value != 9 {
		t.Fatalf("quorum read: %+v", c.replies)
	}
	before := c.nodes[1].Applied()
	c.step(1, Write{RequestID: "w1", Value: 7})
	if c.nodes[1].Value() != 9 || c.nodes[1].Applied() != before {
		t.Fatal("dedup retry reapplied old value")
	}
	for id := uint64(1); id <= 3; id++ {
		node, err := New(Config{ID: id, Store: c.stores[id], Entropy: c.entropy[id]})
		if err != nil {
			t.Fatal(err)
		}
		c.nodes[id] = node
		c.step(id, Start{})
		if node.Value() != 9 || node.Applied() < before {
			t.Fatalf("recovery lost committed prefix: %+v", node.Status())
		}
		hs, cs, err := c.stores[id].Storage().InitialState()
		if err != nil || hs.GetCommit() < before || !reflect.DeepEqual(cs.Voters, []uint64{1, 2, 3}) {
			t.Fatalf("recovered membership/state: %v %v %v", hs, cs, err)
		}
	}
	c.step(1, Write{RequestID: "w1", Value: 7}) // Result is replicated even on a follower.
	if c.nodes[1].Value() != 9 {
		t.Fatal("recovered retry modified register")
	}
	effects, err := c.nodes[1].Step(Write{RequestID: "w1", Value: 8})
	if err == nil || effects[len(effects)-1].Kind != FailEffect {
		t.Fatal("conflicting retry accepted")
	}
}
func TestIsolatedLeaderHasNoAcknowledgementAndSafeReadBlocks(t *testing.T) {
	c := newCluster(t, Safe)
	c.write(1, "initial", 3)
	c.block = func(from, to uint64) bool { return from == 1 || to == 1 }
	c.step(1, Write{RequestID: "isolated", Value: 99})
	c.pump()
	c.step(1, Read{RequestID: "isolated-read"})
	c.pump()
	if _, ok := c.reply("isolated"); ok {
		t.Fatal("minority write acknowledged")
	}
	if _, ok := c.reply("isolated-read"); ok {
		t.Fatal("minority read returned")
	}
	leader := c.majorityLeader()
	c.write(leader, "majority", 7)
	c.block = nil
	for i := 0; i < 20; i++ {
		for id := uint64(1); id <= 3; id++ {
			c.step(id, Tick{})
		}
		c.pump()
	}
	for id := uint64(1); id <= 3; id++ {
		if c.nodes[id].Value() != 7 {
			t.Fatalf("heal diverged: %+v", c.nodes[id].Status())
		}
	}
	if _, ok := c.reply("isolated"); ok {
		t.Fatal("overwritten minority proposal acknowledged")
	}
}
func TestUnsafeLocalReadSameTargetNegativeControl(t *testing.T) {
	for _, variant := range []Variant{Safe, UnsafeLocalRead} {
		t.Run(string(variant), func(t *testing.T) {
			c := newCluster(t, variant)
			c.block = func(from, to uint64) bool { return from == 1 || to == 1 }
			leader := c.majorityLeader()
			c.write(leader, "completed-majority-write", 7)
			c.step(1, Read{RequestID: "later-read"})
			c.pump()
			r, ok := c.reply("later-read")
			if variant == Safe && ok {
				t.Fatal("safe variant exposed stale value")
			}
			if variant == UnsafeLocalRead && (!ok || r.Value != 0) {
				t.Fatalf("same-target negative control did not expose stale read: %+v", r)
			}
		})
	}
}
func TestReadContextsAreUniqueAndAppliedGate(t *testing.T) {
	c := newCluster(t, Safe)
	c.write(1, "w", 7)
	c.step(1, Read{RequestID: "r1"})
	c.step(1, Read{RequestID: "r2"})
	p1, p2 := c.nodes[1].pending["r1"], c.nodes[1].pending["r2"]
	if p1.context == p2.context || len(p1.context) != 24 {
		t.Fatal("read contexts reused")
	}
	index := c.nodes[1].Applied()
	p1.ready = true
	p1.index = index + 100
	c.pump()
	// A real matching ReadState may replace the deliberately advanced index;
	// test the gate again with no matching ReadState left in this Ready.
	c.nodes[1].pending["gated"] = &request{id: "gated", context: "unmatched", ready: true, index: c.nodes[1].Applied() + 100}
	c.nodes[1].order = append(c.nodes[1].order, "gated")
	c.step(1, Tick{})
	c.pump()
	if _, ok := c.reply("gated"); ok {
		t.Fatal("read bypassed applied-index gate")
	}
	// Restart requires a new election term before admitting reads.
	old := p2.context
	node, _ := New(Config{ID: 1, Store: c.stores[1], Entropy: c.entropy[1]})
	c.nodes[1] = node
	c.step(1, Start{})
	if _, err := node.Step(Read{RequestID: "restarted"}); err == nil {
		t.Fatal("restarted follower admitted read")
	}
	for i := 0; i < 20; i++ {
		for id := uint64(1); id <= 3; id++ {
			c.step(id, Tick{})
		}
		c.pump()
	}
	leader := uint64(0)
	for id := uint64(1); id <= 3; id++ {
		if c.nodes[id].Leader() == id {
			leader = id
			break
		}
	}
	if leader == 0 {
		t.Fatal("no leader after restart")
	}
	c.step(leader, Read{RequestID: "new-read"})
	context := c.nodes[leader].pending["new-read"].context
	if bytes.Equal([]byte(old), []byte(context)) {
		t.Fatal("restart reused read context")
	}
	c.pump()
}
func TestCrashBarrierReleasedPrefixAndDurableRecovery(t *testing.T) {
	for _, boundary := range []Barrier{BeforeAppend, AfterStageBeforeFlush, AfterFlushBeforeSend, AfterSendBeforeApply, AfterApplyBeforeResponse, AfterResponse} {
		t.Run(string(boundary), func(t *testing.T) {
			c := newCluster(t, Safe)
			c.write(1, "previous", 4)
			c.step(1, Write{RequestID: "crash-write", Value: 7})
			last, _ := c.stores[1].Storage().LastIndex()
			fired := false
			c.nodes[1].cfg.Barrier = func(b Barrier) error {
				if !fired && b == boundary && c.nodes[1].rn.BasicStatus().GetCommit() >= last {
					fired = true
					return ErrInjectedCrash
				}
				return nil
			}
			var crashEffects []Effect
			for i := 0; len(c.queue) > 0 && i < 1000; i++ {
				d := c.queue[0]
				c.queue = c.queue[1:]
				if c.nodes[d.to].stopped {
					continue
				}
				effects, err := c.nodes[d.to].Step(Message{Data: d.data})
				c.collect(d.to, effects)
				if err != nil {
					if !errors.Is(err, ErrInjectedCrash) {
						t.Fatal(err)
					}
					crashEffects = effects
					break
				}
			}
			if !fired {
				t.Fatal("barrier not reached")
			}
			if _, err := c.nodes[1].Step(Tick{}); !errors.Is(err, ErrStopped) {
				t.Fatal("crashed target remained mutable")
			}
			foundBarrier := false
			foundReply := false
			foundSend := false
			for _, e := range crashEffects {
				if e.Kind == BarrierEffect && e.Barrier == boundary {
					foundBarrier = true
				}
				if e.Kind == ReplyEffect && e.RequestID == "crash-write" {
					foundReply = true
				}
				if e.Kind == SendEffect {
					foundSend = true
				}
			}
			if !foundBarrier {
				t.Fatal("crash boundary absent from prefix")
			}
			if foundReply != (boundary == AfterResponse) {
				t.Fatalf("response prefix at %s: %+v", boundary, crashEffects)
			}
			if (boundary == BeforeAppend || boundary == AfterStageBeforeFlush || boundary == AfterFlushBeforeSend) && foundSend {
				t.Fatal("messages released before send barrier")
			}
			node, _ := New(Config{ID: 1, Store: c.stores[1], Entropy: c.entropy[1]})
			c.nodes[1] = node
			c.step(1, Start{})
			expected := int64(7)
			if boundary == BeforeAppend || boundary == AfterStageBeforeFlush {
				expected = 4
			}
			if node.Value() != expected {
				t.Fatalf("recovered %d, want %d", node.Value(), expected)
			}
			if foundReply && node.Value() != 7 {
				t.Fatal("completed acknowledged write lost")
			}
			if c.stores[1].staged != nil {
				t.Fatal("crash retained staged state")
			}
		})
	}
}
func TestAdversarialInputLimitsAndStoppedOnEntropy(t *testing.T) {
	c := newCluster(t, Safe)
	cases := []Event{nil, Message{Data: make([]byte, MaxFrameBytes+1)}, Message{Data: []byte{0xff}}, Write{RequestID: "bad/id"}, Read{RequestID: ""}}
	for _, e := range cases {
		if _, err := c.nodes[1].Step(e); err == nil {
			t.Fatalf("accepted %T", e)
		}
	}
	m := &pb.Message{Type: pb.MsgSnap.Enum(), From: new(uint64(2)), To: new(uint64(1)), Snapshot: &pb.Snapshot{}}
	data, _ := proto.Marshal(m)
	if _, err := c.nodes[1].Step(Message{Data: data}); !errors.Is(err, ErrUnsupportedSnapshot) {
		t.Fatal("snapshot accepted")
	}
	m.Type = pb.MsgApp.Enum()
	m.Snapshot = nil
	m.Entries = []*pb.Entry{{Type: pb.EntryConfChangeV2.Enum(), Index: new(uint64(4))}}
	data, _ = proto.Marshal(m)
	if _, err := c.nodes[1].Step(Message{Data: data}); err == nil {
		t.Fatal("dynamic membership accepted")
	}
	c.nodes[1].readSequence = math.MaxUint64
	if _, err := c.nodes[1].Step(Read{RequestID: "overflow"}); !errors.Is(err, ErrLimit) {
		t.Fatal("context counter overflow")
	}
	bad := badEntropy{}
	node, err := New(Config{ID: 1, Store: NewMemoryStore(0), Entropy: bad})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = node.Step(Start{}); err == nil || !node.Status().Stopped {
		t.Fatal("invalid entropy did not stop target")
	}
	if _, err = New(Config{ID: 4, Store: NewMemoryStore(0), Entropy: bad}); err == nil {
		t.Fatal("unsupported voter accepted")
	}
	c.nodes[1].cfg.LogLimitBytes = 1
	if _, err := c.nodes[1].Step(Write{RequestID: "bounded", Value: 1}); !errors.Is(err, ErrLimit) {
		t.Fatal("unbounded proposal")
	}
}

type badEntropy struct{}

func (badEntropy) Draw(string, int) (int, error) { return -1, nil }

func TestMemoryStoreStagingAndOwnership(t *testing.T) {
	c := newCluster(t, Safe)
	s := c.stores[1]
	state, _ := s.Recover()
	hs := cloneHard(state.HardState)
	hs.Term = new(hs.GetTerm() + 1)
	if err := s.Stage(Batch{HardState: hs}); err != nil {
		t.Fatal(err)
	}
	hs.Term = new(uint64(999))
	recovered, err := s.Recover()
	if err != nil || recovered.HardState.GetTerm() != state.HardState.GetTerm() {
		t.Fatal("unflushed state survived")
	}
	hs = cloneHard(state.HardState)
	hs.Term = new(hs.GetTerm() + 1)
	if err = s.Stage(Batch{HardState: hs}); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	recovered, _ = s.Recover()
	if recovered.HardState.GetTerm() != hs.GetTerm() {
		t.Fatal("flushed state lost")
	}
	recovered.Entries[0].Data = nil
	view, _ := s.Storage().Entries(1, 2, 0)
	view[0].Data = nil
	again, _ := s.Recover()
	if len(again.Entries[0].Data) == 0 {
		t.Fatal("storage callers mutated durable state")
	}
	if err = s.Stage(Batch{Snapshot: &pb.Snapshot{}}); !errors.Is(err, ErrUnsupportedSnapshot) {
		t.Fatal("snapshot silently ignored")
	}
	// Corrupt commit beyond the prefix must not replace the readable view.
	bad := cloneHard(hs)
	bad.Commit = new(uint64(10000))
	if err = s.Stage(Batch{HardState: bad}); !errors.Is(err, ErrCorrupt) {
		t.Fatal("out-of-prefix commit accepted")
	}
}
func TestReplicatedCommandCodec(t *testing.T) {
	for _, v := range []int64{0, 7, -7, math.MinInt64, math.MaxInt64} {
		b := encodeCommand("request_1", v)
		c, err := decodeCommand(b)
		if err != nil || c.value != v || c.id != "request_1" {
			t.Fatal("command roundtrip")
		}
		b[0] = 2
		if _, err = decodeCommand(b); err == nil {
			t.Fatal("version accepted")
		}
	}
	b := make([]byte, 24)
	binary.BigEndian.PutUint64(b, 1)
	if _, err := decodeCommand(b); err == nil {
		t.Fatal("foreign context accepted as command")
	}
}

func TestPendingRetriesReproposeAndRenewQuorumContext(t *testing.T) {
	c := newCluster(t, Safe)
	c.step(1, Write{RequestID: "retry", Value: 7})
	first, _ := c.stores[1].Storage().LastIndex()
	c.queue = nil // All messages from the first admitted proposal were dropped.
	c.step(1, Write{RequestID: "retry", Value: 7})
	second, _ := c.stores[1].Storage().LastIndex()
	if second <= first {
		t.Fatal("pending write retry did not repropose")
	}
	c.pump()
	if r, ok := c.reply("retry"); !ok || r.Value != 7 {
		t.Fatal("write retry did not complete")
	}
	count := 0
	for _, r := range c.replies {
		if r.RequestID == "retry" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("duplicate replicated command created multiple replies")
	}
	c.step(1, Read{RequestID: "read-retry"})
	old := c.nodes[1].pending["read-retry"].context
	c.queue = nil
	c.step(1, Read{RequestID: "read-retry"})
	current := c.nodes[1].pending["read-retry"].context
	if old == current {
		t.Fatal("pending read retry reused abandoned quorum context")
	}
	c.pump()
	if r, ok := c.reply("read-retry"); !ok || r.Value != 7 {
		t.Fatal("renewed read retry did not complete")
	}
}

func TestCrashBeforeProposalFlushHasNoDurableTail(t *testing.T) {
	for _, boundary := range []Barrier{BeforeAppend, AfterStageBeforeFlush} {
		t.Run(string(boundary), func(t *testing.T) {
			c := newCluster(t, Safe)
			before, _ := c.stores[1].Recover()
			c.nodes[1].cfg.Barrier = func(b Barrier) error {
				if b == boundary {
					return ErrInjectedCrash
				}
				return nil
			}
			effects, err := c.nodes[1].Step(Write{RequestID: "volatile", Value: 99})
			if !errors.Is(err, ErrInjectedCrash) {
				t.Fatal(err)
			}
			for _, e := range effects {
				if e.Kind == SendEffect || e.Kind == ReplyEffect || e.Kind == PersistEffect {
					t.Fatal("pre-flush crash released effect")
				}
			}
			after, _ := c.stores[1].Recover()
			if len(before.Entries) != len(after.Entries) || !proto.Equal(before.HardState, after.HardState) {
				t.Fatal("volatile proposal survived modeled crash")
			}
		})
	}
}

func TestAdmissionBoundsAndBarrierErrorStopsTarget(t *testing.T) {
	c := newCluster(t, Safe)
	for i := 0; i < MaxRequests; i++ {
		id := fmt.Sprintf("p%d", i)
		c.nodes[1].pending[id] = &request{id: id, context: id}
	}
	if _, err := c.nodes[1].Step(Read{RequestID: "over-cap"}); !errors.Is(err, ErrLimit) {
		t.Fatal("read request cap bypassed")
	}
	if _, err := c.nodes[1].Step(Write{RequestID: "over-cap", Value: 1}); !errors.Is(err, ErrLimit) {
		t.Fatal("write request cap bypassed")
	}
	c = newCluster(t, Safe)
	c.write(1, "dedup", 1)
	c.nodes[1].cfg.Barrier = func(Barrier) error { return errors.New("barrier failure") }
	if _, err := c.nodes[1].Step(Write{RequestID: "dedup", Value: 1}); err == nil || !c.nodes[1].stopped {
		t.Fatal("direct reply barrier error did not stop target")
	}
}

func TestTermOverflowStopsBeforeRaftIncrement(t *testing.T) {
	c := newCluster(t, Safe)
	m := &pb.Message{Type: pb.MsgHeartbeat.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(uint64(math.MaxUint64 - 2))}
	data, _ := proto.Marshal(m)
	c.step(1, Message{Data: data})
	if _, err := c.nodes[1].Step(Campaign{}); !errors.Is(err, ErrLimit) || !c.nodes[1].stopped {
		t.Fatal("term headroom did not stop overflow")
	}
}
