#!/usr/bin/env python3
"""Do not turn kernel-only success into a full release claim."""
import json,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1];spec=json.loads((root/'tests/acceptance_locked/spec.json').read_text());problems=[]
result_path=root/'verification/acceptance_results.json'
results=json.loads(result_path.read_text()) if result_path.exists() else {'criteria':[]}
lookup={r['id']:r for r in results.get('criteria',[])}
for c in spec['criteria']:
 r=lookup.get(c['id'],{})
 if r.get('status')!='PASS':problems.append(c['id']+': '+r.get('status','NOT RUN'))
 if not (root/c['test_path']).exists():problems.append(c['id']+': required test absent '+c['test_path'])
 if not r.get('evidence'):problems.append(c['id']+': no evidence')
review=results.get('independent_review',{})
if review.get('status')!='PASS':problems.append('independent full-project review NOT RUN/PASS')
if results.get('supported_toolchain_verified') is not True:problems.append('supported-toolchain verification missing')
if problems:print('FULL RELEASE BLOCKED\n'+'\n'.join(problems),file=sys.stderr);sys.exit(1)
print('Recorded full-gate prerequisites present; live full verification must still execute.')
