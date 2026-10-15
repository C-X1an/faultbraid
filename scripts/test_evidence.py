#!/usr/bin/env python3
import hashlib,json,os,subprocess,sys,tempfile,time,unittest
from pathlib import Path
from unittest import mock
import run_evidence
from run_evidence import execute
class EvidenceTests(unittest.TestCase):
 def test_source_paths_use_utf8_byte_order(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);names=['i0.dat','\u0130.dat','A.dat','a0.dat','delete-\x7f.dat','\U0001f9ea.dat']
   for name in names:(root/name).write_bytes(b'regular fixture\n')
   result=run_evidence.source_identity(root)
   expected=[{'path':name,'sha256':hashlib.sha256(b'regular fixture\n').hexdigest()} for name in sorted(names,key=lambda name:name.encode('utf-8'))]
   self.assertEqual(result['files'],expected)
   self.assertEqual(result['source_tree_sha256'],hashlib.sha256(json.dumps(expected,sort_keys=True,separators=(',',':'),ensure_ascii=True).encode()).hexdigest())

 def test_real_success_and_failure(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);(root/'input.txt').write_bytes(b'fixture')
   for expected in ((0,7,259) if os.name=='nt' else (0,7)):
    code=f'import sys;sys.stdout.buffer.write(b"actual stdout\\n");sys.stderr.buffer.write(b"actual stderr\\n");sys.exit({expected})'
    r=execute([sys.executable,'-c',code],root,root/'.local-runs',timeout=10)
    self.assertEqual(r['exit_code'],expected);self.assertEqual(r['status'],'PASS' if expected==0 else 'FAIL')
    for stream,want in [('stdout',b'actual stdout\n'),('stderr',b'actual stderr\n')]:
     b=Path(r[stream]['path']).read_bytes();self.assertEqual(b,want);self.assertEqual(hashlib.sha256(b).hexdigest(),r[stream]['sha256'])
    self.assertEqual(r['source']['files'],[{'path':'input.txt','sha256':hashlib.sha256(b'fixture').hexdigest()}])
 def test_timeout_is_failure(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);r=execute([sys.executable,'-c','import time;time.sleep(30)'],root,root/'.local-runs',timeout=.1)
   self.assertEqual(r['status'],'FAIL');self.assertEqual(r['reason'],'timeout');self.assertNotEqual(r['exit_code'],0)
 def test_missing_command_is_blocked(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);r=execute(['/nonexistent/faultbraid-program'],root,root/'.local-runs');self.assertEqual(r['status'],'BLOCKED');self.assertIsNone(r['exit_code'])
 def test_source_symlink_is_rejected(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)
   if os.name=='nt':
    target=root/'target';target.mkdir();subprocess.run(['cmd','/c','mklink','/J',str(root/'alias'),str(target)],check=True,capture_output=True)
   else:
    target=root/'input.txt';target.write_bytes(b'fixture');(root/'alias.txt').symlink_to(target)
   with self.assertRaisesRegex(ValueError,'source tree contains symlink or junction'):
    execute([sys.executable,'-c','pass'],root,root/'.local-runs',timeout=10)
 def test_source_ignored_outputs_do_not_change_identity(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);(root/'input.txt').write_bytes(b'actual included fixture')
   before=run_evidence.source_identity(root)
   for name in ('.local-runs/volatile/result.json','bin/tool.exe','nested/__pycache__/cache.pyc','.pytest_cache/output'):
    path=root/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(b'excluded output')
   self.assertEqual(run_evidence.source_identity(root),before)
 def test_source_alias_inside_ignored_outputs_is_rejected(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);target=root/'target';target.mkdir();(target/'payload.txt').write_bytes(b'actual alias fixture')
   parent=root/'.local-runs'/'deep';parent.mkdir(parents=True);alias=parent/'alias'
   if os.name=='nt':subprocess.run(['cmd','/c','mklink','/J',str(alias),str(target)],check=True,capture_output=True)
   else:alias.symlink_to(target,target_is_directory=True)
   with self.assertRaisesRegex(ValueError,'source tree contains symlink or junction'):
    run_evidence.source_identity(root)
 def test_timeout_cleans_descendant_process(self):
  self._check_descendant_cleanup(parent_exits=False)
 def test_normal_success_cleans_descendant_process(self):
  self._check_descendant_cleanup(parent_exits=True)
 def _check_descendant_cleanup(self,parent_exits):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);pid_path=root/'child.pid';ready_path=root/'child.ready'
   child='import pathlib,time;pathlib.Path('+repr(str(ready_path))+').write_bytes(b"ready");time.sleep(30)'
   # Startup is not the behavior under test. The child readiness handshake has
   # a bounded 10s allowance; the runner's 15s deadline still has to kill the
   # live descendant, with identical timeout/exit/cleanup assertions below.
   parent='import pathlib,subprocess,sys,time\nchild=subprocess.Popen([sys.executable,"-c",'+repr(child)+'])\npathlib.Path('+repr(str(pid_path))+').write_bytes((str(child.pid)+"\\n").encode())\ndeadline=time.monotonic()+10\nwhile not pathlib.Path('+repr(str(ready_path))+').exists() and time.monotonic()<deadline:time.sleep(.01)\nif not pathlib.Path('+repr(str(ready_path))+').exists():raise RuntimeError("descendant did not start")\n'+('sys.exit(0)' if parent_exits else 'time.sleep(30)')
   r=execute([sys.executable,'-c',parent],root,root/'.local-runs',timeout=15)
   self.assertEqual(r['status'],'PASS' if parent_exits else 'FAIL')
   self.assertEqual(r['reason'],'completed' if parent_exits else 'timeout')
   child_pid=int(pid_path.read_bytes().strip())
   try:
    self.assertEqual(ready_path.read_bytes(),b'ready','descendant never executed its fixture')
    self.assertTrue(_wait_dead(child_pid),'command left descendant running')
    print(json.dumps({'fixture':'descendant_cleanup','parent_exits':parent_exits,'child_pid':child_pid,'child_started':True,'child_alive_after_cleanup':False,'run_status':r['status'],'run_reason':r['reason']}))
   finally:_stop_fixture_process(child_pid)
 @unittest.skipUnless(os.name=='nt','Win32 suspended-process ownership boundary')
 def test_job_assignment_failure_cannot_execute_command(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);marker=root/'executed.txt';child_pid_path=root/'child.pid';owned=[]
   child='import time;time.sleep(30)'
   parent='import pathlib,subprocess,sys;pathlib.Path('+repr(str(marker))+').write_text("executed");child=subprocess.Popen([sys.executable,"-c",'+repr(child)+']);pathlib.Path('+repr(str(child_pid_path))+').write_text(str(child.pid))'
   def reject_assignment(job,proc):
    owned.append(proc.pid)
    # Give a wrongly unsuspended command time to run; the real process must stay suspended.
    time.sleep(.2)
    raise OSError('intentional job-assignment failure')
   with mock.patch.object(run_evidence._WindowsJob,'assign',reject_assignment):
    r=execute([sys.executable,'-c',parent],root,root/'.local-runs',timeout=10)
   self.assertEqual(r['status'],'BLOCKED');self.assertEqual(r['reason'],'containment_error');self.assertIsNone(r['exit_code'])
   self.assertEqual(len(owned),1,'negative control did not create a real suspended process')
   try:
    self.assertTrue(_wait_dead(owned[0]),'unassigned suspended process survived')
    self.assertFalse(marker.exists(),'command executed before ownership was established')
    self.assertFalse(child_pid_path.exists(),'unassigned command spawned a child')
    self.assertEqual(hashlib.sha256(Path(r['stderr']['path']).read_bytes()).hexdigest(),r['stderr']['sha256'])
    print(json.dumps({'fixture':'assignment_failure','suspended_pid':owned[0],'process_alive_after_cleanup':False,'command_executed':False,'run_status':r['status'],'run_reason':r['reason']}))
   finally:
    _stop_fixture_process(owned[0])
    if child_pid_path.exists():_stop_fixture_process(int(child_pid_path.read_text()))

