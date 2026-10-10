#!/usr/bin/env python3
"""Capture actual bounded command execution. Never pass credentials as arguments."""
from __future__ import annotations
import argparse,datetime,hashlib,json,os,platform,signal,subprocess,sys,time,uuid
from pathlib import Path
IGNORE={'.git','.local-runs','bin','__pycache__','.pytest_cache'}

class _ContainmentError(OSError):
 """Ownership or cleanup could not be established; never release the command."""

class _WindowsAPI:
 """Documented Win32 calls only; HANDLE declarations preserve 64-bit values."""
 def __init__(self):
  import ctypes
  from ctypes import wintypes
  self.ctypes=ctypes;self.wintypes=wintypes
  class IO_COUNTERS(ctypes.Structure):
   _fields_=[('ReadOperationCount',ctypes.c_uint64),('WriteOperationCount',ctypes.c_uint64),('OtherOperationCount',ctypes.c_uint64),('ReadTransferCount',ctypes.c_uint64),('WriteTransferCount',ctypes.c_uint64),('OtherTransferCount',ctypes.c_uint64)]
  class BASIC_LIMITS(ctypes.Structure):
   _fields_=[('PerProcessUserTimeLimit',ctypes.c_int64),('PerJobUserTimeLimit',ctypes.c_int64),('LimitFlags',wintypes.DWORD),('MinimumWorkingSetSize',ctypes.c_size_t),('MaximumWorkingSetSize',ctypes.c_size_t),('ActiveProcessLimit',wintypes.DWORD),('Affinity',ctypes.c_size_t),('PriorityClass',wintypes.DWORD),('SchedulingClass',wintypes.DWORD)]
  class EXTENDED_LIMITS(ctypes.Structure):
   _fields_=[('BasicLimitInformation',BASIC_LIMITS),('IoInfo',IO_COUNTERS),('ProcessMemoryLimit',ctypes.c_size_t),('JobMemoryLimit',ctypes.c_size_t),('PeakProcessMemoryUsed',ctypes.c_size_t),('PeakJobMemoryUsed',ctypes.c_size_t)]
  class ACCOUNTING(ctypes.Structure):
   _fields_=[('TotalUserTime',ctypes.c_int64),('TotalKernelTime',ctypes.c_int64),('ThisPeriodTotalUserTime',ctypes.c_int64),('ThisPeriodTotalKernelTime',ctypes.c_int64),('TotalPageFaultCount',wintypes.DWORD),('TotalProcesses',wintypes.DWORD),('ActiveProcesses',wintypes.DWORD),('TotalTerminatedProcesses',wintypes.DWORD)]
  class STARTUPINFO(ctypes.Structure):
   _fields_=[('cb',wintypes.DWORD),('lpReserved',wintypes.LPWSTR),('lpDesktop',wintypes.LPWSTR),('lpTitle',wintypes.LPWSTR),('dwX',wintypes.DWORD),('dwY',wintypes.DWORD),('dwXSize',wintypes.DWORD),('dwYSize',wintypes.DWORD),('dwXCountChars',wintypes.DWORD),('dwYCountChars',wintypes.DWORD),('dwFillAttribute',wintypes.DWORD),('dwFlags',wintypes.DWORD),('wShowWindow',wintypes.WORD),('cbReserved2',wintypes.WORD),('lpReserved2',ctypes.POINTER(wintypes.BYTE)),('hStdInput',wintypes.HANDLE),('hStdOutput',wintypes.HANDLE),('hStdError',wintypes.HANDLE)]
  class STARTUPINFOEX(ctypes.Structure):
   _fields_=[('StartupInfo',STARTUPINFO),('lpAttributeList',wintypes.LPVOID)]
  class PROCESS_INFORMATION(ctypes.Structure):
   _fields_=[('hProcess',wintypes.HANDLE),('hThread',wintypes.HANDLE),('dwProcessId',wintypes.DWORD),('dwThreadId',wintypes.DWORD)]
  self.EXTENDED_LIMITS=EXTENDED_LIMITS;self.ACCOUNTING=ACCOUNTING
  self.STARTUPINFOEX=STARTUPINFOEX;self.PROCESS_INFORMATION=PROCESS_INFORMATION
  self.kernel32=ctypes.WinDLL('kernel32',use_last_error=True)
  signatures={
   'CreateJobObjectW':([wintypes.LPVOID,wintypes.LPCWSTR],wintypes.HANDLE),
   'SetInformationJobObject':([wintypes.HANDLE,ctypes.c_int,wintypes.LPVOID,wintypes.DWORD],wintypes.BOOL),
   'QueryInformationJobObject':([wintypes.HANDLE,ctypes.c_int,wintypes.LPVOID,wintypes.DWORD,wintypes.LPVOID],wintypes.BOOL),
   'AssignProcessToJobObject':([wintypes.HANDLE,wintypes.HANDLE],wintypes.BOOL),
   'TerminateJobObject':([wintypes.HANDLE,wintypes.UINT],wintypes.BOOL),
   'CloseHandle':([wintypes.HANDLE],wintypes.BOOL),
   'GetCurrentProcess':([],wintypes.HANDLE),
   'DuplicateHandle':([wintypes.HANDLE,wintypes.HANDLE,wintypes.HANDLE,ctypes.POINTER(wintypes.HANDLE),wintypes.DWORD,wintypes.BOOL,wintypes.DWORD],wintypes.BOOL),
   'InitializeProcThreadAttributeList':([wintypes.LPVOID,wintypes.DWORD,wintypes.DWORD,ctypes.POINTER(ctypes.c_size_t)],wintypes.BOOL),
   'UpdateProcThreadAttribute':([wintypes.LPVOID,wintypes.DWORD,ctypes.c_size_t,wintypes.LPVOID,ctypes.c_size_t,wintypes.LPVOID,wintypes.LPVOID],wintypes.BOOL),
   'DeleteProcThreadAttributeList':([wintypes.LPVOID],None),
   'CreateProcessW':([wintypes.LPCWSTR,wintypes.LPWSTR,wintypes.LPVOID,wintypes.LPVOID,wintypes.BOOL,wintypes.DWORD,wintypes.LPVOID,wintypes.LPCWSTR,wintypes.LPVOID,wintypes.LPVOID],wintypes.BOOL),
   'ResumeThread':([wintypes.HANDLE],wintypes.DWORD),
   'TerminateProcess':([wintypes.HANDLE,wintypes.UINT],wintypes.BOOL),
   'WaitForSingleObject':([wintypes.HANDLE,wintypes.DWORD],wintypes.DWORD),
   'GetExitCodeProcess':([wintypes.HANDLE,ctypes.POINTER(wintypes.DWORD)],wintypes.BOOL),
  }
  for name,(arguments,result) in signatures.items():
   function=getattr(self.kernel32,name);function.argtypes=arguments;function.restype=result
 def error(self):return self.ctypes.WinError(self.ctypes.get_last_error())

