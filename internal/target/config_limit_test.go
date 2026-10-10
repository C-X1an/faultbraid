package target

import (
	"errors"
	"fmt"
	"testing"
)

func TestReviewerFollowerHonorsConfiguredLogLimit(t *testing.T) {
	c := newCluster(t, Safe)
	const limit = uint64(128)
	follower, err := New(Config{ID: 2, Store: c.stores[2], Entropy: c.entropy[2], LogLimitBytes: limit})
	if err != nil {
		t.Fatal(err)
	}
	c.nodes[2] = follower
	c.step(2, Start{})
	caught := false
	for i := 0; i < 15 && !caught; i++ {
		c.step(1, Write{RequestID: fmt.Sprintf("reviewer-limit-%d", i), Value: int64(i)})
		for n := 0; len(c.queue) > 0; n++ {
			if n > 10000 {
				t.Fatal("pump overflow")
			}
			d := c.queue[0]
			c.queue = c.queue[1:]
			effects, e := c.nodes[d.to].Step(Message{Data: d.data})
			c.collect(d.to, effects)
			if errors.Is(e, ErrLimit) && d.to == 2 {
				caught = true
				break
			}
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	state, err := c.stores[2].Recover()
	if err != nil {
		t.Fatal(err)
	}
	if logBytes(state.Entries) > limit {
		t.Fatalf("follower persisted %d bytes beyond configured %d-byte log limit", logBytes(state.Entries), limit)
	}
	if !caught {
		t.Fatal("configured follower log limit did not terminate progress")
	}
}
