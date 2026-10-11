package report

import (
	"errors"
	"html/template"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

const targetPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>FaultBraid - Raft trace inspection</title>
<style>body{font:17px/1.6 system-ui,sans-serif;max-width:90rem;margin:auto;padding:clamp(1rem,4vw,2rem)}h1{font-size:2.4rem;line-height:1.2}table{border-collapse:collapse;width:100%;min-width:44rem;overflow-wrap:anywhere}#actions table{min-width:56rem}#events table{min-width:80rem}th,td{padding:.65rem;text-align:left;vertical-align:top;border-bottom:1px solid}th{white-space:nowrap}code{font-size:.85em;overflow-wrap:anywhere}a:focus-visible,.scroll:focus-visible{outline:3px solid;outline-offset:3px}small{font-size:.85em}.status{font-size:1.5rem;font-weight:700}section{margin-block:2rem}.scroll{overflow:auto;max-height:65vh}caption{text-align:left;font-weight:600;padding:.5rem 0}</style></head>
<body><header><p>FAULTBRAID / SIMULATED INSTRUMENTED RAFT</p><h1>Follow the cause of a failure.</h1><p class="status">{{.Result.Status}}</p><p>{{.Result.Reason}}</p><nav aria-label="Trace sections"><a href="#identity">Reproduction</a> · <a href="#history">History</a> · <a href="#actions">Actions</a> · <a href="#events">Causal records</a></nav></header>
<main><section><h2>What this result means</h2><p>This execution uses the pinned three-voter instrumented Raft target, explicit election entropy, virtual transport and modeled durability. Its register history is checked within the declared budget. INVALID is a counterexample to this history; VALID is a legal witness for this execution; INCONCLUSIVE leaves the question open. These are finite experiments, without a physical power-loss or exhaustive correctness claim.</p><p>This inspector displays saved data. Re-execute <code>replay -in trace.json</code> with the same source build to verify it. Hashes establish consistency, not signatures.</p><p>Tables scroll horizontally; long tables also scroll vertically. Use Tab to focus a table and arrow keys to inspect it.</p>{{if .Failure}}<p>Invariant: <code>{{.Failure.Requirement}}</code>; target: <code>{{.Failure.Variant}}</code>; kind: <code>{{.Failure.Kind}}</code>; fingerprint: <code>{{.Failure.Fingerprint}}</code>.</p>{{end}}</section>
<section id="identity"><h2>Reproduction identity</h2><p>Seed <code>{{.Header.Seed}}</code>; target variant <code>{{.Header.Variant}}</code>; scheduler <code>{{.Header.Scheduler}}</code>; entropy <code>{{.Header.Entropy}}</code>.</p><dl><dt>Source tree SHA256</dt><dd><code>{{.Header.SourceSHA}}</code></dd><dt>Module lock SHA256</dt><dd><code>{{.Header.ModuleSHA}}</code></dd><dt>Instrumented patch SHA256</dt><dd><code>{{.Header.PatchSHA}}</code></dd><dt>Canonical logical trace SHA256</dt><dd><code>{{.Digest}}</code></dd></dl><p>Recorded choices: {{len .Choices}}. Causal records: {{len .Events}}. Checker states: {{.Result.States}}. Legal witness: {{range .Result.Witness}}<code>{{.}}</code> {{else}}none{{end}}.</p></section>
<section id="history"><h2>Logical request history</h2><p>Call/return values are centrally assigned event ordinals. Pending writes may take effect or be omitted. Explicit rejected-only attempts are preserved in causal records and excluded from semantic history; an earlier uncertain admission remains pending across later rejection.</p><div class="scroll" tabindex="0" role="region" aria-label="History table"><table><caption>Integer register initially {{.History.Initial}}</caption><thead><tr><th scope="col">Request ID</th><th scope="col">Kind</th><th scope="col">Value</th><th scope="col">Call</th><th scope="col">Return</th></tr></thead><tbody>{{range .History.Operations}}<tr><td>{{.ID}}</td><td>{{.Kind}}</td><td>{{.Value}}</td><td>{{.Call}}</td><td>{{if .Return}}{{.Return}}{{else}}pending{{end}}</td></tr>{{end}}</tbody></table></div></section>
<section id="actions"><h2>Executable scenario actions</h2><p>Virtual time orders actions; insertion order resolves ties. Request retries retain the same ID and payload.</p><div class="scroll" tabindex="0" role="region" aria-label="Action table"><table><caption>Scenario version {{.Scenario.Version}}</caption><thead><tr><th scope="col">Action</th><th scope="col">Time</th><th scope="col">Kind</th><th scope="col">Node / link</th><th scope="col">Request / value / boundary</th></tr></thead><tbody>{{range .Scenario.Actions}}<tr><td>{{.ID}}</td><td>{{.At}}</td><td>{{.Kind}}</td><td>{{if .Node}}{{.Node}}{{end}}{{if .From}}{{.From}} → {{.To}}{{end}}</td><td>{{if .RequestID}}{{.RequestID}}{{end}} {{if .Value}}{{.Value}}{{end}} {{if .Boundary}}{{.Boundary}} occurrence {{.Occurrence}}{{end}}{{if .Ticks}}delay {{.Ticks}}{{end}}{{if .Count}}count {{.Count}}{{end}}</td></tr>{{end}}</tbody></table></div></section>
<section id="events"><h2>Causal records</h2><p>Parent IDs refer to earlier records. Node incarnation changes on restart. Target transitions hash their typed input, released effects and resulting state; details list the reached durability/response barriers. Replay reconstructs those bytes instead of trusting saved state.</p><div class="scroll" tabindex="0" role="region" aria-label="Causal record table"><table><caption>Ordered logical execution</caption><thead><tr><th scope="col">Event</th><th scope="col">Parent</th><th scope="col">Time</th><th scope="col">Node / incarnation</th><th scope="col">Kind</th><th scope="col">Detail</th><th scope="col">Payload SHA256</th></tr></thead><tbody>{{range .Events}}<tr><td>{{.ID}}</td><td>{{.Parent}}</td><td>{{.At}}</td><td>{{.Node}} / {{.Incarnation}}</td><td>{{.Kind}}</td><td>{{.Detail}}</td><td><code>{{.PayloadSHA}}</code></td></tr>{{end}}</tbody></table></div></section></main><footer><small>Offline report. No scripts, external resources or data uploads. JSON is the portable source of truth.</small></footer></body></html>`

var targetTemplate = template.Must(template.New("target").Parse(targetPage))

func RenderTarget(trace scenario.Trace) ([]byte, error) {
	if trace.Version != 1 || trace.Model != "raft-trace-v1" {
		return nil, scenario.ErrUnsupportedVersion
	}
	if err := trace.Scenario.Validate(); err != nil {
		return nil, err
	}
	if err := trace.History.Validate(); err != nil {
		return nil, err
	}
	if trace.Result.Status != history.Valid && trace.Result.Status != history.Invalid && trace.Result.Status != history.Inconclusive {
		return nil, errors.New("invalid report verdict")
	}
	if len(trace.Events) > 100000 || len(trace.Choices) > 100000 || len(trace.Result.Witness) > history.MaxOperations {
		return nil, errors.New("target report input exceeds limit")
	}
	for _, s := range []string{trace.Digest, trace.Header.SourceSHA, trace.Header.ModuleSHA, trace.Header.PatchSHA, trace.Header.Variant, trace.Header.Scheduler, trace.Header.Entropy, trace.Result.Reason} {
		if len(s) > 1024 {
			return nil, errors.New("target report string exceeds limit")
		}
	}
	if trace.Failure != nil {
		for _, s := range []string{trace.Failure.Requirement, trace.Failure.Variant, trace.Failure.Kind, trace.Failure.Fingerprint} {
			if len(s) > 1024 {
				return nil, errors.New("target report failure string exceeds limit")
			}
		}
	}
	for _, event := range trace.Events {
		if len(event.Kind) > 128 || len(event.Detail) > 1024 || len(event.PayloadSHA) > 64 {
			return nil, errors.New("target report event exceeds limit")
		}
	}
	for _, id := range trace.Result.Witness {
		if len(id) > 64 {
			return nil, errors.New("target report witness exceeds limit")
		}
	}
	var output boundedBuffer
	if err := targetTemplate.Execute(&output, trace); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
