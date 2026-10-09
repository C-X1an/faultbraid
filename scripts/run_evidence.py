#!/usr/bin/env python3
"""Capture actual bounded command execution. Never pass credentials as arguments."""
from __future__ import annotations
import argparse,datetime,hashlib,json,os,platform,signal,subprocess,sys,time,uuid
from pathlib import Path
IGNORE={'.git','.local-runs','bin','__pycache__','.pytest_cache'}
def sha(path:Path)->str:
 h=hashlib.sha256()
 with path.open('rb') as f:
  for b in iter(lambda:f.read(65536),b''):h.update(b)
 return h.hexdigest()
def source_identity(root:Path)->dict:
 files=[]
 for p in sorted(root.rglob('*')):
  rel=p.relative_to(root)
  if any(x in IGNORE for x in rel.parts) or p.is_symlink() or not p.is_file():continue
  files.append({'path':rel.as_posix(),'sha256':sha(p)})
 tree=hashlib.sha256(json.dumps(files,sort_keys=True,separators=(',',':')).encode()).hexdigest()
 def git(*args):
  try:
   r=subprocess.run(['git','-C',str(root),*args],capture_output=True,text=True,timeout=5)
   return r.stdout.strip() if r.returncode==0 else None
  except (OSError,subprocess.TimeoutExpired):return None
 commit=git('rev-parse','HEAD');state=git('status','--porcelain','--untracked-files=all')
 return {'git_commit':commit,'git_dirty':state!='' if state is not None else True,'source_tree_sha256':tree,'files':files}
def fingerprint()->dict:
 cpu='unknown';memory='unknown'
 try:
  for line in Path('/proc/cpuinfo').read_text().splitlines():
   if line.startswith('model name'):cpu=line.split(':',1)[1].strip();break
  for line in Path('/proc/meminfo').read_text().splitlines():
   if line.startswith('MemTotal:'):memory=line.split(':',1)[1].strip();break
 except OSError:pass
 try:
  r=subprocess.run(['go','version'],capture_output=True,text=True,timeout=10)
  go=r.stdout.strip() if r.returncode==0 else 'unavailable'
 except (OSError,subprocess.TimeoutExpired):go='unavailable'
 return {'os':platform.system(),'release':platform.release(),'architecture':platform.machine(),'python':platform.python_version(),'go':go,'cpu':cpu,'logical_cpus':os.cpu_count(),'host_memory':memory,'container_limits':'not inferred from host totals'}
def execute(command:list[str],cwd:Path,outdir:Path,category:str='verification',requirements:list[str]|None=None,timeout:float=120)->dict:
 if not command or not 0<timeout<=1800:raise ValueError('invalid command or timeout')
 if any(a.startswith(('--password','--token','--api-key')) for a in command):raise ValueError('credential arguments must not be logged')
 cwd=cwd.resolve();outdir=outdir.resolve();outdir.mkdir(parents=True,exist_ok=True)
 identity=source_identity(cwd);runtime=fingerprint()
 run_id=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S')+'-'+uuid.uuid4().hex[:10]
 folder=outdir/run_id;folder.mkdir(mode=0o700)
 stdout=folder/'stdout.txt';stderr=folder/'stderr.txt';env=os.environ.copy()
 for key in list(env):
  if key in {'OPENAI_API_KEY','CODEX_API_KEY','GH_TOKEN','GITHUB_TOKEN','AWS_ACCESS_KEY_ID','AWS_SECRET_ACCESS_KEY','AWS_SESSION_TOKEN'}:env.pop(key)
 start=datetime.datetime.now(datetime.timezone.utc).isoformat();clock=time.monotonic();reason='completed';code=None
 with stdout.open('xb') as so,stderr.open('xb') as se:
  try:
   proc=subprocess.Popen(command,cwd=cwd,stdout=so,stderr=se,env=env,start_new_session=os.name=='posix')
   while proc.poll() is None:
    if time.monotonic()-clock>timeout or stdout.stat().st_size+stderr.stat().st_size>20*1024*1024:
     reason='timeout' if time.monotonic()-clock>timeout else 'output_limit'
     try:
      if os.name=='posix':os.killpg(proc.pid,signal.SIGKILL)
      else:proc.kill()
     except ProcessLookupError:pass
     break
    time.sleep(.02)
   code=proc.wait()
   if stdout.stat().st_size+stderr.stat().st_size>20*1024*1024:reason='output_limit'
   status='PASS' if code==0 and reason=='completed' else 'FAIL'
  except OSError:
   status='BLOCKED';reason='spawn_error';se.write(b'Command could not be started.\n')
 end=datetime.datetime.now(datetime.timezone.utc).isoformat()
 record={'schema_version':1,'run_id':run_id,'category':category,'requirements':requirements or [],'command':command,'cwd':str(cwd),'started_utc':start,'ended_utc':end,'duration_seconds':time.monotonic()-clock,'status':status,'reason':reason,'exit_code':code,'environment':runtime,'source':identity,'stdout':{'path':str(stdout),'sha256':sha(stdout),'bytes':stdout.stat().st_size},'stderr':{'path':str(stderr),'sha256':sha(stderr),'bytes':stderr.stat().st_size}}
 (folder/'run.json').write_text(json.dumps(record,indent=2)+'\n')
 # Single orchestrator writes this ledger; parallel workers use distinct ledgers.
 with (outdir/'run_manifest.jsonl').open('a') as f:f.write(json.dumps(record,sort_keys=True)+'\n')
 return record
if __name__=='__main__':
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--cwd',type=Path,default=Path.cwd());p.add_argument('--out',type=Path,default=Path('.local-runs/evidence'));p.add_argument('--category',default='verification');p.add_argument('--requirements',default='');p.add_argument('--timeout',type=float,default=120);p.add_argument('command',nargs=argparse.REMAINDER);a=p.parse_args();cmd=a.command[1:] if a.command[:1]==['--'] else a.command
 try:r=execute(cmd,a.cwd,a.out,a.category,[s for s in a.requirements.split(',') if s],a.timeout)
 except (ValueError,OSError) as e:sys.exit(str(e))
 print(json.dumps({k:r[k] for k in ('run_id','status','exit_code','duration_seconds','reason')}))
 print(r['stdout']['path']);print(r['stderr']['path'])
 sys.exit(0 if r['status']=='PASS' else 125 if r['status']=='BLOCKED' else r['exit_code'] if isinstance(r['exit_code'],int) and 0<r['exit_code']<125 else 1)