def _process_alive(pid:int)->bool:
 if os.name!='nt':
  stat=Path('/proc')/str(pid)/'stat'
  try:
   state=stat.read_text().rsplit(')',1)[1].strip().split()[0]
   if state=='Z':return False
  except (OSError,IndexError):pass
  try:os.kill(pid,0);return True
  except ProcessLookupError:return False
  except PermissionError:return True
 import ctypes
 from ctypes import wintypes
 kernel32=ctypes.WinDLL('kernel32',use_last_error=True)
 kernel32.OpenProcess.argtypes=[wintypes.DWORD,wintypes.BOOL,wintypes.DWORD];kernel32.OpenProcess.restype=wintypes.HANDLE
 kernel32.GetExitCodeProcess.argtypes=[wintypes.HANDLE,ctypes.POINTER(wintypes.DWORD)];kernel32.GetExitCodeProcess.restype=wintypes.BOOL
 kernel32.CloseHandle.argtypes=[wintypes.HANDLE];kernel32.CloseHandle.restype=wintypes.BOOL
 handle=kernel32.OpenProcess(0x1000,False,pid)
 if not handle:
  error=ctypes.get_last_error()
  if error in (87,1168):return False # ERROR_INVALID_PARAMETER / ERROR_NOT_FOUND.
  raise ctypes.WinError(error)
 try:
  code=wintypes.DWORD()
  if not kernel32.GetExitCodeProcess(handle,ctypes.byref(code)):raise ctypes.WinError(ctypes.get_last_error())
  return code.value==259
 finally:kernel32.CloseHandle(handle)
def _wait_dead(pid):
 deadline=time.monotonic()+5
 while _process_alive(pid) and time.monotonic()<deadline:time.sleep(.02)
 return not _process_alive(pid)
def _stop_fixture_process(pid):
 # Test failure cleanup is restricted to the PID created by this fixture.
 if not _process_alive(pid):return
 try:os.kill(pid,9)
 except ProcessLookupError:pass
if __name__=='__main__':unittest.main()
