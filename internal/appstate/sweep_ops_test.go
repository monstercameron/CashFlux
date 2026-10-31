// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/sweep"
)

// sweepFixture returns an app with a funded checking account (5,000.00), an empty savings account
// and a third account, all in USD.
func sweepFixture(t *testing.T) *App {
	t.Helper()
	app := newApp(t, false)
	for _, ac := range []domain.Account{
		{ID: "chk", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(500000, "USD")},
		{ID: "sav", Name: "Savings", Currency: "USD", Type: domain.TypeSavings, Class: domain.ClassAsset,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "bro", Name: "Brokerage", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatalf("PutAccount %s: %v", ac.ID, err)
		}
	}
	return app
}

func TestPutSweepRuleValidatesAndFillsDefaults(t *testing.T) {
	tests := []struct {
		name    string
		rule    domain.SweepRule
		wantErr string
	}{
		{"missing source", domain.SweepRule{DestAccountID: "sav"}, "needs a source and a destination"},
		{"missing destination", domain.SweepRule{SourceAccountID: "chk"}, "needs a source and a destination"},
		{"blank ids are trimmed away", domain.SweepRule{SourceAccountID: "  ", DestAccountID: " "}, "needs a source and a destination"},
		{"same account both ends", domain.SweepRule{SourceAccountID: "chk", DestAccountID: "chk"}, "must differ"},
		{"unknown source", domain.SweepRule{SourceAccountID: "ghost", DestAccountID: "sav"}, "source account not found"},
		{"unknown destination", domain.SweepRule{SourceAccountID: "chk", DestAccountID: "ghost"}, "destination account not found"},
		{"negative keep floor", domain.SweepRule{SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: -1}, "may not be negative"},
		{"negative minimum sweep", domain.SweepRule{SourceAccountID: "chk", DestAccountID: "sav", MinSweepMinor: -5}, "may not be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := sweepFixture(t)
			_, err := app.PutSweepRule(tt.rule)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("PutSweepRule error = %v, want one containing %q", err, tt.wantErr)
			}
			if n := len(app.SweepRules()); n != 0 {
				t.Fatalf("a rejected rule was stored: %d rules", n)
			}
		})
	}

	t.Run("a new rule gets an id and a monthly cadence", func(t *testing.T) {
		app := sweepFixture(t)
		got, err := app.PutSweepRule(domain.SweepRule{SourceAccountID: " chk ", DestAccountID: " sav ", KeepMinor: 300000, Enabled: true})
		if err != nil {
			t.Fatalf("PutSweepRule: %v", err)
		}
		if got.ID == "" || got.Cadence != domain.SweepMonthly || got.SourceAccountID != "chk" || got.DestAccountID != "sav" {
			t.Fatalf("saved rule = %+v, want an id, monthly cadence and trimmed account ids", got)
		}
		rules := app.SweepRules()
		if len(rules) != 1 || rules[0] != got {
			t.Fatalf("stored rules = %+v, want exactly %+v", rules, got)
		}
	})

	t.Run("an explicit cadence and id are kept and the rule is updated in place", func(t *testing.T) {
		app := sweepFixture(t)
		first, err := app.PutSweepRule(domain.SweepRule{ID: "rule-1", SourceAccountID: "chk", DestAccountID: "sav", Cadence: domain.SweepWeekly})
		if err != nil {
			t.Fatalf("PutSweepRule: %v", err)
		}
		if first.ID != "rule-1" || first.Cadence != domain.SweepWeekly {
			t.Fatalf("saved rule = %+v, want id rule-1 and weekly cadence", first)
		}
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "rule-1", SourceAccountID: "chk", DestAccountID: "sav", Cadence: domain.SweepQuarterly}); err != nil {
			t.Fatalf("PutSweepRule update: %v", err)
		}
		rules := app.SweepRules()
		if len(rules) != 1 || rules[0].Cadence != domain.SweepQuarterly {
			t.Fatalf("stored rules = %+v, want one quarterly rule", rules)
		}
	})

	t.Run("an unknown cadence falls back to monthly", func(t *testing.T) {
		app := sweepFixture(t)
		got, err := app.PutSweepRule(domain.SweepRule{SourceAccountID: "chk", DestAccountID: "sav", Cadence: "fortnightly"})
		if err != nil || got.Cadence != domain.SweepMonthly {
			t.Fatalf("PutSweepRule = %+v, %v; want monthly cadence", got, err)
		}
	})
}

