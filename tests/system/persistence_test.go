package system

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/target"
)

var persistenceBarriers = []target.Barrier{
	target.BeforeAppend, target.AfterStageBeforeFlush, target.AfterFlushBeforeSend,
	target.AfterSendBeforeApply, target.AfterApplyBeforeResponse, target.AfterResponse,
}

func persistenceInvokeAll(s *scenario.Scenario, at uint64, kind, id string, value int64) {
	start := len(s.Actions)
	invokeAll(s, at, kind, id, value)
	for i := start; i < len(s.Actions); i++ {
		s.Actions[i].ID = fmt.Sprintf("%s-%d-%d", id, at, *s.Actions[i].Node)
	}
}

func persistenceScenario(boundary target.Barrier) scenario.Scenario {
	s := scenario.Default(0)
	persistenceInvokeAll(&s, 30, "invoke_write", "previous7", 7)
	s.Actions = append(s.Actions,
		scenario.Action{ID: "arm", At: 35, Kind: "crash_at", Node: scenario.Node(2), Boundary: scenario.Text(string(boundary)), Occurrence: scenario.Count(1)},
		scenario.Action{ID: "interrupted", At: 35, Kind: "invoke_write", Node: scenario.Node(2), RequestID: scenario.Text("interrupted9"), Value: scenario.Value(9)},
		scenario.Action{ID: "down-read", At: 40, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("down-read")},
		scenario.Action{ID: "restart", At: 45, Kind: "restart", Node: scenario.Node(2)},
	)
	persistenceInvokeAll(&s, 70, "invoke_read", "recovered", 0)
	persistenceInvokeAll(&s, 85, "invoke_write", "interrupted9", 9)
	persistenceInvokeAll(&s, 100, "invoke_write", "later11", 11)
	persistenceInvokeAll(&s, 115, "invoke_write", "interrupted9", 9)
	persistenceInvokeAll(&s, 125, "invoke_read", "final", 0)
	s.EndTime = scenario.Node(140)
	return s
}

// AC-013: real three-voter RawNode transitions with MemoryStore's declared
// flush contract. The separate file test below uses the same production target.
// Neither test claims physical power loss or definite failure of a timed-out write.
func TestAC013NamedBarrierCrashRecoveryMatrix(t *testing.T) {
	for _, boundary := range persistenceBarriers {
		t.Run(string(boundary), func(t *testing.T) {
			s := persistenceScenario(boundary)
			prefixScenario := s
			prefixScenario.Actions = append([]scenario.Action(nil), s.Actions[:6]...)
			prefixScenario.EndTime = scenario.Node(40)
			prefix := run(t, prefixScenario)
			trace := run(t, s)
			crash := persistenceCrash(t, prefix, boundary)
			fullCrash := persistenceCrash(t, trace, boundary)
			if crash.ID != fullCrash.ID || crash.At != fullCrash.At || fullCrash.Incarnation != 1 {
				t.Fatal("the recovered run changed the executed crash prefix")
			}
			previous := operation(t, prefix, "previous7")
			interrupted := operation(t, prefix, "interrupted9")
			if previous.Return == nil || (interrupted.Return != nil) != (boundary == target.AfterResponse) || operation(t, prefix, "down-read").Return != nil {
				t.Fatal("the crash prefix lost a completed operation or fabricated a response")
			}
			if prefix.Result.Status != history.Valid || trace.Result.Status != history.Valid || prefix.Failure != nil || trace.Failure != nil {
				t.Fatalf("nonlinearizable crash/recovery history: prefix=%+v recovered=%+v", prefix.Result, trace.Result)
			}
			observedReplies := 0
			for _, event := range prefix.Events {
				if event.Kind == "reply" && event.Detail == "interrupted9" {
					observedReplies++
					if event.ID >= crash.ID || event.Parent != crash.Parent {
						t.Fatal("after-response completion was not released before the crash")
					}
				}
			}
			if observedReplies != persistenceBoolCount(boundary == target.AfterResponse) {
				t.Fatal("history completion does not match centrally collected target replies")
			}
			recovered, final := operation(t, trace, "recovered"), operation(t, trace, "final")
			retry, later := operation(t, trace, "interrupted9"), operation(t, trace, "later11")
			wantRecovered := int64(7)
			if boundary == target.AfterSendBeforeApply || boundary == target.AfterApplyBeforeResponse || boundary == target.AfterResponse {
				wantRecovered = 9
			}
			if recovered.Return == nil || recovered.Value != wantRecovered || retry.Return == nil || later.Return == nil || final.Return == nil || final.Value != 11 {
				t.Fatalf("completed recovery/retry/read evidence absent: %+v", trace.History)
			}
			if *previous.Return >= recovered.Call || *later.Return >= final.Call || retry.Call != interrupted.Call || operation(t, trace, "down-read").Return != nil {
				t.Fatal("central invocation/response order or pending uncertainty changed across recovery")
			}
			if interrupted.Return != nil && *retry.Return != *interrupted.Return {
				t.Fatal("retry rewrote an earlier observed completion")
			}
			starts, lateRetryReplies := 0, 0
			for _, event := range trace.Events {
				if event.Kind == "start" && event.Node == 2 && event.Incarnation == 2 && event.At == 45 {
					starts++
				}
				if event.Kind == "reply" && event.Detail == "interrupted9" && event.At >= 115 {
					lateRetryReplies++
				}
			}
			if starts != 1 || lateRetryReplies != 3 || len(trace.History.Operations) != 6 {
				t.Fatal("restart or replicated request-ID retry/dedup evidence absent")
			}
			for _, saved := range []scenario.Trace{prefix, trace} {
				if err := scenario.Replay(context.Background(), saved, identity(t)); err != nil {
					t.Fatal(err)
				}
			}
			data, err := json.Marshal(map[string]any{"barrier": boundary, "crash": crash, "prefix_history": prefix.History, "recovered_history": trace.History, "prefix_digest": prefix.Digest, "recovered_digest": trace.Digest, "source": trace.Header.SourceSHA, "module": trace.Header.ModuleSHA, "patch": trace.Header.PatchSHA})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("AC-013 model crash matrix %s", data)
		})
	}
}

