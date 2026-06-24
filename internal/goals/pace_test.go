// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func goalAt(current, target int64, date time.Time) domain.Goal {
	return domain.Goal{
		TargetAmount:  money.New(target, "USD"),
		CurrentAmount: money.New(current, "USD"),
		TargetDate:    date,
	}
}

func TestClassifyPace(t *testing.T) {
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	soon := now.AddDate(0, 0, 30)
	far := now.AddDate(0, 1, 0).AddDate(0, 0, 40) // > 60d out
	past := now.AddDate(0, 0, -1)

	cases := []struct {
		name string
		goal domain.Goal
		want Pace
	}{
		{"complete", goalAt(1000, 1000, far), PaceComplete},
		{"overfunded is complete", goalAt(1200, 1000, far), PaceComplete},
		{"undated low", goalAt(100, 1000, time.Time{}), PaceNone},
		{"undated near done", goalAt(950, 1000, time.Time{}), PaceFinalStretch},
		{"overdue", goalAt(500, 1000, past), PaceOverdue},
		{"final stretch beats due-soon", goalAt(950, 1000, soon), PaceFinalStretch},
		{"due soon", goalAt(500, 1000, soon), PaceDueSoon},
		{"on track", goalAt(500, 1000, far), PaceOnTrack},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyPace(c.goal, now); got != c.want {
				t.Fatalf("ClassifyPace = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLessForList(t *testing.T) {
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	d1 := now.AddDate(0, 1, 0)
	d2 := now.AddDate(0, 2, 0)

	// Dated sorts before undated.
	dated := domain.Goal{Name: "z", TargetDate: d2, TargetAmount: money.New(100, "USD")}
	undated := domain.Goal{Name: "a", TargetAmount: money.New(100, "USD")}
	if !LessForList(dated, undated) {
		t.Fatal("dated goal should sort before undated")
	}

	// Nearer deadline first.
	near := goalAt(0, 100, d1)
	near.Name = "near"
	farG := goalAt(0, 100, d2)
	farG.Name = "far"
	if !LessForList(near, farG) {
		t.Fatal("nearer deadline should sort first")
	}

	// Same date → higher percent first.
	hi := goalAt(90, 100, d1)
	hi.Name = "hi"
	lo := goalAt(10, 100, d1)
	lo.Name = "lo"
	if !LessForList(hi, lo) {
		t.Fatal("higher percent should sort first at equal date")
	}
}
