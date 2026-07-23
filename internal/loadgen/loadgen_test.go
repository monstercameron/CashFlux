// SPDX-License-Identifier: MIT

package loadgen

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildPlanDeterministic(t *testing.T) {
	for _, s := range Scenarios() {
		a, err := BuildPlan(s, 5, 30*time.Second, 42, DefaultProfile(s))
		if err != nil {
			t.Fatalf("%s: BuildPlan a: %v", s, err)
		}
		b, err := BuildPlan(s, 5, 30*time.Second, 42, DefaultProfile(s))
		if err != nil {
			t.Fatalf("%s: BuildPlan b: %v", s, err)
		}
		if a.TotalEvents() != b.TotalEvents() {
			t.Fatalf("%s: totals differ: %d vs %d", s, a.TotalEvents(), b.TotalEvents())
		}
		for c := range a.Schedules {
			if len(a.Schedules[c]) != len(b.Schedules[c]) {
				t.Fatalf("%s client %d: lengths differ", s, c)
			}
			for i := range a.Schedules[c] {
				if a.Schedules[c][i] != b.Schedules[c][i] {
					t.Fatalf("%s client %d event %d: %v vs %v", s, c, i, a.Schedules[c][i], b.Schedules[c][i])
				}
			}
		}
	}
}

func TestBuildPlanSeedChangesSchedule(t *testing.T) {
	a, _ := BuildPlan(ScenarioSteady, 3, 30*time.Second, 1, DefaultProfile(ScenarioSteady))
	b, _ := BuildPlan(ScenarioSteady, 3, 30*time.Second, 2, DefaultProfile(ScenarioSteady))
	same := a.TotalEvents() == b.TotalEvents()
	if same {
		for c := range a.Schedules {
			for i := range a.Schedules[c] {
				if i < len(b.Schedules[c]) && a.Schedules[c][i] != b.Schedules[c][i] {
					same = false
				}
			}
		}
	}
	if same {
		t.Fatal("different seeds produced identical plans")
	}
}

func TestBuildPlanValidation(t *testing.T) {
	if _, err := BuildPlan(ScenarioSteady, 0, time.Second, 1, Profile{}); err == nil {
		t.Fatal("want error for zero clients")
	}
	if _, err := BuildPlan(ScenarioSteady, 1, 0, 1, Profile{}); err == nil {
		t.Fatal("want error for zero duration")
	}
	if _, err := BuildPlan(Scenario("nope"), 1, time.Second, 1, Profile{}); err == nil {
		t.Fatal("want error for unknown scenario")
	}
}

func TestScheduleOrderedAndBounded(t *testing.T) {
	p, err := BuildPlan(ScenarioMixed, 8, 20*time.Second, 7, DefaultProfile(ScenarioMixed))
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	for c, events := range p.Schedules {
		for i, e := range events {
			if e.At < 0 || e.At >= 20*time.Second {
				t.Fatalf("client %d event %d out of range: %v", c, i, e.At)
			}
			if i > 0 && events[i-1].At > e.At {
				t.Fatalf("client %d events unsorted at %d", c, i)
			}
		}
	}
}

func TestStormCompressesIntoWindow(t *testing.T) {
	p, err := BuildPlan(ScenarioStorm, 4, 60*time.Second, 3, DefaultProfile(ScenarioStorm))
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	limit := time.Duration(float64(60*time.Second) * stormWindow)
	for c, events := range p.Schedules {
		for _, e := range events {
			if e.At > limit {
				t.Fatalf("client %d: storm event at %v exceeds window %v", c, e.At, limit)
			}
		}
	}
}

func TestStampedeSchedulesOneReconnectPerClient(t *testing.T) {
	p, err := BuildPlan(ScenarioStampede, 6, 60*time.Second, 9, DefaultProfile(ScenarioStampede))
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	for c, events := range p.Schedules {
		n := 0
		for _, e := range events {
			if e.Op == OpReconnect {
				n++
				total := float64(60 * time.Second)
				lo := time.Duration(total * (0.5 - stampedeWindow))
				hi := time.Duration(total * (0.5 + stampedeWindow))
				if e.At < lo || e.At > hi {
					t.Fatalf("client %d reconnect at %v outside midpoint window [%v,%v]", c, e.At, lo, hi)
				}
			}
		}
		if n != 1 {
			t.Fatalf("client %d has %d reconnects, want 1", c, n)
		}
	}
}

func TestConflictPairsShareWorkspaces(t *testing.T) {
	p, err := BuildPlan(ScenarioConflict, 6, 10*time.Second, 5, DefaultProfile(ScenarioConflict))
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if p.WorkspaceIDs[0] != p.WorkspaceIDs[1] || p.WorkspaceIDs[2] != p.WorkspaceIDs[3] {
		t.Fatalf("conflict pairs not shared: %v", p.WorkspaceIDs)
	}
	if p.WorkspaceIDs[1] == p.WorkspaceIDs[2] {
		t.Fatalf("distinct pairs share a workspace: %v", p.WorkspaceIDs)
	}
	q, _ := BuildPlan(ScenarioSteady, 3, 10*time.Second, 5, DefaultProfile(ScenarioSteady))
	if q.WorkspaceIDs[0] == q.WorkspaceIDs[1] {
		t.Fatalf("steady clients share a workspace: %v", q.WorkspaceIDs)
	}
}

func TestPercentile(t *testing.T) {
	samples := []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} // sorted, ns
	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, 6}, {95, 10}, {99, 10}, {0, 1},
	}
	for _, c := range cases {
		if got := Percentile(samples, c.p); got != c.want {
			t.Errorf("Percentile(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	if got := Percentile(nil, 50); got != 0 {
		t.Errorf("Percentile(nil) = %v, want 0", got)
	}
}

func TestRecorderSnapshotAndRender(t *testing.T) {
	start := time.Now()
	r := NewRecorder(start)
	r.Record(OpPush, 10*time.Millisecond, nil)
	r.Record(OpPush, 30*time.Millisecond, errors.New("boom"))
	r.Record(OpPull, 5*time.Millisecond, nil)
	r.Count("push_accepted", 1)
	r.AddBytes(2048, 512)

	rep := r.Snapshot(start.Add(2 * time.Second))
	if rep.TotalOps() != 3 || rep.TotalErrors() != 1 {
		t.Fatalf("totals = %d ops / %d errors, want 3/1", rep.TotalOps(), rep.TotalErrors())
	}
	if rep.Counters["push_accepted"] != 1 {
		t.Fatalf("counter = %d, want 1", rep.Counters["push_accepted"])
	}
	if rep.BytesUp != 2048 || rep.BytesDown != 512 {
		t.Fatalf("bytes = %d/%d, want 2048/512", rep.BytesUp, rep.BytesDown)
	}
	text := rep.String()
	for _, want := range []string{"push", "pull", "push_accepted", "2.0KiB"} {
		if !strings.Contains(text, want) {
			t.Errorf("report text missing %q:\n%s", want, text)
		}
	}
}
