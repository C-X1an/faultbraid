// Package scenario runs the actual Raft target with explicit virtual time and transport.
package scenario

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"github.com/C-X1an/faultbraid/internal/target"
)

const MaxActions = 1024
const MaxEndTime = 10000

var ErrUnsupportedVersion = errors.New("unsupported scenario version or target")
var ErrCausality = errors.New("scenario action is not executable in the current state")

type Action struct {
	ID         string  `json:"action_id"`
	At         uint64  `json:"virtual_time"`
	Kind       string  `json:"kind"`
	Node       *uint64 `json:"node,omitempty"`
	RequestID  *string `json:"request_id,omitempty"`
	Value      *int64  `json:"value,omitempty"`
	From       *uint64 `json:"from,omitempty"`
	To         *uint64 `json:"to,omitempty"`
	Ticks      *uint64 `json:"ticks,omitempty"`
	Count      *int    `json:"count,omitempty"`
	Boundary   *string `json:"boundary,omitempty"`
	Occurrence *int    `json:"occurrence,omitempty"`
}

type Scenario struct {
	Version           int      `json:"version"`
	Target            string   `json:"target"`
	Seed              uint64   `json:"seed"`
	Voters            []uint64 `json:"voters"`
	EventLimit        int      `json:"event_limit"`
	QueueLimit        int      `json:"queue_limit"`
	LogLimitBytes     uint64   `json:"log_limit_bytes"`
	CheckerStateLimit int      `json:"checker_state_limit"`
	Actions           []Action `json:"actions"`
	EndTime           *uint64  `json:"end_time,omitempty"`
	Variant           *string  `json:"target_variant,omitempty"`
}

func Decode(r io.Reader) (Scenario, error) {
	var s Scenario
	if err := strictjson.Decode(r, &s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

func (s Scenario) TargetVariant() target.Variant {
	if s.Variant == nil {
		return target.Safe
	}
	return target.Variant(*s.Variant)
}

func (s Scenario) Until() uint64 {
	if s.EndTime != nil {
		return *s.EndTime
	}
	if len(s.Actions) == 0 {
		return 200
	}
	return s.Actions[len(s.Actions)-1].At + 200
}

func validID(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, b := range []byte(s) {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == ':' || b == '-') {
			return false
		}
	}
	return true
}

func validNode(n *uint64) bool { return n != nil && *n >= 1 && *n <= 3 }
func validBoundary(s string) bool {
	switch target.Barrier(s) {
	case target.BeforeAppend, target.AfterStageBeforeFlush, target.AfterFlushBeforeSend, target.AfterSendBeforeApply, target.AfterApplyBeforeResponse, target.AfterResponse:
		return true
	}
	return false
}

func (s Scenario) Validate() error {
	if s.Version != 1 || s.Target != "raft-register-v1" {
		return ErrUnsupportedVersion
	}
	if len(s.Voters) != 3 || s.Voters[0] != 1 || s.Voters[1] != 2 || s.Voters[2] != 3 {
		return errors.New("fixed voters must be [1,2,3]")
	}
	if s.EventLimit < 1 || s.EventLimit > 100000 || s.QueueLimit < 1 || s.QueueLimit > 100000 || s.LogLimitBytes < 256 || s.LogLimitBytes > 64<<20 || s.CheckerStateLimit < 1 || s.CheckerStateLimit > history.MaxStatesLimit || len(s.Actions) > MaxActions {
		return errors.New("scenario resource limit out of range")
	}
	if s.TargetVariant() != target.Safe && s.TargetVariant() != target.UnsafeLocalRead {
		return errors.New("unsupported target variant")
	}
	ids := map[string]bool{}
	requests := map[string]Action{}
	var previous uint64
	for i, a := range s.Actions {
		if !validID(a.ID) || ids[a.ID] || a.At > MaxEndTime || i > 0 && a.At < previous {
			return errors.New("invalid action ID or virtual time order")
		}
		ids[a.ID] = true
		previous = a.At
		want := map[string]bool{}
		switch a.Kind {
		case "invoke_write", "invoke_read":
			want["node"], want["request"], want["value"] = true, true, a.Kind == "invoke_write"
			if !validNode(a.Node) || a.RequestID == nil || !validID(*a.RequestID) || a.Kind == "invoke_write" && a.Value == nil {
				return errors.New("invalid invocation")
			}
			if prior, ok := requests[*a.RequestID]; ok {
				if prior.Kind != a.Kind || a.Kind == "invoke_write" && *prior.Value != *a.Value {
					return errors.New("retry request ID payload conflict")
				}
			} else {
				requests[*a.RequestID] = a
			}
		case "crash", "restart":
			want["node"] = true
			if !validNode(a.Node) {
				return errors.New("invalid crash/restart node")
			}
		case "crash_at":
			want["node"], want["boundary"], want["occurrence"] = true, true, true
			if !validNode(a.Node) || a.Boundary == nil || !validBoundary(*a.Boundary) || a.Occurrence == nil || *a.Occurrence < 1 || *a.Occurrence > 100000 {
				return errors.New("invalid crash barrier")
			}
		case "partition", "heal", "delay_link", "drop_next", "duplicate_next":
			want["from"], want["to"] = true, true
			if !validNode(a.From) || !validNode(a.To) || *a.From == *a.To {
				return errors.New("invalid directed link")
			}
			if a.Kind == "delay_link" {
				want["ticks"] = true
				if a.Ticks == nil || *a.Ticks > MaxEndTime || *a.Ticks > math.MaxUint64-a.At {
					return errors.New("invalid link delay")
				}
			}
			if a.Kind == "drop_next" || a.Kind == "duplicate_next" {
				want["count"] = true
				if a.Count == nil || *a.Count < 1 || *a.Count > 100000 {
					return errors.New("invalid link fault count")
				}
			}
		default:
			return errors.New("unsupported action kind")
		}
		present := map[string]bool{"node": a.Node != nil, "request": a.RequestID != nil, "value": a.Value != nil, "from": a.From != nil, "to": a.To != nil, "ticks": a.Ticks != nil, "count": a.Count != nil, "boundary": a.Boundary != nil, "occurrence": a.Occurrence != nil}
		for key, exists := range present {
			if exists && !want[key] {
				return fmt.Errorf("unrelated action field: %s", key)
			}
		}
	}
	if len(requests) > history.MaxOperations {
		return errors.New("too many logical requests")
	}
	if previous > math.MaxUint64-200 || s.Until() < previous || s.Until() > MaxEndTime {
		return errors.New("end_time outside bounded action schedule")
	}
	return nil
}

func Default(seed uint64) Scenario {
	return Scenario{Version: 1, Target: "raft-register-v1", Seed: seed, Voters: []uint64{1, 2, 3}, EventLimit: 100000, QueueLimit: 100000, LogLimitBytes: 1 << 20, CheckerStateLimit: 100000, Actions: []Action{}}
}

func Node(n uint64) *uint64 { return &n }
func Text(s string) *string { return &s }
func Value(v int64) *int64  { return &v }
func Count(n int) *int      { return &n }
