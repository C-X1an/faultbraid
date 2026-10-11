#!/usr/bin/env python3
"""Measure an actual bounded Raft campaign and replay every selected failure.

Full traces stay in ignored local run directories. This script records all seed
outcomes and byte hashes; it does not claim exhaustive fault coverage or OS replay.
"""
from __future__ import annotations
import argparse
import datetime
import json
import os
import sys
import uuid
from pathlib import Path
from run_evidence import execute, sha, source_identity


def read_object(path: Path, limit: int):
    with path.open('rb') as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise ValueError('bounded campaign artifact exceeds limit')
    value = json.loads(data)
    if not isinstance(value, dict):
        raise ValueError('campaign artifact is not an object')
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--seed-start', type=int, default=0)
    parser.add_argument('--count', type=int, default=1000)
    args = parser.parse_args()
    if not 1 <= args.count <= 1000 or not 0 <= args.seed_start <= 2**64 - args.count:
        parser.error('seed range must fit uint64 and count1..1000')
    root = Path(__file__).resolve().parents[1]
    source = source_identity(root)
    binary = root / 'bin' / ('faultbraid.exe' if os.name == 'nt' else 'faultbraid')
    try:
        build = read_object(root / '.local-runs/build.json', 8 << 20)
        if build['source']['source_tree_sha256'] != source['source_tree_sha256'] or build['binary_sha256'] != sha(binary):
            raise ValueError('source or binary changed since the identified build')
        identity = {'source_tree_sha256': source['source_tree_sha256'],
                    'module_lock_sha256': build['module_lock_sha256'], 'patch_sha256': build['patch_sha256']}
    except (OSError, ValueError, KeyError) as error:
        print('Identified current build required: run make build (' + str(error) + ')', file=sys.stderr)
        return 2
    name = 'campaign-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + uuid.uuid4().hex[:6]
    base = root / '.local-runs' / name
    base.mkdir()
    evidence = base / 'commands'
    seed_dir = base / 'seeds'
    record = execute([str(binary), 'campaign', '-seed-start', str(args.seed_start), '-count', str(args.count), '-outdir', str(seed_dir)], root, evidence, 'raft-seed-campaign', timeout=360, requirements=['AC-024', 'PERF-003', 'FR-010'])
    problems = []
    artifacts = []
    selected = []
    index = None
    try:
        index = read_object(seed_dir / 'index.json', 8 << 20)
        if index['identity'] != identity or index['requested'] != args.count or index['seed_start'] != args.seed_start:
            raise ValueError('campaign index differs from measured inputs')
        entries = index['entries']
        if len(entries) != index['attempted'] or index['attempted'] + index['unrun'] != args.count:
            raise ValueError('campaign attempt/unrun accounting differs')
        counted = {status: 0 for status in ('VALID', 'INVALID', 'INCONCLUSIVE', 'ERROR')}
        for ordinal, entry in enumerate(entries):
            if entry['seed'] != args.seed_start + ordinal or entry['status'] not in counted:
                raise ValueError('seed is omitted, reordered or unclassified')
            counted[entry['status']] += 1
            trace_path = seed_dir / f"seed-{entry['seed']:020d}" / ('partial-trace.json' if entry['status'] == 'ERROR' else 'trace.json')
            trace = read_object(trace_path, 1 << 20)
            if entry['status'] != 'ERROR':
                if any(trace['header'].get(key) != value for key, value in identity.items()) or trace['digest'] != entry['digest'] or trace['result']['status'] != entry['status']:
                    raise ValueError('seed index and actual trace differ')
            artifacts.append({'seed': entry['seed'], 'status': entry['status'], 'trace_bytes': trace_path.stat().st_size, 'trace_file_sha256': sha(trace_path), 'digest': entry['digest']})
            if entry['status'] == 'INVALID':
                selected.append((f"seed-{entry['seed']:020d}", trace_path, entry['digest']))
        expected = {status: index[key] for status, key in (('VALID', 'valid'), ('INVALID', 'invalid'), ('INCONCLUSIVE', 'inconclusive'), ('ERROR', 'errors'))}
        if counted != expected:
            raise ValueError('outcome totals differ from actual seed entries')
        if index['status'] != 'COMPLETE' or index['attempted'] != args.count or index['unrun'] != 0 or index['inconclusive'] != 0 or index['errors'] != 0 or index['callback_errors'] != 0:
            problems.append('campaign is incomplete or contains inconclusive/error outcomes; all outcomes retained')
    except (OSError, ValueError, KeyError, TypeError) as error:
        problems.append('seed artifact/accounting validation: ' + str(error))
    if record['status'] != 'PASS':
        problems.append('seed campaign command ' + record['status'])
    # An explicitly unsafe same-target variant supplies a selected failure even
    # when all normal-target seeds are VALID. Its outcome is reported separately.
    unsafe_dir = base / 'selected-unsafe'
    unsafe = execute([str(binary), 'simulate', '-scenario', 'examples/raft-unsafe.json', '-outdir', str(unsafe_dir)], root, evidence, 'explicit-unsafe-variant', timeout=30, requirements=['AC-024', 'AC-002'])
    try:
        trace_path = unsafe_dir / 'trace.json'
        trace = read_object(trace_path, 1 << 20)
        if unsafe['status'] != 'PASS' or trace['result']['status'] != 'INVALID' or trace['failure'] is None or trace['header']['target_variant'] != 'unsafe-local-read':
            raise ValueError('explicit unsafe variant did not produce its actual counterexample')
        selected.append(('explicit-unsafe-local-read', trace_path, trace['digest']))
    except (OSError, ValueError, KeyError, TypeError) as error:
        problems.append('selected unsafe execution: ' + str(error))
    replay_results = []
    for name, trace_path, digest in selected:
        runs = []
        for repetition in range(10):
            replay = execute([str(binary), 'replay', '-in', str(trace_path)], root, evidence, name + '-replay-' + str(repetition), timeout=30, requirements=['AC-024', 'AC-014'])
            actual = None
            try:
                actual = read_object(Path(replay['stdout']['path']), 1 << 20)
                if replay['status'] != 'PASS' or actual.get('digest') != digest or actual.get('status') != 'PASS' or actual.get('verdict') != 'INVALID':
                    raise ValueError('actual replay did not match the selected failure')
            except (OSError, ValueError, KeyError, TypeError) as error:
                problems.append(name + ' replay' + str(repetition) + ': ' + str(error))
            runs.append({'run_id': replay['run_id'], 'command_status': replay['status'], 'digest': actual.get('digest') if isinstance(actual, dict) else None})
        replay_results.append({'selection': name, 'target_digest': digest, 'requested_replays': 10, 'runs': runs})
    summary = {'schema_version': 1, 'status': 'PASS' if not problems else 'FAIL',
        'requirements': ['AC-024', 'PERF-003'], 'full1000_seed_requirement_measured': args.count == 1000,
        'source': source, 'binary_sha256': sha(binary), 'seed_campaign_run_id': record['run_id'],
        'seed_campaign_duration_seconds': record['duration_seconds'], 'seed_index': index,
        'trace_artifacts': artifacts, 'selected_failures': replay_results, 'problems': problems,
        'scope': 'Finite deterministic virtual-time Raft fault campaign. Explicit unsafe variant is separate. No exhaustive correctness or OS timing claim.'}
    (base / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({'status': summary['status'], 'count': args.count,
        'attempted': index.get('attempted') if isinstance(index, dict) else None,
        'valid': index.get('valid') if isinstance(index, dict) else None,
        'invalid': index.get('invalid') if isinstance(index, dict) else None,
        'inconclusive': index.get('inconclusive') if isinstance(index, dict) else None,
        'errors': index.get('errors') if isinstance(index, dict) else None,
        'selected_failures': len(replay_results), 'replay_attempts': sum(len(item['runs']) for item in replay_results),
        'duration_seconds': record['duration_seconds'], 'artifact': str((base / 'summary.json').relative_to(root)), 'problems': problems}))
    return 0 if not problems else 1


if __name__ == '__main__':
    raise SystemExit(main())
