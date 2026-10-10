// Package history checks finite concurrent integer-register histories.
package history

import (
	"errors"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"io"
)

var ErrUnsupportedVersion = errors.New("unsupported history version")

const MaxOperations = 24
const DefaultMaxStates = 100000
const MaxStatesLimit = 1000000

type Operation struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Value  int64  `json:"value"`
	Call   int64  `json:"call"`
	Return *int64 `json:"return"`
}
type History struct {
	Version    int         `json:"version"`
	Initial    int64       `json:"initial"`
	Operations []Operation `json:"operations"`
}

type Verdict string

const (
	Valid        Verdict = "VALID"
	Invalid      Verdict = "INVALID"
	Inconclusive Verdict = "INCONCLUSIVE"
)

type Result struct {
	Status  Verdict  `json:"status"`
	Witness []string `json:"witness"`
	States  int      `json:"states"`
	Reason  string   `json:"reason"`
}
type Options struct{ MaxStates int }

func Decode(r io.Reader) (History, error) {
	var h History
	if err := strictjson.Decode(r, &h); err != nil {
		return h, err
	}
	return h, h.Validate()
}
func (h History) Validate() error {
	if h.Version != 1 {
		return ErrUnsupportedVersion
	}
	if len(h.Operations) > MaxOperations {
		return errors.New("too many operations")
	}
	ids := make(map[string]bool)
	for _, op := range h.Operations {
		if !validID(op.ID) || ids[op.ID] {
			return errors.New("invalid or duplicate operation ID")
		}
		ids[op.ID] = true
		if op.Kind != "read" && op.Kind != "write" {
			return errors.New("unsupported operation kind")
		}
		if op.Call < 0 || (op.Return != nil && *op.Return <= op.Call) {
			return errors.New("invalid operation interval")
		}
	}
	return nil
}
func validID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}