class _WindowsJob:
 def __init__(self):
  self.handle=None;self.assigned=False
  try:self.api=_WindowsAPI()
  except OSError as error:raise _ContainmentError('could not initialize process containment') from error
  api=self.api;c=api.ctypes;k=api.kernel32
  self.handle=k.CreateJobObjectW(None,None)
  if not self.handle:raise _ContainmentError('could not create process job') from api.error()
  limits=api.EXTENDED_LIMITS();limits.BasicLimitInformation.LimitFlags=0x00002000 # KILL_ON_JOB_CLOSE; no breakaway.
  if not k.SetInformationJobObject(self.handle,9,c.byref(limits),c.sizeof(limits)):
   error=api.error();self.close();raise _ContainmentError('could not configure process job') from error
 def assign(self,proc):
  if not self.api.kernel32.AssignProcessToJobObject(self.handle,proc._handle):
   raise _ContainmentError('could not assign suspended process to job') from self.api.error()
  self.assigned=True
 def terminate(self):
  if self.assigned and not self.api.kernel32.TerminateJobObject(self.handle,1):
   raise _ContainmentError('could not terminate owned job') from self.api.error()
 def wait_empty(self,timeout=10):
  if not self.assigned:return
  api=self.api;c=api.ctypes;deadline=time.monotonic()+timeout
  while True:
   accounting=api.ACCOUNTING()
   if not api.kernel32.QueryInformationJobObject(self.handle,1,c.byref(accounting),c.sizeof(accounting),None):
    raise _ContainmentError('could not inspect owned job') from api.error()
   if accounting.ActiveProcesses==0:return
   if time.monotonic()>=deadline:raise _ContainmentError('owned job did not finish cleanup')
   time.sleep(.01)
 def close(self):
  if self.handle is not None:
   handle=self.handle;self.handle=None
   if not self.api.kernel32.CloseHandle(handle):
    raise _ContainmentError('could not close owned job') from self.api.error()

