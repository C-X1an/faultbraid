#!/usr/bin/env python3
"""Measure the identified production CLI's native three-process conformance.

Real OS scheduling is deliberately not replayable. Every run retains its
actual child protocol bytes, loopback addresses, journal hashes and cleanup.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import socket
import sys
import uuid

from run_evidence import execute, sha, source_identity


def main():
    root = Path(__file__).resolve().parents[1]
    source = source_identity(root)
    binary = root / 'bin' / ('faultbraid.exe' if os.name == 'nt' else 'faultbraid')
    build = json.loads((root / '.local-runs/build.json').read_text(encoding='utf-8'))
    if source != build['source'] or sha(binary) != build['binary_sha256']:
        raise ValueError('build identity differs from current inputs; run make build')
    identity = {'source_tree_sha256': source['source_tree_sha256'],
                'module_lock_sha256': build['module_lock_sha256'],
                'patch_sha256': build['patch_sha256']}
    # Identity field names are specified by internal/buildinfo, not inferred
    # from a successful process exit or a fixture's expected values.
    base = root / '.local-runs' / ('real-lab-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + uuid.uuid4().hex[:6])
    base.mkdir(mode=0o700)
    scenario_file = root / 'examples/raft-real.json'
    records, problems = [], []
    for concurrency in (1, 4, 16):
        directory = base / ('concurrency-' + str(concurrency))
        record = execute([str(binary), 'lab-real', '-scenario', str(scenario_file), '-outdir', str(directory), '-concurrency', str(concurrency)], root, base / 'commands', 'production-native-conformance', ['AC-019'], timeout=70)
        entry = {'concurrency': concurrency, 'run_id': record['run_id'], 'status': record['status']}
        records.append(entry)
        if record['status'] != 'PASS':
            problems.append('concurrency-' + str(concurrency) + ': command did not finish successfully')
            continue
        data = (directory / 'native.json').read_bytes()
        if len(data) > 8 << 20:
            raise ValueError('native artifact exceeds bound')
        result = json.loads(data)
        if result['mode'] != 'native-loopback-conformance-v1' or result['identity'] != identity or result['binary_sha256'] != build['binary_sha256']:
            problems.append('native mode or source/binary identity mismatch')
        if result['result']['status'] != 'VALID' or not result['cleanup_complete'] or 'not replayable' not in result['reason']:
            problems.append('native verdict, cleanup or OS scheduling scope missing')
        if result['request_concurrency'] != concurrency or not 1 <= result['observed_peak_request_concurrency'] <= concurrency:
            problems.append('native concurrency measurement outside declared bound')
        processes, pids = result['processes'], set()
        if len(processes) != 4:
            problems.append('native kill/restart did not produce four process lifetimes')
        for process in processes:
            pid = process['pid']
            if pid <= 0 or pid in pids or pid == os.getpid() or not process['waited'] or not process['socket_closed'] or not process['entropy_choices']:
                problems.append('native PID/Wait/socket/entropy evidence missing')
            pids.add(pid)
            if process['identity'] != identity or process['binary_sha256'] != build['binary_sha256']:
                problems.append('child identity differs from production parent')
            host, port = process['loopback_address'].split(':')
            if host != '127.0.0.1' or int(port) == 0:
                raise ValueError('native address is not bounded loopback')
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as connection:
                connection.settimeout(0.2)
                if connection.connect_ex((host, int(port))) == 0:
                    problems.append('native socket remains reachable after cleanup')
        operations = {op['id']: op for op in result['history']['operations']}
        for write, read, value in [('write7', 'read7', 7), ('write9', 'read9', 9)]:
            if operations[write].get('return') is None or operations[read].get('return') is None or operations[read]['value'] != value or operations[write]['return'] >= operations[read]['call']:
                problems.append('native acknowledged write/read order or deduplicated value mismatch')
        if operations['dead-read'].get('return') is not None:
            problems.append('dead-node read unexpectedly completed')
        if len(result['journals']) != 3:
            problems.append('native journals not retained for all three voters')
        for journal in result['journals']:
            path = directory / journal['path']
            if path.parent != directory or len(path.read_bytes()) != journal['bytes'] or sha(path) != journal['sha256'] or path.read_bytes()[:8] != b'FBRAFTJ1':
                problems.append('native journal bytes/hash/format mismatch')
        killed = next(p for p in processes if p['node_id'] == 2 and p['incarnation'] == 1)
        restarted = next(p for p in processes if p['node_id'] == 2 and p['incarnation'] == 2)
        if killed['stop_reason'] != 'scenario-kill' or killed['last_observed_snapshot']['target_status']['Value'] != 7 or restarted['ready_snapshot']['target_status']['Value'] != 7 or killed['last_observed_snapshot']['successful_file_syncs'] < 3:
            problems.append('native durable kill/restart or successful File.Sync evidence incomplete')
        entry.update({'native_sha256': hashlib.sha256(data).hexdigest(), 'native_bytes': len(data),
                      'artifact': (directory / 'native.json').relative_to(root).as_posix(),
                      'duration_milliseconds': result['duration_milliseconds'],
                      'observed_peak_request_concurrency': result['observed_peak_request_concurrency'],
                      'successful_http_message_posts': result['transport']['successful_http_message_posts'],
                      'journal_bytes': sum(j['bytes'] for j in result['journals']),
                      'actual_distinct_pids': len(pids), 'cleanup_complete': result['cleanup_complete']})
    summary = {'schema_version': 1, 'status': 'FAIL' if problems else 'PASS', 'source': source,
               'binary_sha256': build['binary_sha256'], 'runs': records, 'problems': problems,
               'scope': 'identified production executable; native loopback/File.Sync conformance; no OS replay or physical power-loss guarantee'}
    (base / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({key: value for key, value in summary.items() if key != 'source'}))
    return 1 if problems else 0


if __name__ == '__main__':
    sys.exit(main())
