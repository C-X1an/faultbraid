package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/report"
	"github.com/C-X1an/faultbraid/internal/sim"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
func fail(w io.Writer, code, message string) int {
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
	return 2
}
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return fail(stderr, "INVALID_INPUT", "expected check, minimize, replay, report or demo")
	}
	sub := args[0]
	fs := flag.NewFlagSet(sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	in := fs.String("in", "-", "input file or - for stdin")
	max := fs.Int("max-states", history.DefaultMaxStates, "search budget")
	checks := fs.Int("max-checks", 1000, "reduction budget")
	out := fs.String("out", "", "exclusive report output file")
	dir := fs.String("outdir", "", "new demo directory")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return fail(stderr, "INVALID_INPUT", "invalid command flags")
	}
	if sub != "check" && sub != "minimize" && sub != "replay" && sub != "report" && sub != "demo" {
		return fail(stderr, "INVALID_INPUT", "unsupported command")
	}
	// Reject flags irrelevant to the selected command rather than silently ignore.
	bad := false
	fs.Visit(func(f *flag.Flag) {
		allowed := f.Name == "in" && sub != "demo" || f.Name == "max-states" && (sub == "check" || sub == "minimize" || sub == "report") || f.Name == "max-checks" && sub == "minimize" || f.Name == "out" && sub == "report" || f.Name == "outdir" && sub == "demo"
		if !allowed {
			bad = true
		}
	})
	if bad {
		return fail(stderr, "INVALID_INPUT", "flag not supported for this command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if sub == "demo" {
		if *dir == "" {
			return fail(stderr, "INVALID_INPUT", "outdir is required")
		}
		if err := demo(ctx, *dir); err != nil {
			return fail(stderr, "IO_ERROR", "demo failed; output must be a new trusted directory")
		}
		if err := json.NewEncoder(stdout).Encode(map[string]string{"status": "PASS", "scope": "local kernel, not distributed target", "directory": *dir}); err != nil {
			return 2
		}
		return 0
	}
	var reader io.Reader = stdin
	if *in != "-" {
		f, err := os.Open(*in)
		if err != nil {
			return fail(stderr, "IO_ERROR", "cannot open input")
		}
		defer f.Close()
		reader = f
	}
	if sub == "replay" {
		var t sim.Trace
		if err := strictjson.Decode(reader, &t); err != nil {
			return fail(stderr, "INVALID_INPUT", err.Error())
		}
		if err := sim.Replay(t); err != nil {
			return fail(stderr, "REPLAY_MISMATCH", err.Error())
		}
		if err := json.NewEncoder(stdout).Encode(map[string]string{"status": "PASS", "digest": t.Digest, "scope": t.Model}); err != nil {
			return 2
		}
		return 0
	}
	h, err := history.Decode(reader)
	if err != nil {
		return fail(stderr, "INVALID_INPUT", err.Error())
	}
	opts := history.Options{MaxStates: *max}
	if sub == "minimize" {
		r, err := history.Minimize(ctx, h, opts, *checks)
		if err != nil {
			return fail(stderr, "INVALID_INPUT", err.Error())
		}
		if err := json.NewEncoder(stdout).Encode(r); err != nil {
			return 2
		}
		return 0
	}
	r, err := history.Check(ctx, h, opts)
	if err != nil {
		return fail(stderr, "INVALID_INPUT", err.Error())
	}
	if sub == "report" {
		if *out == "" {
			return fail(stderr, "INVALID_INPUT", "out is required")
		}
		b, err := report.Render(h, r)
		if err != nil {
			return fail(stderr, "LIMIT_EXCEEDED", err.Error())
		}
		if err := report.WriteExclusive(*out, b); err != nil {
			return fail(stderr, "IO_ERROR", "cannot exclusively create output in trusted directory")
		}
		if err := json.NewEncoder(stdout).Encode(map[string]string{"status": "PASS", "verdict": string(r.Status), "output": *out}); err != nil {
			return 2
		}
		if r.Status == history.Inconclusive {
			return 3
		}
		return 0
	}
	if err := json.NewEncoder(stdout).Encode(r); err != nil {
		return 2
	}
	switch r.Status {
	case history.Valid:
		return 0
	case history.Invalid:
		return 1
	default:
		return 3
	}
}
func ret(n int64) *int64 { return &n }
func demo(ctx context.Context, dir string) error {
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	histories := map[string]history.History{
		"valid":   {Version: 1, Initial: 0, Operations: []history.Operation{{ID: "w1", Kind: "write", Value: 7, Call: 1, Return: ret(4)}, {ID: "r1", Kind: "read", Value: 0, Call: 2, Return: ret(3)}}},
		"invalid": {Version: 1, Initial: 0, Operations: []history.Operation{{ID: "w1", Kind: "write", Value: 7, Call: 1, Return: ret(2)}, {ID: "r1", Kind: "read", Value: 0, Call: 3, Return: ret(4)}}},
		"pending": {Version: 1, Initial: 0, Operations: []history.Operation{{ID: "w1", Kind: "write", Value: 7, Call: 1}, {ID: "r1", Kind: "read", Value: 7, Call: 2, Return: ret(3)}}},
	}
	for _, name := range []string{"valid", "invalid", "pending"} {
		h := histories[name]
		r, err := history.Check(ctx, h, history.Options{MaxStates: 100000})
		if err != nil {
			return err
		}
		expected := history.Valid
		if name == "invalid" {
			expected = history.Invalid
		}
		if r.Status != expected {
			return errors.New("demo invariant failed")
		}
		b, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return err
		}
		if err := report.WriteExclusive(filepath.Join(dir, name+".json"), append(b, '\n')); err != nil {
			return err
		}
		html, err := report.Render(h, r)
		if err != nil {
			return err
		}
		if err := report.WriteExclusive(filepath.Join(dir, name+".html"), html); err != nil {
			return err
		}
	}
	events := []sim.Event{{At: 1, Kind: "write", Value: 7}, {At: 2, Kind: "crash"}, {At: 3, Kind: "read"}, {At: 4, Kind: "write", Value: 9}, {At: 5, Kind: "sync"}, {At: 6, Kind: "crash"}, {At: 7, Kind: "read"}}
	trace, err := sim.Run(events)
	if err != nil {
		return err
	}
	if err := sim.Replay(trace); err != nil {
		return err
	}
	b, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return err
	}
	if err := report.WriteExclusive(filepath.Join(dir, "disk-trace.json"), append(b, '\n')); err != nil {
		return err
	}
	reduced, err := history.Minimize(ctx, histories["invalid"], history.Options{MaxStates: 100000}, 1000)
	if err != nil {
		return err
	}
	b, err = json.MarshalIndent(reduced, "", "  ")
	if err != nil {
		return err
	}
	if err := report.WriteExclusive(filepath.Join(dir, "reduced.json"), append(b, '\n')); err != nil {
		return err
	}
	summary := bytes.NewBufferString("FaultBraid local kernel demo\n\nVALID overlap; INVALID stale read; VALID pending write.\nModeled crash reads: 0 then 9. Full distributed target: NOT IMPLEMENTED.\n")
	if _, err := fmt.Fprintf(summary, "Trace digest: %s\n", trace.Digest); err != nil {
		return err
	}
	return report.WriteExclusive(filepath.Join(dir, "README.txt"), summary.Bytes())
}
