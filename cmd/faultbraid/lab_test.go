package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/scenario"
)

func TestNativeCLIRejectsInvalidFlagsBeforeLaunching(t *testing.T) {
	for _, args := range [][]string{
		{}, {"-scenario", "-", "-concurrency", "0"}, {"-scenario", "-", "-concurrency", "17"},
		{"-scenario", "-", "-concurrency", "1", "-in", "-"}, {"-scenario", "-", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runReal(args, strings.NewReader("{}"), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "INVALID_INPUT") {
			t.Fatalf("invalid native flags were accepted: code=%d", code)
		}
	}
	s := scenario.Default(0)
	s.Version = 2
	unsupported, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name, data, code string
	}{
		{"complete-unsupported", string(unsupported), "UNSUPPORTED_VERSION"},
		{"incomplete-supported", "{\"version\":1}", "INVALID_INPUT"},
		{"incomplete-unsupported", "{\"version\":2}", "INVALID_INPUT"},
	} {
		t.Run(input.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			directory := filepath.Join(t.TempDir(), "lab")
			if code := run([]string{"lab-real", "-scenario", "-", "-outdir", directory}, strings.NewReader(input.data), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), input.code) {
				t.Fatalf("native CLI strict schema validation: code=%d stderr=%s", code, stderr.String())
			}
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatalf("invalid scenario created native artifacts: %v", err)
			}
		})
	}
}
