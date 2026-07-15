// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
)

// PauseGoal pauses the goal identified by goalID for the given number of whole
// months, counted from `from` (GL7). While paused the goal's pace stops scolding
// and its contributions aren't expected — pausing is a chosen state, not a
// failure. A non-positive months value clears any existing pause (a resume). The
// stored PausedUntil is derived once via goals.PausedUntilFrom so the UI preview
// and the persisted value agree.
func (a *App) PauseGoal(goalID string, months int, from time.Time) (domain.Goal, error) {
	for _, g := range a.Goals() {
		if g.ID != goalID {
			continue
		}
		g.PausedUntil = goals.PausedUntilFrom(from, months)
		if err := a.PutGoal(g); err != nil {
			return domain.Goal{}, fmt.Errorf("appstate: pause goal: %w", err)
		}
		a.log.Info("goal paused", "goal", goalID, "months", months, "until", g.PausedUntil)
		return g, nil
	}
	return domain.Goal{}, fmt.Errorf("appstate: pause goal: goal %q not found", goalID)
}

// ResumeGoal clears a goal's pause immediately (an explicit "Resume goal"),
// returning the goal to normal pacing.
func (a *App) ResumeGoal(goalID string) (domain.Goal, error) {
	return a.PauseGoal(goalID, 0, time.Time{})
}

// SweepEndedGoalPauses clears any goal whose pause has elapsed (PausedUntil set
// and not after `now`) and, for each, files ONE gentle, dismissible nudge task
// linked to the goal so it resurfaces exactly once at pause end (GL7's guardrail
// against quiet abandonment). Clearing PausedUntil is the once-guard: a swept
// goal has a zero PausedUntil, so a later sweep won't re-fire. It returns the
// number of goals resumed. The nudge is framed as an invitation to resume, never
// a scold. Idempotent — safe to call on every surface entry.
//
// titleFor builds the nudge task's title from the goal's name; the caller passes
// it so the user-facing copy stays in the UI layer (uistate.T) rather than being
// hard-coded here. A nil titleFor falls back to the goal name.
func (a *App) SweepEndedGoalPauses(now time.Time, titleFor func(goalName string) string) (int, error) {
	if titleFor == nil {
		titleFor = func(name string) string { return name }
	}
	resumed := 0
	for _, g := range a.Goals() {
		if g.PausedUntil.IsZero() || g.PausedUntil.After(now) {
			continue
		}
		g.PausedUntil = time.Time{}
		if err := a.PutGoal(g); err != nil {
			return resumed, fmt.Errorf("appstate: sweep ended pauses: %w", err)
		}
		nudge := domain.Task{
			ID:          id.New(),
			Title:       titleFor(g.Name),
			Status:      domain.StatusOpen,
			Priority:    domain.PriorityLow,
			Source:      domain.SourceNudge,
			RelatedType: domain.RelatedGoal,
			RelatedID:   g.ID,
		}
		if err := a.PutTask(nudge); err != nil {
			return resumed, fmt.Errorf("appstate: sweep ended pauses: nudge task: %w", err)
		}
		resumed++
	}
	if resumed > 0 {
		a.log.Info("goal pauses ended", "count", resumed)
	}
	return resumed, nil
}