class _WindowsProcess:
 """Retain the primary thread until a suspended process is safely assigned."""
 def __init__(self,command,cwd,stdout,stderr,env,job):
  import msvcrt
  api=job.api;c=api.ctypes;w=api.wintypes;k=api.kernel32
  self.api=api;self._handle=None;self._thread=None;self.pid=None
  duplicates=[];attributes=None;attribute_ready=False
  try:
   with open(os.devnull,'rb') as stdin:
    for stream in (stdin,stdout,stderr):
     duplicate=w.HANDLE()
     if not k.DuplicateHandle(k.GetCurrentProcess(),w.HANDLE(msvcrt.get_osfhandle(stream.fileno())),k.GetCurrentProcess(),c.byref(duplicate),0,True,2):
      raise _ContainmentError('could not prepare inherited output handles') from api.error()
     duplicates.append(duplicate.value)
    size=c.c_size_t()
    k.InitializeProcThreadAttributeList(None,1,0,c.byref(size))
    if not size.value:raise _ContainmentError('could not size handle inheritance list') from api.error()
    attributes=c.create_string_buffer(size.value)
    if not k.InitializeProcThreadAttributeList(attributes,1,0,c.byref(size)):
     raise _ContainmentError('could not initialize handle inheritance list') from api.error()
    attribute_ready=True;handle_list=(w.HANDLE*len(duplicates))(*duplicates)
    if not k.UpdateProcThreadAttribute(attributes,0,0x00020002,handle_list,c.sizeof(handle_list),None,None):
     raise _ContainmentError('could not restrict inherited handles') from api.error()
    startup=api.STARTUPINFOEX();startup.StartupInfo.cb=c.sizeof(startup)
    startup.StartupInfo.dwFlags=0x00000100 # STARTF_USESTDHANDLES
    startup.StartupInfo.hStdInput,startup.StartupInfo.hStdOutput,startup.StartupInfo.hStdError=duplicates
    startup.lpAttributeList=c.cast(attributes,w.LPVOID)
    info=api.PROCESS_INFORMATION();line=c.create_unicode_buffer(subprocess.list2cmdline(command))
    environment=c.create_unicode_buffer(''.join(key+'='+value+'\0' for key,value in sorted(env.items(),key=lambda item:item[0].casefold()))+'\0')
    # CREATE_SUSPENDED | CREATE_UNICODE_ENVIRONMENT | EXTENDED_STARTUPINFO_PRESENT | CREATE_NO_WINDOW.
    if not k.CreateProcessW(None,line,None,None,True,0x08080404,environment,str(cwd),c.byref(startup),c.byref(info)):
     raise api.error()
    self._handle=info.hProcess;self._thread=info.hThread;self.pid=info.dwProcessId
    try:
     job.assign(self)
     if k.ResumeThread(self._thread)!=1:
      raise _ContainmentError('could not resume owned suspended process') from api.error()
    except OSError as error:
     raise _ContainmentError('suspended process containment failed') from error
  except BaseException:
   # The process is still owned here, including an assignment failure before resume.
   try:
    if self._handle is not None:
     self.kill();self.wait(timeout=10)
   finally:self.close()
   raise
  finally:
   if attribute_ready:k.DeleteProcThreadAttributeList(attributes)
   for handle in duplicates:k.CloseHandle(handle)
  k.CloseHandle(self._thread);self._thread=None
 def poll(self):
  result=self.api.kernel32.WaitForSingleObject(self._handle,0)
  if result==258:return None # WAIT_TIMEOUT, including a live process whose exit code might be 259.
  if result!=0:raise _ContainmentError('could not inspect owned process') from self.api.error()
  code=self.api.wintypes.DWORD()
  if not self.api.kernel32.GetExitCodeProcess(self._handle,self.api.ctypes.byref(code)):
   raise _ContainmentError('could not obtain command exit code') from self.api.error()
  return code.value
 def wait(self,timeout=None):
  milliseconds=0xffffffff if timeout is None else min(int(timeout*1000),0xfffffffe)
  result=self.api.kernel32.WaitForSingleObject(self._handle,milliseconds)
  if result==258:raise _ContainmentError('owned process did not finish cleanup')
  if result!=0:raise _ContainmentError('could not wait for owned process') from self.api.error()
  return self.poll()
 def kill(self):
  if self.poll() is None and not self.api.kernel32.TerminateProcess(self._handle,1):
   raise _ContainmentError('could not terminate owned process') from self.api.error()
 def close(self):
  for name in ('_thread','_handle'):
   handle=getattr(self,name)
   if handle is not None:
    setattr(self,name,None)
    if not self.api.kernel32.CloseHandle(handle):
     raise _ContainmentError('could not close owned process handle') from self.api.error()

def _terminate_owned(proc,job):
 if os.name=='nt':job.terminate()
 elif os.name=='posix':
  try:os.killpg(proc.pid,signal.SIGKILL)
  except ProcessLookupError:pass
 else:proc.kill()

