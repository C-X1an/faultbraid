package labreal

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"github.com/C-X1an/faultbraid/internal/target"
)

type nodeRequest struct {
	kind     string
	proposal proposal
	data     []byte
	ctx      context.Context
	reply    chan response
	diag     chan Snapshot
}

type waiter struct {
	ctx   context.Context
	reply chan response
}

type nodeServer struct {
	ctx      context.Context
	requests chan nodeRequest
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func safeID(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}

func (n *nodeServer) propose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
		jsonResponse(w, http.StatusBadRequest, response{Status: "rejected", Admission: "rejected-before-admission", Code: "INVALID_INPUT"})
		return
	}
	var input proposal
	if err := strictjson.Decode(http.MaxBytesReader(w, r.Body, maxProposeBytes), &input); err != nil || !safeID(input.ID) || input.Kind != "write" && input.Kind != "read" {
		jsonResponse(w, http.StatusBadRequest, response{Status: "rejected", Admission: "rejected-before-admission", Code: "INVALID_INPUT"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()
	request := nodeRequest{kind: "propose", proposal: input, ctx: ctx, reply: make(chan response, 1)}
	select {
	case n.requests <- request:
	case <-ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{ID: input.ID, Status: "rejected", Admission: "rejected-before-admission", Code: "BEFORE_ADMISSION_TIMEOUT"})
		return
	case <-n.ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{ID: input.ID, Status: "rejected", Admission: "rejected-before-admission", Code: "NODE_STOPPED_BEFORE_ADMISSION"})
		return
	}
	select {
	case result := <-request.reply:
		status := http.StatusOK
		if result.Status == "rejected" {
			status = http.StatusConflict
		}
		jsonResponse(w, status, result)
	case <-ctx.Done():
		// Once queued, absence of a response is uncertain. Do not infer that
		// the actor or consensus target failed to admit the operation.
		jsonResponse(w, http.StatusGatewayTimeout, response{ID: input.ID, Status: "pending", Admission: "unknown", Code: "RESPONSE_TIMEOUT"})
	case <-n.ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{ID: input.ID, Status: "pending", Admission: "unknown", Code: "NODE_STOPPED"})
	}
}

func (n *nodeServer) message(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/octet-stream" {
		jsonResponse(w, http.StatusBadRequest, response{Status: "rejected", Admission: "rejected-before-admission", Code: "INVALID_FRAME"})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, target.MaxFrameBytes))
	if err != nil || len(data) == 0 {
		jsonResponse(w, http.StatusBadRequest, response{Status: "rejected", Admission: "rejected-before-admission", Code: "INVALID_FRAME"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	request := nodeRequest{kind: "message", data: data, ctx: ctx, reply: make(chan response, 1)}
	select {
	case n.requests <- request:
	case <-ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{Status: "rejected", Admission: "rejected-before-admission", Code: "MESSAGE_QUEUE_TIMEOUT"})
		return
	case <-n.ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{Status: "rejected", Admission: "rejected-before-admission", Code: "NODE_STOPPED"})
		return
	}
	select {
	case result := <-request.reply:
		status := http.StatusOK
		if result.Status != "complete" {
			status = http.StatusBadRequest
		}
		jsonResponse(w, status, result)
	case <-ctx.Done():
		jsonResponse(w, http.StatusGatewayTimeout, response{Status: "pending", Admission: "unknown", Code: "MESSAGE_RESPONSE_TIMEOUT"})
	case <-n.ctx.Done():
		jsonResponse(w, http.StatusServiceUnavailable, response{Status: "pending", Admission: "unknown", Code: "NODE_STOPPED"})
	}
}

