#!/usr/bin/env python3
"""Check inspectable release artifacts; live verification is still required."""
from __future__ import annotations
import datetime
import fnmatch
import hashlib
import json
import math
import os
import re
import stat
import sys
from pathlib import Path, PurePosixPath

IGNORE = {'.git', '.local-runs', 'bin', '__pycache__', '.pytest_cache'}
MAX_FILE = 8 * 1024 * 1024
SHA = re.compile(r'[0-9a-f]{64}\Z')
RUN_ID = re.compile(r'[0-9]{8}T[0-9]{6}-[0-9a-f]{10}\Z')
EXECUTION_ROOT_FILES = {
    'Makefile', '.go-version', '.gitattributes', '.gitignore', 'go.mod',
    'go.sum', 'dependency.lock.json', 'toolchain.lock.json', 'verification/schema.json',
}
MAKE_PACKAGES = {
    'test': ('',), 'race': ('',), 'verify-seed': ('',), 'verify': ('',),
    'unit': ('internal/', 'cmd/'), 'acceptance': ('tests/acceptance_locked/',),
    'integration': ('tests/integration/',), 'system': ('tests/system/',),
    'target': ('tests/system/',), 'differential': ('tests/differential/',),
    'security': ('tests/security/', 'internal/report/', 'internal/strictjson/'),
    'benchmark': ('internal/history/', 'internal/sim/'),
}


class EvidenceError(ValueError):
    """An artifact cannot establish a release prerequisite."""


def _relative(value: object) -> str:
    if not isinstance(value, str) or not value or '\\' in value or ':' in value:
        raise EvidenceError('path must be normalized public-relative text')
    path = PurePosixPath(value)
    if path.is_absolute() or any(part in {'', '.', '..'} for part in value.split('/')):
        raise EvidenceError('path escapes or aliases the public root: ' + value)
    if any(part in IGNORE | {'private', '.codex'} for part in path.parts):
        raise EvidenceError('path refers to an excluded directory: ' + value)
    return value


