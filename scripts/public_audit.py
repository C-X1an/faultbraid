#!/usr/bin/env python3
"""Audit reviewed working/index files, complete reachable Git history and ZIP bytes.

Heuristic detection is not exhaustive. Manual content review is mandatory.
Ignored runtime trees are not exported; their root entries must not be aliases.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import unicodedata
import zipfile

MAX_FILE = 8 << 20
MAX_TOTAL = 256 << 20
MAX_FILES = 10000
IGNORED = {'.git', '.local-runs', 'bin', '__pycache__', '.pytest_cache'}
FORBIDDEN = {'private', 'runtime', '.codex', 'node_modules', '.venv'}
PRIVATE_NAMES = re.compile(r'(?i)(resume|curriculum.?vitae|portfolio_profile|codex_master_prompt|human_action_required|candidate_(?:scores|research)|research_report|raw_session|session_log)')
RESERVED = re.compile(r'(?i)^(?:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)')
PATTERNS = [
    re.compile(r'\b(?:AKIA|ASIA)[0-9A-Z]{16}\b'),
    re.compile(r'\bgh[pousr]_[A-Za-z0-9]{30,}'),
    re.compile(r'\bgithub_pat_[A-Za-z0-9_]{40,}'),
    re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),
    re.compile(r'\bsk-proj-[A-Za-z0-9_-]{40,}'),
    re.compile(r'\bAIza[0-9A-Za-z_-]{35}\b'),
    re.compile(r'\bxox[baprs]-[A-Za-z0-9-]{20,}'),
    re.compile(r'(?im)^\s*#{1,6}\s*curriculum\s+vitae\b'),
    re.compile(r'(?im)^\s*(?:portfolio_profile|candidate_scores)\s*:'),
    re.compile(r'(?im)^\s*#{1,6}\s*[^\n]*autonomous implementation contract\b'),
    re.compile(r'(?s)"role"\s*:\s*"(?:system|assistant|developer)"\s*,\s*"content"\s*:'),
    re.compile(r'(?i)\b[A-Z]:(?:\\+|/+)Users(?:\\+|/+)[^\s"<>]+'),
]


def alias(info):
    return stat.S_ISLNK(info.st_mode) or bool(getattr(info, 'st_file_attributes', 0) & 0x400)


def safe_components(path):
    absolute = Path(os.path.abspath(path))
    for component in [*reversed(absolute.parents), absolute]:
        try:
            info = component.lstat()
        except FileNotFoundError:
            continue
        if alias(info):
            raise ValueError('aliased filesystem component')
    return absolute


def public_name(name):
    if not name or len(name.encode('utf-8')) > 1024 or unicodedata.normalize('NFC', name) != name:
        return False
    if '\\' in name or ':' in name or any(ord(c) < 32 or ord(c) == 127 for c in name):
        return False
    pieces = name.split('/')
    if any(not p or p in {'.', '..'} or p[-1:] in {' ', '.'} or p != p.strip() or RESERVED.search(p) for p in pieces):
        return False
    if PurePosixPath(name).is_absolute() or any(p.casefold() in FORBIDDEN | IGNORED for p in pieces):
        return False
    leaf = pieces[-1]
    return not PRIVATE_NAMES.search(leaf) and not (leaf.casefold().startswith('.env') and leaf != '.env.example')


class Audit:
    def __init__(self, root):
        self.root = safe_components(root)
        self.errors, self.files, self.expected, self.scanned = [], {}, set(), set()
        self.total = 0
        self.history = {'commits': 0, 'blobs': 0, 'tags': 0, 'ref_objects': 0, 'index_entries': 0, 'complete': False}

    def reject(self, code, location=''):
        # Detected content, malformed names and host paths are never echoed.
        if len(self.errors) < 100:
            self.errors.append({'code': code, 'location_sha256': hashlib.sha256(location.encode()).hexdigest()[:16] if location else ''})

    def content(self, data, location, counted=True):
        if len(data) > MAX_FILE:
            self.reject('oversize_file', location)
            return
        if counted:
            self.total += len(data)
            if self.total > MAX_TOTAL:
                raise ValueError('aggregate audit byte limit exceeded')
        try:
            text = data.decode('utf-8')
        except UnicodeError:
            self.reject('non_utf8_public_content', location)
            return
        if '\x00' in text:
            self.reject('binary_public_content', location)
        if any(pattern.search(text) for pattern in PATTERNS):
            self.reject('sensitive_content_pattern', location)

    def inventory(self):
        inventory = self.root / 'PUBLIC_FILES.txt'
        info = inventory.lstat()
        if alias(info) or not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > 1 << 20:
            raise ValueError('unsafe public inventory')
        raw = inventory.read_bytes()
        names = [line for line in raw.decode('utf-8').splitlines() if line and not line.startswith('#')]
        if len(names) > MAX_FILES:
            raise ValueError('inventory entry limit exceeded')
        seen = set()
        for name in names:
            if not public_name(name) or name.casefold() in seen:
                self.reject('invalid_or_duplicate_inventory_entry')
            seen.add(name.casefold())
        self.expected = set(names)
        if 'PUBLIC_FILES.txt' not in self.expected:
            self.reject('inventory_does_not_allowlist_itself')
        self.inventory_sha = hashlib.sha256(raw).hexdigest()

    def working(self):
        def walk(directory):
            with os.scandir(directory) as entries:
                for entry in entries:
                    path = Path(entry.path)
                    rel = path.relative_to(self.root).as_posix()
                    info = path.lstat()
                    if alias(info) or stat.S_ISREG(info.st_mode) and info.st_nlink != 1:
                        self.reject('filesystem_alias_or_hardlink', rel)
                        continue
                    if entry.name in IGNORED:
                        continue
                    if stat.S_ISDIR(info.st_mode):
                        if entry.name.casefold() in FORBIDDEN:
                            self.reject('prohibited_directory', rel)
                            continue
                        walk(path)
                        continue
                    if not stat.S_ISREG(info.st_mode) or not public_name(rel):
                        self.reject('prohibited_path_or_file_type', rel)
                        continue
                    if len(self.files) >= MAX_FILES or info.st_size > MAX_FILE:
                        self.reject('working_tree_resource_limit', rel)
                        continue
                    data = path.read_bytes()
                    self.files[rel] = data
                    self.content(data, rel)
        walk(self.root)
        if set(self.files) != self.expected:
            for name in sorted(set(self.files) - self.expected):
                self.reject('file_not_allowlisted', name)
            for name in sorted(self.expected - set(self.files)):
                self.reject('allowlisted_file_missing', name)

    def git(self, *args):
        result = subprocess.run(['git', '--no-replace-objects', '-c', 'core.quotePath=false', '-C', str(self.root), *args], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
        if result.returncode or len(result.stdout) > 64 << 20:
            raise ValueError('Git history inspection failed or exceeded bound')
        return result.stdout

    def blob(self, oid, size):
        if oid in self.scanned:
            return
        if len(self.scanned) >= MAX_FILES:
            raise ValueError('historical blob count limit exceeded')
        self.scanned.add(oid)
        self.history['blobs'] += 1
        if size > MAX_FILE:
            self.reject('oversize_historical_blob', oid)
            return
        data = self.git('cat-file', 'blob', oid)
        if len(data) != size:
            raise ValueError('Git blob size changed during inspection')
        self.content(data, oid)

    def git_history(self):
        top = self.git('rev-parse', '--show-toplevel').decode('utf-8').strip()
        # Git may expand a Windows short-name TEMP ancestor. Compare actual
        # filesystem identity after root alias validation, not path spelling.
        if not os.path.samefile(top, self.root):
            raise ValueError('public root is not this repository top level')
        # --no-replace-objects does not disable legacy graft ancestry. Refuse
        # both the effective graft file and its environment override before
        # trusting rev-list as a complete reachable-history enumeration.
        if 'GIT_GRAFT_FILE' in os.environ:
            raise ValueError('Git graft environment override is not auditable')
        graft = Path(self.git('rev-parse', '--git-path', 'info/grafts').decode('utf-8').strip())
        if not graft.is_absolute():
            graft = self.root / graft
        graft = safe_components(graft)
        if os.path.lexists(graft):
            raise ValueError('legacy grafted Git history is not complete')
        if self.git('rev-parse', '--is-shallow-repository').strip() != b'false':
            raise ValueError('incomplete shallow Git history')
        commits = self.git('rev-list', '--all').decode('ascii').splitlines()
        if not commits or len(commits) > MAX_FILES:
            raise ValueError('missing or excessive reachable Git history')
        self.history['commits'] = len(commits)
        for commit in commits:
            if not re.fullmatch(r'[0-9a-f]{40,64}', commit):
                raise ValueError('invalid Git object identifier')
            self.content(self.git('cat-file', 'commit', commit), commit)
            for row in self.git('ls-tree', '-r', '-l', '-z', commit).split(b'\0'):
                if not row:
                    continue
                metadata, name_bytes = row.split(b'\t', 1)
                mode, kind, oid, size = metadata.split()
                name = name_bytes.decode('utf-8')
                if not public_name(name) or mode not in {b'100644', b'100755'} or kind != b'blob':
                    self.reject('prohibited_historical_path_or_mode', name)
                    continue
                self.blob(oid.decode('ascii'), int(size))
        for row in self.git('ls-files', '--stage', '-z').split(b'\0'):
            if not row:
                continue
            metadata, name_bytes = row.split(b'\t', 1)
            mode, oid, stage = metadata.split()
            name = name_bytes.decode('utf-8')
            self.history['index_entries'] += 1
            if not public_name(name) or mode not in {b'100644', b'100755'} or stage != b'0' or name not in self.expected:
                self.reject('unapproved_or_conflicted_index_entry', name)
                continue
            self.blob(oid.decode('ascii'), int(self.git('cat-file', '-s', oid.decode('ascii'))))
        # Annotated tags can retain other tags or blobs without a direct ref
        # to their targets. Inspect the complete target chain rather than only
        # each ref's outer annotation. Unsupported tree refs fail closed.
        pending = [row.split() for row in self.git('for-each-ref', '--format=%(objecttype) %(objectname)').decode('ascii').splitlines()]
        if len(pending) > MAX_FILES:
            raise ValueError('reachable ref count limit exceeded')
        seen, commit_set = set(), set(commits)
        while pending:
            kind, oid = pending.pop()
            if oid in seen:
                continue
            if len(seen) >= MAX_FILES or not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', oid):
                raise ValueError('reachable ref object bound or identifier invalid')
            seen.add(oid)
            self.history['ref_objects'] += 1
            if self.git('cat-file', '-t', oid).decode('ascii').strip() != kind:
                raise ValueError('reachable ref object type mismatch')
            if kind == 'commit':
                if oid not in commit_set:
                    raise ValueError('reachable commit was not inspected')
            elif kind == 'blob':
                self.blob(oid, int(self.git('cat-file', '-s', oid)))
            elif kind == 'tag':
                size = int(self.git('cat-file', '-s', oid))
                if size > MAX_FILE:
                    self.reject('oversize_reachable_tag', oid)
                    continue
                data = self.git('cat-file', 'tag', oid)
                if len(data) != size:
                    raise ValueError('reachable tag size changed during inspection')
                self.history['tags'] += 1
                self.content(data, oid)
                header = data.split(b'\n\n', 1)[0].splitlines()
                if len(header) < 2 or not header[0].startswith(b'object ') or not header[1].startswith(b'type '):
                    raise ValueError('malformed reachable tag header')
                pending.append([header[1][5:].decode('ascii'), header[0][7:].decode('ascii')])
            else:
                self.reject('unsupported_reachable_ref_type', oid)
        self.history['complete'] = not self.errors

    def archive(self, path):
        path = safe_components(path)
        info = path.lstat()
        if alias(info) or not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > MAX_TOTAL:
            raise ValueError('unsafe or excessive archive')
        seen, total = set(), 0
        with zipfile.ZipFile(path) as archive:
            entries = archive.infolist()
            if len(entries) > MAX_FILES:
                raise ValueError('archive file-count limit exceeded')
            for entry in entries:
                name, mode = entry.filename, entry.external_attr >> 16
                if not public_name(name) or name.casefold() in seen or entry.is_dir() or entry.flag_bits & 1 or stat.S_IFMT(mode) not in {0, stat.S_IFREG}:
                    self.reject('invalid_archive_path_type_or_duplicate')
                    continue
                seen.add(name.casefold())
                total += entry.file_size
                if entry.file_size > MAX_FILE or total > MAX_TOTAL:
                    self.reject('archive_resource_limit')
                    continue
                data = archive.read(entry)
                self.content(data, name, counted=False)
                if name not in self.files or data != self.files[name]:
                    self.reject('archive_payload_differs_from_reviewed_file', name)
        if seen != {name.casefold() for name in self.expected}:
            self.reject('archive_inventory_mismatch')
        return {'status': 'FAIL' if self.errors else 'PASS', 'files': len(seen), 'bytes': info.st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}

    def export(self, path):
        path = safe_components(path)
        if path == self.root or self.root in path.parents:
            raise ValueError('export destination must be outside the public tree')
        with path.open('xb') as output:
            with zipfile.ZipFile(output, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
                for name in sorted(self.expected, key=lambda name: name.encode('utf-8')):
                    entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
                    entry.create_system = 3
                    entry.external_attr = (stat.S_IFREG | 0o644) << 16
                    entry.compress_type = zipfile.ZIP_DEFLATED
                    archive.writestr(entry, self.files[name])
        return self.archive(path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    group = parser.add_mutually_exclusive_group()
    group.add_argument('--archive', type=Path)
    group.add_argument('--export', type=Path)
    args = parser.parse_args()
    audit, archive = None, None
    try:
        audit = Audit(args.root)
        audit.inventory()
        audit.working()
        audit.git_history()
        if args.archive:
            archive = audit.archive(args.archive)
        elif args.export and not audit.errors:
            archive = audit.export(args.export)
    except (OSError, ValueError, UnicodeError, subprocess.SubprocessError, zipfile.BadZipFile, KeyError):
        if audit is None:
            print(json.dumps({'status': 'FAIL', 'errors': [{'code': 'unsafe_audit_root_or_input'}]}))
            return 1
        audit.reject('audit_failed_closed')
    result = {'schema_version': 1, 'status': 'FAIL' if audit.errors else 'PASS', 'files': len(audit.files),
              'inventory_sha256': getattr(audit, 'inventory_sha', ''), 'history': audit.history,
              'errors': audit.errors, 'scope': 'exact reviewed working/index files, complete reachable refs and exact ZIP bytes; heuristic detection is not exhaustive; manual content review required'}
    if archive is not None:
        result['archive'] = archive
    print(json.dumps(result))
    return 1 if audit.errors else 0


if __name__ == '__main__':
    sys.exit(main())
