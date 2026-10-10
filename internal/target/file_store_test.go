package target

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRealJournalTargetRecovery(t *testing.T) {
	c := newCluster(t, Safe)
	path := filepath.Join(t.TempDir(), "node1.journal")
	file, err := NewFileStore(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the model store with a journal before any client writes. This
	// bootstraps and elects the exact same Target implementation.
	c.nodes[1], _ = New(Config{ID: 1, Store: file, Entropy: c.entropy[1]})
	c.step(1, Start{})
	for i := 0; i < 30; i++ {
		for id := uint64(1); id <= 3; id++ {
			c.step(id, Tick{})
		}
		c.pump()
	}
	leader := c.nodes[1].Leader()
	if leader == 0 {
		t.Fatal("leader missing")
	}
	c.write(leader, "real-w1", 7)
	c.write(leader, "real-w2", 9)
	for i := 0; i < 3; i++ {
		c.step(leader, Tick{})
		c.pump()
	}
	if c.nodes[1].Value() != 9 {
		t.Fatal("journal node did not receive committed writes")
	}
	applied := c.nodes[1].Applied()
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	file, err = NewFileStore(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	node, err := New(Config{ID: 1, Store: file, Entropy: c.entropy[1]})
	if err != nil {
		t.Fatal(err)
	}
	effects, err := node.Step(Start{})
	if err != nil {
		t.Fatal(err)
	}
	if node.Value() != 9 || node.Applied() != applied {
		t.Fatalf("fsync journal lost completed state: %+v", node.Status())
	}
	effects, err = node.Step(Write{RequestID: "real-w1", Value: 7})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range effects {
		if e.Kind == ReplyEffect && e.RequestID == "real-w1" && e.Value == 7 {
			found = true
		}
	}
	if !found || node.Value() != 9 {
		t.Fatal("journal did not recover replicated retry result")
	}
	if _, err = node.Step(Write{RequestID: "real-w1", Value: 10}); err == nil {
		t.Fatal("journal recovery lost payload identity")
	}
}
func journalFixture(t *testing.T) ([]byte, int) {
	t.Helper()
	c := newCluster(t, Safe)
	state, err := c.stores[1].Recover()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.journal")
	s, err := NewFileStore(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Stage(Batch{Entries: state.Entries, HardState: state.HardState}); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := cloneHard(state.HardState)
	h.Term = new(h.GetTerm() + 1)
	h.Vote = new(uint64(0))
	if err = s.Stage(Batch{HardState: h}); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data, len(first)
}
func TestFileStoreEveryInterruptedTailRetainsCompletePrefix(t *testing.T) {
	data, prefix := journalFixture(t)
	for cut := prefix; cut < len(data); cut++ {
		path := filepath.Join(t.TempDir(), "tail.journal")
		if err := os.WriteFile(path, data[:cut], 0600); err != nil {
			t.Fatal(err)
		}
		s, err := NewFileStore(path, 0)
		if err != nil {
			t.Fatalf("cut %d: %v", cut-prefix, err)
		}
		state, err := s.Recover()
		if err != nil {
			t.Fatal(err)
		}
		if state.HardState.GetTerm() != 2 || state.HardState.GetCommit() != 4 {
			t.Fatalf("cut %d lost prefix: %v", cut-prefix, state.HardState)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() != int64(prefix) {
			t.Fatalf("cut %d did not remove torn suffix", cut-prefix)
		}
		// Reusing the recovered journal appends to the validated prefix, rather
		// than leaving an ignored torn record in front of future successful fsyncs.
		h := cloneHard(state.HardState)
		h.Term = new(uint64(3))
		h.Vote = new(uint64(0))
		if err = s.Stage(Batch{HardState: h}); err != nil {
			t.Fatal(err)
		}
		if err = s.Flush(); err != nil {
			t.Fatal(err)
		}
		s.Close()
		s, err = NewFileStore(path, 0)
		if err != nil {
			t.Fatal(err)
		}
		state, err = s.Recover()
		s.Close()
		if err != nil || state.HardState.GetTerm() != 3 {
			t.Fatal("append after prefix recovery failed")
		}
	}
}
func TestFileStoreCorruptionFailsClosed(t *testing.T) {
	data, prefix := journalFixture(t)
	// Cover magic, length, complemented length, checksum and protobuf payload,
	// including both a prior record and the complete final record.
	offsets := []int{0, 8, 15, 16, 23, 24, 55, 56, prefix, prefix + 7, prefix + 8, prefix + 16, len(data) - 1}
	for _, offset := range offsets {
		t.Run(string(rune(offset+65)), func(t *testing.T) {
			altered := append([]byte(nil), data...)
			altered[offset] ^= 1
			path := filepath.Join(t.TempDir(), "corrupt.journal")
			if err := os.WriteFile(path, altered, 0600); err != nil {
				t.Fatal(err)
			}
			if s, err := NewFileStore(path, 0); !errors.Is(err, ErrCorrupt) {
				if s != nil {
					s.Close()
				}
				t.Fatalf("byte %d corruption accepted: %v", offset, err)
			}
			actual, _ := os.ReadFile(path)
			if !bytes.Equal(actual, altered) {
				t.Fatal("corruption was concealed by truncation")
			}
		})
	}
}
func TestFileStoreStageDiscardAndUncertainIOStopsMutation(t *testing.T) {
	data, prefix := journalFixture(t)
	path := filepath.Join(t.TempDir(), "stage.journal")
	if err := os.WriteFile(path, data[:prefix], 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewFileStore(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, _ := s.Recover()
	h := cloneHard(state.HardState)
	h.Term = new(uint64(3))
	h.Vote = new(uint64(0))
	if err = s.Stage(Batch{HardState: h}); err != nil {
		t.Fatal(err)
	}
	recovered, _ := s.Recover()
	if recovered.HardState.GetTerm() != 2 {
		t.Fatal("staged journal data survived crash")
	}
	if err = s.Stage(Batch{HardState: h}); err != nil {
		t.Fatal(err)
	}
	// Closing the actual file injects a real failed IO boundary, not a model
	// pretending that MemoryStorage is a filesystem.
	if err = s.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err == nil {
		t.Fatal("closed-file flush succeeded")
	}
	if err = s.Stage(Batch{}); !errors.Is(err, ErrStopped) {
		t.Fatal("uncertain IO allowed continued writes")
	}
	if _, err = s.Recover(); !errors.Is(err, ErrStopped) {
		t.Fatal("poisoned handle recovered without reopening")
	}
}
func TestFileStoreBoundsAndChecksummedSemanticCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.journal")
	s, err := NewFileStore(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	// A checksummed but semantically invalid batch must be rejected too.
	payload, err := encodeBatch(Batch{HardState: &pb.HardState{Term: new(uint64(1)), Commit: new(uint64(99))}})
	if err != nil {
		t.Fatal(err)
	}
	record := testRecord(payload)
	if _, err = s.file.Write(record); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = NewFileStore(path, 0); !errors.Is(err, ErrCorrupt) {
		if s != nil {
			s.Close()
		}
		t.Fatalf("invalid checked record accepted: %v", err)
	}
	if _, err = NewFileStore(filepath.Join(t.TempDir(), "huge.journal"), maxLogLimitBytes+1); !errors.Is(err, ErrLimit) {
		t.Fatal("unbounded journal limit")
	}
	small := filepath.Join(t.TempDir(), "oversize.journal")
	if err = os.WriteFile(small, make([]byte, 5000), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = NewFileStore(small, 1); !errors.Is(err, ErrLimit) {
		t.Fatal("oversize journal read")
	}
}
func testRecord(payload []byte) []byte {
	r := make([]byte, 48+len(payload))
	binary.BigEndian.PutUint64(r, uint64(len(payload)))
	binary.BigEndian.PutUint64(r[8:], ^uint64(len(payload)))
	// Use the journal's checksum without weakening the semantic decoder test.
	sum := shaSum(payload)
	copy(r[16:48], sum)
	copy(r[48:], payload)
	return r
}
func shaSum(data []byte) []byte { h := sha256.Sum256(data); return h[:] }

type auditStore struct {
	Store
	flushed    bool
	flushError error
	stages     int
	flushes    int
}

func (s *auditStore) Stage(b Batch) error { s.flushed = false; s.stages++; return s.Store.Stage(b) }
func (s *auditStore) Flush() error {
	s.flushes++
	if s.flushError != nil {
		return s.flushError
	}
	if err := s.Store.Flush(); err != nil {
		return err
	}
	s.flushed = true
	return nil
}
func TestReadyDurableBeforeMessagesApplyAndReply(t *testing.T) {
	c := newCluster(t, Safe)
	a := &auditStore{Store: c.stores[1], flushed: true}
	c.nodes[1].cfg.Store = a
	var beforeValue int64
	c.nodes[1].cfg.Barrier = func(b Barrier) error {
		if b == BeforeAppend {
			beforeValue = c.nodes[1].Value()
		}
		if b == AfterStageBeforeFlush && a.flushed {
			t.Fatal("stage already durable")
		}
		if b == AfterFlushBeforeSend || b == AfterSendBeforeApply || b == AfterApplyBeforeResponse || b == AfterResponse {
			if !a.flushed {
				t.Fatalf("%s before flush", b)
			}
		}
		if b == AfterSendBeforeApply && c.nodes[1].Value() != beforeValue {
			t.Fatal("apply preceded release boundary")
		}
		return nil
	}
	c.write(1, "audited", 7)
	if a.stages == 0 || a.stages != a.flushes {
		t.Fatal("Ready did not stage+flush")
	}
	a.flushError = errors.New("uncertain flush")
	effects, err := c.nodes[1].Step(Write{RequestID: "failed-flush", Value: 9})
	if err == nil || !c.nodes[1].stopped {
		t.Fatal("failed persistence did not stop target")
	}
	for _, e := range effects {
		if e.Kind == SendEffect || e.Kind == ReplyEffect {
			t.Fatal("failed flush released dependent effect")
		}
	}
	if _, err = c.nodes[1].Step(Tick{}); !errors.Is(err, ErrStopped) {
		t.Fatal("persistence failure kept target mutable")
	}
}
func TestAdversarialFramesRejectPanicShapes(t *testing.T) {
	c := newCluster(t, Safe)
	term := c.nodes[1].Status().Term
	messages := []*pb.Message{
		{Type: pb.MsgHeartbeat.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term), Commit: new(uint64(100000))},
		{Type: pb.MsgAppResp.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term), Index: new(uint64(100000))},
		{Type: pb.MsgApp.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term), Entries: []*pb.Entry{{}}},
		{Type: pb.MsgApp.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term), Index: new(uint64(3)), Entries: []*pb.Entry{{Type: pb.EntryNormal.Enum(), Index: new(uint64(99)), Term: new(term)}}},
		{Type: pb.MsgHeartbeat.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term), Responses: []*pb.Message{{Type: pb.MsgHup.Enum()}}},
	}
	for _, m := range messages {
		data, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
		if _, err := c.nodes[1].Step(Message{Data: data}); err == nil {
			t.Fatalf("accepted panic shape %v", m)
		}
	}
	// Duplicate top-level protobuf fields are rejected by canonical validation.
	data, _ := proto.Marshal(&pb.Message{Type: pb.MsgHeartbeat.Enum(), From: new(uint64(2)), To: new(uint64(1)), Term: new(term)})
	data = append(data, 0x08, byte(pb.MsgHeartbeat))
	if _, err := c.nodes[1].Step(Message{Data: data}); err == nil {
		t.Fatal("duplicate protobuf field accepted")
	}
}
func TestEntropyChoicesAndEffectTraceRepeatExactly(t *testing.T) {
	var want []byte
	for i := 0; i < 10; i++ {
		c := newCluster(t, Safe)
		var effects []Effect
		for _, event := range []Event{Write{RequestID: "w", Value: 7}, Read{RequestID: "r"}, Tick{}} {
			effects = append(effects, c.step(1, event)...)
			c.pump()
		}
		data, err := json.Marshal(struct {
			Effects []Effect
			Replies []Effect
			Choices [3][]raftadapter.Choice
		}{effects, c.replies, [3][]raftadapter.Choice{c.entropy[1].Choices(), c.entropy[2].Choices(), c.entropy[3].Choices()}})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			want = data
		} else if !bytes.Equal(want, data) {
			t.Fatalf("run %d differs", i)
		}
	}
}
