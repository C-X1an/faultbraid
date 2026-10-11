package security

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func FuzzScenarioSchema(f *testing.F) {
	data, err := json.Marshal(campaign.KnownBadScenario())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte(`{"version":1,"version":2}`))
	f.Add([]byte(`{"version":null}`))
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := scenario.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		if err := decoded.Validate(); err != nil {
			t.Fatal("decoder accepted invalid scenario bounds or actions")
		}
		canonical, err := json.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		again, err := scenario.Decode(bytes.NewReader(canonical))
		if err != nil || !reflect.DeepEqual(decoded, again) {
			t.Fatal("strict scenario roundtrip changed typed input")
		}
	})
}

func FuzzTargetTraceReplay(f *testing.F) {
	identity, err := buildinfo.FromFiles("../..")
	if err != nil {
		f.Fatal(err)
	}
	baseline, err := scenario.Run(context.Background(), campaign.KnownBadScenario(), scenario.Options{Identity: identity})
	if err != nil {
		f.Fatal(err)
	}
	data, err := json.Marshal(baseline)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte(`{"version":1,"model":"raft-trace-v1"}`))
	f.Add([]byte(`null`))
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		saved, err := scenario.DecodeTrace(bytes.NewReader(data))
		if err != nil {
			return
		}
		// This fuzz experiment has its own real deadline. Timeout/cancellation
		// can reject replay; it must never become an affirmative replay result.
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := scenario.Replay(ctx, saved, identity); err != nil {
			return
		}
		if ctx.Err() != nil || saved.Identity() != identity {
			t.Fatal("successful replay lacks source identity or finished after cancellation")
		}
		// A successful replay must agree with a separately executed fresh-seed
		// target, including output, entropy and every causal event. Digest alone
		// is insufficient even when an input author recomputes it.
		fresh, err := scenario.Run(ctx, saved.Scenario, scenario.Options{Identity: identity})
		if ctx.Err() != nil {
			return // This independent extra run has no verdict after its deadline.
		}
		if err != nil || !reflect.DeepEqual(saved, fresh) {
			t.Fatal("affirmative replay differs from actual fresh-seed target execution")
		}
	})
}
