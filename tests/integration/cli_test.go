package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealCLI(t *testing.T) {
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "faultbraid")
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
	call(2, "check", "-max-states", "nope")
	call(2, "missing")
	call(2, "check", "-out", "ignored.html")
	dir := filepath.Join(tmp, "demo")
	call(0, "demo", "-outdir", dir)
	call(2, "demo", "-outdir", dir)
	tracePath := filepath.Join(dir, "disk-trace.json")
	call(0, "replay", "-in", tracePath)
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
