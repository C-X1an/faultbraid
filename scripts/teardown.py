#!/usr/bin/env python3
"""Remove only flat, tool-named synthetic kernel demos; preserve all evidence."""
import json
import os
from pathlib import Path
import re
import stat
import sys

FILES = {'.faultbraid-owned', 'README.txt', 'valid.json', 'invalid.json', 'pending.json',
         'valid.html', 'invalid.html', 'pending.html', 'disk-trace.json', 'reduced.json'}
NAME = re.compile(r'demo-[0-9]{8}T[0-9]{6}-[0-9a-f]{6}')


def alias(info):
    return stat.S_ISLNK(info.st_mode) or bool(getattr(info, 'st_file_attributes', 0) & 0x400)


def safe_directory(path):
    absolute = Path(os.path.abspath(path))
    for parent in [*reversed(absolute.parents), absolute]:
        info = parent.lstat()
        if alias(info) or not stat.S_ISDIR(info.st_mode):
            raise ValueError('unsafe teardown directory')
    return absolute


def main():
    removed, rejected = 0, 0
    root = Path(__file__).resolve().parents[1] / '.local-runs'
    if not root.exists():
        print(json.dumps({'status': 'PASS', 'removed_synthetic_demos': 0, 'preserved_evidence': True}))
        return 0
    try:
        root = safe_directory(root)
        ready = []
        for candidate in sorted(root.glob('demo-*')):
            marker = candidate / '.faultbraid-owned'
            info = candidate.lstat()
            if alias(info) or not stat.S_ISDIR(info.st_mode):
                rejected += 1
                continue
            if not marker.exists():
                continue
            marker_info = marker.lstat()
            if alias(marker_info) or not stat.S_ISREG(marker_info.st_mode) or marker_info.st_nlink != 1:
                rejected += 1
                continue
            if marker_info.st_size > 32 or marker.read_bytes() not in {b'synthetic-demo-v1\n', b'synthetic-demo-v1\r\n'}:
                continue
            full = safe_directory(candidate)
            if full.parent != root or not NAME.fullmatch(full.name):
                rejected += 1
                continue
            children = list(full.iterdir())
            if any(child.name not in FILES or alias(child.lstat()) or not stat.S_ISREG(child.lstat().st_mode) or child.lstat().st_nlink != 1 for child in children):
                rejected += 1
                continue
            ready.append((full, children))
        # All candidates are validated before deletion. Concurrent hostile
        # operator renames remain outside the trusted local filesystem contract.
        if rejected:
            raise ValueError('unsafe marked demo preserved')
        for full, children in ready:
            if full.parent != root or safe_directory(full) != full:
                raise ValueError('teardown containment changed')
            for child in children:
                info = child.lstat()
                if alias(info) or not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                    raise ValueError('teardown file identity changed')
                child.unlink()
            full.rmdir()
            removed += 1
    except (OSError, ValueError):
        print(json.dumps({'status': 'FAIL', 'removed_synthetic_demos': removed, 'unsafe_candidates_preserved': True, 'preserved_evidence': True}))
        return 1
    print(json.dumps({'status': 'PASS', 'removed_synthetic_demos': removed, 'preserved_evidence': True}))
    return 0


if __name__ == '__main__':
    sys.exit(main())
