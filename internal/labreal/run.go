package labreal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"github.com/C-X1an/faultbraid/internal/target"
)

const maxRealProcesses = 64
const maxAllOutput = 64 << 20

type child struct {
	command           *exec.Cmd
	owner             *processOwner
	index             int
	node, incarnation uint64
	address           string
	stdout, stderr    *os.File
	waited            chan struct{}
	waitErr           error
}
type clientTask struct {
	action scenario.Action
	child  *child
}
type frameTask struct {
	source, destination *child
	data                []byte
}
type clientOutcome struct {
	ordinal  int64
	task     clientTask
	response response
	err      error
}
type frameOutcome struct{ delivered, rejected bool }
type observation struct {
	child  *child
	wire   *wire
	client *clientOutcome
	frame  *frameOutcome
	exited bool
	err    error
}
type logicalRequest struct {
	operation history.Operation
	possible  bool
}
type outputBudget struct {
	mu    sync.Mutex
	bytes int
}

// childOutput is used by exec's bounded pipe-copy goroutine. It persists exactly
// the retained bytes and decodes newline-delimited protocol records; a long or
// malformed line becomes an explicit run error, never a claimed target result.
type childOutput struct {
	ctx      context.Context
	file     *os.File
	child    *child
	events   chan<- observation
	budget   *outputBudget
	protocol bool
	total    int
	partial  []byte
}

func (w *childOutput) Write(data []byte) (int, error) {
	w.budget.mu.Lock()
	if len(data) > maxChildOutput-w.total || len(data) > maxAllOutput-w.budget.bytes {
		w.budget.mu.Unlock()
		return 0, ErrLimit
	}
	w.budget.bytes += len(data)
	w.budget.mu.Unlock()
	w.total += len(data)
	n, err := w.file.Write(data)
	if err != nil {
		return n, err
	}
	if !w.protocol {
		return n, nil
	}
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		part := data
		if end >= 0 {
			part = data[:end]
		}
		if len(part) > maxWireLine-len(w.partial) {
			return n, ErrLimit
		}
		w.partial = append(w.partial, part...)
		if end < 0 {
			break
		}
		var record wire
		if err := strictjson.Decode(bytes.NewReader(w.partial), &record); err != nil {
			return n, errors.New("real lab: malformed child protocol record")
		}
		w.partial = w.partial[:0]
		select {
		case w.events <- observation{child: w.child, wire: &record}:
		case <-w.ctx.Done():
			// Shutdown stops live observation delivery, but must keep draining
			// the bounded pipe until the owned kill and Wait. Closing this pipe
			// early can make the child's next protocol write fail on its own.
		}
		data = data[end+1:]
	}
	return n, nil
}

type lab struct {
	ctx                          context.Context
	cancel                       context.CancelFunc
	options                      Options
	result                       Result
	directory, executable, owner string
	events                       chan observation
	clients                      chan clientTask
	frames                       chan frameTask
	workers                      sync.WaitGroup
	children                     []*child
	active                       [4]*child
	incarnations                 [4]uint64
	requests                     map[string]*logicalRequest
	requestOrder                 []string
	ordinal                      int64
	observations                 int
	budget                       outputBudget
	inflight, peak               atomic.Int64
	http                         *http.Client
}

// validateReal rejects unsupported faults and invalid crash/restart causality
// before creating output or launching any process. Virtual-time action ticks
// mean elapsed wall time only in this adapter; no deterministic replay claim.
func validateReal(s scenario.Scenario, o Options) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if !o.Identity.Identified() || o.Identity != buildinfo.Current() {
		return ErrIdentity
	}
	if o.Concurrency < 1 || o.Concurrency > maxConcurrentRequests {
		return ErrLimit
	}
	if s.Until() > MaxRealTicks {
		return ErrLimit
	}
	alive := [4]bool{false, true, true, true}
	processes := 3
	for _, action := range s.Actions {
		switch action.Kind {
		case "invoke_read", "invoke_write":
		case "crash":
			if !alive[*action.Node] {
				return scenario.ErrCausality
			}
			alive[*action.Node] = false
		case "restart":
			if alive[*action.Node] {
				return scenario.ErrCausality
			}
			alive[*action.Node] = true
			processes++
		default:
			return ErrUnsupportedAction
		}
	}
	if processes > maxRealProcesses {
		return ErrLimit
	}
	return nil
}

