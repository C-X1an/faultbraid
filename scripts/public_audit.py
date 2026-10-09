#!/usr/bin/env python3
"""Fail closed against an exact public file list and common secret patterns."""
import argparse,hashlib,json,re,sys
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--root',type=Path,default=Path(__file__).resolve().parents[1]);a=p.parse_args();root=a.root.resolve()
ignore={'.git','.local-runs','bin','__pycache__','.pytest_cache'}
forbidden={'private','runtime','.codex','node_modules','.venv'}
actual=set();errors=[]
for f in root.rglob('*'):
 rel=f.relative_to(root)
 if any(x in ignore for x in rel.parts):continue
 if f.is_symlink():errors.append('symlink: '+rel.as_posix());continue
 if not f.is_file():continue
 name=rel.as_posix();actual.add(name)
 if any(x in forbidden for x in rel.parts) or f.name.startswith('.env') and f.name!='.env.example':errors.append('prohibited path: '+name)
 if re.search(r'(?i)(resume|portfolio_profile|codex_master_prompt|human_action_required|candidate_scores|research_report)',f.name):errors.append('private filename: '+name)
 if f.stat().st_size>8*1024*1024:errors.append('oversize public file: '+name);continue
 try:text=f.read_text()
 except UnicodeError:errors.append('unexpected binary: '+name);continue
 patterns=[r'AKIA[0-9A-Z]{16}',r'gh[pousr]_[A-Za-z0-9]{30,}',r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',r'sk-proj-[A-Za-z0-9_-]{40,}']
 for pattern in patterns:
  if re.search(pattern,text):errors.append('possible secret in '+name)
try:expected={line for line in (root/'PUBLIC_FILES.txt').read_text().splitlines() if line and not line.startswith('#')}
except OSError:expected=set();errors.append('missing reviewed PUBLIC_FILES.txt')
for name in sorted(actual-expected):errors.append('not allowlisted: '+name)
for name in sorted(expected-actual):errors.append('allowlisted file missing: '+name)
if errors:print('\n'.join(errors),file=sys.stderr);sys.exit(1)
print(json.dumps({'status':'PASS','files':len(actual),'scope':'exact path inventory and heuristic scan; content/history review and online secret scan still required'}))
