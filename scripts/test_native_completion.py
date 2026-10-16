"""Synthetic receipt fixtures validate the recipe checker, not a Raft target."""
import copy
import json
from pathlib import Path
import unittest

from native_completion import completion_problems


class NativeCompletionEvidenceTests(unittest.TestCase):
    def fixture(self):
        plan = {'version': 1, 'completion_dependencies': [
            {'action_id': 'retry', 'request_id': 'new-write'},
            {'action_id': 'read', 'request_id': 'old-write', 'attempt_actions': ['retry']}],
            'required_completions': ['read-result']}
        attempts = []
        for action, request, call, returned in [('initial', 'old-write', 1, 2), ('new', 'new-write', 3, 4), ('retry', 'old-write', 5, 6), ('read', 'read-result', 7, 8)]:
            attempts.extend([{'action_id': action, 'request_id': request, 'ordinal': call, 'phase': 'invoke', 'admission': 'awaiting-response'},
                             {'action_id': action, 'request_id': request, 'ordinal': returned, 'phase': 'complete', 'admission': 'admitted'}])
        result = {key: copy.deepcopy(plan[key]) for key in ('completion_dependencies', 'required_completions')}
        result['scenario'] = {'actions': [{'action_id': action, 'kind': kind} for action, kind in
                              [('initial', 'invoke_write'), ('new', 'invoke_write'), ('retry', 'invoke_write'), ('read', 'invoke_read')]]}
        result['attempts'] = attempts
        result['completion_barriers'] = [
            {'action_id': 'retry', 'request_id': 'new-write', 'acknowledgment_ordinal': 4, 'acknowledgment_action_id': 'new'},
            {'action_id': 'read', 'request_id': 'old-write', 'acknowledgment_ordinal': 6, 'acknowledgment_action_id': 'retry'},
            {'request_id': 'read-result', 'acknowledgment_ordinal': 8, 'acknowledgment_action_id': 'read'}]
        return result, plan

    def full_fixture(self):
        # Only receipt-validator fixtures: the exact production inputs do not
        # turn these synthetic response/dispatch records into a target run.
        root = Path(__file__).resolve().parents[1]
        plan = json.loads((root / 'examples/raft-real-completions.json').read_bytes())
        scenario = json.loads((root / 'examples/raft-real.json').read_bytes())
        result = {key: copy.deepcopy(plan[key]) for key in ('completion_dependencies', 'required_completions')}
        result.update(scenario=scenario, attempts=[], action_dispatches=[], completion_barriers=[])
        counter = 0
        nonrequests = {row['action_id'] for row in plan['completion_dependencies']}
        for action in scenario['actions']:
            if action['kind'] in ('invoke_read', 'invoke_write'):
                for phase, admission in [('invoke', 'awaiting-response'), ('complete', 'admitted')]:
                    counter += 1
                    result['attempts'].append({'action_id': action['action_id'], 'request_id': action['request_id'], 'ordinal': counter, 'phase': phase, 'admission': admission})
            elif action['action_id'] in nonrequests:
                result['action_dispatches'].append({'action_id': action['action_id'], 'kind': action['kind'], 'observed_ordinal': counter})
        declarations = [(row['action_id'], row['request_id'], row.get('attempt_actions', [])) for row in plan['completion_dependencies']]
        declarations += [('', request, []) for request in plan['required_completions']]
        for action, request, eligible in declarations:
            receipt = next(row for row in result['attempts'] if row['phase'] == 'complete' and row['request_id'] == request and (not eligible or row['action_id'] in eligible))
            result['completion_barriers'].append({'action_id': action, 'request_id': request, 'acknowledgment_ordinal': receipt['ordinal'], 'acknowledgment_action_id': receipt['action_id']})
        return result, plan

    def test_actual_receipt_order_fixture(self):
        result, plan = self.fixture()
        self.assertEqual(completion_problems(result, plan), [])

    def test_initial_logical_ack_cannot_stand_in_for_retry(self):
        result, plan = self.fixture()
        result['completion_barriers'][1].update(acknowledgment_ordinal=2, acknowledgment_action_id='initial')
        self.assertTrue(any('retry actions' in problem for problem in completion_problems(result, plan)))

    def test_pending_rejected_or_fabricated_receipt_cannot_acknowledge(self):
        for phase, admission in [('pending', 'unknown'), ('rejected', 'rejected-before-admission'), ('complete', 'unknown')]:
            with self.subTest(phase=phase, admission=admission):
                result, plan = self.fixture()
                result['attempts'][5].update(phase=phase, admission=admission)
                self.assertTrue(any('actual admitted complete' in problem for problem in completion_problems(result, plan)))
        result, plan = self.fixture()
        result['completion_barriers'][1]['acknowledgment_ordinal'] = 99
        self.assertTrue(completion_problems(result, plan))

    def test_read_invocation_must_follow_retry_receipt(self):
        result, plan = self.fixture()
        result['attempts'][6]['ordinal'] = 5
        self.assertTrue(any('preceded' in problem for problem in completion_problems(result, plan)))

    def test_missing_duplicate_or_changed_plan_blocks(self):
        for change in ('missing', 'duplicate', 'changed'):
            with self.subTest(change=change):
                result, plan = self.fixture()
                if change == 'missing': result['completion_barriers'].pop()
                elif change == 'duplicate': result['completion_barriers'].append(copy.deepcopy(result['completion_barriers'][0]))
                else: result['required_completions'] = []
                self.assertTrue(completion_problems(result, plan))

    def test_full_production_plan_nonrequest_dispatch_positive(self):
        result, plan = self.full_fixture()
        self.assertEqual(len(plan['completion_dependencies']), 13)
        self.assertEqual(len(result['action_dispatches']), 1)
        self.assertEqual(completion_problems(result, plan), [])

    def test_nonrequest_floor_cannot_precede_acknowledgement(self):
        result, plan = self.full_fixture()
        barrier = next(row for row in result['completion_barriers'] if row['action_id'] == 'kill-node2')
        result['action_dispatches'][0]['observed_ordinal'] = barrier['acknowledgment_ordinal'] - 1
        self.assertTrue(any('non-request dispatch preceded' in problem for problem in completion_problems(result, plan)))

    def test_nonrequest_missing_duplicate_unknown_or_invalid_dispatch_blocks(self):
        for change in ('missing', 'duplicate', 'unknown', 'wrong-kind', 'bool', 'negative'):
            with self.subTest(change=change):
                result, plan = self.full_fixture()
                if change == 'missing': result['action_dispatches'].clear()
                elif change == 'duplicate': result['action_dispatches'].append(copy.deepcopy(result['action_dispatches'][0]))
                elif change == 'unknown': result['action_dispatches'][0]['action_id'] = 'unknown'
                elif change == 'wrong-kind': result['action_dispatches'][0]['kind'] = 'restart'
                elif change == 'bool': result['action_dispatches'][0]['observed_ordinal'] = True
                else: result['action_dispatches'][0]['observed_ordinal'] = -1
                self.assertTrue(completion_problems(result, plan))

    def test_client_invocation_cannot_replace_nonrequest_dispatch(self):
        result, plan = self.full_fixture()
        result['action_dispatches'].clear()
        result['attempts'].append({'action_id': 'kill-node2', 'request_id': 'read7', 'ordinal': 99, 'phase': 'invoke', 'admission': 'awaiting-response'})
        self.assertTrue(any('non-request' in problem for problem in completion_problems(result, plan)))

    def test_unknown_scenario_kind_cannot_release(self):
        result, plan = self.fixture()
        result['scenario']['actions'][-1]['kind'] = 'unknown'
        self.assertTrue(any('supported scenario kind' in problem for problem in completion_problems(result, plan)))

    def test_request_dispatch_cannot_replace_actual_invocation(self):
        result, plan = self.fixture()
        result['attempts'] = [row for row in result['attempts'] if not (row['action_id'] == 'read' and row['phase'] == 'invoke')]
        result['action_dispatches'] = [{'action_id': 'read', 'kind': 'invoke_read', 'observed_ordinal': 99}]
        self.assertTrue(any('dependent invocation' in problem for problem in completion_problems(result, plan)))


if __name__ == '__main__':
    unittest.main()