// Run launches only its own current executable's built-in lab-node command.
// Each process owns the same target transitions, an actual loopback listener,
// and an actual FileStore. IO ordering and elections depend on native scheduling.
// A timeout is pending unless an affirmative pre-admission response proves
// rejection, including across retries after an earlier uncertain outcome.
func Run(ctx context.Context, s scenario.Scenario, options Options) (result Result, runErr error) {
	started := time.Now()
	result = Result{Version: 1, Mode: "native-loopback-conformance-v1", Identity: options.Identity,
		TickMilliseconds: TickDuration.Milliseconds(), Concurrency: options.Concurrency,
		EntropyVersion: buildinfo.EntropyVersion, Scenario: s,
		History:  history.History{Version: 1, Operations: []history.Operation{}},
		Check:    history.Result{Status: history.Inconclusive, Reason: "real lab did not finish"},
		Attempts: []Attempt{}, Processes: []Process{}, Journals: []Journal{}}
	if err := validateReal(s, options); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		result.Reason = "cancelled before launch"
		return result, err
	}
	current, err := os.Executable()
	if err != nil {
		return result, err
	}
	executable, err := regularPath(options.Executable, false)
	if err != nil {
		return result, err
	}
	current, err = regularPath(current, false)
	if err != nil || executable != current {
		return result, ErrUnsafePath
	}
	binarySHA, _, err := fileHash(executable)
	if err != nil {
		return result, err
	}
	directory, err := regularPath(options.Directory, true)
	if err != nil {
		return result, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return result, err
	}
	if err = createOwnedDirectory(directory); err != nil {
		return result, err
	}
	marker, err := os.OpenFile(filepath.Join(directory, ".faultbraid-lab-owner.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	owner := hex.EncodeToString(nonce[:])
	err = json.NewEncoder(marker).Encode(ownership{Version: 1, Owner: owner, Identity: options.Identity})
	if closeErr := marker.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return result, err
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(s.Until())*TickDuration+8*time.Second)
	l := &lab{ctx: bounded, cancel: cancel, options: options, result: result, directory: directory,
		executable: executable, owner: owner, events: make(chan observation, min(s.QueueLimit, 4096)),
		clients: make(chan clientTask, min(s.QueueLimit, scenario.MaxActions)),
		frames:  make(chan frameTask, min(s.QueueLimit, 4096)), requests: map[string]*logicalRequest{}}
	l.result.BinarySHA = binarySHA
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxConnsPerHost: maxConcurrentRequests + 4,
		DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: RequestTimeout}
	l.http = &http.Client{Transport: transport, Timeout: RequestTimeout + time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer transport.CloseIdleConnections()
	l.startWorkers()
	defer func() {
		cleanupErr := l.cleanup()
		runErr = errors.Join(runErr, cleanupErr)
		l.collectHistory()
		if runErr != nil {
			l.result.Check = history.Result{Status: history.Inconclusive, Reason: "native run was interrupted; inspect attempts and cleanup"}
			l.result.Reason = "native scheduling is not replayable; run did not finish"
		} else {
			var checkErr error
			l.result.Check, checkErr = history.Check(ctx, l.result.History, history.Options{MaxStates: s.CheckerStateLimit})
			runErr = errors.Join(runErr, checkErr)
			l.result.Reason = "native scheduling is not replayable; journals prove completed File.Sync calls, not physical power loss"
		}
		l.result.DurationMillis = time.Since(started).Milliseconds()
		l.result.PeakConcurrency = l.peak.Load()
		result = l.result
	}()
	for node := uint64(1); node <= 3; node++ {
		if err := l.start(node); err != nil {
			return result, err
		}
	}
	readyDeadline := time.NewTimer(5 * time.Second)
	defer readyDeadline.Stop()
	for !l.allReady() {
		select {
		case event := <-l.events:
			if err := l.observe(event); err != nil {
				return result, err
			}
		case <-readyDeadline.C:
			return result, errors.New("real lab: readiness deadline exceeded")
		case <-l.ctx.Done():
			return result, l.ctx.Err()
		}
	}
	// A ready record is paired with an actual HTTP readiness check before user
	// workload admission. The fixed-address check also prevents URL redirection.
	for node := uint64(1); node <= 3; node++ {
		if err := l.health(l.active[node]); err != nil {
			return result, err
		}
	}
	epoch := time.Now()
	for _, action := range s.Actions {
		if err := l.until(epoch.Add(time.Duration(action.At) * TickDuration)); err != nil {
			return result, err
		}
		if err := l.action(action); err != nil {
			return result, err
		}
	}
	if err := l.until(epoch.Add(time.Duration(s.Until()) * TickDuration)); err != nil {
		return result, err
	}
	return result, nil
}

