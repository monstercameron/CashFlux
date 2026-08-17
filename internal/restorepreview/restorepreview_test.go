// SPDX-License-Identifier: MIT

package restorepreview

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func side(txns, accounts int, newest time.Time) Side {
	return Side{Counts: Counts{Transactions: txns, Accounts: accounts}, Newest: newest}
}

func TestLossIsCountedAndSignedTowardTheWarning(t *testing.T) {
	p := Build(side(1200, 6, d(2026, time.August, 10)), side(900, 6, d(2026, time.July, 1)))
	if !p.AnyLoss() {
		t.Fatal("restoring a smaller dataset must report loss")
	}
	if p.LostRecords != 300 {
		t.Errorf("lost = %d, want 300", p.LostRecords)
	}
	var txnLine Line
	for _, l := range p.Lines {
		if l.Entity == "transactions" {
			txnLine = l
		}
	}
	if txnLine.Delta != -300 || !txnLine.Loses() {
		t.Errorf("transactions line = %+v, want a negative delta flagged as loss", txnLine)
	}
}

// A restore that adds 50 budgets and drops 400 transactions has not "gained 350
// things". Netting would say otherwise, and it is the wrong answer.
func TestGrowthNeverOffsetsLoss(t *testing.T) {
	now := Side{Counts: Counts{Transactions: 400, Budgets: 0}}
	backup := Side{Counts: Counts{Transactions: 0, Budgets: 50}}
	p := Build(now, backup)
	if p.LostRecords != 400 {
		t.Errorf("lost = %d, want the full 400 transactions regardless of the budgets gained", p.LostRecords)
	}
}

func TestAPureGainReportsNoLoss(t *testing.T) {
	p := Build(side(100, 2, d(2026, time.January, 1)), side(500, 4, d(2026, time.August, 1)))
	if p.AnyLoss() {
		t.Errorf("a restore that only adds reported %d lost records", p.LostRecords)
	}
	if !p.Newer {
		t.Error("a backup more recent than the current data must be flagged as newer")
	}
	if p.Stale {
		t.Error("a newer backup is not stale")
	}
}

func TestStalenessComesFromTheDATANotTheFile(t *testing.T) {
	// A file's own timestamp says when it was WRITTEN, which is not the same as
	// how current its contents are, and is trivially wrong after a copy.
	p := Build(side(100, 2, d(2026, time.August, 17)), side(100, 2, d(2026, time.July, 18)))
	if !p.Stale {
		t.Fatal("a backup 30 days behind the data must read as stale")
	}
	if p.GapDays != 30 {
		t.Errorf("gap = %d days, want 30", p.GapDays)
	}
	if p.Newer {
		t.Error("a stale backup is not also newer")
	}
}

func TestIdenticalDatesAreNeitherStaleNorNewer(t *testing.T) {
	p := Build(side(100, 2, d(2026, time.August, 1)), side(100, 2, d(2026, time.August, 1)))
	if p.Stale || p.Newer || p.GapDays != 0 {
		t.Errorf("same-date sides reported stale=%v newer=%v gap=%d", p.Stale, p.Newer, p.GapDays)
	}
}

func TestAnUnknownDateMakesNoStalenessClaim(t *testing.T) {
	// An empty dataset has no newest transaction, and inventing a comparison
	// against a zero time would report a two-thousand-year gap.
	p := Build(side(100, 2, time.Time{}), side(100, 2, d(2026, time.August, 1)))
	if p.Stale || p.Newer || p.GapDays != 0 {
		t.Errorf("an unknown date produced stale=%v newer=%v gap=%d", p.Stale, p.Newer, p.GapDays)
	}
}

func TestEntitiesNobodyUsesAreOmitted(t *testing.T) {
	// "0 → 0" rows are noise in a list somebody is reading under pressure.
	p := Build(side(10, 1, time.Time{}), side(20, 1, time.Time{}))
	for _, l := range p.Lines {
		if l.Now == 0 && l.After == 0 {
			t.Errorf("empty entity %q was listed anyway", l.Entity)
		}
	}
	if len(p.Lines) != 2 {
		t.Errorf("lines = %d, want just accounts and transactions", len(p.Lines))
	}
}

func TestLinesComeOutInAStableOrder(t *testing.T) {
	// A destructive confirmation that reshuffles between renders is one nobody
	// can read twice to check they read it right.
	full := Side{Counts: Counts{
		Accounts: 1, Transactions: 2, Categories: 3, Budgets: 4,
		Goals: 5, Tasks: 6, Holdings: 7, Rules: 8,
	}}
	want := []string{"accounts", "transactions", "categories", "budgets", "goals", "tasks", "holdings", "rules"}
	for range 3 {
		p := Build(full, full)
		if len(p.Lines) != len(want) {
			t.Fatalf("lines = %d, want %d", len(p.Lines), len(want))
		}
		for i, l := range p.Lines {
			if l.Entity != want[i] {
				t.Fatalf("line %d = %q, want %q", i, l.Entity, want[i])
			}
		}
	}
}

func TestTotalCountsEverything(t *testing.T) {
	c := Counts{Accounts: 1, Transactions: 2, Budgets: 3, Goals: 4, Categories: 5, Tasks: 6, Holdings: 7, Rules: 8}
	if got := c.Total(); got != 36 {
		t.Errorf("total = %d, want 36 — an entity missing from Total is one nobody sees lost", got)
	}
}

func TestAnEmptyComparisonIsHarmless(t *testing.T) {
	p := Build(Side{}, Side{})
	if len(p.Lines) != 0 || p.AnyLoss() || p.Stale || p.Newer {
		t.Errorf("two empty datasets produced %+v", p)
	}
}
