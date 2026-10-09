#!/usr/bin/env python3
"""Run the kernel suite in a fresh local committed checkout, not a fresh VM."""
import json,os,subprocess,sys,tempfile
from pathlib import Path
from run_evidence import execute
root=Path(__file__).resolve().parents[1];out=root/'.local-runs/clean'
try:
 dirty=subprocess.check_output(['git','status','--porcelain','--untracked-files=all'],cwd=root,text=True)
 commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()
except subprocess.CalledProcessError:sys.exit('Commit reviewed public source first; no clean-checkout claim without a commit')
if dirty.strip():sys.exit('Commit reviewed changes before clean verification; ignored run files are allowed')
with tempfile.TemporaryDirectory(prefix='faultbraid-clean-') as td:
 work=Path(td)/'source';r=execute(['git','clone','--no-hardlinks',str(root),str(work)],root,out,'clean-clone',timeout=30)
 if r['status']!='PASS':sys.exit(1)
 # New build cache demonstrates no reliance on old compiled packages. Same host.
 old=os.environ.get('GOCACHE');os.environ['GOCACHE']=str(Path(td)/'gocache')
 try:
  r=execute(['make','verify-seed'],work,out,'clean-kernel-verification',timeout=600)
  demo=execute(['make','demo'],work,out,'clean-demo',timeout=60) if r['status']=='PASS' else None
  teardown=execute(['make','teardown'],work,out,'clean-teardown',timeout=30) if demo is not None else None
  success=r['status']=='PASS' and demo is not None and demo['status']=='PASS' and teardown is not None and teardown['status']=='PASS'

 finally:
  if old is None:os.environ.pop('GOCACHE',None)
  else:os.environ['GOCACHE']=old
 summary={'status':'PASS' if success else 'FAIL','commit':commit,'clean_checkout':True,'fresh_build_cache':True,'same_host_and_toolchain':True,'fresh_virtual_machine':False,'full_target_verified':False,'run_id':r['run_id']}
 out.mkdir(parents=True,exist_ok=True);(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))
 sys.exit(0 if success else 1)
