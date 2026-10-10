package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/C-X1an/faultbraid/internal/scenario"
)

func rehash(t *testing.T, trace scenario.Trace) scenario.Trace {
	t.Helper()
	trace.Digest = ""
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	trace.Digest = hex.EncodeToString(sum[:])
	return trace
}
func cloneTrace(t *testing.T, trace scenario.Trace) scenario.Trace {
	t.Helper()
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	var result scenario.Trace
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestAC014TenIdenticalTargetRunsAndActualReplay(t *testing.T) {
	for _, s := range []scenario.Scenario{majorityScenario(0), staleReadScenario(true)} {
		baseline := run(t, s)
		for i := 0; i < 10; i++ {
			repeated := run(t, s)
			if repeated.Digest != baseline.Digest {
				t.Fatalf("repeat%d logical trace changed", i)
			}
			if err := scenario.Replay(context.Background(), repeated, identity(t)); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("10/10 identical actual variant=%s digest=%s source=%s", baseline.Header.Variant, baseline.Digest, baseline.Header.SourceSHA)
	}
}
func TestTargetReplayRejectsTamperingEvenAfterRehash(t *testing.T) {
	baseline := run(t, majorityScenario(0))
	for _, kind := range []string{"event", "choice", "extra-choice", "missing-choice", "choice-order", "source", "result", "scenario"} {
		trace := cloneTrace(t, baseline)
		switch kind {
		case "event":
			trace.Events[0].Detail = "altered"
		case "choice":
			trace.Choices[0].Value = (trace.Choices[0].Value + 1) % trace.Choices[0].Bound
		case "extra-choice":
			choice := trace.Choices[len(trace.Choices)-1]
			choice.Sequence++
			trace.Choices = append(trace.Choices, choice)
		case "missing-choice":
			trace.Choices = trace.Choices[1:]
		case "choice-order":
			trace.Choices[0], trace.Choices[1] = trace.Choices[1], trace.Choices[0]
		case "source":
			trace.Header.SourceSHA = trace.Header.PatchSHA
		case "result":
			trace.Result.Reason = "edited"
		case "scenario":
			trace.Scenario.Seed++
		}
		trace = rehash(t, trace)
		if err := scenario.Replay(context.Background(), trace, identity(t)); err == nil {
			t.Fatalf("%s edited/rehash trace trusted", kind)
		}
	}
}
func TestIncarnationDropsOldFramesAfterRestart(t *testing.T) {
	s := majorityScenario(0)
	s.Actions = s.Actions[:3]
	s.Actions = append(s.Actions, scenario.Action{ID: "delay", At: 40, Kind: "delay_link", From: scenario.Node(2), To: scenario.Node(1), Ticks: scenario.Node(15)}, scenario.Action{ID: "crash", At: 42, Kind: "crash", Node: scenario.Node(1)}, scenario.Action{ID: "restart", At: 43, Kind: "restart", Node: scenario.Node(1)})
	s.EndTime = scenario.Node(65)
	trace := run(t, s)
	found := false
	for _, record := range trace.Events {
		if record.Kind == "drop" && record.Detail == "incarnation-or-down" && record.Node == 1 && record.At >= 55 {
			found = true
		}
	}
	if !found {
		t.Fatal("pre-crash delayed frame crossed restarted incarnation")
	}
	if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
		t.Fatal(err)
	}
}