def _digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _pairs(items: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in items:
        if key in result:
            raise EvidenceError('duplicate JSON field: ' + key)
        result[key] = value
    return result


def _json(data: bytes) -> object:
    if len(data) > MAX_FILE:
        raise EvidenceError('JSON artifact exceeds byte limit')
    def bad_constant(value):
        raise EvidenceError('non-finite JSON number: ' + value)
    try:
        result = json.loads(data.decode('utf-8'), object_pairs_hook=_pairs, parse_constant=bad_constant)
    except (UnicodeError, json.JSONDecodeError, RecursionError) as error:
        raise EvidenceError('malformed JSON artifact') from error
    pending = [(result, 0)]
    while pending:
        item, depth = pending.pop()
        if depth > 20:
            raise EvidenceError('JSON artifact exceeds nesting limit')
        if isinstance(item, dict):
            pending.extend((value, depth + 1) for value in item.values())
        elif isinstance(item, list):
            pending.extend((value, depth + 1) for value in item)
    return result


def execution_path(name: str) -> bool:
    """Version-1 input set; ordinary prose and result indexes are not inputs."""
    parts = PurePosixPath(name).parts
    if name in EXECUTION_ROOT_FILES or parts[0] in {'cmd', 'internal', 'scripts', 'tests', 'examples', '.github'}:
        return True
    if name.startswith('docs/codec/'):
        return not name.endswith('.md')
    if name.startswith('third_party/raft-patch/'):
        return name.endswith(('.py', '.patch', '.json'))
    return name.endswith(('.go', '.py', '.sh', '.ps1', '.c', '.h')) and not name.startswith('verification/')


def _alias(path: Path) -> bool:
    info = path.lstat()
    return stat.S_ISLNK(info.st_mode) or bool(getattr(info, 'st_file_attributes', 0) & getattr(stat, 'FILE_ATTRIBUTE_REPARSE_POINT', 0x400))


def _tool(command: list[str]) -> str:
    return command[0].replace('\\', '/').rsplit('/', 1)[-1].lower().removesuffix('.exe')


def _package_tree(directory: str, selector: str) -> bool:
    prefix = selector.rstrip('/')
    return not prefix or directory == prefix or directory.startswith(prefix + '/')


def _python_args(command: list[str]) -> list[str]:
    if not re.fullmatch(r'python[0-9.]*', _tool(command)):
        return []
    args = command[1:]
    while args and args[0] in {'-u', '-B', '-I', '-E', '-s', '-S'}:
        args = args[1:]
    return args


def _script_command(command: list[str], path: str) -> bool:
    args = _python_args(command)
    return bool(args) and args[0].replace('\\', '/') == path


def _evidence_unittest(command: list[str]) -> bool:
    if _script_command(command, 'scripts/test_evidence.py'):
        return True
    args = _python_args(command)
    if args[:2] != ['-m', 'unittest']:
        return False
    args = [arg for arg in args[2:] if arg not in {'-v', '-q', '--verbose', '--quiet'}]
    if 'discover' not in args:
        return any(arg in {'test_evidence', 'scripts.test_evidence'} or arg.startswith(('test_evidence.', 'scripts.test_evidence.')) for arg in args)
    if args[0] != 'discover':
        return False
    start, pattern = '.', 'test*.py'
    for index, arg in enumerate(args):
        if arg in {'-s', '--start-directory', '-p', '--pattern'}:
            if index + 1 == len(args):
                return False
            if arg in {'-s', '--start-directory'}:
                start = args[index + 1]
            else:
                pattern = args[index + 1]
        elif arg.startswith('--start-directory='):
            start = arg.split('=', 1)[1]
        elif arg.startswith('--pattern='):
            pattern = arg.split('=', 1)[1]
    return start == 'scripts' and fnmatch.fnmatchcase('test_evidence.py', pattern)


class Gate:
    def __init__(self, root: Path):
        self.root = root.absolute()
        if _alias(self.root):
            raise EvidenceError('public root is a filesystem alias')
        self.inventory = set()
        for line in self._read('PUBLIC_FILES.txt').decode('utf-8').splitlines():
            if not line or line.startswith('#'):
                continue
            name = _relative(line)
            if name in self.inventory:
                raise EvidenceError('duplicate public inventory path: ' + name)
            self.inventory.add(name)
        self.current = self._execution_files()
        self.runs: dict[str, tuple[dict, bytes, bytes]] = {}
        self.run_errors: dict[str, str] = {}

    def _owned(self, name: str, allowlisted: bool = False) -> Path:
        name = _relative(name)
        if allowlisted and name not in self.inventory:
            raise EvidenceError('evidence is not in reviewed PUBLIC_FILES.txt: ' + name)
        path = self.root
        for part in PurePosixPath(name).parts:
            path /= part
            if _alias(path):
                raise EvidenceError('filesystem alias rejected: ' + name)
        if not path.is_file() or path.stat().st_nlink != 1:
            raise EvidenceError('artifact must be an owned regular file: ' + name)
        if not path.resolve().is_relative_to(self.root.resolve()):
            raise EvidenceError('artifact is outside public root: ' + name)
        return path

    def _read(self, name: str, allowlisted: bool = False) -> bytes:
        path = self._owned(name, allowlisted)
        if path.stat().st_size > MAX_FILE:
            raise EvidenceError('artifact exceeds byte limit: ' + name)
        return path.read_bytes()

    def _object(self, name: str, allowlisted: bool = False) -> dict:
        value = _json(self._read(name, allowlisted))
        if not isinstance(value, dict):
            raise EvidenceError('artifact must be a JSON object: ' + name)
        return value

    def _execution_files(self) -> dict[str, str]:
        files = {}
        def walk_error(error):
            raise EvidenceError('source tree cannot be fully inspected') from error
        for base, directories, names in os.walk(self.root, followlinks=False, onerror=walk_error):
            directories[:] = sorted(name for name in directories if name not in IGNORE)
            for directory in directories:
                if _alias(Path(base) / directory):
                    raise EvidenceError('source directory is an alias: ' + directory)
            for filename in sorted(names):
                path = Path(base) / filename
                relative = path.relative_to(self.root).as_posix()
                if any(part in IGNORE for part in PurePosixPath(relative).parts):
                    continue
                if _alias(path):
                    raise EvidenceError('source file is an alias: ' + relative)
                if execution_path(relative):
                    files[relative] = _digest(self._read(relative))
        return files

    def _source(self, source: object) -> None:
        if not isinstance(source, dict) or not isinstance(source.get('files'), list):
            raise EvidenceError('recorded source files missing')
        if type(source.get('git_dirty')) is not bool:
            raise EvidenceError('recorded source dirty state missing')
        commit = source.get('git_commit')
        if commit is not None and not re.fullmatch(r'[0-9a-f]{40}|[0-9a-f]{64}', str(commit)):
            raise EvidenceError('invalid recorded commit')
        files = source['files']
        if not files or len(files) > 10000:
            raise EvidenceError('recorded source files missing or excessive')
        recorded = {}
        for item in files:
            if not isinstance(item, dict) or set(item) != {'path', 'sha256'}:
                raise EvidenceError('malformed recorded source file')
            name = _relative(item['path'])
            if name in recorded or not isinstance(item['sha256'], str) or not SHA.fullmatch(item['sha256']):
                raise EvidenceError('duplicate path or invalid source hash: ' + name)
            recorded[name] = item['sha256']
        raw_hash = _digest(json.dumps(files, sort_keys=True, separators=(',', ':')).encode())
        if source.get('source_tree_sha256') != raw_hash:
            raise EvidenceError('recorded source-tree digest mismatch')
        recorded_inputs = {name: digest for name, digest in recorded.items() if execution_path(name)}
        changes = sorted(name for name in set(recorded_inputs) | set(self.current) if recorded_inputs.get(name) != self.current.get(name))
        if changes:
            raise EvidenceError('execution source/input changed: ' + ', '.join(changes[:3]))

    def _run(self, reference: object) -> tuple[dict, bytes, bytes]:
        name = _relative(reference)
        if name in self.run_errors:
            raise EvidenceError(self.run_errors[name])
        if name in self.runs:
            return self.runs[name]
        try:
            parts = PurePosixPath(name).parts
            if len(parts) != 4 or parts[:2] != ('verification', 'curated') or parts[3] != 'run.json':
                raise EvidenceError('evidence must name a curated run.json')
            record = self._object(name, True)
            if type(record.get('schema_version')) is not int or record['schema_version'] != 1:
                raise EvidenceError('unsupported run schema')
            if record.get('run_id') != parts[2] or not RUN_ID.fullmatch(parts[2]):
                raise EvidenceError('run ID does not match artifact directory')
            if record.get('status') != 'PASS' or record.get('reason') != 'completed' or type(record.get('exit_code')) is not int or record['exit_code'] != 0:
                raise EvidenceError('run did not complete with PASS and exit 0')
            command = record.get('command')
            if not isinstance(command, list) or not 0 < len(command) <= 256 or any(not isinstance(arg, str) or not arg or len(arg) > 4096 for arg in command):
                raise EvidenceError('run command missing or malformed')
            if not isinstance(record.get('category'), str) or not record['category'] or not isinstance(record.get('cwd'), str) or not record['cwd']:
                raise EvidenceError('run category/working directory missing')
            requirements = record.get('requirements')
            if not isinstance(requirements, list) or any(not isinstance(item, str) for item in requirements) or len(set(requirements)) != len(requirements):
                raise EvidenceError('run requirement tags missing or malformed')
            try:
                start = datetime.datetime.fromisoformat(record['started_utc'])
                end = datetime.datetime.fromisoformat(record['ended_utc'])
                duration = record['duration_seconds']
                if start.utcoffset() != datetime.timedelta(0) or end.utcoffset() != datetime.timedelta(0) or end < start or type(duration) not in {int, float} or not math.isfinite(duration) or duration < 0:
                    raise ValueError('invalid interval/duration')
            except (KeyError, TypeError, ValueError) as error:
                raise EvidenceError('run completion timing missing or malformed') from error
            environment = record.get('environment')
            if not isinstance(environment, dict) or any(not isinstance(environment.get(field), str) or not environment[field] for field in ('os', 'architecture', 'python', 'go')):
                raise EvidenceError('run runtime fingerprint missing')
            self._source(record.get('source'))
            streams = []
            for stream in ('stdout', 'stderr'):
                output = record.get(stream)
                expected_path = '/'.join(parts[:3] + (stream + '.txt',))
                if not isinstance(output, dict) or output.get('path') != expected_path:
                    raise EvidenceError(stream + ' must refer to its local owned run stream')
                data = self._read(expected_path, True)
                if type(output.get('bytes')) is not int or output['bytes'] != len(data) or output.get('sha256') != _digest(data):
                    raise EvidenceError(stream + ' byte count or SHA256 mismatch')
                streams.append(data)
            result = (record, streams[0], streams[1])
            self.runs[name] = result
            return result
        except (EvidenceError, OSError, UnicodeError) as error:
            self.run_errors[name] = str(error)
            raise EvidenceError(str(error)) from error

    def _evidence(self, references: object) -> list[tuple[dict, bytes, bytes]]:
        if not isinstance(references, list) or not references or len(references) > 256:
            raise EvidenceError('nonempty run evidence list required')
        if any(not isinstance(item, str) for item in references) or len(set(references)) != len(references):
            raise EvidenceError('duplicate or malformed evidence references')
        return [self._run(reference) for reference in references]

    def _covers(self, run: tuple[dict, bytes, bytes], criterion: dict) -> bool:
        record, output, errors = run
        if not {criterion['id'], criterion['parent_requirement']} & set(record['requirements']):
            return False
        command, path = record['command'], criterion['test_path']
        tool, text, error_text = _tool(command), output.decode('utf-8', errors='replace'), errors.decode('utf-8', errors='replace')
        make_targets = command[1:] if tool == 'make' else []
        if tool == 'make' and any(arg.startswith(('-f', '--file', '--makefile', '-C', '--directory')) or '=' in arg for arg in command[1:]):
            return False
        if tool == 'go' and any(arg in {'-C', '-c', '-list'} or arg.startswith(('-overlay', '-modfile', '-exec', '-toolexec', '-list=')) for arg in command[1:]):
            return False
        if path.endswith('.go'):
            directory = str(PurePosixPath(path).parent)
            module_match = re.search(rb'^module\s+(\S+)\s*$', self._read('go.mod'), re.MULTILINE)
            if module_match is None:
                raise EvidenceError('module identity missing from go.mod')
            package = module_match[1].decode() + '/' + directory
            is_go_test = tool == 'go' and len(command) > 1 and command[1] == 'test'
            if not is_go_test and not any(_package_tree(directory, prefix) for target in make_targets for prefix in MAKE_PACKAGES.get(target, ())):
                return False
            if is_go_test and not any(arg == './...' or arg == './' + directory or arg.startswith('./') and arg.endswith('/...') and _package_tree(directory, arg[2:-4]) for arg in command[2:]):
                return False
            test_names = set(re.findall(r'(?m)^func\s+(Test\w+)\s*\(', self._read(path).decode('utf-8')))
            package_pass = test_pass = False
            logical_output = []
            plain_package = None
            for line in text.splitlines():
                if line.startswith('{'):
                    try:
                        event = _json(line.encode())
                    except EvidenceError:
                        continue
                    if isinstance(event, dict) and event.get('Action') == 'pass' and event.get('Package') == package:
                        package_pass |= 'Test' not in event
                        test_pass |= event.get('Test', '').split('/')[0] in test_names
                    if isinstance(event, dict) and event.get('Package') == package and isinstance(event.get('Output'), str):
                        logical_output.append(event['Output'])
                elif line.startswith('pkg: '):
                    plain_package = line[5:].strip()
                elif line.startswith('ok'):
                    if re.match(r'^ok\s+' + re.escape(package) + r'\s+', line) and 'no tests to run' not in line and '[no test files]' not in line:
                        package_pass = True
                    plain_package = None
                elif re.match(r'^\s*--- PASS: (\S+)\s+\(', line):
                    test_pass |= re.match(r'^\s*--- PASS: (\S+)\s+\(', line)[1].split('/')[0] in test_names
                elif plain_package == package:
                    logical_output.append(line + '\n')
            if criterion['required_test_level'] == 'performance':
                benchmark_command = (is_go_test and any(arg == '-bench' or arg.startswith('-bench=') for arg in command)) or 'benchmark' in make_targets
                benchmark_names = set()
                for test_path in self.current:
                    if str(PurePosixPath(test_path).parent) == directory and test_path.endswith('_test.go'):
                        benchmark_names.update(re.findall(r'(?m)^func\s+(Benchmark\w+)\s*\(', self._read(test_path).decode('utf-8')))
                measured = set(re.findall(r'(?m)^(Benchmark\w+)(?:/[^\s]+)?(?:-\d+)?\s+[^\n]*\bns/op\b', ''.join(logical_output)))
                return benchmark_command and package_pass and bool(measured & benchmark_names)
            # A parent PASS does not show that every acceptance subtest ran.
            # Complete package execution is required until a separately
            # reviewed criterion-to-subtest mapping can establish selection.
            if is_go_test and any(arg in {'-run', '-skip', '-args', '-test.run', '-test.skip'} or arg.startswith(('-run=', '-skip=', '-test.run=', '-test.skip=')) for arg in command[2:]):
                return False
            restricted = any(arg == '-json' or arg.startswith('-json=') for arg in command)
            return package_pass and bool(test_names) and (test_pass or not restricted)
        if path == 'scripts/test_evidence.py':
            selected = _evidence_unittest(command) or any(target in {'test', 'verify-seed', 'verify'} for target in make_targets)
            return selected and bool(re.search(r'Ran [1-9][0-9]* tests?\b', error_text)) and bool(re.search(r'(?m)^OK\s*$', error_text))
        script_target = {'scripts/clean_verify.py': 'clean-verify', 'scripts/public_audit.py': 'public-audit'}
        selected = (_script_command(command, path) or script_target.get(path) in make_targets or path == 'scripts/public_audit.py' and any(target in {'verify-seed', 'verify'} for target in make_targets))
        if not selected:
            return False
        for line in text.splitlines():
            try:
                summary = _json(line.encode())
            except EvidenceError:
                continue
            if isinstance(summary, dict) and summary.get('status') == 'PASS':
                if path == 'scripts/clean_verify.py':
                    commit = record['source'].get('git_commit')
                    return summary.get('clean_checkout') is True and record['source']['git_dirty'] is False and commit is not None and summary.get('commit') == commit
                if path == 'scripts/public_audit.py':
                    return type(summary.get('files')) is int and summary['files'] > 0
        return False

    def problems(self) -> list[str]:
        problems = []
        spec_bytes = self._read('tests/acceptance_locked/spec.json', True)
        lock = self._read('tests/acceptance_locked/spec.sha256', True).decode().strip().split()[0]
        if lock != _digest(spec_bytes):
            raise EvidenceError('locked acceptance specification digest mismatch')
        spec, results = _json(spec_bytes), self._object('verification/acceptance_results.json', True)
        if not isinstance(spec, dict) or spec.get('acceptance_spec_locked') is not True or not isinstance(spec.get('criteria'), list) or not spec['criteria']:
            raise EvidenceError('locked acceptance criteria missing')
        criteria, rows = spec['criteria'], results.get('criteria')
        ids = [criterion['id'] for criterion in criteria]
        if len(set(ids)) != len(ids):
            raise EvidenceError('duplicate locked criterion ID')
        if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
            raise EvidenceError('acceptance result rows missing or malformed')
        lookup = {}
        for row in rows:
            if row.get('id') in lookup or row.get('id') not in ids:
                raise EvidenceError('duplicate or unknown acceptance result ID')
            lookup[row['id']] = row
        for criterion in criteria:
            name, row = criterion['id'], lookup.get(criterion['id'], {})
            if row.get('status') != 'PASS':
                problems.append(name + ': ' + str(row.get('status', 'NOT RUN')))
            try:
                self._owned(criterion['test_path'], True)
                if row.get('status') == 'PASS':
                    runs = self._evidence(row.get('evidence'))
                    if not any(self._covers(run, criterion) for run in runs):
                        raise EvidenceError('no successful evidence executes the required test/measurement')
            except (EvidenceError, OSError, UnicodeError) as error:
                problems.append(name + ': ' + str(error))
        try:
            review = results.get('independent_review')
            if not isinstance(review, dict) or review.get('status') != 'PASS':
                raise EvidenceError('independent full-project review NOT RUN/PASS')
            self._evidence(review.get('evidence'))
            descriptor = review.get('assessment')
            if not isinstance(descriptor, dict):
                raise EvidenceError('source-bound independent assessment missing')
            assessment_path = _relative(descriptor.get('path'))
            if not assessment_path.startswith('verification/reviews/') or not assessment_path.endswith('.json'):
                raise EvidenceError('independent assessment must be a public review JSON artifact')
            data = self._read(assessment_path, True)
            if descriptor.get('sha256') != _digest(data):
                raise EvidenceError('independent assessment SHA256 mismatch')
            assessment = _json(data)
            if not isinstance(assessment, dict) or type(assessment.get('schema_version')) is not int or assessment['schema_version'] != 1 or assessment.get('kind') != 'independent-full-project-review' or assessment.get('status') != 'PASS' or assessment.get('scope') != 'full-project':
                raise EvidenceError('independent assessment is not a full-project PASS')
            reviewer = assessment.get('reviewer')
            if not isinstance(reviewer, dict) or not isinstance(reviewer.get('id'), str) or not reviewer['id'].strip() or reviewer.get('method') != 'separate-read-only-verifier':
                raise EvidenceError('independent assessment reviewer/method missing')
            self._source(assessment.get('source'))
            reviewed = assessment.get('criteria')
            if not isinstance(reviewed, list) or len(reviewed) != len(ids) or set(reviewed) != set(ids):
                raise EvidenceError('independent assessment omits locked criteria')
            observations = assessment.get('observations')
            if not isinstance(observations, list) or not observations or any(not isinstance(item, str) or not item.strip() for item in observations):
                raise EvidenceError('independent assessment observations missing')
            findings = assessment.get('findings')
            if not isinstance(findings, list) or any(not isinstance(item, dict) or item.get('status') != 'RESOLVED' or not isinstance(item.get('summary'), str) or not item['summary'].strip() for item in findings):
                raise EvidenceError('independent assessment has unresolved/malformed findings')
        except (EvidenceError, OSError, UnicodeError) as error:
            problems.append('review: ' + str(error))
        try:
            if results.get('supported_toolchain_verified') is not True:
                raise EvidenceError('supported-toolchain verification missing')
            runs = self._evidence(results.get('supported_toolchain_evidence'))
            pin, matched = self._read('.go-version').decode().strip(), False
            for record, output, _ in runs:
                command = record['command']
                direct = _script_command(command, 'scripts/supported_toolchain.py')
                actual = record['environment']['go']
                if direct and re.fullmatch(r'go version go' + re.escape(pin) + r' [A-Za-z0-9_-]+/[A-Za-z0-9_-]+', actual) and actual in output.decode('utf-8').splitlines():
                    matched = True
            if not matched:
                raise EvidenceError('supported-toolchain evidence lacks the pinned runtime check')
        except (EvidenceError, OSError, UnicodeError) as error:
            problems.append('toolchain: ' + str(error))
        return problems


def main() -> int:
    try:
        problems = Gate(Path(__file__).absolute().parents[1]).problems()
    except (EvidenceError, OSError, UnicodeError, KeyError, TypeError, IndexError) as error:
        problems = ['malformed or missing gate input: ' + str(error)]
    if problems:
        print('FULL RELEASE BLOCKED\n' + '\n'.join(problems), file=sys.stderr)
        return 1
    print('Current inspectable artifact prerequisites present; live full verification must still execute.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
