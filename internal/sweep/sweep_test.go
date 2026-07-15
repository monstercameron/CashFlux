// SPDX-License-Identifier: MIT

package sweep

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func rule() domain.SweepRule {
	return domain.SweepRule{
		ID: "s1", SourceAccountID: "chk", DestAccountID: "sav",
		KeepMinor: 3000_00, Cadence: domain.SweepMonthly, Enabled: true,
	}
}

func TestIsDue(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	r := rule()
	if !IsDue(r, now) {
		t.Error("never-proposed enabled rule should be due")
	}
	r.LastProposed = now.AddDate(0, 0, -10).Format("2006-01-02")
	if IsDue(r, now) {
		t.Error("10 days into a monthly cadence should not be due")
	}
	r.LastProposed = now.AddDate(0, 0, -31).Format("2006-01-02")
	if !IsDue(r, now) {
		t.Error("31 days into a monthly cadence should be due")
	}
	r.Enabled = false
	if IsDue(r, now) {
		t.Error("disabled rule is never due")
	}
}

func TestExcessRespectsEarmarks(t *testing.T) {
	r := rule()
	// $5,000 balance, keep $3,000, $500 earmarked → sweep $1,500 (earmark protected).
	got := ExcessMinor(r, Inputs{BalanceMinor: 5000_00, EarmarkedMinor: 500_00})
	if got != 1500_00 {
		t.Errorf("excess = %d, want %d", got, 1500_00)
	}
	// Balance below keep → no excess.
	if got := ExcessMinor(r, Inputs{BalanceMinor: 2000_00}); got != 0 {
		t.Errorf("under-floor excess = %d, want 0", got)
	}
}

func TestPropose(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	r := rule()
	p, ok := Propose(r, Inputs{BalanceMinor: 5000_00, Currency: "USD"}, now)
	if !ok {
		t.Fatal("expected a proposal")
	}
	if p.AmountMinor != 2000_00 || p.SourceAccountID != "chk" || p.DestAccountID != "sav" {
		t.Errorf("proposal = %+v", p)
	}

	// Same source/dest → no proposal.
	bad := r
	bad.DestAccountID = "chk"
	if _, ok := Propose(bad, Inputs{BalanceMinor: 5000_00}, now); ok {
		t.Error("same-account sweep should not propose")
	}

	// Excess below MinSweepMinor → suppressed.
	r.MinSweepMinor = 500_00
	if _, ok := Propose(r, Inputs{BalanceMinor: 3100_00, Currency: "USD"}, now); ok {
		t.Error("trivial excess below threshold should be suppressed")
	}

	// Not due → no proposal.
	r.MinSweepMinor = 0
	r.LastProposed = now.AddDate(0, 0, -5).Format("2006-01-02")
	if _, ok := Propose(r, Inputs{BalanceMinor: 9000_00, Currency: "USD"}, now); ok {
		t.Error("rule not due should not propose")
	}
}