func TestSweepRulesComeBackSortedBySourceThenID(t *testing.T) {
	app := sweepFixture(t)
	for _, r := range []domain.SweepRule{
		{ID: "b", SourceAccountID: "sav", DestAccountID: "chk"},
		{ID: "z", SourceAccountID: "chk", DestAccountID: "sav"},
		{ID: "a", SourceAccountID: "sav", DestAccountID: "bro"},
		{ID: "m", SourceAccountID: "chk", DestAccountID: "bro"},
	} {
		if _, err := app.PutSweepRule(r); err != nil {
			t.Fatalf("PutSweepRule %s: %v", r.ID, err)
		}
	}
	var got []string
	for _, r := range app.SweepRules() {
		got = append(got, r.SourceAccountID+"/"+r.ID)
	}
	want := "chk/m chk/z sav/a sav/b"
	if strings.Join(got, " ") != want {
		t.Fatalf("order = %q, want %q", strings.Join(got, " "), want)
	}
}

func TestDeleteSweepRuleRemovesOnlyThatRule(t *testing.T) {
	app := sweepFixture(t)
	for _, id := range []string{"keep", "drop"} {
		if _, err := app.PutSweepRule(domain.SweepRule{ID: id, SourceAccountID: "chk", DestAccountID: "sav"}); err != nil {
			t.Fatalf("PutSweepRule %s: %v", id, err)
		}
	}
	if err := app.DeleteSweepRule("drop"); err != nil {
		t.Fatalf("DeleteSweepRule: %v", err)
	}
	rules := app.SweepRules()
	if len(rules) != 1 || rules[0].ID != "keep" {
		t.Fatalf("rules after delete = %+v, want only keep", rules)
	}
	if err := app.DeleteSweepRule("never-existed"); err != nil {
		t.Fatalf("deleting an unknown rule should be a no-op, got %v", err)
	}
	if len(app.SweepRules()) != 1 {
		t.Fatal("deleting an unknown rule changed the stored rules")
	}
}

func TestSweepProposalsMoveOnlyTheExcessAboveFloorAndGoalEarmarks(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	p := props[0]
	if p.RuleID != "r1" || p.SourceAccountID != "chk" || p.DestAccountID != "sav" ||
		p.AmountMinor != 200000 || p.BalanceMinor != 500000 || p.KeepMinor != 300000 || p.Currency != "USD" {
		t.Fatalf("proposal = %+v, want 2,000.00 USD from chk to sav", p)
	}

	// Money earmarked for a goal in the source account is never swept.
	goal := domain.Goal{
		ID: "g1", Name: "Holiday", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		TargetAmount: money.New(100000, "USD"), CurrentAmount: money.Zero("USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "chk", Amount: money.New(50000, "USD")}},
	}
	if err := app.PutGoal(goal); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	props = app.SweepProposals(now)
	if len(props) != 1 || props[0].AmountMinor != 150000 || props[0].EarmarkedMinor != 50000 {
		t.Fatalf("proposals with an earmark = %+v, want 1,500.00 swept and 500.00 earmarked", props)
	}
}

func TestSweepProposalsSkipRulesThatCannotFire(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "off", SourceAccountID: "chk", DestAccountID: "sav", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "empty-source", SourceAccountID: "sav", DestAccountID: "chk", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// A rule whose source account was removed after the rule was saved is skipped, not fatal.
	if err := app.Store().PutSweepRule(domain.SweepRule{ID: "orphan", SourceAccountID: "ghost", DestAccountID: "sav", Enabled: true, Cadence: domain.SweepMonthly}); err != nil {
		t.Fatal(err)
	}
	if props := app.SweepProposals(now); len(props) != 0 {
		t.Fatalf("proposals = %+v, want none for a disabled rule, an empty source and an orphaned rule", props)
	}
}

func TestMarkSweepProposedStampsTheCadenceClock(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.MarkSweepProposed("r1", now); err != nil {
		t.Fatalf("MarkSweepProposed: %v", err)
	}
	got, ok, err := app.Store().GetSweepRule("r1")
	if err != nil || !ok {
		t.Fatalf("GetSweepRule: ok=%v err=%v", ok, err)
	}
	if got.LastProposed != now.Format(time.RFC3339) || got.KeepMinor != 300000 || !got.Enabled {
		t.Fatalf("rule after marking = %+v, want LastProposed %s and the other fields kept", got, now.Format(time.RFC3339))
	}
	// The same sweep is not proposed again inside the cadence window, but is once the window passes.
	if props := app.SweepProposals(now.AddDate(0, 0, 29)); len(props) != 0 {
		t.Fatalf("proposals 29 days later = %+v, want none", props)
	}
	if props := app.SweepProposals(now.AddDate(0, 0, 30)); len(props) != 1 {
		t.Fatalf("proposals 30 days later = %+v, want one", props)
	}
	if err := app.MarkSweepProposed("ghost", now); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("MarkSweepProposed on an unknown rule = %v, want a not-found error", err)
	}
}

