#!/usr/bin/env python3
"""Execute baseline PASS / intentional semantic FAIL / restored PASS controls."""
import hashlib,json,shutil,sys,tempfile
from pathlib import Path
from run_evidence import execute,source_identity
root=Path(__file__).resolve().parents[1];out=root/'.local-runs/mutation';out.mkdir(parents=True,exist_ok=True)
mutants=[
 ('ignore_real_time','internal/history/check.go','*prev.Return < op.Call','false','./internal/history','TestSequentialViolation'),
 ('drop_pending_effect','internal/history/check.go','next = op.Value','next = value','./internal/history','TestPendingWrite'),
 ('budget_as_valid','internal/history/check.go','r.Status = Inconclusive','r.Status = Valid','./internal/history','TestBudget'),
 ('crash_keeps_volatile','internal/sim/sim.go','d.Volatile = d.Durable','d.Volatile = d.Volatile','./internal/sim','TestCrashDurability'),
 ('trust_recorded_output','internal/sim/sim.go','if !reflect.DeepEqual(want, t) {','if !reflect.DeepEqual(want, t) && false {','./internal/sim','TestReplayReexecutes'),
]
results=[];before=source_identity(root)['source_tree_sha256']
for name,path,old,new,pkg,test in mutants:
 with tempfile.TemporaryDirectory(prefix='faultbraid-mut-') as td:
  work=Path(td)/'source';shutil.copytree(root,work,ignore=shutil.ignore_patterns('.git','.local-runs','bin','__pycache__'))
  f=work/path;original=f.read_text()
  if original.count(old)!=1:sys.exit('mutation site changed; inspect contract rather than silently skipping: '+name)
  cmd=['go','test','-count=1','-run','^'+test+'$',pkg]
  baseline=execute(cmd,work,out/name,'mutation-baseline',timeout=90)
  f.write_text(original.replace(old,new,1));mutated=execute(cmd,work,out/name,'mutation-negative-control',timeout=90)
  diagnostic=Path(mutated['stdout']['path']).read_text()+Path(mutated['stderr']['path']).read_text()
  f.write_text(original);restored=execute(cmd,work,out/name,'mutation-restored',timeout=90)
  killed=baseline['status']=='PASS' and mutated['status']=='FAIL' and ('--- FAIL: '+test) in diagnostic and '[build failed]' not in diagnostic and restored['status']=='PASS' and f.read_text()==original
  results.append({'mutation':name,'test':test,'killed_by_semantic_assertion':killed,'baseline_run':baseline['run_id'],'mutant_run':mutated['run_id'],'restored_run':restored['run_id'],'original_sha256':hashlib.sha256(original.encode()).hexdigest(),'mutant_sha256':hashlib.sha256(original.replace(old,new,1).encode()).hexdigest()})
  print(name, 'PASS/FAIL/PASS' if killed else 'CONTROL FAILED')
after=source_identity(root)['source_tree_sha256'];ok=all(r['killed_by_semantic_assertion'] for r in results) and before==after
(out/'summary.json').write_text(json.dumps({'status':'PASS' if ok else 'FAIL','mutations':results,'original_source_unchanged':before==after,'scope':'five selected semantic mutations; not overall mutation coverage'},indent=2)+'\n')
sys.exit(0 if ok else 1)
