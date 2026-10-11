package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestReplayRoutingDiagnosticsNeverEchoLargeInput(t *testing.T) {
	digits := strings.Repeat("9", 20000)
	for name, input := range map[string]string{
		"version":      `{"model":"raft-trace-v1","version":` + digits + `}`,
		"model":        `{"model":` + digits + `,"version":1}`,
		"model-object": `{"model":{"payload":"` + strings.Repeat("sensitive-fixture-", 1000) + `"},"version":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := replayInput(context.Background(), strings.NewReader(input), &stdout, &stderr)
			var envelope struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if code != 2 || stdout.Len() != 0 || stderr.Len() > 512 || json.Unmarshal(stderr.Bytes(), &envelope) != nil || envelope.Error.Code != "INVALID_INPUT" || strings.Contains(stderr.String(), strings.Repeat("9", 64)) || strings.Contains(stderr.String(), "sensitive-fixture-") {
				t.Fatalf("malformed routing leaked input or lost classification: exit=%d stdout-bytes=%d stderr-bytes=%d", code, stdout.Len(), stderr.Len())
			}
		})
	}
}
