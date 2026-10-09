package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestStdinAndErrorEnvelope(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"check"}, strings.NewReader(`{"version":1,"initial":0,"operations":[]}`), &out, &err)
	if code != 0 || !strings.Contains(out.String(), "VALID") {
		t.Fatalf("stdin %d %s %s", code, &out, &err)
	}
	out.Reset()
	err.Reset()
	code = run([]string{"check"}, strings.NewReader(`{"secret":"do-not-echo"}`), &out, &err)
	if code != 2 || strings.Contains(err.String(), "do-not-echo") {
		t.Fatal("error leaks payload")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write(b []byte) (int, error) { return 0, errors.New("broken output") }
func TestOutputFailureNotSuccess(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"check"}, strings.NewReader(`{"version":1,"initial":0,"operations":[]}`), brokenWriter{}, &stderr); code != 2 {
		t.Fatal("output failure reported success")
	}
}
