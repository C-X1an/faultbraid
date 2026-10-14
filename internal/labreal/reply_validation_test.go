package labreal

import (
	"strings"
	"sync/atomic"
	"testing"
)

// These actual loopback HTTP peers deliberately emit a conflicting complete
// reply. They test collector protocol validation in both collection orders;
// they are synthetic peers with parent PID metadata, not native Raft evidence.
func TestNativeCollectorValidatesEveryCompletedWriteReply(t *testing.T) {
	for _, order := range [][]int{{0, 1}, {1, 0}} {
		name := "earlier-correct-then-later-conflicting"
		if order[0] == 1 {
			name = "later-conflicting-then-earlier-correct"
		}
		t.Run(name, func(t *testing.T) {
			var sequence atomic.Int64
			l := schedulingFixture(t, func(input proposal) response {
				value := input.Value
				if sequence.Add(1) == 2 {
					value++
				}
				return response{ID: input.ID, Status: "complete", Admission: "admitted", Value: value}
			})
			outcomes := make([]clientOutcome, 0, 2)
			for range 2 {
				task := schedulingInvoke(t, l, "validated-write", "invoke_write", 47)
				outcomes = append(outcomes, l.propose(task))
			}
			if outcomes[0].err != nil || outcomes[1].err != nil || outcomes[0].response.Status != "complete" || outcomes[1].response.Status != "complete" || outcomes[0].response.Value != 47 || outcomes[1].response.Value != 48 {
				t.Fatal("actual HTTP fixture did not supply a correct and conflicting complete reply")
			}
			for _, index := range order {
				r := l.requests["validated-write"]
				previousReturn := r.operation.Return
				err := l.clientResult(outcomes[index])
				if index == 0 {
					if err != nil {
						t.Fatalf("correct complete write response was rejected: %v", err)
					}
					continue
				}
				if err == nil || !strings.Contains(err.Error(), "write result disagrees with replicated payload") {
					t.Fatal("conflicting complete write escaped validation after an earlier completion")
				}
				if r.operation.Return != previousReturn || r.operation.Value != 47 || !r.possible {
					t.Fatal("rejected conflicting reply erased an earlier result or invented completion")
				}
				attempt := l.result.Attempts[len(l.result.Attempts)-1]
				if attempt.Phase != "complete" || attempt.Admission != "admitted" || attempt.Value != 48 {
					t.Fatal("protocol error erased the actual conflicting reply attempt")
				}
			}
			l.collectHistory()
			operation := l.requests["validated-write"].operation
			if len(l.result.Attempts) != 4 || len(l.result.History.Operations) != 1 || operation.Return == nil || *operation.Return != outcomes[0].ordinal || operation.Value != 47 {
				t.Fatal("collection order changed attempts or the valid original completion")
			}
		})
	}
}
