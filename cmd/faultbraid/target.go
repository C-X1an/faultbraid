package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/report"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/sim"
	"github.com/C-X1an/faultbraid/internal/strictjson"
)

func inputReader(name string, stdin io.Reader) (io.Reader, func(), error) {
	if name == "-" {
		return stdin, func() {}, nil
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, nil, errors.New("cannot open input")
	}
	return file, func() { _ = file.Close() }, nil
}

func inputFailure(stderr io.Writer, err error) int {
	if errors.Is(err, scenario.ErrUnsupportedVersion) || errors.Is(err, sim.ErrUnsupportedVersion) {
		return fail(stderr, "UNSUPPORTED_VERSION", err.Error())
	}
	return fail(stderr, "INVALID_INPUT", err.Error())
}

func replayInput(ctx context.Context, reader io.Reader, stdout, stderr io.Writer) int {
	data, err := strictjson.ReadValue(reader)
	if err != nil {
		return inputFailure(stderr, err)
	}
	// ReadValue has validated the entire JSON syntax. Inspect only the routing
	// fields here, then require the selected model's full strict typed decoder.
	var envelope struct {
		Model   *string `json:"model"`
		Version *int    `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fail(stderr, "INVALID_INPUT", "trace routing fields do not match schema")
	}
	if envelope.Model == nil || envelope.Version == nil {
		return fail(stderr, "INVALID_INPUT", "trace requires an integer version and string model")
	}
	model := *envelope.Model
	var digest, verdict string
	switch model {
	case "disk-register-v1":
		var trace sim.Trace
		if err := strictjson.Decode(bytes.NewReader(data), &trace); err != nil {
			return inputFailure(stderr, err)
		}
		if err := sim.Replay(trace); err != nil {
			if errors.Is(err, sim.ErrUnsupportedVersion) {
				return inputFailure(stderr, err)
			}
			return fail(stderr, "REPLAY_MISMATCH", err.Error())
		}
		digest = trace.Digest
	case "raft-trace-v1":
		trace, err := scenario.DecodeTrace(bytes.NewReader(data))
		if err != nil {
			return inputFailure(stderr, err)
		}
		if err := scenario.Replay(ctx, trace, buildinfo.Current()); err != nil {
			if ctx.Err() != nil {
				_ = json.NewEncoder(stdout).Encode(map[string]string{"status": "INCONCLUSIVE", "reason": "replay deadline or cancellation"})
				return 3
			}
			return fail(stderr, "REPLAY_MISMATCH", err.Error())
		}
		digest, verdict = trace.Digest, string(trace.Result.Status)
	default:
		return fail(stderr, "UNSUPPORTED_VERSION", "unsupported trace model")
	}
	if err := json.NewEncoder(stdout).Encode(map[string]string{"status": "PASS", "digest": digest, "scope": model, "verdict": verdict}); err != nil {
		return 2
	}
	return 0
}

func writeJSON(path string, value any, limit int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data)+1 > limit {
		return errors.New("artifact exceeds byte limit")
	}
	return report.WriteExclusive(path, append(data, '\n'))
}

func writeTrace(dir string, trace scenario.Trace) error {
	for _, artifact := range []struct {
		name  string
		value any
		limit int
	}{
		{"trace.json", trace, scenario.MaxTraceBytes}, {"scenario.json", trace.Scenario, strictjson.MaxBytes}, {"history.json", trace.History, strictjson.MaxBytes}, {"result.json", trace.Result, strictjson.MaxBytes},
	} {
		if err := writeJSON(filepath.Join(dir, artifact.name), artifact.value, artifact.limit); err != nil {
			return err
		}
	}
	view, err := report.RenderTarget(trace)
	if err != nil {
		return err
	}
	return report.WriteExclusive(filepath.Join(dir, "report.html"), view)
}

func runTarget(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	sub := args[0]
	flags := flag.NewFlagSet(sub, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	scenarioFile := flags.String("scenario", "", "scenario file or - for stdin")
	traceFile := flags.String("trace", "", "trace file or - for stdin")
	dir := flags.String("outdir", "", "new output directory")
	checks := flags.Int("max-checks", 1000, "bounded candidate evaluations")
	strategy := flags.String("strategy", campaign.DDMin, "ddmin or greedy")
	start := flags.Uint64("seed-start", 0, "first campaign seed")
	count := flags.Int("count", 1000, "bounded number of seeds")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return fail(stderr, "INVALID_INPUT", "invalid command flags")
	}
	bad := false
	flags.Visit(func(f *flag.Flag) {
		allowed := f.Name == "outdir" || sub == "simulate" && f.Name == "scenario" || sub == "shrink-scenario" && (f.Name == "trace" || f.Name == "max-checks" || f.Name == "strategy") || sub == "campaign" && (f.Name == "seed-start" || f.Name == "count")
		if !allowed {
			bad = true
		}
	})
	if bad {
		return fail(stderr, "INVALID_INPUT", "flag not supported for this command")
	}
	if *dir == "" {
		*dir = os.Getenv("FAULTBRAID_RUN_DIR")
	}
	if *dir == "" {
		return fail(stderr, "INVALID_INPUT", "outdir is required")
	}
	if sub == "simulate" && *scenarioFile == "" || sub == "shrink-scenario" && *traceFile == "" {
		return fail(stderr, "INVALID_INPUT", "scenario or trace input is required")
	}
	if sub == "shrink-scenario" && (*checks < 1 || *checks > campaign.MaxChecks || *strategy != campaign.DDMin && *strategy != campaign.Greedy) || sub == "campaign" && (*count < 1 || *count > campaign.MaxSeeds || uint64(*count-1) > math.MaxUint64-*start) {
		return fail(stderr, "INVALID_INPUT", "invalid bounded reduction or seed range")
	}
	identity := buildinfo.Current()
	if !identity.Identified() {
		return fail(stderr, "IO_ERROR", "identified build required; use make build")
	}
	deadline := 10 * time.Second
	if sub == "campaign" {
		deadline = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if sub == "campaign" {
		if err := report.NewDirectory(*dir); err != nil {
			return fail(stderr, "IO_ERROR", "output must be a new trusted directory")
		}
		index, runErr := campaign.RunSeeds(ctx, *start, *count, identity, func(entry campaign.RunEntry, trace scenario.Trace) error {
			seedDir := filepath.Join(*dir, fmt.Sprintf("seed-%020d", entry.Seed))
			if err := report.NewDirectory(seedDir); err != nil {
				return err
			}
			if err := writeJSON(filepath.Join(seedDir, "summary.json"), entry, report.MaxReportBytes); err != nil {
				return err
			}
			if entry.Status == "ERROR" {
				return writeJSON(filepath.Join(seedDir, "partial-trace.json"), trace, scenario.MaxTraceBytes)
			}
			// Stream raw JSON for every seed; do not retain all traces in memory.
			if err := writeJSON(filepath.Join(seedDir, "trace.json"), trace, scenario.MaxTraceBytes); err != nil {
				return err
			}
			return writeJSON(filepath.Join(seedDir, "history.json"), trace.History, strictjson.MaxBytes)
		})
		if err := writeJSON(filepath.Join(*dir, "index.json"), index, report.MaxReportBytes); err != nil {
			return fail(stderr, "IO_ERROR", "cannot preserve campaign index")
		}
		if err := json.NewEncoder(stdout).Encode(index); err != nil {
			return 2
		}
		if runErr != nil {
			return fail(stderr, "IO_ERROR", "campaign execution or artifact failure; index records all attempted seeds")
		}
		if index.Status != "COMPLETE" || index.Inconclusive != 0 {
			return 3
		}
		return 0
	}
	name := *scenarioFile
	if sub == "shrink-scenario" {
		name = *traceFile
	}
	reader, closeInput, err := inputReader(name, stdin)
	if err != nil {
		return fail(stderr, "IO_ERROR", err.Error())
	}
	defer closeInput()
	if sub == "simulate" {
		s, err := scenario.Decode(reader)
		if err != nil {
			return inputFailure(stderr, err)
		}
		trace, err := scenario.Run(ctx, s, scenario.Options{Identity: identity})
		if err != nil {
			return fail(stderr, "INVALID_INPUT", "scenario could not execute under the declared target and causal schedule")
		}
		if err := report.NewDirectory(*dir); err != nil {
			return fail(stderr, "IO_ERROR", "output must be a new trusted directory")
		}
		if err := writeTrace(*dir, trace); err != nil {
			return fail(stderr, "IO_ERROR", "cannot preserve trace artifacts")
		}
		summary := struct {
			Status    string          `json:"status"`
			Verdict   history.Verdict `json:"verdict"`
			Digest    string          `json:"digest"`
			Scope     string          `json:"scope"`
			Directory string          `json:"directory"`
		}{"PASS", trace.Result.Status, trace.Digest, trace.Model, *dir}
		if err := writeJSON(filepath.Join(*dir, "summary.json"), summary, strictjson.MaxBytes); err != nil {
			return fail(stderr, "IO_ERROR", "cannot preserve run summary")
		}
		if err := json.NewEncoder(stdout).Encode(summary); err != nil {
			return 2
		}
		if trace.Result.Status == history.Inconclusive {
			return 3
		}
		return 0
	}
	trace, err := scenario.DecodeTrace(reader)
	if err != nil {
		return inputFailure(stderr, err)
	}
	reduction, err := campaign.Reduce(ctx, trace, campaign.Options{Identity: identity, MaxChecks: *checks, Strategy: *strategy})
	if err != nil {
		return fail(stderr, "INVALID_INPUT", "scenario reduction requires an executable same-source INVALID baseline")
	}
	if err := report.NewDirectory(*dir); err != nil {
		return fail(stderr, "IO_ERROR", "output must be a new trusted directory")
	}
	if err := writeTrace(*dir, reduction.Trace); err != nil {
		return fail(stderr, "IO_ERROR", "cannot preserve reduced trace artifacts")
	}
	if err := writeJSON(filepath.Join(*dir, "reduction.json"), reduction, report.MaxReportBytes); err != nil {
		return fail(stderr, "IO_ERROR", "cannot preserve reduction accounting")
	}
	if err := json.NewEncoder(stdout).Encode(reduction); err != nil {
		return 2
	}
	if reduction.Status == history.Inconclusive {
		return 3
	}
	return 0
}
