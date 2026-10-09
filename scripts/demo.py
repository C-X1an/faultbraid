#!/usr/bin/env python3
import datetime,json,subprocess,uuid
from pathlib import Path
root=Path(__file__).resolve().parents[1];runs=root/'.local-runs';runs.mkdir(exist_ok=True)
name='demo-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S')+'-'+uuid.uuid4().hex[:6];out=runs/name
subprocess.run([str(root/'bin/faultbraid'),'demo','-outdir',str(out)],check=True)
(out/'.faultbraid-owned').write_text('synthetic-demo-v1\n')
print('Open locally:',out/'invalid.html')