func (l *lab) allReady() bool {
	for node := 1; node <= 3; node++ {
		if l.active[node] == nil || l.active[node].address == "" {
			return false
		}
	}
	return true
}
func (l *lab) start(node uint64) error {
	l.incarnations[node]++
	incarnation := l.incarnations[node]
	seed := l.result.Scenario.Seed ^ node*0x9e3779b97f4a7c15 ^ (incarnation-1)*0xbf58476d1ce4e5b9
	relative := fmt.Sprintf("node-%d-incarnation-%d", node, incarnation)
	stdout, err := os.OpenFile(filepath.Join(l.directory, relative+".stdout.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	stderr, err := os.OpenFile(filepath.Join(l.directory, relative+".stderr.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		_ = stdout.Close()
		return err
	}
	c := &child{index: len(l.children), node: node, incarnation: incarnation, stdout: stdout, stderr: stderr, waited: make(chan struct{})}
	c.command = exec.Command(l.executable, "lab-node", "-id", strconv.FormatUint(node, 10), "-incarnation", strconv.FormatUint(incarnation, 10),
		"-directory", l.directory, "-owner", l.owner, "-seed", strconv.FormatUint(seed, 10),
		"-variant", string(l.result.Scenario.TargetVariant()), "-log-limit", strconv.FormatUint(l.result.Scenario.LogLimitBytes, 10),
		"-event-limit", strconv.Itoa(l.result.Scenario.EventLimit))
	c.command.Stdout = &childOutput{ctx: l.ctx, file: stdout, child: c, events: l.events, budget: &l.budget, protocol: true}
	c.command.Stderr = &childOutput{ctx: l.ctx, file: stderr, child: c, events: l.events, budget: &l.budget}
	c.command.WaitDelay = time.Second
	c.owner, err = startOwned(c.command)
	if err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return fmt.Errorf("real lab: owned child startup: %w", err)
	}
	l.children = append(l.children, c)
	l.active[node] = c
	l.result.Processes = append(l.result.Processes, Process{Node: node, Incarnation: incarnation, PID: c.command.Process.Pid,
		Identity: l.options.Identity, BinarySHA: l.result.BinarySHA, EntropySeed: seed, Choices: []raftadapter.Choice{},
		Journal: fmt.Sprintf("node-%d.journal", node), Stdout: relative + ".stdout.ndjson", Stderr: relative + ".stderr.ndjson"})
	go func() {
		c.waitErr = c.command.Wait()
		close(c.waited)
		select {
		case l.events <- observation{child: c, exited: true}:
		case <-l.ctx.Done():
		}
	}()
	return nil
}

