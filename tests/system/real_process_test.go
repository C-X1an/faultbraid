package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/labreal"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// The system-test executable has a built-in helper dispatch, just as the CLI
// has lab-node. Both parent and children identify the actual source files used
// by this test run. This is a real same-test-binary process integration, not a
// claim that go test produced the production CLI's injected build identity.
func TestMain(m *testing.M) {
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Fprintln(os.Stderr, "test source directory unavailable")
		os.Exit(2)
	}
	measured, err := buildinfo.FromFiles(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "test source identity unavailable")
		os.Exit(2)
	}
	buildinfo.SourceTreeSHA, buildinfo.ModuleLockSHA, buildinfo.PatchSHA = measured.SourceTreeSHA, measured.ModuleLockSHA, measured.PatchSHA
	if len(os.Args) > 1 && os.Args[1] == "lab-node" {
		os.Exit(labreal.NodeMain(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func realArtifactDirectory(t *testing.T) string {
	t.Helper()
	base, err := filepath.Abs("../../.local-runs/real-process-tests")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	parent, err := os.MkdirTemp(base, "actual-")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, "lab")
}
func realScenario() scenario.Scenario {
	s := scenario.Default(0)
	for index := 0; index < 16; index++ {
		s.Actions = append(s.Actions, scenario.Action{ID: fmt.Sprintf("write7-burst-%d", index), At: 40, Kind: "invoke_write", Node: scenario.Node(uint64(index%3 + 1)), RequestID: scenario.Text("write7"), Value: scenario.Value(7)})
	}
	invokeAll(&s, 70, "invoke_read", "read7", 0)
	s.Actions = append(s.Actions,
		scenario.Action{ID: "kill-node2", At: 90, Kind: "crash", Node: scenario.Node(2)},
		scenario.Action{ID: "read-dead-node2", At: 95, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("dead-read")},
		scenario.Action{ID: "restart-node2", At: 110, Kind: "restart", Node: scenario.Node(2)})
	invokeAll(&s, 150, "invoke_write", "write9", 9)
	invokeAll(&s, 170, "invoke_write", "write7", 7)
	// Retry identity cannot revert a later acknowledged write after restart.
	for i := len(s.Actions) - 3; i < len(s.Actions); i++ {
		s.Actions[i].ID += "-retry"
	}
	invokeAll(&s, 200, "invoke_read", "read9", 0)
	s.EndTime = scenario.Node(230)
	return s
}
func nativeOperation(t *testing.T, result labreal.Result, id string) history.Operation {
	t.Helper()
	for _, operation := range result.History.Operations {
		if operation.ID == id {
			return operation
		}
	}
	t.Fatalf("native history lacks logical request %s: %+v", id, result.History)
	return history.Operation{}
}
func retainNativeResult(t *testing.T, directory string, result labreal.Result) {
	t.Helper()
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "native-result.json")
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	// Report only a relative engineering-artifact location, not a private path.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained actual native fixture=%s", filepath.ToSlash(rel))
}

func TestAC019ThreeRealProcessesJournalKillRestartAndCleanup(t *testing.T) {
	for _, concurrency := range []int{1, 4, 16} {
		t.Run(fmt.Sprintf("concurrency-%d", concurrency), func(t *testing.T) {
			directory := realArtifactDirectory(t)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
			defer cancel()
			result, runErr := labreal.Run(ctx, realScenario(), labreal.Options{Executable: executable, Directory: directory, Identity: buildinfo.Current(), Concurrency: concurrency})
			retainNativeResult(t, directory, result)
			if runErr != nil || result.Check.Status != history.Valid || !result.CleanupComplete {
				t.Fatalf("native conformance error=%v result=%+v cleanup=%v reason=%s", runErr, result.Check, result.CleanupComplete, result.Reason)
			}
			if result.Mode != "native-loopback-conformance-v1" || !strings.Contains(result.Reason, "not replayable") || result.TickMilliseconds != 25 || result.Concurrency != concurrency || result.PeakConcurrency < 1 || result.PeakConcurrency > int64(concurrency) {
				t.Fatal("native limits/observed concurrency/OS limitation missing")
			}
			if result.Transport.Sent == 0 || result.Transport.Delivered == 0 || result.Transport.Bytes == 0 {
				t.Fatal("real HTTP transport did not execute")
			}
			binary, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(binary)
			if result.BinarySHA != hex.EncodeToString(digest[:]) || result.Identity != buildinfo.Current() {
				t.Fatal("native executable/source identity mismatch")
			}
			pids := map[int]bool{}
			var killed, restarted *labreal.Process
			for index := range result.Processes {
				process := &result.Processes[index]
				if process.PID <= 0 || pids[process.PID] || process.PID == os.Getpid() || !process.Waited || !process.SocketClosed || process.BinarySHA != result.BinarySHA || process.Identity != result.Identity || len(process.Choices) == 0 {
					t.Fatalf("actual PID/ownership/entropy evidence missing: %+v", process)
				}
				pids[process.PID] = true
				host, port, err := net.SplitHostPort(process.Address)
				if err != nil || host != "127.0.0.1" || port == "0" {
					t.Fatal("child did not bind real loopback socket")
				}
				connection, err := net.DialTimeout("tcp4", process.Address, 100*time.Millisecond)
				if err == nil {
					_ = connection.Close()
					t.Fatal("socket remained open after owned process cleanup")
				}
				stdout, err := os.ReadFile(filepath.Join(directory, process.Stdout))
				if err != nil || !bytesContainsActualReady(stdout, process.PID, process.Address) {
					t.Fatal("retained real ready output missing")
				}
				if process.Node == 2 && process.Incarnation == 1 {
					killed = process
				}
				if process.Node == 2 && process.Incarnation == 2 {
					restarted = process
				}
			}
			if len(result.Processes) != 4 || killed == nil || restarted == nil || killed.Stop != "scenario-kill" || killed.Last.Status.Value != 7 || killed.Last.Syncs < 3 || restarted.Ready.Status.Value != 7 || restarted.Ready.Status.Applied < killed.Last.Status.Applied || restarted.Ready.JournalBytes < killed.Last.JournalBytes {
				t.Fatalf("actual durable restart evidence incomplete: killed=%+v restarted=%+v", killed, restarted)
			}
			if len(result.Journals) != 3 {
				t.Fatal("three actual journals were not retained")
			}
			var journalBytes int64
			for _, journal := range result.Journals {
				data, err := os.ReadFile(filepath.Join(directory, journal.Path))
				if err != nil || len(data) <= 8 || string(data[:8]) != "FBRAFTJ1" || int64(len(data)) != journal.Bytes {
					t.Fatal("actual journal bytes/format mismatch")
				}
				hash := sha256.Sum256(data)
				if hex.EncodeToString(hash[:]) != journal.SHA {
					t.Fatal("actual journal digest mismatch")
				}
				journalBytes += journal.Bytes
			}
			write7, read7, write9, read9 := nativeOperation(t, result, "write7"), nativeOperation(t, result, "read7"), nativeOperation(t, result, "write9"), nativeOperation(t, result, "read9")
			dead := nativeOperation(t, result, "dead-read")
			if write7.Return == nil || read7.Return == nil || read7.Value != 7 || *write7.Return >= read7.Call || write9.Return == nil || read9.Return == nil || read9.Value != 9 || *write9.Return >= read9.Call || dead.Return != nil {
				t.Fatalf("native acknowledged/history conformance failed: %+v", result.History)
			}
			for _, process := range result.Processes {
				if process.Stop == "run-cleanup" && process.Last.Status.Value != 9 {
					t.Fatal("retry reverted later durable value")
				}
			}
			t.Logf("AC-019 actual same-test-binary processes=%d distinct-pids=%d socket-closure=%v http-posts=%d journal-bytes=%d configured-concurrency=%d observed-peak=%d duration-ms=%d source=%s binary=%s", len(result.Processes), len(pids), result.CleanupComplete, result.Transport.Delivered, journalBytes, concurrency, result.PeakConcurrency, result.DurationMillis, result.Identity.SourceTreeSHA, result.BinarySHA)
		})
	}
}
func bytesContainsActualReady(data []byte, pid int, address string) bool {
	for _, line := range strings.Split(string(data), "\n") {
		var record struct {
			Type    string `json:"type"`
			PID     int    `json:"pid"`
			Address string `json:"address"`
		}
		if json.Unmarshal([]byte(line), &record) == nil && record.Type == "ready" && record.PID == pid && record.Address == address {
			return true
		}
	}
	return false
}

func TestRealProcessRejectsUnsupportedFaultAndExternalExecutableBeforeLaunch(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := scenario.Default(0)
	s.Actions = []scenario.Action{{ID: "partition", At: 10, Kind: "partition", From: scenario.Node(1), To: scenario.Node(2)}}
	output := filepath.Join(t.TempDir(), "uncreated")
	result, err := labreal.Run(context.Background(), s, labreal.Options{Executable: executable, Directory: output, Identity: buildinfo.Current(), Concurrency: 1})
	if err != labreal.ErrUnsupportedAction || len(result.Processes) != 0 {
		t.Fatal("unsupported real fault launched children")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("unsupported fault created output")
	}
	s = scenario.Default(0)
	copyPath := filepath.Join(t.TempDir(), "external.exe")
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, data, 0700); err != nil {
		t.Fatal(err)
	}
	result, err = labreal.Run(context.Background(), s, labreal.Options{Executable: copyPath, Directory: output, Identity: buildinfo.Current(), Concurrency: 1})
	if err != labreal.ErrUnsafePath || len(result.Processes) != 0 {
		t.Fatal("operator-supplied external executable was launched")
	}
}