func (n *nodeServer) diagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	request := nodeRequest{kind: "diagnostics", ctx: r.Context(), diag: make(chan Snapshot, 1)}
	select {
	case n.requests <- request:
	case <-r.Context().Done():
		return
	case <-n.ctx.Done():
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	select {
	case snapshot := <-request.diag:
		jsonResponse(w, http.StatusOK, snapshot)
	case <-r.Context().Done():
	case <-n.ctx.Done():
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

type countingEntropy struct {
	recorder *raftadapter.Recorder
	emit     func(raftadapter.Choice) error
	node     uint64
	count    int
}

func (e *countingEntropy) Draw(stream string, bound int) (int, error) {
	value, err := e.recorder.Draw(stream, bound)
	if err != nil {
		return 0, err
	}
	choice := raftadapter.Choice{Node: e.node, Stream: stream, Sequence: uint64(e.count), Bound: bound, Value: value}
	if err := e.emit(choice); err != nil {
		return 0, err
	}
	e.count++
	return value, nil
}

// NodeMain is the built-in child command. Paths are supplied only by the
// trusted parent; the scenario and HTTP request schemas contain no path or
// executable. The only listener is hard-coded to 127.0.0.1:0.
func NodeMain(args []string, stdout, stderr io.Writer) int {
	fail := func(code string) int {
		_ = json.NewEncoder(stderr).Encode(struct {
			Error string `json:"error"`
		}{code})
		return 2
	}
	flags := flag.NewFlagSet("lab-node", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	id := flags.Uint64("id", 0, "fixed voter ID")
	incarnation := flags.Uint64("incarnation", 0, "owned process lifetime")
	directory := flags.String("directory", "", "owned lab directory")
	owner := flags.String("owner", "", "directory ownership identifier")
	seed := flags.Uint64("seed", 0, "explicit per-lifetime election seed")
	variant := flags.String("variant", string(target.Safe), "target variant")
	logLimit := flags.Uint64("log-limit", target.DefaultLogLimitBytes, "bounded journal/log")
	eventLimit := flags.Int("event-limit", 100000, "bounded actor transitions")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *id < 1 || *id > 3 || *incarnation < 1 || *incarnation > 1024 || *eventLimit < 1 || *eventLimit > 100000 {
		return fail("INVALID_NODE_FLAGS")
	}
	identity := buildinfo.Current()
	if !identity.Identified() {
		return fail("UNIDENTIFIED_BUILD")
	}
	if decoded, err := hex.DecodeString(*owner); err != nil || len(decoded) != 16 {
		return fail("INVALID_DIRECTORY_OWNER")
	}
	dir, err := regularPath(*directory, false)
	if err != nil {
		return fail("UNSAFE_DIRECTORY")
	}
	markerPath, err := regularPath(filepath.Join(dir, ".faultbraid-lab-owner.json"), false)
	if err != nil {
		return fail("UNOWNED_DIRECTORY")
	}
	marker, err := os.Open(markerPath)
	if err != nil {
		return fail("UNOWNED_DIRECTORY")
	}
	var ownership ownership
	err = strictjson.Decode(io.LimitReader(marker, 1025), &ownership)
	_ = marker.Close()
	if err != nil || ownership.Version != 1 || ownership.Owner != *owner || ownership.Identity != identity {
		return fail("UNOWNED_DIRECTORY")
	}
	journal, err := regularPath(filepath.Join(dir, fmt.Sprintf("node-%d.journal", *id)), true)
	if err != nil {
		return fail("UNSAFE_JOURNAL")
	}
	executable, err := os.Executable()
	if err != nil {
		return fail("UNIDENTIFIED_EXECUTABLE")
	}
	binarySHA, _, err := fileHash(executable)
	if err != nil {
		return fail("UNIDENTIFIED_EXECUTABLE")
	}
	store, err := target.NewFileStore(journal, *logLimit)
	if err != nil {
		return fail("JOURNAL_OPEN_FAILED")
	}
	defer store.Close()
	encoder := json.NewEncoder(stdout)
	emit := func(record wire) error {
		record.Node, record.Incarnation, record.PID = *id, *incarnation, os.Getpid()
		record.Identity, record.BinarySHA = identity, binarySHA
		return encoder.Encode(record)
	}
	recorder, err := raftadapter.NewRecorder(*id, *seed)
	if err != nil {
		return fail("ENTROPY_INITIALIZATION_FAILED")
	}
	entropy := &countingEntropy{recorder: recorder, node: *id, emit: func(choice raftadapter.Choice) error { return emit(wire{Type: "entropy", Choice: &choice}) }}
	node, err := target.New(target.Config{ID: *id, Store: store, Entropy: entropy, LogLimitBytes: *logLimit, Variant: target.Variant(*variant)})
	if err != nil {
		return fail("TARGET_INITIALIZATION_FAILED")
	}
	snapshot := func() Snapshot {
		var size int64
		if info, err := os.Stat(journal); err == nil {
			size = info.Size()
		}
		return Snapshot{Status: node.Status(), Syncs: store.SyncCount(), JournalBytes: size, Choices: entropy.count}
	}
	waiters := map[string][]waiter{}
	release := func(effects []target.Effect) error {
		for _, effect := range effects {
			switch effect.Kind {
			case target.SendEffect:
				if err := emit(wire{Type: "send", To: effect.To, Data: base64.StdEncoding.EncodeToString(effect.Data)}); err != nil {
					return err
				}
			case target.ReplyEffect:
				for _, waiting := range waiters[effect.RequestID] {
					select {
					case waiting.reply <- response{ID: effect.RequestID, Status: "complete", Admission: "admitted", Value: effect.Value}:
					default:
					}
				}
				delete(waiters, effect.RequestID)
			}
		}
		return nil
	}
	effects, err := node.Step(target.Start{})
	if releaseErr := release(effects); err == nil {
		err = releaseErr
	}
	if err != nil {
		return fail("TARGET_START_FAILED")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fail("LOOPBACK_LISTEN_FAILED")
	}
	limited := &boundedListener{Listener: listener, slots: make(chan struct{}, 64), done: make(chan struct{})}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	n := &nodeServer{ctx: ctx, requests: make(chan nodeRequest, maxNodeQueue)}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/propose", n.propose)
	mux.HandleFunc("/v1/message", n.message)
	mux.HandleFunc("/v1/diagnostics", n.diagnostics)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jsonResponse(w, http.StatusOK, struct {
			Ready bool `json:"ready"`
		}{true})
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 8192}
	server.SetKeepAlivesEnabled(false)
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(limited) }()
	defer func() {
		stop()
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
		_ = server.Close()
		_ = limited.Close()
	}()
	if err := emit(wire{Type: "ready", Address: listener.Addr().String(), Snapshot: snapshot()}); err != nil {
		return fail("READY_OUTPUT_FAILED")
	}
	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()
	transitions := 1
	step := func(event target.Event) error {
		if transitions >= *eventLimit {
			return ErrLimit
		}
		transitions++
		effects, err := node.Step(event)
		// A released prefix remains observable even if a later boundary fails.
		if releaseErr := release(effects); err == nil {
			err = releaseErr
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			_ = emit(wire{Type: "stop", Snapshot: snapshot(), Code: "CANCELLED"})
			return 0
		case err := <-serveError:
			if !errors.Is(err, http.ErrServerClosed) {
				return fail("LOOPBACK_SERVER_FAILED")
			}
			return 0
		case <-ticker.C:
			if err := step(target.Tick{}); err != nil {
				return fail("TARGET_TICK_FAILED")
			}
			for id, group := range waiters {
				live := group[:0]
				for _, waiting := range group {
					if waiting.ctx.Err() == nil {
						live = append(live, waiting)
					}
				}
				if len(live) == 0 {
					delete(waiters, id)
				} else {
					waiters[id] = live
				}
			}
		case request := <-n.requests:
			if request.ctx.Err() != nil {
				if request.reply != nil {
					request.reply <- response{ID: request.proposal.ID, Status: "rejected", Admission: "rejected-before-admission", Code: "CANCELLED_BEFORE_ADMISSION"}
				}
				continue
			}
			if request.kind == "diagnostics" {
				request.diag <- snapshot()
				continue
			}
			if request.kind == "message" {
				err := step(target.Message{Data: request.data})
				result := response{Status: "complete", Admission: "admitted"}
				if err != nil {
					result.Status, result.Code = "rejected", "TARGET_FRAME_REJECTED"
				}
				request.reply <- result
				if node.Status().Stopped || errors.Is(err, ErrLimit) {
					return fail("TARGET_MESSAGE_STOPPED")
				}
				continue
			}
			input := request.proposal
			waiters[input.ID] = append(waiters[input.ID], waiter{ctx: request.ctx, reply: request.reply})
			var event target.Event = target.Read{RequestID: input.ID}
			if input.Kind == "write" {
				event = target.Write{RequestID: input.ID, Value: input.Value}
			}
			err := step(event)
			if err != nil {
				result := response{ID: input.ID, Status: "pending", Admission: "unknown", Code: "TARGET_STOPPED"}
				if !node.Status().Stopped && !errors.Is(err, ErrLimit) {
					// Step's direct input errors precede admission; drain errors
					// mark Target.Stopped and remain uncertain.
					result.Status, result.Admission, result.Code = "rejected", "rejected-before-admission", "TARGET_REJECTED_BEFORE_ADMISSION"
				}
				group := waiters[input.ID]
				if len(group) > 0 {
					group = group[:len(group)-1]
					if len(group) == 0 {
						delete(waiters, input.ID)
					} else {
						waiters[input.ID] = group
					}
				}
				select {
				case request.reply <- result:
				default:
				}
				if node.Status().Stopped || errors.Is(err, ErrLimit) {
					return fail("TARGET_PROPOSAL_STOPPED")
				}
			}
		}
	}
}

type boundedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}

func (l *boundedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	connection, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &boundedConnection{Conn: connection, release: func() { <-l.slots }}, nil
}
func (l *boundedListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

type boundedConnection struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *boundedConnection) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