def _cleanup_owned(proc,job):
 # Descendants retain the output handles. Finish their cleanup before hashing files.
 _terminate_owned(proc,job)
 if os.name=='nt':job.wait_empty()
 try:proc.wait(timeout=10)
 except subprocess.TimeoutExpired as error:raise _ContainmentError('owned process did not finish cleanup') from error

def sha(path:Path)->str:
 h=hashlib.sha256()
 with path.open('rb') as f:
  for b in iter(lambda:f.read(65536),b''):h.update(b)
 return h.hexdigest()
def source_identity(root:Path)->dict:
 files=[]
 for p in sorted(root.rglob('*'),key=lambda path:path.relative_to(root).as_posix().encode('utf-8')):
  rel=p.relative_to(root)
  if p.is_symlink() or (hasattr(p,'is_junction') and p.is_junction()):raise ValueError('source tree contains symlink or junction: '+rel.as_posix())
  if any(x in IGNORE for x in rel.parts):continue
  if not p.is_file():continue
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
 if os.name=='nt':
  try:
   powershell=Path(os.environ.get('SystemRoot',r'C:\Windows'))/'System32/WindowsPowerShell/v1.0/powershell.exe'
   query="$probeCpu = @(Get-CimInstance -ClassName Win32_Processor -Property Name -OperationTimeoutSec 5 -ErrorAction Stop | Select-Object -ExpandProperty Name); $probeMemory = (Get-CimInstance -ClassName Win32_ComputerSystem -Property TotalPhysicalMemory -OperationTimeoutSec 5 -ErrorAction Stop).TotalPhysicalMemory; [ordered]@{cpu=($probeCpu -join '; ');host_memory_bytes=[uint64]$probeMemory} | ConvertTo-Json -Compress"
   probe=subprocess.run([str(powershell),'-NoLogo','-NoProfile','-NonInteractive','-Command',query],capture_output=True,text=True,timeout=10)
   if probe.returncode==0:
    hardware=json.loads(probe.stdout)
    if isinstance(hardware.get('cpu'),str) and 0<len(hardware['cpu'].strip())<=512:cpu=hardware['cpu'].strip()
    size=hardware.get('host_memory_bytes')
    if isinstance(size,int) and not isinstance(size,bool) and 0<size<2**64:memory=str(size)+' bytes'
  except (OSError,subprocess.TimeoutExpired,ValueError,AttributeError):pass
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
 proc=None;job=None;status='BLOCKED'
 with stdout.open('xb') as so,stderr.open('xb') as se:
  try:
   if os.name=='nt':
    job=_WindowsJob();proc=_WindowsProcess(command,cwd,so,se,env,job)
   else:
    proc=subprocess.Popen(command,cwd=cwd,stdin=subprocess.DEVNULL,stdout=so,stderr=se,env=env,start_new_session=os.name=='posix')
   while proc.poll() is None:
    if time.monotonic()-clock>timeout or stdout.stat().st_size+stderr.stat().st_size>20*1024*1024:
     reason='timeout' if time.monotonic()-clock>timeout else 'output_limit'
     _terminate_owned(proc,job)
     break
    time.sleep(.02)
   code=proc.wait(timeout=10)
   if stdout.stat().st_size+stderr.stat().st_size>20*1024*1024:reason='output_limit'
   status='PASS' if code==0 and reason=='completed' else 'FAIL'
  except _ContainmentError:
   status='BLOCKED';reason='containment_error';se.write(b'Command containment could not be established or completed.\n')
  except OSError:
   status='BLOCKED';reason='spawn_error';se.write(b'Command could not be started.\n')
  except subprocess.TimeoutExpired:
   status='BLOCKED';reason='containment_error';se.write(b'Owned command cleanup did not complete.\n')
  finally:
   try:
    if proc is not None:_cleanup_owned(proc,job)
   except OSError:
    status='BLOCKED';reason='containment_error';se.write(b'Owned process cleanup could not be completed.\n')
   finally:
    try:
     if proc is not None and os.name=='nt':proc.close()
    except OSError:
     status='BLOCKED';reason='containment_error';se.write(b'Owned process handles could not be closed.\n')
    finally:
     if job is not None:
      try:job.close()
      except OSError:
       status='BLOCKED';reason='containment_error';se.write(b'Owned job cleanup could not be completed.\n')
   if status!='BLOCKED' and stdout.stat().st_size+stderr.stat().st_size>20*1024*1024:
    status='FAIL';reason='output_limit'
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