func TestApproveSweepProposalMovesMoneyThenMarksTheRule(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now); err != nil {
		t.Fatalf("ApproveSweepProposal: %v", err)
	}

	var out, in int64
	for _, tx := range app.Transactions() {
		if tx.Desc != "Surplus sweep" {
			t.Fatalf("unexpected transaction %+v", tx)
		}
		switch tx.AccountID {
		case "chk":
			out = tx.Amount.Amount
		case "sav":
			in = tx.Amount.Amount
		}
	}
	if out != -200000 || in != 200000 {
		t.Fatalf("transfer legs = %d out, %d in; want -200000 and 200000", out, in)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed != "2026-10-31T00:00:00Z" {
		t.Fatalf("LastProposed = %q, want %q", rule.LastProposed, "2026-10-31T00:00:00Z")
	}
	if props := app.SweepProposals(now.AddDate(0, 0, 1)); len(props) != 0 {
		t.Fatalf("an approved sweep was proposed again: %+v", props)
	}
}

func TestApproveSweepProposalFailedTransferLeavesTheRuleOpen(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	err := app.ApproveSweepProposal(sweep.Proposal{RuleID: "r1", SourceAccountID: "chk", DestAccountID: "ghost", AmountMinor: 1000}, "Sweep", now)
	if err == nil || !strings.Contains(err.Error(), "sweep transfer") || errors.Is(err, ErrSweepPostedUnmarked) {
		t.Fatalf("ApproveSweepProposal = %v, want a plain sweep transfer error", err)
	}
	if n := len(app.Transactions()); n != 0 {
		t.Fatalf("%d transactions posted by a failed approval", n)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed != "" {
		t.Fatalf("LastProposed = %q, want it untouched so the proposal can be retried", rule.LastProposed)
	}
}

func TestApproveSweepProposalRefusesAMissingRuleBeforeMovingMoney(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	err := app.ApproveSweepProposal(sweep.Proposal{RuleID: "deleted-rule", SourceAccountID: "chk", DestAccountID: "sav", AmountMinor: 1000}, "Sweep", now)
	if !errors.Is(err, ErrSweepProposalStale) || errors.Is(err, ErrSweepPostedUnmarked) {
		t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale and no money moved", err)
	}
	if n := len(app.Transactions()); n != 0 {
		t.Fatalf("%d transactions posted for a proposal whose rule is gone", n)
	}
}

func TestSweepRuleWritesReportStoreFailures(t *testing.T) {
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatalf("closing the store: %v", err)
	}
	if err := app.DeleteSweepRule("r1"); err == nil || !strings.Contains(err.Error(), "delete sweep rule") {
		t.Errorf("DeleteSweepRule on a closed store = %v, want a wrapped delete error", err)
	}
	if err := app.MarkSweepProposed("r1", time.Now()); err == nil || !strings.Contains(err.Error(), "load sweep rule") {
		t.Errorf("MarkSweepProposed on a closed store = %v, want a wrapped load error", err)
	}
}