func TestRealProcessCancellationKeepsUnknownAndCleansOwnedChildren(t *testing.T) {
	directory := realArtifactDirectory(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := scenario.Default(7)
	// Killing two voters prevents quorum. Queued writes remain possible effects;
	// parent cancellation cannot turn them into successful or failed calls.
	s.Actions = []scenario.Action{
		{ID: "kill-one", At: 30, Kind: "crash", Node: scenario.Node(1)},
		{ID: "kill-three", At: 31, Kind: "crash", Node: scenario.Node(3)},
		{ID: "uncertain-write", At: 32, Kind: "invoke_write", Node: scenario.Node(2), RequestID: scenario.Text("uncertain"), Value: scenario.Value(7)},
		{ID: "dead-retry", At: 33, Kind: "invoke_write", Node: scenario.Node(1), RequestID: scenario.Text("uncertain"), Value: scenario.Value(7)},
		{ID: "kill-survivor", At: 34, Kind: "crash", Node: scenario.Node(2)}}
	s.EndTime = scenario.Node(100)
	bounded, stopDeadline := context.WithTimeout(context.Background(), 20*time.Second)
	defer stopDeadline()
	ctx, cancel := context.WithCancel(bounded)
	defer cancel()
	// Actual ready records and socket closure establish that all three scheduled
	// crashes, including the one after both request intents, have been reached.
	// Cancellation timing must not depend on child startup taking less than 900ms.
	observed := make(chan bool, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		addresses := make([]string, 3)
		pids := make(map[int]bool)
		for {
			for i := range addresses {
				if addresses[i] != "" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(directory, fmt.Sprintf("node-%d-incarnation-1.stdout.ndjson", i+1)))
				if err != nil {
					continue
				}
				for _, line := range strings.Split(string(data), "\n") {
					var ready struct {
						Type        string `json:"type"`
						Node        uint64 `json:"node_id"`
						Incarnation uint64 `json:"incarnation"`
						PID         int    `json:"pid"`
						Address     string `json:"address"`
					}
					if json.Unmarshal([]byte(line), &ready) != nil || ready.Type != "ready" || ready.Node != uint64(i+1) || ready.Incarnation != 1 || ready.PID <= 0 || pids[ready.PID] {
						continue
					}
					host, _, err := net.SplitHostPort(ready.Address)
					if err == nil && host == "127.0.0.1" {
						addresses[i], pids[ready.PID] = ready.Address, true
						break
					}
				}
			}
			closed := len(pids) == 3
			seenAddresses := make(map[string]bool)
			for _, address := range addresses {
				if address == "" || seenAddresses[address] {
					closed = false
					continue
				}
				seenAddresses[address] = true
				connection, err := net.DialTimeout("tcp4", address, 50*time.Millisecond)
				if err == nil {
					_ = connection.Close()
					closed = false
				}
			}
			if closed {
				observed <- true
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				observed <- false
				return
			case <-ticker.C:
			}
		}
	}()
	result, runErr := labreal.Run(ctx, s, labreal.Options{Executable: executable, Directory: directory, Identity: buildinfo.Current(), Concurrency: 4})
	cancel()
	retainNativeResult(t, directory, result)
	if !<-observed || !errors.Is(runErr, context.Canceled) {
		t.Fatalf("cancellation did not follow actual owned socket closure: %v", runErr)
	}
	if runErr == nil || result.Check.Status != history.Inconclusive || !result.CleanupComplete || len(result.Processes) != 3 {
		t.Fatalf("cancelled run invented success or leaked children: err=%v result=%+v cleanup=%v", runErr, result.Check, result.CleanupComplete)
	}
	operation := nativeOperation(t, result, "uncertain")
	if operation.Return != nil {
		t.Fatal("cancelled minority write was reported completed")
	}
	for _, process := range result.Processes {
		if process.Stop != "scenario-kill" || !process.Waited || !process.SocketClosed {
			t.Fatal("cancellation did not complete owned cleanup")
		}
	}
	t.Logf("cancelled real run status=%s pending=%d waited-processes=%d cleanup=%v", result.Check.Status, len(result.History.Operations), len(result.Processes), result.CleanupComplete)
}