func (l *lab) health(c *child) error {
	ctx, cancel := context.WithTimeout(l.ctx, time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.address+"/healthz", nil)
	if err != nil {
		return err
	}
	response, err := l.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var readiness struct {
		Ready bool `json:"ready"`
	}
	if response.StatusCode != http.StatusOK || strictjson.Decode(io.LimitReader(response.Body, 129), &readiness) != nil || !readiness.Ready {
		return errors.New("real lab: HTTP readiness check failed")
	}
	return nil
}

func (l *lab) until(deadline time.Time) error {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case event := <-l.events:
			if err := l.observe(event); err != nil {
				return err
			}
		case <-timer.C:
			return nil
		case <-l.ctx.Done():
			return l.ctx.Err()
		}
	}
}
func (l *lab) action(action scenario.Action) error {
	node := *action.Node
	switch action.Kind {
	case "crash":
		c := l.active[node]
		if c == nil {
			return scenario.ErrCausality
		}
		l.snapshot(c)
		l.active[node] = nil // queued frames retain the old socket, never a restarted incarnation.
		return l.stop(c, "scenario-kill")
	case "restart":
		if l.active[node] != nil {
			return scenario.ErrCausality
		}
		if err := l.start(node); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		for l.active[node].address == "" {
			if err := l.until(minTime(deadline, time.Now().Add(10*time.Millisecond))); err != nil {
				return err
			}
			if time.Now().After(deadline) {
				return errors.New("real lab: restarted child readiness deadline exceeded")
			}
		}
		return l.health(l.active[node])
	case "invoke_write", "invoke_read":
		ordinal := atomic.AddInt64(&l.ordinal, 1)
		id := *action.RequestID
		if l.requests[id] == nil {
			kind, value := "read", int64(0)
			if action.Kind == "invoke_write" {
				kind, value = "write", *action.Value
			}
			l.requests[id] = &logicalRequest{operation: history.Operation{ID: id, Kind: kind, Value: value, Call: ordinal}}
			l.requestOrder = append(l.requestOrder, id)
		}
		c := l.active[node]
		pid := 0
		if c != nil {
			pid = c.command.Process.Pid
		}
		l.result.Attempts = append(l.result.Attempts, Attempt{Ordinal: ordinal, Action: action.ID, RequestID: id, Node: node, PID: pid,
			Kind: l.requests[id].operation.Kind, Value: l.requests[id].operation.Value, Phase: "invoke", Admission: "awaiting-response"})
		task := clientTask{action: action, child: c}
		select {
		case l.clients <- task:
			return nil
		default:
			return ErrLimit
		}
	}
	return ErrUnsupportedAction
}
func minTime(first, second time.Time) time.Time {
	if first.Before(second) {
		return first
	}
	return second
}

