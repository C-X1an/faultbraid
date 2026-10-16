package labreal

import (
	"errors"

	"github.com/C-X1an/faultbraid/internal/scenario"
)

var ErrCompletionDependency = errors.New("real lab: invalid acknowledgment dependency")

// CompletionDependency delays one native action until a previously invoked
// logical request has a genuine complete/admitted response. Action ticks remain
// elapsed wall-time lower bounds; the fixed epoch is never rebased.
type CompletionDependency struct {
	ActionID  string `json:"action_id"`
	RequestID string `json:"request_id"`
	// Empty accepts the logical request's earliest complete/admitted response.
	// Otherwise an actual response to one of these prior actions is required.
	AttemptActions []string `json:"attempt_actions,omitempty"`
}

// CompletionBarrier records the actual response-receipt ordinal that satisfied
// a declared dependency. An empty ActionID identifies a terminal requirement.
type CompletionBarrier struct {
	ActionID               string `json:"action_id,omitempty"`
	RequestID              string `json:"request_id"`
	AcknowledgmentOrdinal  int64  `json:"acknowledgment_ordinal"`
	AcknowledgmentActionID string `json:"acknowledgment_action_id"`
}

// ActionDispatch observes entry to a configured non-request action, after all
// declared completion waits and before the action's side effects. ObservedOrdinal
// is a floor from the existing common invocation/response counter, not a newly
// assigned event ordinal or a dispatch timestamp.
type ActionDispatch struct {
	ActionID        string `json:"action_id"`
	Kind            string `json:"kind"`
	ObservedOrdinal int64  `json:"observed_ordinal"`
}

type completedAttempt struct {
	requestID string
	ordinal   int64
}

func checkedCompletionOptions(s scenario.Scenario, options Options) (Options, error) {
	if len(options.CompletionDependencies) > scenario.MaxActions || len(options.RequiredCompletions) > scenario.MaxActions {
		return options, errors.Join(ErrCompletionDependency, ErrLimit)
	}
	actions := map[string]int{}
	firstInvoke := map[string]int{}
	for index, action := range s.Actions {
		actions[action.ID] = index
		if action.Kind == "invoke_read" || action.Kind == "invoke_write" {
			if _, exists := firstInvoke[*action.RequestID]; !exists {
				firstInvoke[*action.RequestID] = index
			}
		}
	}
	seen := map[[2]string]bool{}
	attemptCount := 0
	for _, dependency := range options.CompletionDependencies {
		actionIndex, actionExists := actions[dependency.ActionID]
		requestIndex, requestExists := firstInvoke[dependency.RequestID]
		// Strictly preceding invocation indices also reject every dependency
		// cycle: a later/future request can never be a predecessor.
		key := [2]string{dependency.ActionID, dependency.RequestID}
		if !actionExists || !requestExists || requestIndex >= actionIndex || seen[key] {
			return options, ErrCompletionDependency
		}
		seen[key] = true
		attempts := map[string]bool{}
		for _, id := range dependency.AttemptActions {
			index, exists := actions[id]
			if !exists || index >= actionIndex || attempts[id] {
				return options, ErrCompletionDependency
			}
			action := s.Actions[index]
			if (action.Kind != "invoke_read" && action.Kind != "invoke_write") || *action.RequestID != dependency.RequestID {
				return options, ErrCompletionDependency
			}
			attempts[id] = true
			attemptCount++
			if attemptCount > scenario.MaxActions {
				return options, errors.Join(ErrCompletionDependency, ErrLimit)
			}
		}
	}
	terminal := map[string]bool{}
	for _, id := range options.RequiredCompletions {
		if _, exists := firstInvoke[id]; !exists || terminal[id] {
			return options, ErrCompletionDependency
		}
		terminal[id] = true
	}
	options.CompletionDependencies = append([]CompletionDependency(nil), options.CompletionDependencies...)
	for index := range options.CompletionDependencies {
		options.CompletionDependencies[index].AttemptActions = append([]string(nil), options.CompletionDependencies[index].AttemptActions...)
	}
	options.RequiredCompletions = append([]string(nil), options.RequiredCompletions...)
	return options, nil
}

func (l *lab) completionAcknowledgment(dependency CompletionDependency) (string, int64) {
	if len(dependency.AttemptActions) == 0 {
		if request := l.requests[dependency.RequestID]; request != nil {
			return request.ackAction, request.ackOrdinal
		}
		return "", 0
	}
	var action string
	var ordinal int64
	for _, id := range dependency.AttemptActions {
		if actual, exists := l.completedAttempts[id]; exists && actual.requestID == dependency.RequestID && (ordinal == 0 || actual.ordinal < ordinal) {
			action, ordinal = id, actual.ordinal
		}
	}
	return action, ordinal
}

func (l *lab) awaitCompletion(dependency CompletionDependency) error {
	for {
		if err := l.ctx.Err(); err != nil {
			return err
		}
		if action, ordinal := l.completionAcknowledgment(dependency); ordinal > 0 {
			l.result.CompletionBarriers = append(l.result.CompletionBarriers, CompletionBarrier{dependency.ActionID, dependency.RequestID, ordinal, action})
			return nil
		}
		select {
		case event := <-l.events:
			if err := l.observe(event); err != nil {
				return err
			}
		case <-l.ctx.Done():
			return l.ctx.Err()
		}
	}
}

func (l *lab) awaitActionDependencies(actionID string) error {
	for _, dependency := range l.options.CompletionDependencies {
		if dependency.ActionID == actionID {
			if err := l.awaitCompletion(dependency); err != nil {
				return err
			}
		}
	}
	return nil
}