func persistenceBoolCount(ok bool) int {
	if ok {
		return 1
	}
	return 0
}

func persistenceCrash(t *testing.T, trace scenario.Trace, boundary target.Barrier) scenario.Record {
	t.Helper()
	var crash scenario.Record
	for _, event := range trace.Events {
		if event.Kind == "crash" {
			if crash.ID != 0 || event.Node != 2 || event.Detail != "barrier" || event.Parent == 0 || event.Parent >= event.ID {
				t.Fatal("missing or unrelated named-barrier crash")
			}
			crash = event
		}
	}
	if crash.ID == 0 {
		t.Fatal("the named barrier never fired")
	}
	step := trace.Events[crash.Parent-1]
	boundaries := strings.Split(step.Detail, ",")
	if len(boundaries) == 0 || boundaries[len(boundaries)-1] != string(boundary) || step.Node != crash.Node || step.At != crash.At {
		t.Fatalf("crash was not caused by %s: %+v", boundary, step)
	}
	return crash
}

// journalObservedStore delegates to the production Store and keeps only call
// ordering diagnostics. It never implements an alternative persistence model.
type journalObservedStore struct {
	target.Store
	path                          string
	unflushed                     bool
	stages, flushes, growingFlush int
}

func (s *journalObservedStore) Stage(b target.Batch) error {
	if err := s.Store.Stage(b); err != nil {
		return err
	}
	s.unflushed = true
	s.stages++
	return nil
}

func (s *journalObservedStore) Flush() error {
	var before int64
	if s.path != "" {
		info, err := os.Stat(s.path)
		if err != nil {
			return err
		}
		before = info.Size()
	}
	if err := s.Store.Flush(); err != nil {
		return err
	}
	s.unflushed = false
	s.flushes++
	if s.path != "" {
		info, err := os.Stat(s.path)
		if err != nil {
			return err
		}
		if info.Size() > before {
			s.growingFlush++
		}
	}
	return nil
}

