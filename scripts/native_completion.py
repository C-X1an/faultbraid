"""Check recorded native acknowledgement dependencies against actual attempts.

Synthetic unit fixtures test this validator only; they are not target evidence.
"""


def completion_problems(result, plan):
    problems = []
    dependencies = plan['completion_dependencies']
    required = plan['required_completions']
    if (result.get('completion_dependencies', []) != dependencies or
            result.get('required_completions', []) != required):
        problems.append('native completion configuration differs from the declared plan')
    actions = {}
    for action in result.get('scenario', {}).get('actions', []):
        if action['action_id'] in actions:
            problems.append('duplicate native scenario action')
        actions[action['action_id']] = action['kind']
    nonrequests = {row['action_id'] for row in dependencies
                   if actions.get(row['action_id']) in ('crash', 'restart')}
    dispatches = {}
    for dispatch in result.get('action_dispatches', []):
        action = dispatch['action_id']
        if action in dispatches:
            problems.append('duplicate native non-request dispatch observation')
        dispatches[action] = dispatch
    if set(dispatches) != nonrequests or len(result.get('action_dispatches', [])) != len(nonrequests):
        problems.append('native non-request dispatch inventory differs from the plan')
    attempts = result.get('attempts', [])
    invocations = {}
    completed = {}
    for attempt in attempts:
        if attempt.get('phase') == 'invoke':
            if attempt['action_id'] in invocations:
                problems.append('duplicate native action invocation')
            invocations[attempt['action_id']] = attempt
        if attempt.get('phase') == 'complete' and attempt.get('admission') == 'admitted':
            key = (attempt['action_id'], attempt['ordinal'])
            if key in completed:
                problems.append('duplicate native completion receipt')
            completed[key] = attempt
    barriers = result.get('completion_barriers', [])
    expected = [(row['action_id'], row['request_id'], row.get('attempt_actions', [])) for row in dependencies]
    expected += [('', request, []) for request in required]
    if len(barriers) != len(expected):
        problems.append('native acknowledgement barrier inventory differs from the plan')
    by_key = {}
    for barrier in barriers:
        key = (barrier.get('action_id', ''), barrier.get('request_id'))
        if key in by_key:
            problems.append('duplicate native acknowledgement barrier')
        by_key[key] = barrier
    for action, request, eligible in expected:
        barrier = by_key.get((action, request))
        if barrier is None:
            problems.append('declared native acknowledgement barrier is missing')
            continue
        ordinal = barrier.get('acknowledgment_ordinal')
        acknowledged_action = barrier.get('acknowledgment_action_id')
        receipt = completed.get((acknowledged_action, ordinal))
        invocation = invocations.get(acknowledged_action)
        if (type(ordinal) is not int or ordinal <= 0 or receipt is None or
                receipt.get('request_id') != request or invocation is None or
                invocation.get('request_id') != request or
                type(invocation.get('ordinal')) is not int or invocation['ordinal'] >= ordinal):
            problems.append('native barrier lacks an actual admitted complete receipt')
            continue
        if eligible and acknowledged_action not in eligible:
            problems.append('native barrier used a completion outside its declared retry actions')
        if action:
            kind = actions.get(action)
            if kind in ('invoke_read', 'invoke_write'):
                dependent = invocations.get(action)
                if dependent is None or type(dependent.get('ordinal')) is not int or ordinal >= dependent['ordinal']:
                    problems.append('native dependent invocation preceded its actual acknowledgement')
            elif kind in ('crash', 'restart'):
                dispatch = dispatches.get(action)
                observed = dispatch.get('observed_ordinal') if dispatch is not None else None
                if (dispatch is None or dispatch.get('kind') != kind or
                        type(observed) is not int or observed < 0):
                    problems.append('native non-request dispatch observation is missing or invalid')
                    continue
                # This is a snapshot of the existing common counter at genuine
                # action entry, not an assigned dispatch ordinal or timestamp.
                # Equality proves the acknowledged event already existed.
                if observed < ordinal:
                    problems.append('native non-request dispatch preceded its actual acknowledgement')
            else:
                problems.append('native dependent action has no supported scenario kind')
    return problems
