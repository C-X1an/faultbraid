package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRealCLI(t *testing.T) {
	tmp := t.TempDir()
	binaryName := "faultbraid"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(tmp, binaryName)
	root := "../.."
	build := exec.Command("go", "build", "-o", binary, "./cmd/faultbraid")
	build.Dir = root
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %s %v", b, e)
	}
	call := func(want int, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(binary, args...)
		b, e := cmd.CombinedOutput()
		code := 0
		if e != nil {
			if exit, ok := e.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(e)
			}
		}
		if code != want {
			t.Fatalf("%v exit=%d want%d output%s", args, code, want, b)
		}
		return b
	}
	for _, tt := range []struct {
		name    string
		exit    int
		verdict string
	}{{"valid", 0, "VALID"}, {"invalid", 1, "INVALID"}, {"pending", 0, "VALID"}} {
		b := call(tt.exit, "check", "-in", filepath.Join(root, "examples", tt.name+".json"))
		var m map[string]any
		if e := json.Unmarshal(b, &m); e != nil || m["status"] != tt.verdict {
			t.Fatalf("bad output%s %v", b, e)
		}
	}
	call(3, "check", "-in", filepath.Join(root, "examples/valid.json"), "-max-states", "1")
	minimized := call(3, "minimize", "-in", filepath.Join(root, "examples/invalid.json"), "-max-checks", "1")
	var reduction map[string]any
	if e := json.Unmarshal(minimized, &reduction); e != nil || reduction["status"] != "INCONCLUSIVE" || reduction["one_minimal"] != false {
		t.Fatalf("bad inconclusive reduction %s %v", minimized, e)
	}
	call(2, "check", "-max-states", "nope")
	call(2, "missing")
	call(2, "check", "-out", "ignored.html")
	unsupportedHistory := filepath.Join(tmp, "history-v2.json")
	if e := os.WriteFile(unsupportedHistory, []byte(`{"version":2,"initial":0,"operations":[]}`), 0600); e != nil {
		t.Fatal(e)
	}
	if b := call(2, "check", "-in", unsupportedHistory); !strings.Contains(string(b), "UNSUPPORTED_VERSION") {
		t.Fatalf("wrong history version classification: %s", b)
	}
	dir := filepath.Join(tmp, "demo")
	call(0, "demo", "-outdir", dir)
	call(2, "demo", "-outdir", dir)
	tracePath := filepath.Join(dir, "disk-trace.json")
	call(0, "replay", "-in", tracePath)
	for name, replacement := range map[string]any{"version": float64(2), "model": "other-model"} {
		original, e := os.ReadFile(tracePath)
		if e != nil {
			t.Fatal(e)
		}
		var unsupported map[string]any
		if e = json.Unmarshal(original, &unsupported); e != nil {
			t.Fatal(e)
		}
		unsupported[name] = replacement
		encoded, e := json.Marshal(unsupported)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(tmp, "unsupported-"+name+".json")
		if e = os.WriteFile(path, encoded, 0600); e != nil {
			t.Fatal(e)
		}
		if b := call(2, "replay", "-in", path); !strings.Contains(string(b), "UNSUPPORTED_VERSION") {
			t.Fatalf("wrong trace %s classification: %s", name, b)
		}
	}
	b, e := os.ReadFile(tracePath)
	if e != nil {
		t.Fatal(e)
	}
	var trace map[string]any
	if e = json.Unmarshal(b, &trace); e != nil {
		t.Fatal(e)
	}
	trace["steps"].([]any)[0].(map[string]any)["volatile"] = 999
	b, e = json.Marshal(trace)
	if e != nil {
		t.Fatal(e)
	}
	bad := filepath.Join(tmp, "tampered.json")
	if e = os.WriteFile(bad, b, 0600); e != nil {
		t.Fatal(e)
	}
	call(2, "replay", "-in", bad)
	out := filepath.Join(tmp, "report.html")
	call(0, "report", "-in", filepath.Join(root, "examples/pending.json"), "-out", out)
	call(2, "report", "-in", filepath.Join(root, "examples/pending.json"), "-out", out)
	html, e := os.ReadFile(out)
	if e != nil || !strings.Contains(string(html), "VALID") {
		t.Fatal("report missing real verdict")
	}
	call(0, "minimize", "-in", filepath.Join(root, "examples/invalid.json"))
	call(2, "minimize", "-in", filepath.Join(root, "examples/valid.json"))
}
