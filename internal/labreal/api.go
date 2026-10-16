// Package labreal runs the same instrumented Raft target in three owned child
// processes. The parent relays copied frames over real loopback HTTP, while
// each child owns a checksummed FileStore journal. This is native conformance,
// not an OS-deterministic replay or a physical power-loss certification.
package labreal

import (
	"errors"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/target"
)

const TickDuration = 25 * time.Millisecond
const RequestTimeout = 2 * time.Second
const MaxRealTicks = 1000
const maxConcurrentRequests = 16
const maxNodeQueue = 128
const maxWireLine = 128 << 10
const maxChildOutput = 16 << 20
const maxProposeBytes = 1024

var ErrUnsupportedAction = errors.New("real lab: unsupported scenario action")
var ErrUnsafePath = errors.New("real lab: output/executable path must be an owned regular path without aliases")
var ErrIdentity = errors.New("real lab: identified current build required")
var ErrLimit = errors.New("real lab: resource limit reached")

// Executable must be the caller's own current executable, which dispatches the
// built-in lab-node command. Scenario input can never select a command or a path.
// Directory must be new, with an existing trusted parent. The lab retains only
// its owned files there. Identity is checked against each child's actual build
// identity and executable bytes before workload admission.
type Options struct {
	Executable  string
	Directory   string
	Identity    buildinfo.Identity
	Concurrency int
	// Optional explicit native causality; empty preserves the original wall-time schedule.
	CompletionDependencies []CompletionDependency
	// After the fixed nominal end, await these requests within the existing run context.
	RequiredCompletions []string
}

type Snapshot struct {
	Status       target.Status `json:"target_status"`
	Syncs        uint64        `json:"successful_file_syncs"`
	JournalBytes int64         `json:"journal_bytes"`
	Choices      int           `json:"entropy_choices"`
}

type Process struct {
	Node                  uint64               `json:"node_id"`
	Incarnation           uint64               `json:"incarnation"`
	PID                   int                  `json:"pid"`
	Address               string               `json:"loopback_address"`
	Identity              buildinfo.Identity   `json:"identity"`
	BinarySHA             string               `json:"binary_sha256"`
	EntropySeed           uint64               `json:"entropy_seed"`
	Choices               []raftadapter.Choice `json:"entropy_choices"`
	Journal               string               `json:"journal"`
	Stdout                string               `json:"stdout"`
	Stderr                string               `json:"stderr"`
	Ready                 Snapshot             `json:"ready_snapshot"`
	Last                  Snapshot             `json:"last_observed_snapshot"`
	Stop                  string               `json:"stop_reason"`
	Waited                bool                 `json:"waited"`
	ExitCode              int                  `json:"exit_code"`
	SocketClosed          bool                 `json:"socket_closed"`
	ProtocolTrailingBytes int                  `json:"protocol_trailing_bytes"`
}

type Attempt struct {
	Ordinal   int64  `json:"ordinal"`
	Action    string `json:"action_id"`
	RequestID string `json:"request_id"`
	Node      uint64 `json:"node_id"`
	PID       int    `json:"pid"`
	Kind      string `json:"kind"`
	Phase     string `json:"phase"`
	Admission string `json:"admission"`
	Value     int64  `json:"value"`
	Code      string `json:"code"`
}

type Journal struct {
	Node  uint64 `json:"node_id"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	SHA   string `json:"sha256"`
}

type Transport struct {
	Sent       uint64 `json:"sent_frames"`
	Delivered  uint64 `json:"successful_http_message_posts"`
	Dropped    uint64 `json:"dropped_frames"`
	Rejected   uint64 `json:"rejected_http_message_posts"`
	Bytes      uint64 `json:"sent_frame_bytes"`
	Unresolved uint64 `json:"unresolved_frame_outcomes"`
}

type Result struct {
	Version                int                    `json:"version"`
	Mode                   string                 `json:"mode"`
	Identity               buildinfo.Identity     `json:"identity"`
	BinarySHA              string                 `json:"binary_sha256"`
	TickMilliseconds       int64                  `json:"tick_milliseconds"`
	Concurrency            int                    `json:"request_concurrency"`
	PeakConcurrency        int64                  `json:"observed_peak_request_concurrency"`
	DurationMillis         int64                  `json:"duration_milliseconds"`
	EntropyVersion         string                 `json:"entropy_version"`
	Scenario               scenario.Scenario      `json:"scenario"`
	History                history.History        `json:"history"`
	Check                  history.Result         `json:"result"`
	Attempts               []Attempt              `json:"attempts"`
	Processes              []Process              `json:"processes"`
	Journals               []Journal              `json:"journals"`
	Transport              Transport              `json:"transport"`
	CompletionDependencies []CompletionDependency `json:"completion_dependencies,omitempty"`
	RequiredCompletions    []string               `json:"required_completions,omitempty"`
	CompletionBarriers     []CompletionBarrier    `json:"completion_barriers,omitempty"`
	ActionDispatches       []ActionDispatch       `json:"action_dispatches,omitempty"`
	CleanupComplete        bool                   `json:"cleanup_complete"`
	Reason                 string                 `json:"reason"`
}

type proposal struct {
	ID    string `json:"request_id"`
	Kind  string `json:"kind"`
	Value int64  `json:"value"`
}

type response struct {
	ID        string `json:"request_id"`
	Status    string `json:"status"`
	Admission string `json:"admission"`
	Value     int64  `json:"value"`
	Code      string `json:"code"`
}

type wire struct {
	Type        string              `json:"type"`
	Node        uint64              `json:"node_id"`
	Incarnation uint64              `json:"incarnation"`
	PID         int                 `json:"pid"`
	Address     string              `json:"address,omitempty"`
	Identity    buildinfo.Identity  `json:"identity"`
	BinarySHA   string              `json:"binary_sha256"`
	Snapshot    Snapshot            `json:"snapshot"`
	To          uint64              `json:"to,omitempty"`
	Data        string              `json:"data,omitempty"`
	Code        string              `json:"code,omitempty"`
	Choice      *raftadapter.Choice `json:"choice,omitempty"`
}

type ownership struct {
	Version  int                `json:"version"`
	Owner    string             `json:"owner"`
	Identity buildinfo.Identity `json:"identity"`
}
