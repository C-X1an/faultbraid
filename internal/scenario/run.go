package scenario

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/target"
	raft "go.etcd.io/raft/v3"
)

type Options struct {
	Identity buildinfo.Identity
	choices  []raftadapter.Choice
	replay   bool
}

type queued struct {
	at, sequence, parent         uint64
	kind                         string
	action                       int
	from, to, sourceInc, destInc uint64
	data                         []byte
}
type eventHeap []queued

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	return h[i].at < h[j].at || h[i].at == h[j].at && h[i].sequence < h[j].sequence
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(v any)   { *h = append(*h, v.(queued)) }
func (h *eventHeap) Pop() any {
	old := *h
	v := old[len(old)-1]
	old[len(old)-1] = queued{}
	*h = old[:len(old)-1]
	return v
}

type link struct {
	blocked         bool
	delay           uint64
	drop, duplicate int
}
type armed struct {
	boundary  target.Barrier
	remaining int
}
type node struct {
	target   *target.Target
	store    *target.MemoryStore
	entropy  raftadapter.Entropy
	replayer *raftadapter.Replayer
	inc      uint64
	alive    bool
	crash    *armed
}
type execution struct {
	ctx                context.Context
	trace              Trace
	nodes              [4]*node
	links              [4][4]link
	queue              eventHeap
	now, next, ordinal uint64
	count, bytes       int
	reason             string
	ops                map[string]int
}

type trackedEntropy struct {
	owner  *execution
	node   uint64
	source raftadapter.Entropy
	next   uint64
}

func (e *trackedEntropy) Draw(stream string, bound int) (int, error) {
	value, err := e.source.Draw(stream, bound)
	if err != nil {
		return 0, err
	}
	choice := raftadapter.Choice{Node: e.node, Stream: stream, Sequence: e.next, Bound: bound, Value: value}
	e.next++
	e.owner.trace.Choices = append(e.owner.trace.Choices, choice)
	data, _ := json.Marshal(choice)
	e.owner.bytes += len(data) + 1
	if e.owner.bytes > MaxTraceBytes-8192 {
		e.owner.reason = "trace byte limit"
		return 0, target.ErrLimit
	}
	return value, nil
}

func Run(ctx context.Context, s Scenario, opts Options) (Trace, error) { return run(ctx, s, opts) }

func run(ctx context.Context, s Scenario, opts Options) (Trace, error) {
	if err := s.Validate(); err != nil {
		return Trace{}, err
	}
	if opts.Identity == (buildinfo.Identity{}) {
		opts.Identity = buildinfo.Current()
	}
	if !opts.Identity.Identified() {
		return Trace{}, errors.New("identified build required for target execution")
	}
	x := &execution{ctx: ctx, ops: map[string]int{}, trace: Trace{Version: 1, Model: "raft-trace-v1", Scenario: s,
		Header:  Header{opts.Identity.SourceTreeSHA, opts.Identity.ModuleLockSHA, opts.Identity.PatchSHA, buildinfo.SchedulerVersion, buildinfo.EntropyVersion, s.Seed, string(s.TargetVariant()), 0},
		Choices: []raftadapter.Choice{}, Events: []Record{}, History: history.History{Version: 1, Operations: []history.Operation{}}}}
	initial, _ := json.Marshal(x.trace)
	x.bytes = len(initial)
	for id := uint64(1); id <= 3; id++ {
		var entropy raftadapter.Entropy
		n := &node{store: target.NewMemoryStore(s.LogLimitBytes), inc: 1, alive: true}
		if opts.replay {
			choices := []raftadapter.Choice{}
			for _, choice := range opts.choices {
				if choice.Node == id {
					choices = append(choices, choice)
				}
			}
			replayer, err := raftadapter.NewReplayer(id, choices)
			if err != nil {
				return x.trace, err
			}
			n.replayer = replayer
			entropy = replayer
		} else {
			recorder, err := raftadapter.NewRecorder(id, s.Seed^(id*0x9e3779b97f4a7c15))
			if err != nil {
				return x.trace, err
			}
			entropy = recorder
		}
		n.entropy = &trackedEntropy{owner: x, node: id, source: entropy}
		x.nodes[id] = n
		for to := uint64(1); to <= 3; to++ {
			x.links[id][to].delay = 1
		}
	}
	for _, choice := range opts.choices {
		if choice.Node < 1 || choice.Node > 3 {
			return x.trace, errors.New("entropy record has invalid node")
		}
	}
	for id := uint64(1); id <= 3; id++ {
		if err := x.start(id, 0); err != nil {
			return x.trace, err
		}
		if x.reason != "" {
			break
		}
	}
	for i, a := range s.Actions {
		x.admit(queued{at: a.At, kind: "action", action: i})
	}
	if s.Until() > 0 {
		x.admit(queued{at: 1, kind: "tick"})
	}
	for x.queue.Len() > 0 && x.reason == "" {
		if err := ctx.Err(); err != nil {
			x.reason = "cancelled execution"
			break
		}
		e := heap.Pop(&x.queue).(queued)
		if e.at > s.Until() {
			break
		}
		x.now = e.at
		if x.count >= s.EventLimit {
			x.reason = "event limit"
			break
		}
		switch e.kind {
		case "action":
			if err := x.action(s.Actions[e.action]); err != nil {
				return x.trace, err
			}
		case "tick":
			for id := uint64(1); id <= 3 && x.reason == ""; id++ {
				if x.nodes[id].alive {
					if err := x.step(id, target.Tick{}, 0, "tick", ""); err != nil {
						return x.trace, err
					}
				}
			}
			if x.now < s.Until() {
				x.admit(queued{at: x.now + 1, kind: "tick"})
			}
		case "message":
			n := x.nodes[e.to]
			if !n.alive || n.inc != e.destInc || x.nodes[e.from].inc != e.sourceInc {
				x.record(e.parent, e.to, "drop", e.data, "incarnation-or-down")
				x.count++
				continue
			}
			if x.links[e.from][e.to].blocked {
				x.record(e.parent, e.to, "drop", e.data, "partition")
				x.count++
				continue
			}
			if err := x.step(e.to, target.Message{Data: e.data}, e.parent, "message", fmt.Sprintf("from=%d,source_inc=%d;", e.from, e.sourceInc)); err != nil {
				return x.trace, err
			}
		}
	}
	for id := uint64(1); id <= 3; id++ {
		if x.nodes[id].replayer != nil {
			if err := x.nodes[id].replayer.Finish(); err != nil {
				return x.trace, err
			}
		}
	}
	if x.reason != "" {
		x.trace.Result = history.Result{Status: history.Inconclusive, Witness: []string{}, Reason: x.reason}
	} else {
		result, err := history.Check(ctx, x.trace.History, history.Options{MaxStates: s.CheckerStateLimit})
		if err != nil {
			return x.trace, err
		}
		x.trace.Result = result
		if result.Status == history.Invalid {
			x.trace.Failure = failure(x.trace.Header.Variant)
		}
	}
	if x.trace.Result.Witness == nil {
		x.trace.Result.Witness = []string{}
	}
	d, err := digest(x.trace)
	x.trace.Digest = d
	data, encodeErr := json.Marshal(x.trace)
	if encodeErr != nil {
		return x.trace, encodeErr
	}
	if len(data)+1 > MaxTraceBytes {
		return x.trace, errors.New("trace encoding exceeds byte limit")
	}
	return x.trace, err
}