func (l *lab) observe(event observation) error {
	l.observations++
	if l.observations > l.result.Scenario.EventLimit {
		return ErrLimit
	}
	if event.err != nil {
		return event.err
	}
	if event.client != nil {
		return l.clientResult(*event.client)
	}
	if event.frame != nil {
		if event.frame.delivered {
			l.result.Transport.Delivered++
		} else if event.frame.rejected {
			l.result.Transport.Rejected++
		} else {
			l.result.Transport.Dropped++
		}
		return nil
	}
	c := event.child
	if event.exited {
		if l.active[c.node] == c {
			return errors.New("real lab: child exited before an owned stop")
		}
		return nil
	}
	w := event.wire
	if w == nil || w.Node != c.node || w.Incarnation != c.incarnation || w.PID != c.command.Process.Pid || w.Identity != l.options.Identity || w.BinarySHA != l.result.BinarySHA {
		return errors.New("real lab: child identity/protocol mismatch")
	}
	record := &l.result.Processes[c.index]
	if record.Waited && w.Type != "send" {
		return nil
	}
	switch w.Type {
	case "entropy":
		if w.Choice == nil || w.Choice.Stream != "raft-election" || w.Choice.Node != c.node || w.Choice.Sequence != uint64(len(record.Choices)) || w.Choice.Bound < 1 || w.Choice.Value < 0 || w.Choice.Value >= w.Choice.Bound {
			return errors.New("real lab: child entropy record mismatch")
		}
		record.Choices = append(record.Choices, *w.Choice)
	case "ready":
		host, port, err := net.SplitHostPort(w.Address)
		number, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || host != "127.0.0.1" || number < 1 || number > 65535 || c.address != "" || w.Snapshot.Status.ID != c.node || !w.Snapshot.Status.Started || w.Snapshot.Status.Stopped {
			return errors.New("real lab: invalid readiness record")
		}
		c.address, record.Address = w.Address, w.Address
		record.Ready, record.Last = w.Snapshot, w.Snapshot
	case "send":
		if w.To < 1 || w.To > 3 || w.To == c.node || len(w.Data) > base64.StdEncoding.EncodedLen(target.MaxFrameBytes) {
			return errors.New("real lab: invalid frame route")
		}
		data, err := base64.StdEncoding.DecodeString(w.Data)
		if err != nil || len(data) == 0 || len(data) > target.MaxFrameBytes || base64.StdEncoding.EncodeToString(data) != w.Data {
			return errors.New("real lab: invalid frame encoding")
		}
		l.result.Transport.Sent++
		l.result.Transport.Bytes += uint64(len(data))
		destination := l.active[w.To]
		if l.active[c.node] != c || destination == nil || destination.address == "" {
			l.result.Transport.Dropped++
			return nil
		}
		select {
		case l.frames <- frameTask{source: c, destination: destination, data: data}:
		default:
			return ErrLimit
		}
	case "stop":
		record.Last = w.Snapshot
	default:
		return errors.New("real lab: unknown child protocol record")
	}
	return nil
}

func (l *lab) startWorkers() {
	for worker := 0; worker < l.options.Concurrency; worker++ {
		l.workers.Add(1)
		go func() {
			defer l.workers.Done()
			for {
				select {
				case task := <-l.clients:
					outcome := l.propose(task)
					l.events <- observation{client: &outcome}
				case <-l.ctx.Done():
					return
				}
			}
		}()
	}
	for worker := 0; worker < 4; worker++ {
		l.workers.Add(1)
		go func() {
			defer l.workers.Done()
			for {
				select {
				case frame := <-l.frames:
					outcome := l.deliver(frame)
					select {
					case l.events <- observation{frame: &outcome}:
					case <-l.ctx.Done():
						return
					}
				case <-l.ctx.Done():
					return
				}
			}
		}()
	}
}
func (l *lab) propose(task clientTask) clientOutcome {
	inflight := l.inflight.Add(1)
	defer l.inflight.Add(-1)
	for previous := l.peak.Load(); inflight > previous; previous = l.peak.Load() {
		if l.peak.CompareAndSwap(previous, inflight) {
			break
		}
	}
	outcome := clientOutcome{task: task}
	id := *task.action.RequestID
	outcome.response = response{ID: id, Status: "pending", Admission: "unknown", Code: "TRANSPORT_OUTCOME_UNKNOWN"}
	if task.child == nil {
		outcome.response.Code = "NODE_UNAVAILABLE"
		return l.stampClientOutcome(outcome)
	}
	input := proposal{ID: id, Kind: "read"}
	if task.action.Kind == "invoke_write" {
		input.Kind, input.Value = "write", *task.action.Value
	}
	data, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(l.ctx, RequestTimeout+250*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+task.child.address+"/v1/propose", bytes.NewReader(data))
	if err != nil {
		outcome.err = err
		return l.stampClientOutcome(outcome)
	}
	request.Header.Set("Content-Type", "application/json")
	result, err := l.http.Do(request)
	if err != nil {
		return l.stampClientOutcome(outcome)
	}
	defer result.Body.Close()
	var response response
	if err := strictjson.Decode(io.LimitReader(result.Body, 2049), &response); err != nil || response.ID != id || len(response.Code) > 128 {
		outcome.err = errors.New("real lab: invalid proposal response")
		return l.stampClientOutcome(outcome)
	}
	valid := response.Status == "complete" && response.Admission == "admitted" && result.StatusCode == http.StatusOK ||
		response.Status == "rejected" && response.Admission == "rejected-before-admission" && result.StatusCode >= 400 && result.StatusCode < 600 ||
		response.Status == "pending" && response.Admission == "unknown" && result.StatusCode >= 400 && result.StatusCode < 600
	if !valid {
		outcome.err = errors.New("real lab: response admission classification mismatch")
		return l.stampClientOutcome(outcome)
	}
	outcome.response = response
	return l.stampClientOutcome(outcome)
}