func (s *journalObservedStore) Recover() (target.State, error) {
	state, err := s.Store.Recover()
	if err == nil {
		s.unflushed = false
	}
	return state, err
}

type journalDelivery struct {
	from, to, sourceInc, destInc uint64
	data                         []byte
}

type journalMatrixCluster struct {
	t         *testing.T
	nodes     [4]*target.Target
	stores    [4]*journalObservedStore
	files     [4]*target.FileStore
	entropy   [4]*raftadapter.Recorder
	inc       [4]uint64
	queue     []journalDelivery
	history   history.History
	ordinal   int64
	arm       target.Barrier
	armIndex  uint64
	fired     bool
	crash     []target.Effect
	crashNode uint64
}

func newJournalMatrixCluster(t *testing.T, onDisk bool) *journalMatrixCluster {
	t.Helper()
	c := &journalMatrixCluster{t: t, history: history.History{Version: 1, Operations: []history.Operation{}}}
	directory := ""
	if onDisk {
		directory = t.TempDir()
		t.Cleanup(func() {
			for id := uint64(1); id <= 3; id++ {
				if c.files[id] != nil {
					if err := c.files[id].Close(); err != nil {
						t.Errorf("close voter %d journal: %v", id, err)
					}
				}
			}
		})
	}
	for id := uint64(1); id <= 3; id++ {
		c.inc[id] = 1
		var store target.Store = target.NewMemoryStore(0)
		path := ""
		if onDisk {
			path = filepath.Join(directory, fmt.Sprintf("node%d.journal", id))
			file, err := target.NewFileStore(path, 0)
			if err != nil {
				t.Fatal(err)
			}
			c.files[id], store = file, file
		}
		c.stores[id] = &journalObservedStore{Store: store, path: path}
		c.entropy[id], _ = raftadapter.NewRecorder(id, 100+id)
		c.newNode(id)
		c.step(id, target.Start{})
	}
	c.step(1, target.Campaign{})
	c.pump()
	if c.nodes[1].Leader() != 1 {
		t.Fatal("the declared election did not produce voter 1 as leader")
	}
	return c
}

func (c *journalMatrixCluster) newNode(id uint64) {
	c.t.Helper()
	node, err := target.New(target.Config{ID: id, Store: c.stores[id], Entropy: c.entropy[id], Barrier: func(b target.Barrier) error {
		// Observe actual Stage and successful Flush calls. A syntax-preserving
		// bypass of target.Store.Flush must fail at this real target boundary.
		if b == target.AfterFlushBeforeSend && c.stores[id].unflushed {
			return errors.New("durability invariant: after-flush barrier preceded successful Store.Flush")
		}
		if id == 1 && !c.fired && c.arm != "" && b == c.arm && c.nodes[id].Status().Commit >= c.armIndex {
			c.fired = true
			return target.ErrInjectedCrash
		}
		return nil
	}})
	if err != nil {
		c.t.Fatal(err)
	}
	c.nodes[id] = node
}

func (c *journalMatrixCluster) invoke(id uint64, event target.Event) {
	c.t.Helper()
	var op history.Operation
	switch e := event.(type) {
	case target.Write:
		op = history.Operation{ID: e.RequestID, Kind: "write", Value: e.Value}
	case target.Read:
		op = history.Operation{ID: e.RequestID, Kind: "read"}
	default:
		c.t.Fatal("client invocation requires read or write")
	}
	newRequest := true
	for _, prior := range c.history.Operations {
		if prior.ID == op.ID {
			newRequest = false
			if prior.Kind != op.Kind || op.Kind == "write" && prior.Value != op.Value {
				c.t.Fatal("test retry conflicts with the original invocation")
			}
		}
	}
	if newRequest {
		c.ordinal++
		op.Call = c.ordinal
		c.history.Operations = append(c.history.Operations, op)
	}
	c.step(id, event)
}

