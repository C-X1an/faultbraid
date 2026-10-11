// Package report renders static, escaped reports and exclusively creates files.
package report

import (
	"bytes"
	"errors"
	"github.com/C-X1an/faultbraid/internal/history"
	"html/template"
	"io"
	"os"
	"path/filepath"
)

const MaxReportBytes = 8 << 20
const page = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>FaultBraid - history inspection</title>
<style>body{font:17px/1.6 system-ui,sans-serif;max-width:70rem;margin:auto;padding:clamp(1rem,4vw,2rem)}h1{font-size:2.4rem;line-height:1.2}table{border-collapse:collapse;width:100%;min-width:44rem;overflow-wrap:anywhere}th,td{padding:.65rem;text-align:left;vertical-align:top;border-bottom:1px solid}th{white-space:nowrap}code{font-size:.9em;overflow-wrap:anywhere}a:focus-visible,.scroll:focus-visible{outline:3px solid;outline-offset:3px}small{font-size:.85em}.status{font-size:1.5rem;font-weight:700}section{margin-block:2rem}.scroll{overflow:auto;max-height:65vh}caption{text-align:left;font-weight:600;padding:.5rem 0}</style></head>
<body><header><p>FAULTBRAID / EXPERIMENTAL KERNEL</p><h1>A history you can inspect.</h1><p class="status">{{.Result.Status}}</p><p>{{.Result.Reason}}</p></header>
<main><section aria-labelledby="scope"><h2 id="scope">What this result means</h2><p>This bounded check concerns one integer-register history. It is not a distributed-target test, security guarantee or proof about every possible execution.</p><p>Initial value: <code>{{.History.Initial}}</code>. Visited search states: <code>{{.Result.States}}</code>.</p><p>Legal witness: {{range .Result.Witness}}<code>{{.}}</code> {{else}}none{{end}}. Pending writes may be included or omitted; pending reads have no observed result.</p></section>
<section aria-labelledby="ops"><h2 id="ops">Operation intervals</h2><p>Use Tab to focus the table and arrow keys to scroll it.</p><div class="scroll" tabindex="0" role="region" aria-label="Operation interval table"><table><caption>Input history, version {{.History.Version}}</caption><thead><tr><th scope="col">ID</th><th scope="col">Kind</th><th scope="col">Value</th><th scope="col">Call</th><th scope="col">Return</th></tr></thead><tbody>{{range .History.Operations}}<tr><td>{{.ID}}</td><td>{{.Kind}}</td><td>{{.Value}}</td><td>{{.Call}}</td><td>{{if .Return}}{{.Return}}{{else}}pending{{end}}</td></tr>{{end}}</tbody></table></div></section>
<section><h2>Reproduce, do not infer</h2><p>Use the original JSON and the same source and search budget. INVALID means all admissible orders were exhausted. INCONCLUSIVE means the search stopped without a verdict. Diagnostic history reduction does not establish an executable failure scenario.</p></section></main><footer><small>Local report. No external resources, scripts or data uploads.</small></footer></body></html>`

var tmpl = template.Must(template.New("report").Parse(page))

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > MaxReportBytes-b.Len() {
		return 0, errors.New("report output exceeds limit")
	}
	return b.Buffer.Write(data)
}

func (b *boundedBuffer) WriteString(data string) (int, error) {
	if len(data) > MaxReportBytes-b.Len() {
		return 0, errors.New("report output exceeds limit")
	}
	return b.Buffer.WriteString(data)
}

func Render(h history.History, r history.Result) ([]byte, error) {
	if len(h.Operations) > history.MaxOperations || len(r.Witness) > history.MaxOperations || len(r.Reason) > 1024 {
		return nil, errors.New("report input exceeds limit")
	}
	for _, op := range h.Operations {
		if len(op.ID) > 1024 || len(op.Kind) > 1024 {
			return nil, errors.New("report string exceeds limit")
		}
	}
	for _, id := range r.Witness {
		if len(id) > 1024 {
			return nil, errors.New("report string exceeds limit")
		}
	}
	if r.Status != history.Valid && r.Status != history.Invalid && r.Status != history.Inconclusive {
		return nil, errors.New("invalid report status")
	}
	var b boundedBuffer
	if err := tmpl.Execute(&b, struct {
		History history.History
		Result  history.Result
	}{h, r}); err != nil {
		return nil, err
	}
	if b.Len() > MaxReportBytes {
		return nil, errors.New("report output exceeds limit")
	}
	return b.Bytes(), nil
}

// WriteExclusive never overwrites an existing leaf and rejects existing symlink
// parents. The local operator and filesystem are trusted; this is not a defense
// against a privileged concurrent parent-directory rename race.
func WriteExclusive(path string, data []byte) error {
	if len(data) > MaxReportBytes {
		return errors.New("output exceeds limit")
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := safeParents(full); err != nil {
		return err
	}
	f, err := openExclusive(full)
	if err != nil {
		return err
	}
	if err := protectFile(f); err != nil {
		_ = f.Close()
		_ = os.Remove(full)
		return err
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func safeParents(full string) error {
	for parent := filepath.Dir(full); ; parent = filepath.Dir(parent) {
		st, err := os.Lstat(parent)
		if err != nil {
			return err
		}
		if outputAlias(st) || !st.IsDir() {
			return errors.New("unsafe output parent")
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
	}
	return nil
}

// NewDirectory exclusively creates a trusted operator-selected output directory.
// On Windows its DACL is restricted before any report or trace is written.
func NewDirectory(path string) error {
	full, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := safeParents(full); err != nil {
		return err
	}
	if err := os.Mkdir(full, 0700); err != nil {
		return err
	}
	if err := protectDirectory(full); err != nil {
		_ = os.Remove(full) // Still empty; never recursively remove operator data.
		return err
	}
	return nil
}
