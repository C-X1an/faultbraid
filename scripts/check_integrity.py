#!/usr/bin/env python3
"""Check pre-implementation acceptance lock and obvious test-integrity hazards."""
import hashlib,json,re,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1]
p=root/'tests/acceptance_locked/spec.json';expected=p.with_suffix('.sha256').read_text().strip()
errors=[]
if hashlib.sha256(p.read_bytes()).hexdigest()!=expected:errors.append('acceptance lock changed; never regenerate to hide a failed criterion')
spec=json.loads(p.read_text())
if spec.get('acceptance_spec_locked') is not True:errors.append('acceptance lock disabled')
for f in root.rglob('*_test.go'):
 if '.local-runs' in f.parts:continue
 text=f.read_text()
 if re.search(r'\b(?:t|b)\.(?:Skip|Skipf|SkipNow)\s*\(',text):errors.append('skip in '+str(f.relative_to(root)))
 if re.search(r'if\s+false\s*\{',text):errors.append('disabled test block in '+str(f.relative_to(root)))
if errors:print('\n'.join(errors),file=sys.stderr);sys.exit(1)
print(json.dumps({'status':'PASS','criteria':len(spec['criteria']),'acceptance_sha256':expected,'scope':'lock and narrow static hazards; not proof that all tests are adequate'}))