func (c *journalMatrixCluster) step(id uint64, event target.Event) {
	c.t.Helper()
	effects, err := c.nodes[id].Step(event)
	for _, effect := range effects {
		switch effect.Kind {
		case target.SendEffect:
			c.queue = append(c.queue, journalDelivery{id, effect.To, c.inc[id], c.inc[effect.To], append([]byte(nil), effect.Data...)})
		case target.ReplyEffect:
			found := false
			for i := range c.history.Operations {
				op := &c.history.Operations[i]
				if op.ID == effect.RequestID {
					found = true
					if op.Kind == "write" && op.Value != effect.Value {
						c.t.Fatal("write acknowledgement has the wrong payload")
					}
					if op.Return == nil {
						c.ordinal++
						end := c.ordinal
						op.Return = &end
						if op.Kind == "read" {
							op.Value = effect.Value
						}
					}
				}
			}
			if !found {
				c.t.Fatal("reply lacks a central invocation")
			}
		}
	}
	if errors.Is(err, target.ErrInjectedCrash) {
		if c.crashNode != 0 {
			c.t.Fatal("more than one injected crash")
		}
		c.crashNode, c.crash = id, effects
		return
	}
	if err != nil {
		c.t.Fatalf("node %d event %T: %v", id, event, err)
	}
}

func (c *journalMatrixCluster) pump() {
	c.t.Helper()
	for count := 0; len(c.queue) > 0; count++ {
		if count >= 10000 {
			c.t.Fatal("the declared message schedule exceeded 10000 deliveries")
		}
		delivery := c.queue[0]
		c.queue = c.queue[1:]
		if delivery.sourceInc != c.inc[delivery.from] || delivery.destInc != c.inc[delivery.to] || c.nodes[delivery.to].Status().Stopped {
			continue
		}
		c.step(delivery.to, target.Message{Data: delivery.data})
		if c.crashNode != 0 {
			return
		}
	}
}

func (c *journalMatrixCluster) op(id string) history.Operation {
	c.t.Helper()
	for _, op := range c.history.Operations {
		if op.ID == id {
			return op
		}
	}
	c.t.Fatalf("central operation %s absent", id)
	return history.Operation{}
}

func (c *journalMatrixCluster) completedWrite(id string, value int64) {
	c.t.Helper()
	c.invoke(1, target.Write{RequestID: id, Value: value})
	c.pump()
	if c.op(id).Return == nil {
		c.t.Fatalf("write %s did not receive a quorum acknowledgement", id)
	}
}

func TestAC013FlushBeforeSendAndResponse(t *testing.T) {
	c := newJournalMatrixCluster(t, false)
	c.completedWrite("flush-order-write", 7)
	c.invoke(1, target.Read{RequestID: "flush-order-read"})
	c.pump()
	w, r := c.op("flush-order-write"), c.op("flush-order-read")
	if r.Return == nil || r.Value != 7 || *w.Return >= r.Call {
		t.Fatal("flush-order positive control lacks a completed majority write followed by a safe read")
	}
	for id := uint64(1); id <= 3; id++ {
		store := c.stores[id]
		if store.unflushed || store.stages == 0 || store.flushes != store.stages {
			t.Fatal("durability invariant: Ready effects preceded successful Store.Flush")
		}
	}
	data, err := json.Marshal(c.history)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("AC-013 Flush ordering positive control completed real three-voter write/read; history=%s", data)
}

