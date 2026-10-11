package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/labreal"
	"github.com/C-X1an/faultbraid/internal/report"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// runReal owns only the built-in current executable and a new operator-selected
// directory. It preserves native outcomes separately from replayable traces.
func runReal(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lab-real", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("scenario", "", "scenario file or - for stdin")
	directory := flags.String("outdir", "", "new native artifact directory")
	concurrency := flags.Int("concurrency", 4, "maximum simultaneous requests, 1 through 16")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *input == "" || *concurrency < 1 || *concurrency > 16 {
		return fail(stderr, "INVALID_INPUT", "invalid lab-real flags")
	}
	if *directory == "" {
		*directory = os.Getenv("FAULTBRAID_RUN_DIR")
	}
	if *directory == "" {
		return fail(stderr, "INVALID_INPUT", "outdir is required")
	}
	reader, closeInput, err := inputReader(*input, stdin)
	if err != nil {
		return fail(stderr, "IO_ERROR", "cannot open scenario")
	}
	defer closeInput()
	s, err := scenario.Decode(reader)
	if err != nil {
		return inputFailure(stderr, err)
	}
	identity := buildinfo.Current()
	if !identity.Identified() {
		return fail(stderr, "IO_ERROR", "identified build required; use make build")
	}
	executable, err := os.Executable()
	if err != nil {
		return fail(stderr, "IO_ERROR", "current executable unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, runErr := labreal.Run(ctx, s, labreal.Options{Executable: executable, Directory: *directory, Identity: identity, Concurrency: *concurrency})
	if info, err := os.Lstat(filepath.Join(*directory, ".faultbraid-lab-owner.json")); result.BinarySHA != "" && err == nil && info.Mode().IsRegular() {
		for _, artifact := range []struct {
			name  string
			value any
		}{
			{"native.json", result}, {"history.json", result.History}, {"result.json", result.Check},
		} {
			if err := writeJSON(filepath.Join(*directory, artifact.name), artifact.value, report.MaxReportBytes); err != nil {
				return fail(stderr, "IO_ERROR", "cannot preserve native artifacts")
			}
		}
		view, err := report.Render(result.History, result.Check)
		if err != nil {
			return fail(stderr, "LIMIT_EXCEEDED", "cannot render bounded native history")
		}
		if err := report.WriteExclusive(filepath.Join(*directory, "history.html"), view); err != nil {
			return fail(stderr, "IO_ERROR", "cannot preserve native history view")
		}
	}
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled) || errors.Is(runErr, labreal.ErrLimit) {
			_ = json.NewEncoder(stdout).Encode(map[string]any{"status": "INCONCLUSIVE", "scope": result.Mode, "cleanup_complete": result.CleanupComplete})
			return 3
		}
		return fail(stderr, "IO_ERROR", "native run could not finish; inspect retained artifacts when present")
	}
	if !result.CleanupComplete {
		return fail(stderr, "IO_ERROR", "native child cleanup incomplete")
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return 2
	}
	if result.Check.Status == history.Inconclusive {
		return 3
	}
	return 0
}