// Stamp the decoded/classified response or transport error at the parent
// client boundary, before closing its body or queueing it for collection.
// Calls share this clock; unfinished calls still overlap and workers run
// concurrently even when a ready timer advances the action scheduler.
func (l *lab) stampClientOutcome(outcome clientOutcome) clientOutcome {
	outcome.ordinal = atomic.AddInt64(&l.ordinal, 1)
	return outcome
}
func (l *lab) deliver(frame frameTask) frameOutcome {
	ctx, cancel := context.WithTimeout(l.ctx, time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+frame.destination.address+"/v1/message", bytes.NewReader(frame.data))
	if err != nil {
		return frameOutcome{}
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	result, err := l.http.Do(request)
	if err != nil {
		return frameOutcome{}
	}
	defer result.Body.Close()
	var response response
	if result.StatusCode != http.StatusOK || strictjson.Decode(io.LimitReader(result.Body, 2049), &response) != nil || response.Status != "complete" {
		return frameOutcome{rejected: true}
	}
	return frameOutcome{delivered: true}
}
func (l *lab) clientResult(outcome clientOutcome) error {
	input := outcome.task.action
	id := *input.RequestID
	r := l.requests[id]
	if r == nil {
		return errors.Join(outcome.err, errors.New("real lab: response without a collected invocation"))
	}
	reply := outcome.response
	if outcome.err != nil {
		// A malformed or interrupted response cannot establish pre-admission
		// rejection. Retain the invocation and an explicit uncertain attempt
		// before propagating the protocol error to the run's INCONCLUSIVE path.
		reply = response{ID: id, Status: "pending", Admission: "unknown", Value: r.operation.Value, Code: "PROPOSAL_PROTOCOL_ERROR"}
	}
	ordinal := outcome.ordinal
	if ordinal == 0 {
		// An undispatched intent receives its uncertain outcome during cleanup.
		ordinal = atomic.AddInt64(&l.ordinal, 1)
	}
	pid := 0
	if outcome.task.child != nil {
		pid = outcome.task.child.command.Process.Pid
	}
	l.result.Attempts = append(l.result.Attempts, Attempt{Ordinal: ordinal, Action: input.ID, RequestID: id, Node: *input.Node, PID: pid,
		Kind: r.operation.Kind, Phase: reply.Status, Admission: reply.Admission, Value: reply.Value, Code: reply.Code})
	if reply.Admission != "rejected-before-admission" {
		r.possible = true
	}
	if reply.Status == "complete" && r.operation.Kind == "write" && reply.Value != r.operation.Value {
		return errors.New("real lab: write result disagrees with replicated payload")
	}
	if reply.Status == "complete" && (r.operation.Return == nil || ordinal < *r.operation.Return) {
		r.operation.Return = new(ordinal)
		if r.operation.Kind == "read" {
			r.operation.Value = reply.Value
		}
	}
	return outcome.err
}
func (l *lab) collectHistory() {
	// Worker receipt order can differ from observation delivery order.
	sort.Slice(l.result.Attempts, func(i, j int) bool { return l.result.Attempts[i].Ordinal < l.result.Attempts[j].Ordinal })
	l.result.History.Operations = []history.Operation{}
	for _, id := range l.requestOrder {
		r := l.requests[id]
		if r.possible || r.operation.Return != nil {
			l.result.History.Operations = append(l.result.History.Operations, r.operation)
		}
	}
}

func (l *lab) snapshot(c *child) {
	if c.address == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.address+"/v1/diagnostics", nil)
	if err != nil {
		return
	}
	response, err := l.http.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	var snapshot Snapshot
	if response.StatusCode == http.StatusOK && strictjson.Decode(io.LimitReader(response.Body, 2049), &snapshot) == nil && snapshot.Status.ID == c.node {
		l.result.Processes[c.index].Last = snapshot
	}
}
func (l *lab) stop(c *child, reason string) error {
	record := &l.result.Processes[c.index]
	if record.Waited {
		return nil
	}
	record.Stop = reason
	// A failed child may have exited before its observation reaches the parent.
	// Waiting for an already dead process is cleanup, not a successful kill.
	var waitFailure error
	select {
	case <-c.waited:
		record.Stop = "unexpected-exit"
		waitFailure = errors.Join(errors.New("real lab: child exited before an owned stop"), c.waitErr)
	default:
	}
	killErr := c.owner.Kill()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-c.waited:
		record.Waited = true
		record.ExitCode = c.command.ProcessState.ExitCode()
		if waitFailure == nil && !ownedKillExit(c.command, c.waitErr) {
			record.Stop = "unexpected-exit"
			waitFailure = errors.Join(errors.New("real lab: child exit did not match the owned kill"), c.waitErr)
		}
	case <-timer.C:
		return errors.Join(killErr, errors.New("real lab: child did not exit after kill"))
	}
	closeErr := c.owner.Close()
	stdoutErr, stderrErr := c.stdout.Close(), c.stderr.Close()
	protocolErr := l.retainedProtocol(c)
	if c.address != "" {
		connection, err := net.DialTimeout("tcp4", c.address, 100*time.Millisecond)
		if err != nil {
			record.SocketClosed = true
		} else {
			_ = connection.Close()
		}
	}
	if !record.SocketClosed {
		return errors.Join(waitFailure, killErr, closeErr, stdoutErr, stderrErr, protocolErr, errors.New("real lab: stopped socket closure was not established"))
	}
	return errors.Join(waitFailure, killErr, closeErr, stdoutErr, stderrErr, protocolErr)
}

