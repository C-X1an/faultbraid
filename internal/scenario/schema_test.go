package scenario

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCodecGoldenPredatesImplementation(t *testing.T) {
	data, err := os.ReadFile("../../docs/codec/raft-trace-v1.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	trace, err := DecodeTrace(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(append(encoded, '\n'), data) {
		t.Fatal("typed codec drift from golden field/array encoding")
	}
	d, err := digest(trace)
	if err != nil || d != trace.Digest {
		t.Fatal("canonical golden digest mismatch", d, err)
	}
	if trace.Identity().Identified() {
		t.Fatal("synthetic golden incorrectly identified as execution")
	}
}

func TestStrictScenarioActionsAndBounds(t *testing.T) {
	valid := Default(0)
	valid.Actions = []Action{{ID: "write", At: 0, Kind: "invoke_write", Node: Node(1), RequestID: Text("write"), Value: Value(0)}}
	data, _ := json.Marshal(valid)
	if _, err := Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []string{
		strings.Replace(string(data), `"seed":0`, `"seed":0,"seed":1`, 1),
		strings.Replace(string(data), `"value":0`, `"value":null`, 1),
		strings.Replace(string(data), `"node":1`, `"node":0`, 1),
		strings.Replace(string(data), `"value":0`, `"value":0,"ticks":1`, 1),
		strings.Replace(string(data), `"event_limit":100000`, `"event_limit":100001`, 1),
	} {
		if _, err := Decode(strings.NewReader(edit)); err == nil {
			t.Fatal("malformed scenario admitted")
		}
	}
}
