#!/usr/bin/env python3
"""Resolve and record a real vulnerability tool revision, then run it."""
import json,subprocess,sys
from pathlib import Path
from run_evidence import execute
root=Path(__file__).resolve().parents[1];out=root/'.local-runs/security'
if subprocess.run([sys.executable,str(root/'scripts/supported_toolchain.py')]).returncode:sys.exit(1)
r=execute(['go','list','-m','-json','golang.org/x/vuln@latest'],root,out,'security-tool-resolution',timeout=60)
if r['status']!='PASS':sys.exit('BLOCKED/FAIL: cannot resolve vulnerability tool; no scan success claimed')
metadata=json.loads(Path(r['stdout']['path']).read_text());version=metadata['Version']
if not version.startswith('v') or metadata.get('Path')!='golang.org/x/vuln':sys.exit('unexpected tool module metadata')
(out/'resolved-tool.json').write_text(json.dumps(metadata,indent=2)+'\n')
r=execute(['go','run','golang.org/x/vuln/cmd/govulncheck@'+version,'./...'],root,out,'govulncheck',timeout=600)
sys.exit(0 if r['status']=='PASS' else 1)
