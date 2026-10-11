#!/usr/bin/env python3
"""Execute implemented checks from a fresh committed clone and fresh build cache.

The native host, toolchain and verified module-download cache are shared. This
is not a fresh VM, Linux runtime test or independent full-release approval.
The owned temporary checkout is retained for inspecting its actual artifacts.
"""
import datetime
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import uuid

from run_evidence import execute, source_identity


def main():
    root = Path(__file__).resolve().parents[1]
    dirty = subprocess.check_output(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=root, text=True)
    if dirty.strip():
        sys.exit('Commit reviewed public changes before clean verification')
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    name = 'clean-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + uuid.uuid4().hex[:6]
    out = root / '.local-runs' / name
    out.mkdir(parents=True, mode=0o700)
    temporary = Path(tempfile.mkdtemp(prefix='fb-clean-'))
    work = temporary / 'source'
    # This marker and the precise path stay in local evidence. No arbitrary
    # filesystem deletion is performed, including on failure.
    (temporary / '.faultbraid-clean-owner').write_bytes(b'fresh-checkout-evidence-v1\n')
    clone = execute(['git', '-c', 'core.autocrlf=false', 'clone', '--no-hardlinks', str(root), str(work)], root, out, 'clean-clone', ['AC-020'], timeout=60)
    records = [{'step': 'clone', 'run_id': clone['run_id'], 'status': clone['status']}]
    success = clone['status'] == 'PASS'
    exact_source = False
    if success:
        cloned_commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=work, text=True).strip()
        exact_source = cloned_commit == commit and source_identity(work)['files'] == source_identity(root)['files']
        if not exact_source:
            success = False
            records.append({'step': 'exact-committed-source-bytes', 'status': 'FAIL'})
    old_cache = os.environ.get('GOCACHE')
    os.environ['GOCACHE'] = str(temporary / 'build-cache')
    checks = [('verify-seed', 1200), ('fuzz', 180), ('mutation', 300),
              ('benchmark', 180), ('security-online', 900), ('campaign', 300),
              ('real-lab', 300), ('demo', 180), ('teardown', 60)]
    try:
        for target, timeout in checks:
            if not success:
                records.append({'step': target, 'status': 'NOT RUN'})
                continue
            record = execute(['make', target], work, out, 'clean-' + target, ['AC-020'], timeout=timeout)
            records.append({'step': target, 'run_id': record['run_id'], 'status': record['status']})
            success = record['status'] == 'PASS'
    finally:
        if old_cache is None:
            os.environ.pop('GOCACHE', None)
        else:
            os.environ['GOCACHE'] = old_cache
    summary = {'schema_version': 1, 'status': 'PASS' if success else 'FAIL', 'commit': commit,
               'clean_checkout': clone['status'] == 'PASS' and exact_source, 'fresh_build_cache': True,
               'exact_committed_source_bytes': exact_source,
               'same_host_and_toolchain': True, 'shared_verified_module_cache': True,
               'fresh_virtual_machine': False, 'fresh_checkout_retained': True,
               'implemented_checks_passed': success, 'full_product_verified': False,
               'independent_release_review': 'separate gate', 'steps': records}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n', encoding='utf-8')
    # The location is intentionally local metadata, not console/export content.
    (out / 'retained-checkout.json').write_text(json.dumps({'path': str(work), 'owner_marker': str(temporary / '.faultbraid-clean-owner')}) + '\n', encoding='utf-8')
    print(json.dumps(summary))
    return 0 if success else 1


if __name__ == '__main__':
    sys.exit(main())