func (x *execution) admit(e queued) {
	if x.reason != "" {
		return
	}
	if x.queue.Len() >= x.trace.Scenario.QueueLimit {
		x.reason = "queue limit"
		return
	}
	if e.at < x.now || x.next == math.MaxUint64 {
		x.reason = "scheduler admission limit"
		return
	}
	e.sequence = x.next
	x.next++
	heap.Push(&x.queue, e)
}

func (x *execution) record(parent, nodeID uint64, kind string, payload []byte, detail string) uint64 {
	if len(detail) > 1024 {
		detail = detail[:1024]
	}
	if len(x.trace.Events) >= x.trace.Scenario.EventLimit {
		x.reason = "event record limit"
		return 0
	}
	record := Record{ID: uint64(len(x.trace.Events) + 1), Parent: parent, Node: nodeID, At: x.now, Kind: kind, PayloadSHA: hash(payload), Detail: detail}
	if nodeID > 0 {
		record.Incarnation = x.nodes[nodeID].inc
	}
	data, _ := json.Marshal(record)
	if x.bytes+len(data)+1 > MaxTraceBytes-8192 {
		x.reason = "trace byte limit"
		return 0
	}
	x.bytes += len(data) + 1
	x.trace.Events = append(x.trace.Events, record)
	return record.ID
}

func (x *execution) start(id, parent uint64) error {
	n := x.nodes[id]
	t, err := target.New(target.Config{ID: id, Store: n.store, Entropy: n.entropy, LogLimitBytes: x.trace.Scenario.LogLimitBytes, Variant: x.trace.Scenario.TargetVariant(), Barrier: func(b target.Barrier) error {
		if n.crash != nil && n.crash.boundary == b {
			n.crash.remaining--
			if n.crash.remaining == 0 {
				n.crash = nil
				return target.ErrInjectedCrash
			}
		}
		return nil
	}})
	if err != nil {
		return err
	}
	n.target = t
	return x.step(id, target.Start{}, parent, "start", "")
}

