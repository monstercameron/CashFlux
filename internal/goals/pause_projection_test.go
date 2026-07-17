// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Pause honesty (#65): projections and the needed-per-month pace must respect a
// pause window — contributions restart at PausedUntil, not today.

func TestProjectRespectsPause(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	paused := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	g := domain.Goal{
		ID:            "g",
		TargetAmount:  money.New(100000, "USD"),
		CurrentAmount: money.New(0, "USD"),
		PausedUntil:   paused,
	}
	monthly := money.New(50000, "USD") // 2 payments

	got, ok, err := Project(g, monthly, now)
	if err != nil || !ok {
		t.Fatalf("Project err=%v ok=%t", err, ok)
	}
	// 2 payments starting at the pause end → lands one month after PausedUntil,
	// NOT one month after now.
	want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("paused projection = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}

	// Unpaused control: same goal without the pause lands from now.
	g.PausedUntil = time.Time{}
	got2, _, _ := Project(g, monthly, now)
	if !got2.Before(want) {
		t.Errorf("unpaused projection %s should land before the paused one %s", got2.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestMonthlyNeededRespectsPause(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	target := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	g := domain.Goal{
		ID:            "g",
		TargetAmount:  money.New(60000, "USD"),
		CurrentAmount: money.New(0, "USD"),
		TargetDate:    target,
	}
	perLive, ok, err := MonthlyNeeded(g, now)
	if err != nil || !ok {
		t.Fatalf("live MonthlyNeeded err=%v ok=%t", err, ok)
	}

	// Paused until October: only ~3 contributing months remain, so the ask rises.
	g.PausedUntil = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	perPaused, ok, err := MonthlyNeeded(g, now)
	if err != nil || !ok {
		t.Fatalf("paused MonthlyNeeded err=%v ok=%t", err, ok)
	}
	if perPaused.Amount <= perLive.Amount {
		t.Errorf("paused ask %d must exceed live ask %d (fewer contributing months)", perPaused.Amount, perLive.Amount)
	}

	// A pause extending past the target date leaves no contributing months —
	// there is no honest monthly figure, so ok must be false.
	g.PausedUntil = target.AddDate(0, 1, 0)
	if _, ok, err := MonthlyNeeded(g, now); err != nil || ok {
		t.Errorf("pause past target: ok=%t err=%v, want ok=false (no achievable monthly)", ok, err)
	}
}
