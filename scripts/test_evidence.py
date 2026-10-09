#!/usr/bin/env python3
import hashlib,json,sys,tempfile,unittest
from pathlib import Path
from run_evidence import execute
class EvidenceTests(unittest.TestCase):
 def test_real_success_and_failure(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d);(root/'input.txt').write_text('fixture')
   for expected in (0,7):
    r=execute([sys.executable,'-c',f'import sys;print("actual stdout");print("actual stderr",file=sys.stderr);sys.exit({expected})'],root,root/'.local-runs',timeout=10)
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
if __name__=='__main__':unittest.main()
