// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPauseGoalSetsPausedUntil(t *testing.T) {
	app := newAllocApp(t)
	allocSeedGoal(t, app, "g1", "Trip", 100000, 0)
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	g, err := app.PauseGoal("g1", 2, from)
	if err != nil {
		t.Fatalf("PauseGoal: %v", err)
	}
	want := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if !g.PausedUntil.Equal(want) {
		t.Errorf("PausedUntil = %v, want %v", g.PausedUntil, want)
	}
	if !g.IsPaused(from) {
		t.Error("goal should read paused at from")
	}

	// Resume clears the pause.
	g, err = app.ResumeGoal("g1")
	if err != nil {
		t.Fatalf("ResumeGoal: %v", err)
	}
	if !g.PausedUntil.IsZero() {
		t.Errorf("PausedUntil after resume = %v, want zero", g.PausedUntil)
	}
}

func TestSweepEndedGoalPausesResumesAndNudgesOnce(t *testing.T) {
	app := newAllocApp(t)
	allocSeedGoal(t, app, "g1", "Trip", 100000, 0)
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if _, err := app.PauseGoal("g1", 2, from); err != nil {
		t.Fatalf("PauseGoal: %v", err)
	}

	// Before the pause ends: nothing to sweep.
	beforeEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	n, err := app.SweepEndedGoalPauses(beforeEnd, nil)
	if err != nil {
		t.Fatalf("SweepEndedGoalPauses: %v", err)
	}
	if n != 0 {
		t.Errorf("swept %d before pause end, want 0", n)
	}

	// After the pause ends: exactly one resume + one nudge task.
	afterEnd := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	n, err = app.SweepEndedGoalPauses(afterEnd, func(name string) string { return "Resume " + name })
	if err != nil {
		t.Fatalf("SweepEndedGoalPauses: %v", err)
	}
	if n != 1 {
		t.Fatalf("swept %d at pause end, want 1", n)
	}
	var nudges []domain.Task
	for _, tk := range app.Tasks() {
		if tk.Source == domain.SourceNudge && tk.RelatedType == domain.RelatedGoal && tk.RelatedID == "g1" {
			nudges = append(nudges, tk)
		}
	}
	if len(nudges) != 1 || nudges[0].Title != "Resume Trip" {
		t.Fatalf("nudge tasks = %+v, want one titled 'Resume Trip'", nudges)
	}

	// Running the sweep again does not re-fire (PausedUntil is cleared).
	n, err = app.SweepEndedGoalPauses(afterEnd, nil)
	if err != nil {
		t.Fatalf("second SweepEndedGoalPauses: %v", err)
	}
	if n != 0 {
		t.Errorf("second sweep swept %d, want 0 (once-guard)", n)
	}
}
