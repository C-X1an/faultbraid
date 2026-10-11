#!/usr/bin/env python3
"""Create inspectable synthetic kernel and actual instrumented-Raft demos."""
import datetime
import json
import os
from pathlib import Path
import sys
import uuid

from run_evidence import execute


def main():
    root = Path(__file__).resolve().parents[1]
    runs = root / '.local-runs'
    runs.mkdir(exist_ok=True)
    suffix = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + uuid.uuid4().hex[:6]
    kernel = Path('.local-runs') / ('demo-' + suffix)
    target = Path('.local-runs') / ('target-demo-' + suffix)
    (root / target).mkdir(mode=0o700)
    binary = root / 'bin' / ('faultbraid.exe' if os.name == 'nt' else 'faultbraid')
    evidence = runs / 'demo-evidence'
    records = []

    def command(args):
        record = execute([str(binary), *map(str, args)], root, evidence, 'synthetic-demo', ['AC-014', 'AC-017'], timeout=30)
        records.append({'run_id': record['run_id'], 'status': record['status']})
        if record['status'] != 'PASS':
            raise ValueError('demo command failed; original evidence retained')
        return json.loads(Path(record['stdout']['path']).read_text(encoding='utf-8'))

    command(['demo', '-outdir', kernel])
    (root / kernel / '.faultbraid-owned').write_bytes(b'synthetic-demo-v1\n')
    for name in ('safe', 'unsafe'):
        result = command(['simulate', '-scenario', 'examples/raft-' + name + '.json', '-outdir', target / name])
        expected = 'VALID' if name == 'safe' else 'INVALID'
        if result['verdict'] != expected:
            raise ValueError('actual demo target produced an unexpected verdict')
    trace = target / 'unsafe' / 'trace.json'
    for _ in range(10):
        if command(['replay', '-in', trace])['verdict'] != 'INVALID':
            raise ValueError('actual unsafe demo replay changed verdict')
    reduction = command(['shrink-scenario', '-trace', trace, '-outdir', target / 'reduced'])
    if reduction['status'] != 'INVALID' or not reduction['one_minimal']:
        raise ValueError('actual causal demo reduction not verified')
    command(['replay', '-in', target / 'reduced' / 'trace.json'])
    print(json.dumps({'status': 'PASS', 'kernel_history_view': (kernel / 'invalid.html').as_posix(),
                      'actual_raft_inspector': (target / 'unsafe' / 'report.html').as_posix(),
                      'reduced_raft_inspector': (target / 'reduced' / 'report.html').as_posix(),
                      'unsafe_replays': 10, 'records': records,
                      'scope': 'synthetic inputs executing the real instrumented target; separate modeled disk kernel'}))
    return 0


if __name__ == '__main__':
    sys.exit(main())