func TestApproveSweepProposalCommitsRowsReceiptAndStampTogether(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now); err != nil {
		t.Fatal(err)
	}
	receipts, err := app.Store().ListAllTransferReceipts()
	if err != nil || len(receipts) != 1 {
		t.Fatalf("receipts = %+v, %v; want one", receipts, err)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed != "2026-10-31T00:00:00Z" {
		t.Fatalf("LastProposed = %q, want the approval day at midnight UTC", rule.LastProposed)
	}

	// The stamp is lost (an older build, a manual edit); retrying repairs it and posts nothing more.
	rule.LastProposed = ""
	if err := app.Store().PutSweepRule(rule); err != nil {
		t.Fatal(err)
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if n := len(app.Transactions()); n != 2 {
		t.Fatalf("%d transactions after the retry, want the two legs of one transfer", n)
	}
	rule, _, _ = app.Store().GetSweepRule("r1")
	// Transfers are dated by calendar day, so the repair stamps the posted transfer's day.
	if want := now.Truncate(24 * time.Hour).Format(time.RFC3339); rule.LastProposed != want {
		t.Fatalf("LastProposed after retry = %q, want it repaired to the transfer's date %q", rule.LastProposed, want)
	}
}

func TestApproveSweepProposalRefusesADisabledRule(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if err := app.ApproveSweepProposal(props[0], "Sweep", now); !errors.Is(err, ErrSweepProposalStale) {
		t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale", err)
	}
	if n := len(app.Transactions()); n != 0 {
		t.Fatalf("%d transactions posted for a disabled rule", n)
	}
}

func TestSweepProposalsReserveGoalMoneyAndProposeNothingWhenGoalsCannotBeRead(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if len(app.SweepProposals(now)) != 1 {
		t.Fatal("expected a proposal while the store is readable")
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if got := app.SweepProposals(now); len(got) != 0 {
		t.Fatalf("proposals with an unreadable store = %+v, want none", got)
	}
}

func TestMarkSweepProposedUsesTheClockForAZeroTime(t *testing.T) {
	clock := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, clock)
	for _, ac := range []domain.Account{refsAccount("a"), refsAccount("b")} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "a", DestAccountID: "b", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.MarkSweepProposed("r1", time.Time{}); err != nil {
		t.Fatal(err)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed != clock.Format(time.RFC3339) {
		t.Fatalf("LastProposed = %q, want the clock time", rule.LastProposed)
	}
}

// TestSweepPairErrorMatchesPutSweepRule checks that the exported pair rule and the rule store agree
// on every ordered pair of a mixed set of accounts, so a picker built on SweepPairError offers
// exactly the pairs PutSweepRule accepts.
func TestSweepPairErrorMatchesPutSweepRule(t *testing.T) {
	app := sweepFixture(t)
	for _, ac := range []domain.Account{
		{ID: "card", Name: "Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "eur", Name: "Euro", Currency: "EUR", Type: domain.TypeChecking, Class: domain.ClassAsset,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "old", Name: "Old", Currency: "USD", Type: domain.TypeSavings, Class: domain.ClassAsset,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Archived: true},
	} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatalf("PutAccount %s: %v", ac.ID, err)
		}
	}
	accounts := app.Accounts()
	if len(accounts) != 6 {
		t.Fatalf("fixture has %d accounts, want 6", len(accounts))
	}
	wantErr := map[string]string{
		"chk>chk":  "must differ",
		"card>chk": "liability",
		"chk>eur":  "same currency",
		"old>chk":  "archived",
		"chk>old":  "archived",
	}
	accepted := 0
	for _, src := range accounts {
		for _, dest := range accounts {
			key := src.ID + ">" + dest.ID
			pairErr := SweepPairError(src, dest)
			_, putErr := app.PutSweepRule(domain.SweepRule{SourceAccountID: src.ID, DestAccountID: dest.ID, Enabled: true})
			if (pairErr == nil) != (putErr == nil) {
				t.Errorf("%s: SweepPairError = %v but PutSweepRule = %v", key, pairErr, putErr)
			}
			if want, ok := wantErr[key]; ok && (pairErr == nil || !strings.Contains(pairErr.Error(), want)) {
				t.Errorf("%s: SweepPairError = %v, want one containing %q", key, pairErr, want)
			}
			if pairErr == nil {
				accepted++
			}
		}
	}
	// Active USD assets are chk, sav and bro: 3 sources x 2 other destinations, plus the card as a
	// destination from each of them.
	if want := 3*2 + 3; accepted != want {
		t.Errorf("accepted pairs = %d, want %d", accepted, want)
	}
}

func TestApproveSweepProposalPostsAtMostOncePerCadenceWindow(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now); err != nil {
		t.Fatal(err)
	}
	smaller := props[0]
	smaller.AmountMinor -= 50000
	if err := app.ApproveSweepProposal(smaller, "Surplus sweep", now.Add(time.Hour)); !errors.Is(err, ErrSweepAlreadyPosted) {
		t.Fatalf("a second approval of a different amount in the same window = %v, want ErrSweepAlreadyPosted", err)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed == "" {
		t.Fatal("LastProposed is empty, want the rule still stamped for the sweep that posted")
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now.Add(2*time.Hour)); !errors.Is(err, ErrSweepAlreadyPosted) {
		t.Fatalf("re-approving the amount that posted = %v, want ErrSweepAlreadyPosted", err)
	}
	if after, _, _ := app.Store().GetSweepRule("r1"); after.LastProposed != rule.LastProposed {
		t.Fatalf("LastProposed = %q after the refused replays, want %q", after.LastProposed, rule.LastProposed)
	}
	if n := len(app.Transactions()); n != 2 {
		t.Fatalf("%d transactions after two approvals of different amounts, want the two legs of one transfer", n)
	}
	if got := bookedBalance(t, app, "chk"); got != 300000 {
		t.Fatalf("checking = %d, want 300000", got)
	}
}

func TestApproveSweepProposalRefusesARuleThatIsNotDueOrBelowItsMinimum(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	t.Run("not due", func(t *testing.T) {
		app := sweepFixture(t)
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		props := app.SweepProposals(now)
		if len(props) != 1 {
			t.Fatalf("proposals = %+v, want one", props)
		}
		if err := app.MarkSweepProposed("r1", now.AddDate(0, 0, -3)); err != nil {
			t.Fatal(err)
		}
		if err := app.ApproveSweepProposal(props[0], "Sweep", now); !errors.Is(err, ErrSweepProposalStale) {
			t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale", err)
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d transactions posted for a rule that is not due", n)
		}
	})
	t.Run("below the minimum", func(t *testing.T) {
		app := sweepFixture(t)
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, MinSweepMinor: 150000, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		props := app.SweepProposals(now)
		if len(props) != 1 {
			t.Fatalf("proposals = %+v, want one", props)
		}
		small := props[0]
		small.AmountMinor = 100000
		if err := app.ApproveSweepProposal(small, "Sweep", now); !errors.Is(err, ErrSweepProposalStale) {
			t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale", err)
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d transactions posted below the minimum", n)
		}
	})
}

func TestPutSweepRuleKeepsTheStoredLastProposedStamp(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 {
		t.Fatalf("proposals = %+v, want one", props)
	}
	if err := app.ApproveSweepProposal(props[0], "Surplus sweep", now); err != nil {
		t.Fatal(err)
	}
	want := "2026-10-31T00:00:00Z"

	// An edit from a fresh struct (no LastProposed) must not rewind the cadence clock.
	saved, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 100000, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if saved.LastProposed != want {
		t.Fatalf("returned LastProposed = %q, want %q", saved.LastProposed, want)
	}
	rule, _, _ := app.Store().GetSweepRule("r1")
	if rule.LastProposed != want || rule.KeepMinor != 100000 {
		t.Fatalf("stored rule = %+v, want the edit kept and LastProposed %q", rule, want)
	}
	if props := app.SweepProposals(now.AddDate(0, 0, 1)); len(props) != 0 {
		t.Fatalf("an edited rule was proposed again inside its cadence window: %+v", props)
	}
}

// sweepWindowStart returns 09:00 UTC on the first day of the 30-day cadence window that contains day,
// so a test can replay a monthly sweep later in the same window without crossing into the next one.
func sweepWindowStart(day time.Time) time.Time {
	days := day.UTC().Unix() / 86400
	return time.Unix(days/30*30*86400, 0).UTC().Add(9 * time.Hour)
}

func TestApproveSweepProposalReplayNeverMovesTheCadenceClock(t *testing.T) {
	base := sweepWindowStart(time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	replay := base.AddDate(0, 0, 19)
	if sweepOperationID(domain.SweepRule{ID: "r1", Cadence: domain.SweepMonthly}, base) !=
		sweepOperationID(domain.SweepRule{ID: "r1", Cadence: domain.SweepMonthly}, replay) {
		t.Fatal("test setup: the replay fell into a different cadence window")
	}
	setup := func(t *testing.T) (*App, sweep.Proposal) {
		t.Helper()
		app := sweepFixture(t)
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Cadence: domain.SweepMonthly, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		props := app.SweepProposals(base)
		if len(props) != 1 {
			t.Fatalf("proposals = %+v, want one", props)
		}
		if err := app.ApproveSweepProposal(props[0], "Surplus sweep", base); err != nil {
			t.Fatal(err)
		}
		return app, props[0]
	}
	want := "2026-10-04T00:00:00Z"

	t.Run("a stamped replay is refused and leaves the stamp alone", func(t *testing.T) {
		app, prop := setup(t)
		if err := app.ApproveSweepProposal(prop, "Surplus sweep", replay); !errors.Is(err, ErrSweepAlreadyPosted) {
			t.Fatalf("replay = %v, want ErrSweepAlreadyPosted", err)
		}
		rule, _, _ := app.Store().GetSweepRule("r1")
		if rule.LastProposed != want {
			t.Fatalf("LastProposed = %q after the replay, want %q", rule.LastProposed, want)
		}
		if n := len(app.Transactions()); n != 2 {
			t.Fatalf("%d transactions, want the two legs of one transfer", n)
		}
	})

	for name, lost := range map[string]string{
		"a missing stamp":                         "",
		"a stamp that is older than the transfer": base.AddDate(0, 0, -5).Format(time.RFC3339),
	} {
		t.Run("repairing "+name+" uses the transfer's date, not the replay's time", func(t *testing.T) {
			app, prop := setup(t)
			rule, _, _ := app.Store().GetSweepRule("r1")
			rule.LastProposed = lost
			if err := app.Store().PutSweepRule(rule); err != nil {
				t.Fatal(err)
			}
			if err := app.ApproveSweepProposal(prop, "Surplus sweep", replay); err != nil {
				t.Fatalf("repair replay = %v, want nil", err)
			}
			rule, _, _ = app.Store().GetSweepRule("r1")
			if wantDay := base.Truncate(24 * time.Hour).Format(time.RFC3339); rule.LastProposed != wantDay {
				t.Fatalf("LastProposed = %q after the repair, want the transfer date %q", rule.LastProposed, wantDay)
			}
			if err := app.ApproveSweepProposal(prop, "Surplus sweep", replay); !errors.Is(err, ErrSweepAlreadyPosted) {
				t.Fatalf("replay after the repair = %v, want ErrSweepAlreadyPosted", err)
			}
			if n := len(app.Transactions()); n != 2 {
				t.Fatalf("%d transactions, want the two legs of one transfer", n)
			}
		})
	}
}
