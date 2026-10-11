package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/strictjson"
)

func TestIdentifiedTargetCLIArtifactsAndReplay(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "python3", "scripts/build.py")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("actual identified build failed: %v %s", err, output)
	}
	identity, err := buildinfo.FromFiles(root)
	if err != nil || !identity.Identified() {
		t.Fatalf("cannot measure execution inputs: %v", err)
	}
	name := "faultbraid"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(root, "bin", name)
	tmp := t.TempDir()
	call := func(want int, input []byte, args ...string) ([]byte, []byte) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		cmd.Stdin = bytes.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatalf("actual CLI launch failed: %v", err)
			}
		}
		if ctx.Err() != nil || code != want {
			t.Fatalf("command %s exit=%d want=%d timeout=%v stdout=%s stderr=%s", args[0], code, want, ctx.Err(), stdout.Bytes(), stderr.Bytes())
		}
		return stdout.Bytes(), stderr.Bytes()
	}
	marshal := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	readTrace := func(dir string) scenario.Trace {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, "trace.json"))
		if err != nil {
			t.Fatal(err)
		}
		trace, err := scenario.DecodeTrace(bytes.NewReader(data))
		if err != nil || trace.Identity() != identity {
			t.Fatalf("artifact does not bind actual source/dependency/patch: %v", err)
		}
		for _, filename := range []string{"scenario.json", "history.json", "result.json", "report.html"} {
			if info, err := os.Stat(filepath.Join(dir, filename)); err != nil || info.Size() == 0 {
				t.Fatalf("missing artifact %s: %v", filename, err)
			}
		}
		return trace
	}

	safe := scenario.Default(0)
	safe.EndTime = scenario.Node(60)
	for node := uint64(1); node <= 3; node++ {
		id := fmt.Sprintf("write-at-%d", node)
		safe.Actions = append(safe.Actions, scenario.Action{ID: id, At: 29 + node, Kind: "invoke_write", Node: scenario.Node(node), RequestID: scenario.Text("write7"), Value: scenario.Value(7)})
	}
	for node := uint64(1); node <= 3; node++ {
		safe.Actions = append(safe.Actions, scenario.Action{ID: fmt.Sprintf("read-at-%d", node), At: 44 + node, Kind: "invoke_read", Node: scenario.Node(node), RequestID: scenario.Text("read7")})
	}
	safeDir := filepath.Join(tmp, "safe")
	call(0, marshal(safe), "simulate", "-scenario", "-", "-outdir", safeDir)
	safeTrace := readTrace(safeDir)
	if safeTrace.Result.Status != history.Valid || len(safeTrace.History.Operations) != 2 {
		t.Fatal("actual quorum-safe CLI execution has wrong result")
	}
	for _, op := range safeTrace.History.Operations {
		if op.Return == nil || op.Value != 7 {
			t.Fatal("majority write/read did not actually complete with7")
		}
	}
	call(0, nil, "replay", "-in", filepath.Join(safeDir, "trace.json"))
	before, err := os.ReadFile(filepath.Join(safeDir, "trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	call(2, marshal(safe), "simulate", "-scenario", "-", "-outdir", safeDir)
	after, err := os.ReadFile(filepath.Join(safeDir, "trace.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("exclusive output overwrote existing evidence")
	}

	bad := campaign.KnownBadScenario()
	bad.Actions = append([]scenario.Action{{ID: "unrelated-delay", At: 20, Kind: "delay_link", From: scenario.Node(1), To: scenario.Node(3), Ticks: scenario.Node(1)}}, bad.Actions...)
	badDir := filepath.Join(tmp, "unsafe")
	call(0, marshal(bad), "simulate", "-scenario", "-", "-outdir", badDir)
	baseline := readTrace(badDir)
	if baseline.Result.Status != history.Invalid || baseline.Failure == nil {
		t.Fatal("actual unsafe target failed to produce its counterexample")
	}
	for i := 0; i < 10; i++ {
		output, _ := call(0, nil, "replay", "-in", filepath.Join(badDir, "trace.json"))
		var replay struct {
			Status  string `json:"status"`
			Digest  string `json:"digest"`
			Scope   string `json:"scope"`
			Verdict string `json:"verdict"`
		}
		if err := json.Unmarshal(output, &replay); err != nil || replay.Status != "PASS" || replay.Digest != baseline.Digest || replay.Verdict != "INVALID" {
			t.Fatal("CLI replay claimed a changed result")
		}
	}
	for _, strategy := range []string{campaign.DDMin, campaign.Greedy} {
		dir := filepath.Join(tmp, strategy)
		output, _ := call(0, nil, "shrink-scenario", "-trace", filepath.Join(badDir, "trace.json"), "-outdir", dir, "-strategy", strategy)
		var reduction campaign.Reduction
		if err := json.Unmarshal(output, &reduction); err != nil || reduction.Status != history.Invalid || !reduction.OneMinimal || !reduction.BaselineReplayed || len(reduction.Trace.Scenario.Actions) >= len(bad.Actions) || reduction.Trace.Identity() != identity || reduction.Trace.Failure == nil || *reduction.Trace.Failure != *baseline.Failure {
			t.Fatalf("CLI reduction lost actual failure/provenance: %v", err)
		}
		readTrace(dir)
		call(0, nil, "replay", "-in", filepath.Join(dir, "trace.json"))
	}
	unknownDir := filepath.Join(tmp, "unknown-reduction")
	output, _ := call(3, nil, "shrink-scenario", "-trace", filepath.Join(badDir, "trace.json"), "-outdir", unknownDir, "-max-checks", "1")
	var unknown campaign.Reduction
	if err := json.Unmarshal(output, &unknown); err != nil || unknown.Status != history.Inconclusive || unknown.OneMinimal {
		t.Fatal("budget exhaustion became a definitive CLI result")
	}
	limited := safe
	limited.EventLimit = 1
	limitedDir := filepath.Join(tmp, "limited")
	call(3, marshal(limited), "simulate", "-scenario", "-", "-outdir", limitedDir)
	if readTrace(limitedDir).Result.Status != history.Inconclusive {
		t.Fatal("event budget became a semantic verdict")
	}

	for _, edit := range []string{"source", "event", "digest", "version"} {
		changed := baseline
		changed.Events = append([]scenario.Record{}, baseline.Events...)
		wantCode := "REPLAY_MISMATCH"
		switch edit {
		case "source":
			changed.Header.SourceSHA = strings.Repeat("0", 64)
		case "event":
			changed.Events[0].Detail += "changed"
		case "digest":
			changed.Digest = strings.Repeat("0", 64)
		case "version":
			changed.Version = 2
			wantCode = "UNSUPPORTED_VERSION"
		}
		if edit == "source" || edit == "event" {
			changed.Digest = ""
			sum := sha256.Sum256(marshal(changed))
			changed.Digest = hex.EncodeToString(sum[:])
		}
		_, stderr := call(2, marshal(changed), "replay", "-in", "-")
		if !bytes.Contains(stderr, []byte(wantCode)) {
			t.Fatalf("tamper %s wrong classification: %s", edit, stderr)
		}
	}
	for _, input := range [][]byte{
		[]byte(`{"model":"raft-trace-v1","version":1,"version":1}`),
		[]byte(`{"model":"raft-trace-v1","version":1.0}`),
		[]byte(`{"model":"raft-trace-v1","version":null}`),
		[]byte(strings.Repeat("[", strictjson.MaxDepth+2) + "0" + strings.Repeat("]", strictjson.MaxDepth+2)),
		bytes.Repeat([]byte(" "), strictjson.MaxBytes+1),
		{0xff},
	} {
		_, stderr := call(2, input, "replay", "-in", "-")
		if !bytes.Contains(stderr, []byte("INVALID_INPUT")) {
			t.Fatal("malformed replay input misclassified")
		}
	}
	call(2, marshal(safe), "simulate", "-scenario", "-", "-outdir", filepath.Join(tmp, "badflags"), "-count", "1")
	call(2, nil, "campaign", "-outdir", filepath.Join(tmp, "overflow"), "-count", "2", "-seed-start", "18446744073709551615")

	campaignDir := filepath.Join(tmp, "campaign")
	output, _ = call(0, nil, "campaign", "-outdir", campaignDir, "-count", "3")
	var index campaign.Index
	if err := json.Unmarshal(output, &index); err != nil || index.Identity != identity || index.Requested != 3 || index.Attempted != 3 || index.Unrun != 0 || index.Valid+index.Invalid+index.Inconclusive+index.Errors != 3 || index.Errors != 0 || index.Status != "COMPLETE" || len(index.Entries) != 3 {
		t.Fatalf("CLI campaign did not account for every seed: %v", err)
	}
	for _, entry := range index.Entries {
		path := filepath.Join(campaignDir, fmt.Sprintf("seed-%020d", entry.Seed), "trace.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal("attempted campaign seed has no trace")
		}
		trace, err := scenario.DecodeTrace(bytes.NewReader(data))
		if err != nil || trace.Digest != entry.Digest || trace.Identity() != identity {
			t.Fatal("campaign summary differs from its actual trace")
		}
		call(0, nil, "replay", "-in", path)
	}
	t.Logf("actual identified CLI: safe completed2 operations; unsafe replay10/10; ddmin/greedy executable reductions; budgets exit3; campaign attempted3/unrun0; source=%s", identity.SourceTreeSHA)
}
