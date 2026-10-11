#!/usr/bin/env python3
"""Execute baseline PASS / intentional semantic FAIL / byte-restored PASS controls."""
import hashlib
import json
import shutil
import sys
import tempfile
from pathlib import Path

from run_evidence import execute, source_identity


MUTANTS = [
    ('ignore_real_time', 'internal/history/check.go', '*prev.Return < op.Call',
     'false', './internal/history', 'TestSequentialViolation', 'stale read accepted'),
    ('drop_pending_effect', 'internal/history/check.go', 'next = op.Value',
     'next = value', './internal/history', 'TestPendingWrite', 'pending write inclusion/omission'),
    ('budget_as_valid', 'internal/history/check.go', 'r.Status = Inconclusive',
     'r.Status = Valid', './internal/history', 'TestBudget', 'budget is not unknown'),
    ('crash_keeps_volatile', 'internal/sim/sim.go', 'd.Volatile = d.Durable',
     'd.Volatile = d.Volatile', './internal/sim', 'TestCrashDurability', 'unflushed write survived'),
    ('trust_recorded_output', 'internal/sim/sim.go', 'if !reflect.DeepEqual(want, t) {',
     'if !reflect.DeepEqual(want, t) && false {', './internal/sim',
     'TestReplayReexecutes', 'tampered output accepted'),
]


def digest(data):
    return hashlib.sha256(data).hexdigest()


def output(record):
    return ''.join(Path(record[stream]['path']).read_bytes().decode('utf-8', errors='replace')
                   for stream in ('stdout', 'stderr'))


def passed_test(record, test):
    return (record['status'] == 'PASS' and record['reason'] == 'completed'
            and record['exit_code'] == 0 and '--- PASS: ' + test in output(record))


def control(root, out, before, mutation):
    name, path, old, new, pkg, test, assertion = mutation
    with tempfile.TemporaryDirectory(prefix='faultbraid-mut-') as td:
        work = Path(td) / 'source'
        shutil.copytree(root, work, ignore=shutil.ignore_patterns(
            '.git', '.local-runs', 'bin', '__pycache__', '.pytest_cache'))
        if source_identity(work)['files'] != before['files']:
            raise ValueError('disposable source copy differs from original: ' + name)
        site = work / path
        original = site.read_bytes()
        old_bytes, new_bytes = old.encode('utf-8'), new.encode('utf-8')
        if original.count(old_bytes) != 1:
            raise ValueError('mutation site changed; inspect contract rather than silently skipping: ' + name)
        mutant = original.replace(old_bytes, new_bytes, 1)
        expected_mutant = [dict(row, sha256=digest(mutant)) if row['path'] == path else row
                           for row in before['files']]
        command = ['go', 'test', '-count=1', '-v', '-run', '^' + test + '$', pkg]
        baseline = execute(command, work, out / name, 'mutation-baseline', timeout=90)
        try:
            site.write_bytes(mutant)
            tested_mutant = site.read_bytes()
            mutated = execute(command, work, out / name, 'mutation-negative-control', timeout=90)
            mutant_source_exact = (tested_mutant == mutant
                                   and mutated['source']['files'] == expected_mutant
                                   and source_identity(work)['files'] == expected_mutant)
        finally:
            site.write_bytes(original)
        restored = execute(command, work, out / name, 'mutation-restored', timeout=90)
        restored_bytes = site.read_bytes()
        exact_restore = (restored_bytes == original
                         and baseline['source']['files'] == before['files']
                         and restored['source']['files'] == before['files']
                         and source_identity(work)['files'] == before['files'])
        diagnostic = output(mutated)
        killed = (passed_test(baseline, test) and mutated['status'] == 'FAIL'
                  and mutated['reason'] == 'completed' and mutated['exit_code'] == 1
                  and '--- FAIL: ' + test in diagnostic and assertion in diagnostic
                  and '[build failed]' not in diagnostic and 'panic:' not in diagnostic
                  and passed_test(restored, test) and mutant_source_exact and exact_restore)
        result = {
            'mutation': name, 'site': path, 'test': test,
            'expected_semantic_assertion': assertion,
            'killed_by_semantic_assertion': killed,
            'baseline_run': baseline['run_id'], 'mutant_run': mutated['run_id'],
            'restored_run': restored['run_id'],
            'original_sha256': digest(original), 'mutant_sha256': digest(tested_mutant),
            'restored_sha256': digest(restored_bytes),
            'baseline_source_sha256': baseline['source']['source_tree_sha256'],
            'mutant_source_sha256': mutated['source']['source_tree_sha256'],
            'restored_source_sha256': restored['source']['source_tree_sha256'],
            'only_declared_site_changed': mutant_source_exact,
            'exact_isolated_source_restored': exact_restore,
        }
        print(name, 'PASS/FAIL/PASS; exact byte restoration' if killed else 'CONTROL FAILED', flush=True)
        return result


def main():
    root = Path(__file__).resolve().parents[1]
    out = root / '.local-runs/mutation'
    out.mkdir(parents=True, exist_ok=True)
    before = source_identity(root)
    results = [control(root, out, before, mutation) for mutation in MUTANTS]
    after = source_identity(root)
    unchanged = before['files'] == after['files']
    ok = all(row['killed_by_semantic_assertion'] for row in results) and unchanged
    summary = {
        'schema_version': 1, 'status': 'PASS' if ok else 'FAIL', 'mutations': results,
        'original_source_unchanged': unchanged,
        'original_source_sha256': before['source_tree_sha256'],
        'final_source_sha256': after['source_tree_sha256'],
        'scope': 'five selected semantic mutations; not overall mutation coverage',
    }
    (out / 'summary.json').write_bytes((json.dumps(summary, indent=2) + '\n').encode('utf-8'))
    return 0 if ok else 1


if __name__ == '__main__':
    sys.exit(main())