// Re-read the retained bounded stdout after Wait, so buffered observations at
// shutdown cannot omit selected entropy or a ready socket from process evidence.
// An interrupted final line is counted separately, never decoded as a record.
func (l *lab) retainedProtocol(c *child) error {
	path := filepath.Join(l.directory, l.result.Processes[c.index].Stdout)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(io.LimitReader(file, maxChildOutput+1), maxWireLine+2)
	record := &l.result.Processes[c.index]
	record.Choices = []raftadapter.Choice{}
	seeded, err := raftadapter.NewRecorder(c.node, record.EntropySeed)
	if err != nil {
		return err
	}
	for {
		line, err := reader.ReadSlice('\n')
		if len(line) > maxWireLine+1 {
			return ErrLimit
		}
		if errors.Is(err, io.EOF) {
			record.ProtocolTrailingBytes = len(line)
			return nil
		}
		if err != nil {
			return err
		}
		var w wire
		if strictjson.Decode(bytes.NewReader(line), &w) != nil || w.Node != c.node || w.Incarnation != c.incarnation || w.PID != record.PID || w.Identity != l.options.Identity || w.BinarySHA != l.result.BinarySHA {
			return errors.New("real lab: retained protocol identity/record mismatch")
		}
		switch w.Type {
		case "ready":
			host, port, splitErr := net.SplitHostPort(w.Address)
			number, portErr := strconv.Atoi(port)
			if splitErr != nil || portErr != nil || host != "127.0.0.1" || number < 1 || number > 65535 || w.Snapshot.Status.ID != c.node {
				return errors.New("real lab: retained readiness mismatch")
			}
			if c.address != "" && c.address != w.Address {
				return errors.New("real lab: retained socket identity changed")
			}
			if c.address == "" {
				c.address = w.Address
			}
			record.Address = w.Address
			record.Ready = w.Snapshot
		case "entropy":
			if w.Choice == nil || w.Choice.Stream != "raft-election" || w.Choice.Node != c.node || w.Choice.Sequence != uint64(len(record.Choices)) || w.Choice.Bound < 1 || w.Choice.Value < 0 || w.Choice.Value >= w.Choice.Bound {
				return errors.New("real lab: retained entropy mismatch")
			}
			record.Choices = append(record.Choices, *w.Choice)
			value, err := seeded.Draw(w.Choice.Stream, w.Choice.Bound)
			if err != nil || value != w.Choice.Value {
				return errors.New("real lab: retained choice disagrees with its explicit seed")
			}
		case "send", "stop":
		default:
			return errors.New("real lab: retained unknown protocol type")
		}
	}
}
func (l *lab) cleanup() error {
	var cleanupErr error
	// Capture diagnostics while transport is still active. They are observations
	// from the serialized target actor, not values inferred from a final history.
	for node := 1; node <= 3; node++ {
		if c := l.active[node]; c != nil {
			l.snapshot(c)
		}
	}
	l.cancel()
	for node := 1; node <= 3; node++ {
		l.active[node] = nil
	}
	for _, c := range l.children {
		if !l.result.Processes[c.index].Waited {
			cleanupErr = errors.Join(cleanupErr, l.stop(c, "run-cleanup"))
		}
	}
	// Proposal workers send every observed response even during cancellation.
	// Drain before joining so a bounded event queue cannot strand those workers.
	done := make(chan struct{})
	go func() { l.workers.Wait(); close(done) }()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-l.events:
			if event.client != nil {
				cleanupErr = errors.Join(cleanupErr, l.clientResult(*event.client))
			}
		case <-done:
			for {
				select {
				case event := <-l.events:
					if event.client != nil {
						cleanupErr = errors.Join(cleanupErr, l.clientResult(*event.client))
					}
				default:
					goto drained
				}
			}
		case <-timer.C:
			cleanupErr = errors.Join(cleanupErr, errors.New("real lab: worker cleanup deadline exceeded"))
			goto drained
		}
	}
