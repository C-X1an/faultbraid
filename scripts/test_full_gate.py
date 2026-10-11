#!/usr/bin/env python3
"""Validator unit fixtures are synthetic inputs, never release evidence."""
import copy
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from full_gate import EvidenceError, Gate, execution_path
from run_evidence import source_identity


def digest(data):
    return hashlib.sha256(data).hexdigest()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


class Fixture:
    """A complete miniature artifact set; none of its claimed runs is a release run."""
    def __init__(self, base):
        self.root = base / 'synthetic-unit-public'
        self.root.mkdir()
        files = {
            'README.md': b'SYNTHETIC VALIDATOR UNIT FIXTURE. Not execution or release evidence.\n',
            'go.mod': b'module faultbraid\n\ngo 1.26.0\n',
            '.go-version': b'1.27.2\n',
            'scripts/full_gate.py': Path(__file__).with_name('full_gate.py').read_bytes(),
            'scripts/test_evidence.py': b'# Synthetic unit fixture, not an executed suite.\n',
            'scripts/supported_toolchain.py': b'# Synthetic unit fixture, not a runtime check.\n',
            'scripts/clean_verify.py': b'# Synthetic unit fixture, not a clean checkout.\n',
            'scripts/public_audit.py': b'# Synthetic unit fixture, not a privacy audit.\n',
            'tests/acceptance_locked/acceptance_test.go': b'package fixture\nfunc TestFixture(t *testing.T) {}\n',
            'internal/history/checker_test.go': b'package fixture\nfunc TestFixture(t *testing.T) {}\nfunc BenchmarkFixture(b *testing.B) {}\n',
            'docs/codec/unit.golden.json': b'{"fixture_only":true}\n',
            'third_party/raft-patch/lock.json': b'{"fixture_only":true}\n',
            'dependency.lock.json': b'{"fixture_only":true}\n',
            'toolchain.lock.json': b'{"fixture_only":true}\n',
        }
        for name, data in files.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        self.criteria = [
            {'id': 'AC-001', 'parent_requirement': 'FR-001', 'test_path': 'tests/acceptance_locked/acceptance_test.go', 'required_test_level': 'contract'},
            {'id': 'AC-018', 'parent_requirement': 'NFR-003', 'test_path': 'scripts/test_evidence.py', 'required_test_level': 'integration'},
            {'id': 'AC-020', 'parent_requirement': 'NFR-004', 'test_path': 'scripts/clean_verify.py', 'required_test_level': 'reproducibility'},
            {'id': 'AC-021', 'parent_requirement': 'SEC-003', 'test_path': 'scripts/public_audit.py', 'required_test_level': 'release'},
            {'id': 'AC-022', 'parent_requirement': 'PERF-001', 'test_path': 'internal/history/checker_test.go', 'required_test_level': 'performance'},
        ]
        self.spec_path = self.root / 'tests/acceptance_locked/spec.json'
        write_json(self.spec_path, {'schema_version': 1, 'acceptance_spec_locked': True, 'fixture_only': 'synthetic validator unit fixture', 'criteria': self.criteria})
        (self.spec_path.parent / 'spec.sha256').write_text(digest(self.spec_path.read_bytes()) + '\n', encoding='utf-8')
        self.source = source_identity(self.root)
        # Metadata format fixtures only; this temporary directory is not a Git release.
        self.source.update(git_commit='a' * 40, git_dirty=False)
        self.refs = []
        self.records = []
        go_output = '\n'.join(json.dumps(item) for item in [
            {'Action': 'pass', 'Package': 'faultbraid/tests/acceptance_locked', 'Test': 'TestFixture'},
            {'Action': 'pass', 'Package': 'faultbraid/tests/acceptance_locked'},
            {'Action': 'output', 'Package': 'faultbraid/internal/history', 'Output': 'BenchmarkFixture-8  100  25 ns/op  12 states/op  64 B/op  1 allocs/op\n'},
            {'Action': 'pass', 'Package': 'faultbraid/internal/history'},
        ]) + '\n'
        self.go = self.run(['go', 'test', '-json', '-count=1', '-bench=.', './...'], ['AC-001', 'AC-022'], go_output.encode())
        unit = self.run(['python', '-m', 'unittest', 'discover', '-s', 'scripts', '-p', 'test_*.py', '-v'], ['AC-018'], b'', b'Ran 4 tests in 0.01s\n\nOK\n')
        clean = self.run(['python', 'scripts/clean_verify.py'], ['AC-020'], (json.dumps({'status': 'PASS', 'clean_checkout': True, 'commit': 'a' * 40}) + '\n').encode())
        audit = self.run(['python', 'scripts/public_audit.py'], ['AC-021'], b'{"status":"PASS","files":16}\n')
        self.toolchain = self.run(['python', 'scripts/supported_toolchain.py'], [], b'go version go1.27.2 windows/amd64\n')
        self.assessment_path = self.root / 'verification/reviews/synthetic-unit-review.json'
        self.assessment = {
            'schema_version': 1, 'kind': 'independent-full-project-review', 'status': 'PASS', 'scope': 'full-project',
            'reviewer': {'id': 'synthetic-unit-fixture-only', 'method': 'separate-read-only-verifier'},
            'source': copy.deepcopy(self.source), 'criteria': [item['id'] for item in self.criteria],
            'observations': ['Synthetic validator unit fixture. No independent release review was performed.'],
            'findings': [],
        }
        write_json(self.assessment_path, self.assessment)
        self.results_path = self.root / 'verification/acceptance_results.json'
        self.results = {
            'scope': 'Synthetic validator unit fixture; never release evidence',
            'criteria': [{'id': item['id'], 'status': 'PASS', 'evidence': [reference]} for item, reference in zip(self.criteria, [self.go, unit, clean, audit, self.go])],
            'independent_review': {'status': 'PASS', 'evidence': [self.go], 'assessment': {'path': self.assessment_path.relative_to(self.root).as_posix(), 'sha256': digest(self.assessment_path.read_bytes())}},
            'supported_toolchain_verified': True, 'supported_toolchain_evidence': [self.toolchain],
        }
        self.save_results()
        self.inventory()

    def run(self, command, requirements, stdout, stderr=b''):
        run_id = '20261010T120000-' + f'{len(self.refs):010x}'
        directory = self.root / 'verification/curated' / run_id
        directory.mkdir(parents=True)
        record = {
            'schema_version': 1, 'run_id': run_id, 'category': 'synthetic-validator-unit-fixture',
            'requirements': requirements, 'command': command, 'cwd': '<synthetic unit public fixture>',
            'started_utc': '2026-10-10T12:00:00+00:00', 'ended_utc': '2026-10-10T12:00:01+00:00',
            'duration_seconds': 1, 'status': 'PASS', 'reason': 'completed', 'exit_code': 0,
            'environment': {'os': 'Windows', 'architecture': 'AMD64', 'python': '3.13.14', 'go': 'go version go1.27.2 windows/amd64'},
            'source': copy.deepcopy(self.source),
        }
        for stream, data in [('stdout', stdout), ('stderr', stderr)]:
            path = directory / (stream + '.txt')
            path.write_bytes(data)
            record[stream] = {'path': path.relative_to(self.root).as_posix(), 'sha256': digest(data), 'bytes': len(data)}
        path = directory / 'run.json'
        write_json(path, record)
        self.refs.append(path.relative_to(self.root).as_posix())
        self.records.append(record)
        return self.refs[-1]

    def inventory(self):
        names = sorted(path.relative_to(self.root).as_posix() for path in self.root.rglob('*') if path.is_file() and '__pycache__' not in path.parts)
        names = sorted(set(names) | {'PUBLIC_FILES.txt'})
        (self.root / 'PUBLIC_FILES.txt').write_text('\n'.join(names) + '\n', encoding='utf-8')

    def save_results(self):
        write_json(self.results_path, self.results)

    def change_run(self, reference, change):
        path = self.root / reference
        record = json.loads(path.read_text(encoding='utf-8'))
        change(record)
        write_json(path, record)

    def change_assessment(self, change):
        change(self.assessment)
        write_json(self.assessment_path, self.assessment)
        self.results['independent_review']['assessment']['sha256'] = digest(self.assessment_path.read_bytes())
        self.save_results()


class FullGateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='faultbraid-gate-unit-')
        self.addCleanup(self.temporary.cleanup)
        self.fixture = Fixture(Path(self.temporary.name))

    def blocked(self, needle):
        try:
            problems = Gate(self.fixture.root).problems()
        except EvidenceError as error:
            problems = [str(error)]
        self.assertTrue(problems, 'a tampered fixture must not pass')
        self.assertIn(needle, '\n'.join(problems))

    def test_complete_local_synthetic_fixture(self):
        self.assertEqual(Gate(self.fixture.root).problems(), [])

    def test_cli_current_fixture_and_missing_artifact_exit(self):
        script = self.fixture.root / 'scripts/full_gate.py'
        result = subprocess.run([sys.executable, str(script)], capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('live full verification must still execute', result.stdout)
        (self.fixture.root / self.fixture.go).unlink()
        result = subprocess.run([sys.executable, str(script)], capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 1)
        self.assertIn('FULL RELEASE BLOCKED', result.stderr)
        self.assertNotIn('Traceback', result.stderr)

    def test_pass_label_does_not_override_failed_exit_or_timeout(self):
        for field, value in [('exit_code', 7), ('exit_code', False), ('reason', 'timeout'), ('status', 'BLOCKED')]:
            with self.subTest(field=field, value=value):
                original = (self.fixture.root / self.fixture.go).read_bytes()
                self.fixture.change_run(self.fixture.go, lambda record: record.update({field: value}))
                self.blocked('did not complete with PASS')
                (self.fixture.root / self.fixture.go).write_bytes(original)

    def test_fake_pass_command_and_output_are_insufficient(self):
        self.fixture.change_run(self.fixture.go, lambda record: record.update(command=['true']))
        self.blocked('no successful evidence executes')

    def test_pass_echo_with_matching_stream_hash_is_insufficient(self):
        def replace(record):
            path = self.fixture.root / record['stdout']['path']
            path.write_bytes(b'PASS\n')
            record['stdout'].update(bytes=5, sha256=digest(b'PASS\n'))
        self.fixture.change_run(self.fixture.go, replace)
        self.blocked('no successful evidence executes')

    def test_output_bytes_and_hash_are_verified(self):
        for field, value in [('bytes', 0), ('bytes', True), ('sha256', '0' * 64)]:
            with self.subTest(field=field):
                original = (self.fixture.root / self.fixture.go).read_bytes()
                self.fixture.change_run(self.fixture.go, lambda record: record['stdout'].update({field: value}))
                self.blocked('byte count or SHA256 mismatch')
                (self.fixture.root / self.fixture.go).write_bytes(original)

    def test_stream_tampering_is_detected(self):
        path = self.fixture.root / self.fixture.records[0]['stdout']['path']
        path.write_bytes(path.read_bytes() + b'tampered\n')
        self.blocked('byte count or SHA256 mismatch')

    def test_source_test_change_and_new_input_are_detected(self):
        path = self.fixture.root / self.fixture.criteria[0]['test_path']
        path.write_bytes(path.read_bytes() + b'// changed assertion\n')
        self.blocked('execution source/input changed')

    def test_new_executable_input_requires_new_execution(self):
        (self.fixture.root / 'tests/new-input.json').write_bytes(b'{"new":true}\n')
        self.blocked('tests/new-input.json')

    def test_relevant_locks_patch_and_codec_golden_changes(self):
        for name in ['docs/codec/unit.golden.json', 'third_party/raft-patch/lock.json', 'dependency.lock.json', 'toolchain.lock.json', '.go-version']:
            with self.subTest(name=name):
                path = self.fixture.root / name
                original = path.read_bytes()
                path.write_bytes(original + b'\n')
                self.blocked('execution source/input changed')
                path.write_bytes(original)

    def test_raw_source_digest_must_match_preserved_files(self):
        self.fixture.change_run(self.fixture.go, lambda record: record['source'].update(source_tree_sha256='0' * 64))
        self.blocked('source-tree digest mismatch')

    def test_rehashed_source_omission_still_blocks(self):
        def omit(record):
            source = record['source']
            source['files'] = [item for item in source['files'] if item['path'] != self.fixture.criteria[0]['test_path']]
            source['source_tree_sha256'] = digest(json.dumps(source['files'], sort_keys=True, separators=(',', ':')).encode())
        self.fixture.change_run(self.fixture.go, omit)
        self.blocked('execution source/input changed')

    def test_documentation_and_curated_indexes_can_follow_execution(self):
        (self.fixture.root / 'README.md').write_text('Updated synthetic unit fixture prose.\n', encoding='utf-8')
        (self.fixture.root / 'docs/report.md').parent.mkdir(exist_ok=True)
        (self.fixture.root / 'docs/report.md').write_text('Synthetic unit fixture curation note.\n', encoding='utf-8')
        self.fixture.results['note'] = 'Synthetic unit fixture later curation'
        self.fixture.save_results()
        self.fixture.inventory()
        self.assertEqual(Gate(self.fixture.root).problems(), [])

    def test_missing_or_not_allowlisted_required_files_block(self):
        path = self.fixture.root / self.fixture.criteria[0]['test_path']
        path.unlink()
        self.blocked('execution source/input changed')

    def test_unreviewed_stream_is_rejected(self):
        inventory = self.fixture.root / 'PUBLIC_FILES.txt'
        remove = self.fixture.records[0]['stdout']['path']
        inventory.write_text('\n'.join(line for line in inventory.read_text().splitlines() if line != remove) + '\n', encoding='utf-8')
        self.blocked('not in reviewed PUBLIC_FILES.txt')

    def test_outside_and_noncanonical_evidence_references(self):
        for value in ['../outside/run.json', '/tmp/run.json', 'C:/private/run.json', 'verification\\curated\\run.json', 'verification/./curated/run.json']:
            with self.subTest(value=value):
                self.fixture.results['criteria'][0]['evidence'] = [value]
                self.fixture.save_results()
                self.blocked('path')

    def test_original_absolute_stream_path_is_not_local_evidence(self):
        self.fixture.change_run(self.fixture.go, lambda record: record['stdout'].update(path='/old-host/.local-runs/stdout.txt'))
        self.blocked('local owned run stream')

    def test_hard_linked_stream_is_rejected(self):
        path = self.fixture.root / self.fixture.records[0]['stdout']['path']
        link = Path(self.temporary.name) / 'stream-alias.txt'
        os.link(path, link)
        self.blocked('owned regular file')

    def test_real_directory_alias_to_external_matching_evidence_is_rejected(self):
        run_directory = (self.fixture.root / self.fixture.go).parent
        outside = Path(self.temporary.name) / 'external-run'
        self.assertTrue(run_directory.resolve().is_relative_to(self.fixture.root.resolve()))
        self.assertTrue(outside.parent.resolve() == Path(self.temporary.name).resolve())
        run_directory.rename(outside)
        if os.name == 'nt':
            subprocess.run(['cmd', '/c', 'mklink', '/J', str(run_directory), str(outside)], check=True, capture_output=True)
            self.addCleanup(run_directory.rmdir)
        else:
            run_directory.symlink_to(outside, target_is_directory=True)
            self.addCleanup(run_directory.unlink)
        self.blocked('alias')

    def test_compile_only_test_run_does_not_pass_acceptance(self):
        def compile_only(record):
            record['command'] = ['go', 'test', '-run', '^$', './...']
            output = b'ok  faultbraid/tests/acceptance_locked  0.01s [no tests to run]\n'
            (self.fixture.root / record['stdout']['path']).write_bytes(output)
            record['stdout'].update(bytes=len(output), sha256=digest(output))
        self.fixture.change_run(self.fixture.go, compile_only)
        self.blocked('no successful evidence executes')

    def test_list_only_command_does_not_execute_assertions(self):
        self.fixture.change_run(self.fixture.go, lambda record: record.update(command=['go', 'test', '-list=.', './...']))
        self.blocked('no successful evidence executes')

    def test_json_package_pass_without_executed_required_test_is_insufficient(self):
        def package_only(record):
            data = b'{"Action":"pass","Package":"faultbraid/tests/acceptance_locked"}\n'
            (self.fixture.root / record['stdout']['path']).write_bytes(data)
            record['stdout'].update(bytes=len(data), sha256=digest(data))
        self.fixture.change_run(self.fixture.go, package_only)
        self.blocked('AC-001: no successful evidence executes')

    def test_wrong_package_and_unrelated_test_dont_cover_required_file(self):
        def unrelated(record):
            record['command'] = ['go', 'test', '-json', '-run=TestOther', './...']
            data = b'{"Action":"pass","Package":"faultbraid/tests/acceptance_locked","Test":"TestOther"}\n{"Action":"pass","Package":"faultbraid/tests/acceptance_locked"}\n'
            (self.fixture.root / record['stdout']['path']).write_bytes(data)
            record['stdout'].update(bytes=len(data), sha256=digest(data))
        self.fixture.change_run(self.fixture.go, unrelated)
        self.blocked('AC-001: no successful evidence executes')

    def test_selected_subtest_parent_pass_is_not_other_acceptance_evidence(self):
        # Reproduce the observed attribution error: AC005 and its parent pass,
        # while the required AC001 subtest never executes. These are synthetic
        # validator inputs; the independent review retained the real Go run.
        def selected(record):
            record['command'] = ['go', 'test', '-json', '-count=1', '-run', '^TestFixture/AC005$', './...']
            events = [
                {'Action': 'pass', 'Package': 'faultbraid/tests/acceptance_locked', 'Test': 'TestFixture/AC005'},
                {'Action': 'pass', 'Package': 'faultbraid/tests/acceptance_locked', 'Test': 'TestFixture'},
                {'Action': 'pass', 'Package': 'faultbraid/tests/acceptance_locked'},
            ]
            data = ('\n'.join(json.dumps(event) for event in events) + '\n').encode()
            (self.fixture.root / record['stdout']['path']).write_bytes(data)
            record['stdout'].update(bytes=len(data), sha256=digest(data))
        self.fixture.change_run(self.fixture.go, selected)
        self.blocked('AC-001: no successful evidence executes')

    def test_skip_selection_cannot_be_credited_as_complete_package(self):
        self.fixture.change_run(self.fixture.go, lambda record: record.update(command=['go', 'test', '-json', '-skip=AC001', './...']))
        self.blocked('AC-001: no successful evidence executes')

    def test_benchmark_output_without_benchmark_execution_is_rejected(self):
        self.fixture.change_run(self.fixture.go, lambda record: record.update(command=['go', 'test', '-json', './...']))
        self.blocked('AC-022: no successful evidence executes')

    def test_benchmark_from_another_package_is_not_history_measurement(self):
        for use_json in (False, True):
            with self.subTest(json=use_json):
                def wrong_package(record):
                    record['command'] = ['go', 'test', '-count=1', '-bench=BenchmarkOther', './...'] + (['-json'] if use_json else [])
                    if use_json:
                        events = [
                            {'Action':'pass','Package':'faultbraid/tests/acceptance_locked','Test':'TestFixture'},
                            {'Action':'pass','Package':'faultbraid/tests/acceptance_locked'},
                            {'Action':'pass','Package':'faultbraid/internal/history'},
                            {'Action':'output','Package':'faultbraid/internal/sim','Output':'BenchmarkOther-8  1  25 ns/op\n'},
                            {'Action':'pass','Package':'faultbraid/internal/sim'},
                        ]
                        data = ('\n'.join(json.dumps(event) for event in events)+'\n').encode()
                    else:
                        data = b'ok  faultbraid/tests/acceptance_locked  0.01s\nok  faultbraid/internal/history  0.01s\ngoos: windows\ngoarch: amd64\npkg: faultbraid/internal/sim\nBenchmarkOther-8  1  25 ns/op\nok  faultbraid/internal/sim  0.01s\n'
                    (self.fixture.root / record['stdout']['path']).write_bytes(data)
                    record['stdout'].update(bytes=len(data), sha256=digest(data))
                self.fixture.change_run(self.fixture.go, wrong_package)
                self.blocked('AC-022: no successful evidence executes')

    def test_plain_benchmark_binds_package_and_source_name(self):
        def plain(record):
            record['command'] = ['go','test','-count=1','-bench=.','./...']
            data = b'ok  faultbraid/tests/acceptance_locked  0.01s\npkg: faultbraid/internal/sim\nBenchmarkOther-8  1  99 ns/op\nok  faultbraid/internal/sim  0.01s\npkg: faultbraid/internal/history\nBenchmarkFixture-8  100  25 ns/op\nok  faultbraid/internal/history  0.01s\n'
            (self.fixture.root / record['stdout']['path']).write_bytes(data)
            record['stdout'].update(bytes=len(data),sha256=digest(data))
        self.fixture.change_run(self.fixture.go,plain)
        self.assertEqual(Gate(self.fixture.root).problems(),[])
        def unknown_name(record):
            data=(self.fixture.root / record['stdout']['path']).read_bytes().replace(b'BenchmarkFixture-',b'BenchmarkUndeclared-')
            (self.fixture.root / record['stdout']['path']).write_bytes(data)
            record['stdout'].update(bytes=len(data),sha256=digest(data))
        self.fixture.change_run(self.fixture.go,unknown_name)
        self.blocked('AC-022: no successful evidence executes')

    def test_requirement_binding_is_required(self):
        self.fixture.change_run(self.fixture.go, lambda record: record.update(requirements=['unrelated']))
        self.blocked('no successful evidence executes')

    def test_boolean_review_and_toolchain_claims_are_insufficient(self):
        self.fixture.results['independent_review'] = {'status': 'PASS'}
        self.fixture.results.pop('supported_toolchain_evidence')
        self.fixture.save_results()
        self.blocked('nonempty run evidence list required')

    def test_clean_checkout_summary_must_bind_a_clean_recorded_commit(self):
        reference = self.fixture.results['criteria'][2]['evidence'][0]
        self.fixture.change_run(reference, lambda record: record['source'].update(git_dirty=True))
        self.blocked('AC-020: no successful evidence executes')
        self.fixture.change_run(reference, lambda record: record['source'].update(git_dirty=False, git_commit='b' * 40))
        self.blocked('AC-020: no successful evidence executes')

    def test_source_bound_assessment_is_hashed_and_covers_every_criterion(self):
        self.fixture.results['independent_review']['assessment']['sha256'] = '0' * 64
        self.fixture.save_results()
        self.blocked('assessment SHA256 mismatch')
        self.fixture.change_assessment(lambda report: report['criteria'].pop())
        self.blocked('assessment omits locked criteria')

    def test_partial_self_review_and_open_finding_are_rejected(self):
        original = copy.deepcopy(self.fixture.assessment)
        for change, needle in [
            (lambda report: report.update(scope='kernel'), 'not a full-project PASS'),
            (lambda report: report['reviewer'].update(method='author-self-review'), 'reviewer/method missing'),
            (lambda report: report.update(findings=[{'id': 'F1', 'status': 'OPEN', 'summary': 'Synthetic unresolved finding'}]), 'unresolved/malformed findings'),
        ]:
            with self.subTest(needle=needle):
                self.fixture.assessment = copy.deepcopy(original)
                self.fixture.change_assessment(change)
                self.blocked(needle)

    def test_wrong_go_pin_cannot_be_overridden_by_true_boolean(self):
        self.fixture.change_run(self.fixture.toolchain, lambda record: record['environment'].update(go='go version go1.23.2 linux/amd64'))
        self.blocked('pinned runtime check')

    def test_python_cannot_echo_expected_summary_with_script_name_as_argument(self):
        self.fixture.change_run(self.fixture.toolchain, lambda record: record.update(command=['python', '-c', 'print("go version go1.27.2 windows/amd64")', 'scripts/supported_toolchain.py']))
        self.blocked('pinned runtime check')

    def test_unrelated_unittest_pattern_does_not_execute_evidence_test(self):
        reference = self.fixture.results['criteria'][1]['evidence'][0]
        self.fixture.change_run(reference, lambda record: record.update(command=['python', '-m', 'unittest', 'discover', '-s', 'scripts', '-p', 'test_full_gate.py']))
        self.blocked('AC-018: no successful evidence executes')

    def test_external_makefile_and_go_overlay_are_not_current_source_checks(self):
        for command in [['make', '-f', '/external/other.mk', 'test'], ['make', 'GO=other', 'test'], ['go', 'test', '-overlay=/external/overlay.json', './...']]:
            with self.subTest(command=command):
                original = (self.fixture.root / self.fixture.go).read_bytes()
                self.fixture.change_run(self.fixture.go, lambda record: record.update(command=command))
                self.blocked('no successful evidence executes')
                (self.fixture.root / self.fixture.go).write_bytes(original)

    def test_duplicate_result_and_duplicate_json_fields_are_rejected(self):
        self.fixture.results['criteria'].append(copy.deepcopy(self.fixture.results['criteria'][0]))
        self.fixture.save_results()
        with self.assertRaisesRegex(EvidenceError, 'duplicate or unknown acceptance result ID'):
            Gate(self.fixture.root).problems()
        path = self.fixture.root / self.fixture.go
        path.write_text('{"status":"FAIL","status":"PASS"}', encoding='utf-8')
        self.fixture.results['criteria'].pop()
        self.fixture.save_results()
        self.blocked('duplicate JSON field')

    def test_spec_digest_is_checked_without_rewriting_it(self):
        self.fixture.spec_path.write_bytes(self.fixture.spec_path.read_bytes() + b'\n')
        with self.assertRaisesRegex(EvidenceError, 'locked acceptance specification digest mismatch'):
            Gate(self.fixture.root).problems()

    def test_manifest_id_schema_runtime_and_timing_are_validated(self):
        for field, value, needle in [('run_id', 'other-run', 'run ID'), ('schema_version', 2, 'unsupported run schema'), ('environment', {}, 'runtime fingerprint'), ('duration_seconds', -1, 'completion timing')]:
            with self.subTest(field=field):
                original = (self.fixture.root / self.fixture.go).read_bytes()
                self.fixture.change_run(self.fixture.go, lambda record: record.update({field: value}))
                self.blocked(needle)
                (self.fixture.root / self.fixture.go).write_bytes(original)

    def test_execution_input_policy_is_explicit(self):
        for name in ['cmd/main.go', 'internal/report/template.html', 'scripts/runner.py', 'tests/input.txt', 'examples/history.json', 'docs/codec/trace.golden.json', 'third_party/raft-patch/election-random.patch', 'third_party/raft-patch/lock.json', 'go.mod', 'go.sum', '.go-version', 'dependency.lock.json', 'toolchain.lock.json', 'Makefile', '.github/workflows/ci.yml']:
            self.assertTrue(execution_path(name), name)
        for name in ['README.md', 'docs/adr/008.md', 'PUBLIC_FILES.txt', 'verification/acceptance_results.json', 'verification/curated/run/stdout.txt', 'verification/reviews/review.json', 'third_party/raft-patch/PROVENANCE.md']:
            self.assertFalse(execution_path(name), name)


if __name__ == '__main__':
    unittest.main()
