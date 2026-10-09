package report

import (
	"github.com/C-X1an/faultbraid/internal/history"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapesAndExclusiveOutput(t *testing.T) {
	payload := `<script>alert("x")</script>`
	h := history.History{Version: 1, Operations: []history.Operation{{ID: payload, Kind: "read"}}}
	r := history.Result{Status: history.Inconclusive, Reason: payload, Witness: []string{payload}}
	b, e := Render(h, r)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "<script>") || !strings.Contains(string(b), "&lt;script&gt;") {
		t.Fatal("unescaped hostile HTML")
	}
	if !strings.Contains(string(b), `scope="col"`) || !strings.Contains(string(b), "Content-Security-Policy") {
		t.Fatal("missing semantic/access controls")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "view.html")
	if e := WriteExclusive(out, b); e != nil {
		t.Fatal(e)
	}
	if e := WriteExclusive(out, b); e == nil {
		t.Fatal("overwrote output")
	}
	st, e := os.Stat(out)
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("output permissions")
	}
	alias := filepath.Join(dir, "alias.html")
	if e := os.Symlink(out, alias); e != nil {
		t.Fatal(e)
	}
	if e := WriteExclusive(alias, b); e == nil {
		t.Fatal("followed leaf symlink")
	}
	parent := filepath.Join(dir, "alias-dir")
	if e := os.Symlink(dir, parent); e != nil {
		t.Fatal(e)
	}
	if e := WriteExclusive(filepath.Join(parent, "new.html"), b); e == nil {
		t.Fatal("followed parent symlink")
	}
}
func TestReportBounds(t *testing.T) {
	h := history.History{Version: 1}
	r := history.Result{Status: history.Valid, Reason: strings.Repeat("x", 1025)}
	if _, e := Render(h, r); e == nil {
		t.Fatal("unbounded reason")
	}
}