drained:
	// Unstarted queued client requests never reached the owned endpoint, but the
	// logical intent remains conservatively pending. Earlier uncertainty is kept.
	for {
		select {
		case task := <-l.clients:
			cleanupErr = errors.Join(cleanupErr, l.clientResult(clientOutcome{task: task, response: response{ID: *task.action.RequestID, Status: "pending", Admission: "unknown", Code: "RUN_ENDED_BEFORE_DISPATCH"}}))
		default:
			goto clientsDrained
		}
	}
clientsDrained:
	l.result.Transport.Unresolved = l.result.Transport.Sent - min(l.result.Transport.Sent, l.result.Transport.Delivered+l.result.Transport.Rejected+l.result.Transport.Dropped)
	for node := uint64(1); node <= 3; node++ {
		name := fmt.Sprintf("node-%d.journal", node)
		sha, bytes, err := fileHash(filepath.Join(l.directory, name))
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		l.result.Journals = append(l.result.Journals, Journal{Node: node, Path: name, Bytes: bytes, SHA: sha})
	}
	l.result.CleanupComplete = cleanupErr == nil
	for _, record := range l.result.Processes {
		l.result.CleanupComplete = l.result.CleanupComplete && record.Waited && record.SocketClosed
	}
	return cleanupErr
}
