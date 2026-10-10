#!/usr/bin/env python3
import hashlib,json,os,subprocess,sys,tempfile,time,unittest
from pathlib import Path
from run_evidence import execute
class EvidenceTests(unittest.TestCase):
 def test_real_success_and_failure(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);(root/'input.txt').write_bytes(b'fixture')
   for expected in (0,7):
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
 def test_timeout_cleans_descendant_process(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);pid_path=root/'child.pid'
   child='import time;time.sleep(30)'
   parent='import pathlib,subprocess,sys,time;child=subprocess.Popen([sys.executable,"-c",'+repr(child)+']);pathlib.Path('+repr(str(pid_path))+').write_bytes((str(child.pid)+"\\n").encode());time.sleep(30)'
   r=execute([sys.executable,'-c',parent],root,root/'.local-runs',timeout=.5)
   self.assertEqual(r['reason'],'timeout')
   child_pid=int(pid_path.read_bytes().strip())
   deadline=time.monotonic()+5
   while _process_alive(child_pid) and time.monotonic()<deadline:time.sleep(.05)
   self.assertFalse(_process_alive(child_pid),'timed-out command left descendant running')

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
 handle=ctypes.windll.kernel32.OpenProcess(0x1000,False,pid)
 if not handle:return False
 try:
  code=ctypes.c_ulong()
  return bool(ctypes.windll.kernel32.GetExitCodeProcess(handle,ctypes.byref(code))) and code.value==259
 finally:ctypes.windll.kernel32.CloseHandle(handle)
if __name__=='__main__':unittest.main()