func TestAC013FileJournalCrashRecoveryMatrix(t *testing.T) {
	for _, boundary := range persistenceBarriers {
		t.Run(string(boundary), func(t *testing.T) {
			c := newJournalMatrixCluster(t, true)
			c.completedWrite("previous7", 7)
			c.invoke(1, target.Write{RequestID: "interrupted9", Value: 9})
			last, err := c.stores[1].Storage().LastIndex()
			if err != nil {
				t.Fatal(err)
			}
			c.arm, c.armIndex = boundary, last
			c.pump()
			if !c.fired || c.crashNode != 1 || len(c.crash) == 0 {
				t.Fatal("the on-disk commit crash barrier did not fire")
			}
			lastBarrier, replies := target.Barrier(""), 0
			for _, effect := range c.crash {
				if effect.Kind == target.BarrierEffect {
					lastBarrier = effect.Barrier
				}
				if effect.Kind == target.ReplyEffect {
					if effect.RequestID != "interrupted9" {
						t.Fatal("on-disk crash released an unrelated response")
					}
					replies++
				}
			}
			interrupted := c.op("interrupted9")
			if lastBarrier != boundary || replies != persistenceBoolCount(boundary == target.AfterResponse) || (interrupted.Return != nil) != (boundary == target.AfterResponse) {
				t.Fatal("on-disk response prefix does not preserve acknowledged/pending semantics")
			}
			// A centrally invoked read receives no reply from the stopped target.
			c.ordinal++
			c.history.Operations = append(c.history.Operations, history.Operation{ID: "down-read", Kind: "read", Call: c.ordinal})
			if effects, err := c.nodes[1].Step(target.Read{RequestID: "down-read"}); !errors.Is(err, target.ErrStopped) || len(effects) != 0 {
				t.Fatal("the crashed on-disk target remained mutable or fabricated a response")
			}
			// Reopen every journal as a new FileStore/Target so retained memory
			// cannot hide data loss. Undelivered messages are discarded explicitly.
			c.queue = nil
			c.arm, c.crashNode = "", 0
			for id := uint64(1); id <= 3; id++ {
				path := c.stores[id].path
				before, err := c.stores[id].Store.Recover()
				if err != nil {
					t.Fatal(err)
				}
				bytes, err := os.ReadFile(path)
				if err != nil || len(bytes) <= 8 || c.stores[id].growingFlush < 2 {
					t.Fatal("actual checksummed journal writes are absent")
				}
				if err := c.files[id].Close(); err != nil {
					t.Fatal(err)
				}
				file, err := target.NewFileStore(path, 0)
				if err != nil {
					t.Fatal(err)
				}
				c.files[id] = file
				c.stores[id] = &journalObservedStore{Store: file, path: path}
				reopened, err := file.Recover()
				if err != nil || !reflect.DeepEqual(before, reopened) {
					t.Fatal("closing and reopening changed the flushed durable prefix")
				}
				c.inc[id]++
				c.newNode(id)
				c.step(id, target.Start{})
				want := int64(7)
				if id == 1 && boundary != target.BeforeAppend && boundary != target.AfterStageBeforeFlush {
					want = 9
				}
				if c.nodes[id].Value() != want {
					t.Fatalf("on-disk committed-prefix recovery node=%d value=%d want=%d", id, c.nodes[id].Value(), want)
				}
				sum := sha256.Sum256(bytes)
				t.Logf("AC-013 file boundary=%s node=%d incarnation=%d journal_bytes=%d journal_sha256=%x recovered_commit=%d recovered_value=%d", boundary, id, c.inc[id], len(bytes), sum, reopened.HardState.GetCommit(), c.nodes[id].Value())
			}
			c.step(1, target.Campaign{})
			c.pump()
			c.invoke(1, target.Read{RequestID: "recovered9"})
			c.pump()
			recovered := c.op("recovered9")
			if recovered.Return == nil || recovered.Value != 9 {
				t.Fatal("reopened three-voter journals did not produce a quorum-safe read of the committed prefix")
			}
			c.completedWrite("interrupted9", 9)
			if interrupted.Return != nil && *c.op("interrupted9").Return != *interrupted.Return {
				t.Fatal("journal recovery rewrote the observed acknowledgement")
			}
			c.completedWrite("later11", 11)
			c.completedWrite("interrupted9", 9)
			c.invoke(1, target.Read{RequestID: "final11"})
			c.pump()
			final, later := c.op("final11"), c.op("later11")
			if final.Return == nil || final.Value != 11 || *later.Return >= final.Call || c.op("down-read").Return != nil {
				t.Fatal("an older recovered request-ID retry reverted an acknowledged write or changed a pending timeout")
			}
			result, err := history.Check(context.Background(), c.history, history.Options{MaxStates: 100000})
			if err != nil || result.Status != history.Valid {
				t.Fatalf("actual file-journal history is not linearizable: %+v %v", result, err)
			}
			data, err := json.Marshal(c.history)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("AC-013 actual file reopen history=%s source=%s patch=%s; journal Flush calls execute File.Sync, no physical power-loss claim", data, identity(t).SourceTreeSHA, identity(t).PatchSHA)
		})
	}
}