func (x *execution) step(id uint64, input target.Event, parent uint64, kind, prefix string) error {
	if x.count >= x.trace.Scenario.EventLimit {
		x.reason = "event limit"
		return nil
	}
	x.count++
	n := x.nodes[id]
	effects, err := n.target.Step(input)
	batch := struct {
		Input   target.Event
		Effects []target.Effect
		State   target.Status
	}{input, effects, n.target.Status()}
	payload, encodeErr := json.Marshal(batch)
	if encodeErr != nil {
		return encodeErr
	}
	boundaries := []string{}
	for _, effect := range effects {
		if effect.Kind == target.BarrierEffect {
			boundaries = append(boundaries, string(effect.Barrier))
		}
	}
	stepID := x.record(parent, id, kind, payload, prefix+strings.Join(boundaries, ","))
	for _, effect := range effects {
		switch effect.Kind {
		case target.SendEffect:
			if effect.To < 1 || effect.To > 3 {
				return errors.New("target emitted invalid destination")
			}
			policy := &x.links[id][effect.To]
			if policy.drop > 0 {
				policy.drop--
				x.record(stepID, effect.To, "drop", effect.Data, "drop-next")
				continue
			}
			copies := 1
			if policy.duplicate > 0 {
				policy.duplicate--
				copies = 2
				x.record(stepID, effect.To, "duplicate", effect.Data, "duplicate-next")
			}
			for i := 0; i < copies; i++ {
				x.admit(queued{at: x.now + policy.delay, kind: "message", parent: stepID, from: id, to: effect.To, sourceInc: n.inc, destInc: x.nodes[effect.To].inc, data: append([]byte(nil), effect.Data...)})
			}
		case target.ReplyEffect:
			index, ok := x.ops[effect.RequestID]
			if !ok {
				return errors.New("target emitted reply without invocation")
			}
			op := &x.trace.History.Operations[index]
			if op.Kind == "write" && effect.Value != op.Value {
				return errors.New("target write response payload mismatch")
			}
			if op.Return == nil {
				x.ordinal++
				end := int64(x.ordinal)
				op.Return = &end
				if op.Kind == "read" {
					op.Value = effect.Value
				}
			}
			x.record(stepID, id, "reply", []byte(fmt.Sprintf("%s:%d:%d", effect.RequestID, effect.Value, effect.Index)), effect.RequestID)
		}
	}
	if errors.Is(err, target.ErrInjectedCrash) {
		n.alive = false
		x.record(stepID, id, "crash", nil, "barrier")
		return nil
	}
	if errors.Is(err, target.ErrLimit) {
		x.reason = "target resource limit"
		return nil
	}
	if errors.Is(err, raft.ErrProposalDropped) {
		x.record(stepID, id, "rejected", nil, "not-leader-before-admission")
		return nil
	}
	return err
}

func (x *execution) action(a Action) error {
	data, _ := json.Marshal(a)
	parent := x.record(0, 0, "action", data, a.ID+":"+a.Kind)
	if x.reason != "" {
		return nil
	}
	switch a.Kind {
	case "invoke_write", "invoke_read":
		id := *a.Node
		n := x.nodes[id]
		if _, ok := x.ops[*a.RequestID]; !ok {
			x.ordinal++
			value := int64(0)
			if a.Value != nil {
				value = *a.Value
			}
			x.ops[*a.RequestID] = len(x.trace.History.Operations)
			x.trace.History.Operations = append(x.trace.History.Operations, history.Operation{ID: *a.RequestID, Kind: strings.TrimPrefix(a.Kind, "invoke_"), Value: value, Call: int64(x.ordinal)})
		}
		if !n.alive {
			x.count++
			x.record(parent, id, "unavailable", nil, "pending")
			return nil
		}
		if a.Kind == "invoke_write" {
			return x.step(id, target.Write{RequestID: *a.RequestID, Value: *a.Value}, parent, "invoke-write", "")
		}
		return x.step(id, target.Read{RequestID: *a.RequestID}, parent, "invoke-read", "")
	case "crash":
		n := x.nodes[*a.Node]
		if !n.alive {
			return ErrCausality
		}
		n.alive = false
		if _, err := n.store.Recover(); err != nil {
			return err
		}
		x.record(parent, *a.Node, "crash", nil, "action")
	case "restart":
		n := x.nodes[*a.Node]
		if n.alive {
			return ErrCausality
		}
		n.inc++
		n.alive = true
		return x.start(*a.Node, parent)
	case "crash_at":
		n := x.nodes[*a.Node]
		if !n.alive {
			return ErrCausality
		}
		n.crash = &armed{target.Barrier(*a.Boundary), *a.Occurrence}
	case "partition", "heal", "delay_link", "drop_next", "duplicate_next":
		policy := &x.links[*a.From][*a.To]
		switch a.Kind {
		case "partition":
			if policy.blocked {
				return ErrCausality
			}
			policy.blocked = true
		case "heal":
			if !policy.blocked {
				return ErrCausality
			}
			policy.blocked = false
		case "delay_link":
			policy.delay = *a.Ticks
		case "drop_next":
			policy.drop = *a.Count
		case "duplicate_next":
			policy.duplicate = *a.Count
		}
	default:
		return errors.New("unsupported action")
	}
	x.count++
	return nil
}
