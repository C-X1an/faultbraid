package scenario

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/strictjson"
)

const MaxTraceBytes = strictjson.MaxBytes

type Header struct {
	SourceSHA string `json:"source_tree_sha256"`
	ModuleSHA string `json:"module_lock_sha256"`
	PatchSHA  string `json:"patch_sha256"`
	Scheduler string `json:"scheduler_version"`
	Entropy   string `json:"entropy_version"`
	Seed      uint64 `json:"seed"`
	Variant   string `json:"target_variant"`
	Initial   int64  `json:"initial"`
}

type Record struct {
	ID          uint64 `json:"event_id"`
	Parent      uint64 `json:"parent_event_id"`
	Node        uint64 `json:"node_id"`
	Incarnation uint64 `json:"incarnation"`
	At          uint64 `json:"virtual_time"`
	Kind        string `json:"kind"`
	PayloadSHA  string `json:"payload_sha256"`
	Detail      string `json:"detail"`
}

type Failure struct {
	Requirement string `json:"requirement"`
	Variant     string `json:"target_variant"`
	Kind        string `json:"kind"`
	Fingerprint string `json:"fingerprint"`
}

type Trace struct {
	Version  int                  `json:"version"`
	Model    string               `json:"model"`
	Header   Header               `json:"header"`
	Scenario Scenario             `json:"scenario"`
	Choices  []raftadapter.Choice `json:"choices"`
	Events   []Record             `json:"events"`
	History  history.History      `json:"history"`
	Result   history.Result       `json:"result"`
	Failure  *Failure             `json:"failure"`
	Digest   string               `json:"digest"`
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func digest(trace Trace) (string, error) {
	trace.Digest = ""
	data, err := json.Marshal(trace)
	if err != nil {
		return "", err
	}
	return hash(data), nil
}

func failure(variant string) *Failure {
	f := &Failure{Requirement: "FR-002", Variant: variant, Kind: "nonlinearizable-register"}
	f.Fingerprint = hash([]byte(f.Requirement + "\x00" + f.Variant + "\x00" + f.Kind))
	return f
}

func DecodeTrace(r io.Reader) (Trace, error) {
	var t Trace
	if err := strictjson.Decode(r, &t); err != nil {
		return t, err
	}
	if t.Version != 1 || t.Model != "raft-trace-v1" {
		return t, ErrUnsupportedVersion
	}
	if err := t.Scenario.Validate(); err != nil {
		return t, err
	}
	return t, nil
}

func (t Trace) Identity() buildinfo.Identity {
	return buildinfo.Identity{SourceTreeSHA: t.Header.SourceSHA, ModuleLockSHA: t.Header.ModuleSHA, PatchSHA: t.Header.PatchSHA}
}

// Replay executes the actual target using exact recorded entropy, then executes
// it with the declared seed. Benign entropy edits plus a recomputed hash also fail.
func Replay(ctx context.Context, saved Trace, identity buildinfo.Identity) error {
	if saved.Version != 1 || saved.Model != "raft-trace-v1" {
		return ErrUnsupportedVersion
	}
	if !identity.Identified() || saved.Identity() != identity {
		return errors.New("replay source, module or patch identity mismatch")
	}
	want, err := digest(saved)
	if err != nil || want != saved.Digest {
		return errors.New("replay trace digest mismatch")
	}
	actual, err := run(ctx, saved.Scenario, Options{Identity: identity, choices: saved.Choices, replay: true})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, saved) {
		return errors.New("replay target execution mismatch")
	}
	fresh, err := Run(ctx, saved.Scenario, Options{Identity: identity})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(fresh, saved) {
		return errors.New("replay seed entropy mismatch")
	}
	return nil
}
