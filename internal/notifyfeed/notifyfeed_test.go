// SPDX-License-Identifier: MIT

package notifyfeed

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
)

func TestLargeTransactionCandidates(t *testing.T) {
	since := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	text := func(desc string, amount int64) (string, string) {
		return "Large charge: " + desc, fmt.Sprintf("%d", amount)
	}
	txns := []domain.Transaction{
		{ID: "a", Desc: "TV", Amount: money.New(-60000, "USD"), Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},    // big, in window
		{ID: "b", Desc: "Coffee", Amount: money.New(-500, "USD"), Date: time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC)},  // small
		{ID: "c", Desc: "Old TV", Amount: money.New(-90000, "USD"), Date: time.Date(2026, time.May, 30, 0, 0, 0, 0, time.UTC)}, // big but before since
		{ID: "d", Desc: "Bonus", Amount: money.New(80000, "USD"), Date: time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC)},  // income, excluded
	}
	got, err := LargeTransactionCandidates("rule-large", txns, 50000, since, rates, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1 (the in-window big expense): %+v", len(got), got)
	}
	c := got[0]
	if c.RuleID != "rule-large" || c.Event != notify.EventLargeTransaction {
		t.Errorf("RuleID/Event = %q/%q", c.RuleID, c.Event)
	}
	if c.OccurrenceKey != "txn:a" {
		t.Errorf("OccurrenceKey = %q, want txn:a", c.OccurrenceKey)
	}
	if c.Severity != notify.SeverityWarning {
		t.Errorf("Severity = %v, want warning", c.Severity)
	}
}

func TestLargeTransactionCandidatesZeroThreshold(t *testing.T) {
	txns := []domain.Transaction{{ID: "a", Amount: money.New(-99999, "USD"), Date: time.Now()}}
	got, err := LargeTransactionCandidates("r", txns, 0, time.Time{}, currency.Rates{Base: "USD"},
		func(string, int64) (string, string) { return "", "" })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("zero threshold should yield nothing, got %+v", got)
	}
}

func TestBackupCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)
	text := func(days int) (string, string) {
		return "Back up your data", fmt.Sprintf("%d days since your last backup", days)
	}

	t.Run("not due yields nothing", func(t *testing.T) {
		last := now.AddDate(0, 0, -3) // weekly cadence, only 3 days ago
		if got := BackupCandidates("rule-backup", backup.Weekly, last, now, text); got != nil {
			t.Errorf("got %d candidates, want none: %+v", len(got), got)
		}
	})

	t.Run("off never fires", func(t *testing.T) {
		if got := BackupCandidates("rule-backup", backup.Off, time.Time{}, now, text); got != nil {
			t.Errorf("Off should yield no candidates, got %+v", got)
		}
	})

	t.Run("due monthly fires one, keyed by month", func(t *testing.T) {
		last := now.AddDate(0, -2, 0) // two months ago → overdue
		got := BackupCandidates("rule-backup", backup.Monthly, last, now, text)
		if len(got) != 1 {
			t.Fatalf("got %d candidates, want 1: %+v", len(got), got)
		}
		c := got[0]
		if c.RuleID != "rule-backup" || c.Event != notify.EventBackupDue {
			t.Errorf("RuleID/Event = %q/%q", c.RuleID, c.Event)
		}
		if c.Severity != notify.SeverityInfo {
			t.Errorf("Severity = %v, want info", c.Severity)
		}
		if want := "backup@" + notify.MonthKey(now); c.OccurrenceKey != want {
			t.Errorf("OccurrenceKey = %q, want %q", c.OccurrenceKey, want)
		}
		if c.Title != "Back up your data" {
			t.Errorf("Title = %q", c.Title)
		}
	})

	t.Run("due weekly keyed by ISO week", func(t *testing.T) {
		last := now.AddDate(0, 0, -10) // overdue for weekly
		got := BackupCandidates("rule-backup", backup.Weekly, last, now, text)
		if len(got) != 1 {
			t.Fatalf("got %d candidates, want 1", len(got))
		}
		if want := "backup@" + notify.WeekKey(now); got[0].OccurrenceKey != want {
			t.Errorf("OccurrenceKey = %q, want %q", got[0].OccurrenceKey, want)
		}
	})

	t.Run("never backed up is due immediately", func(t *testing.T) {
		got := BackupCandidates("rule-backup", backup.Monthly, time.Time{}, now, text)
		if len(got) != 1 {
			t.Fatalf("never-backed-up should fire, got %d", len(got))
		}
		if got[0].Body != "0 days since your last backup" {
			t.Errorf("Body = %q, want 0-days (unknown)", got[0].Body)
		}
	})
}

func TestStaleBalanceCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -400) // far beyond any freshness window
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Type: domain.TypeChecking, BalanceAsOf: old},             // stale
		{ID: "sav", Name: "Savings", Type: domain.TypeSavings, BalanceAsOf: now},               // fresh
		{ID: "arch", Name: "Old", Type: domain.TypeChecking, BalanceAsOf: old, Archived: true}, // archived → not stale
	}
	text := func(name string, days int) (string, string) {
		return name + " needs an update", fmt.Sprintf("%d days since the last update", days)
	}

	got := StaleBalanceCandidates("rule-stale", accounts, freshness.DefaultWindows(), now, text)
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1 (the stale checking account): %+v", len(got), got)
	}
	c := got[0]
	if c.RuleID != "rule-stale" {
		t.Errorf("RuleID = %q, want rule-stale", c.RuleID)
	}
	if c.Event != notify.EventStaleBalance {
		t.Errorf("Event = %q, want stale-balance", c.Event)
	}
	if c.Severity != notify.SeverityWarning {
		t.Errorf("Severity = %v, want warning", c.Severity)
	}
	wantKey := "chk@" + notify.WeekKey(now)
	if c.OccurrenceKey != wantKey {
		t.Errorf("OccurrenceKey = %q, want %q", c.OccurrenceKey, wantKey)
	}
	if !c.At.Equal(now) {
		t.Errorf("At = %s, want now", c.At)
	}
	if c.Title != "Checking needs an update" {
		t.Errorf("Title = %q", c.Title)
	}
	if c.Body != "400 days since the last update" {
		t.Errorf("Body = %q", c.Body)
	}
}

func TestBudgetCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)
	statuses := []budgeting.Status{
		{Budget: domain.Budget{ID: "food", Name: "Food"}, State: budgeting.StateOver},
		{Budget: domain.Budget{ID: "fun", Name: "Fun"}, State: budgeting.StateNear},
		{Budget: domain.Budget{ID: "rent", Name: "Rent"}, State: budgeting.StateOK}, // OK → no candidate
	}
	text := func(name string, over bool) (string, string) {
		if over {
			return name + " over budget", "over"
		}
		return name + " near budget", "near"
	}

	got := BudgetCandidates("rule-budget", statuses, now, text)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2 (over + near): %+v", len(got), got)
	}
	by := map[string]notify.Candidate{}
	for _, c := range got {
		by[c.Title] = c
	}
	over := by["Food over budget"]
	if over.Event != notify.EventBudgetThreshold || over.Severity != notify.SeverityCritical {
		t.Errorf("over candidate = %+v, want budget-threshold + critical", over)
	}
	if over.OccurrenceKey != "food:over@"+notify.MonthKey(now) {
		t.Errorf("over key = %q", over.OccurrenceKey)
	}
	near := by["Fun near budget"]
	if near.Severity != notify.SeverityWarning {
		t.Errorf("near candidate severity = %v, want warning", near.Severity)
	}
	if near.OccurrenceKey != "fun:near@"+notify.MonthKey(now) {
		t.Errorf("near key = %q", near.OccurrenceKey)
	}
}

func TestBillDueCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)
	mk := func(id string, days int) bills.Bill {
		return bills.Bill{
			AccountID: id, Name: id, Amount: money.New(5000, "USD"),
			DueDate: now.AddDate(0, 0, days), DaysUntil: days,
		}
	}
	upcoming := []bills.Bill{
		mk("today", 0),  // due today → critical
		mk("soon", 5),   // within window → warning
		mk("later", 20), // beyond the 7-day window → excluded
	}
	text := func(name string, days int) (string, string) {
		return name + " due", fmt.Sprintf("%d days", days)
	}

	got := BillDueCandidates("rule-bill", upcoming, 7, now, text)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2 (today, soon): %+v", len(got), got)
	}
	by := map[string]notify.Candidate{}
	for _, c := range got {
		by[c.Title] = c
	}
	today := by["today due"]
	if today.Event != notify.EventBillDue || today.Severity != notify.SeverityCritical {
		t.Errorf("today candidate = %+v, want bill-due + critical", today)
	}
	if today.OccurrenceKey != "today@"+now.Format("2006-01-02") {
		t.Errorf("today key = %q", today.OccurrenceKey)
	}
	if by["soon due"].Severity != notify.SeverityWarning {
		t.Errorf("soon severity = %v, want warning", by["soon due"].Severity)
	}

	// A non-positive window falls back to 7 days (so "later" still excluded).
	if dflt := BillDueCandidates("r", upcoming, 0, now, text); len(dflt) != 2 {
		t.Errorf("default window got %d, want 2", len(dflt))
	}
}

func TestDigestCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)

	got := DigestCandidates("rule-digest", notify.WeekKey(now), "Your week", "You spent $X.", now)
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1: %+v", len(got), got)
	}
	c := got[0]
	if c.Event != notify.EventDigest || c.Severity != notify.SeverityInfo {
		t.Errorf("candidate = %+v, want digest + info", c)
	}
	if c.OccurrenceKey != "digest@"+notify.WeekKey(now) {
		t.Errorf("key = %q", c.OccurrenceKey)
	}
	if c.Title != "Your week" || c.Body != "You spent $X." {
		t.Errorf("title/body = %q / %q", c.Title, c.Body)
	}
	// Empty title → nothing to summarize.
	if none := DigestCandidates("r", notify.MonthKey(now), "", "", now); none != nil {
		t.Errorf("empty title got %+v, want nil", none)
	}
}

func TestLowBalanceCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	text := func(name string, bal int64) (string, string) {
		return name + " low", fmt.Sprintf("%d", bal)
	}

	mkAsset := func(id string, opening int64) domain.Account {
		return domain.Account{
			ID:             id,
			Name:           id,
			Type:           domain.TypeChecking,
			Currency:       "USD",
			OpeningBalance: money.New(opening, "USD"),
		}
	}
	mkLiab := func(id string, opening int64) domain.Account {
		return domain.Account{
			ID:             id,
			Name:           id,
			Type:           domain.TypeCreditCard,
			Currency:       "USD",
			OpeningBalance: money.New(opening, "USD"),
		}
	}

	t.Run("below floor fires", func(t *testing.T) {
		accounts := []domain.Account{mkAsset("chk", 5000)} // $50 < $100 floor
		got, err := LowBalanceCandidates("r", accounts, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d, want 1", len(got))
		}
		c := got[0]
		if c.Event != notify.EventLowBalance {
			t.Errorf("Event = %q, want low-balance", c.Event)
		}
		if c.Severity != notify.SeverityWarning {
			t.Errorf("Severity = %v, want warning", c.Severity)
		}
		wantKey := "lowbal:chk@" + notify.WeekKey(now)
		if c.OccurrenceKey != wantKey {
			t.Errorf("OccurrenceKey = %q, want %q", c.OccurrenceKey, wantKey)
		}
		if c.Title != "chk low" {
			t.Errorf("Title = %q", c.Title)
		}
	})

	t.Run("at floor does not fire", func(t *testing.T) {
		accounts := []domain.Account{mkAsset("sav", 10000)} // exactly at floor
		got, err := LowBalanceCandidates("r", accounts, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("at-floor: got %d, want 0", len(got))
		}
	})

	t.Run("above floor does not fire", func(t *testing.T) {
		accounts := []domain.Account{mkAsset("sav", 50000)}
		got, err := LowBalanceCandidates("r", accounts, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("above-floor: got %d, want 0", len(got))
		}
	})

	t.Run("zero floor disables", func(t *testing.T) {
		accounts := []domain.Account{mkAsset("chk", 0)}
		got, err := LowBalanceCandidates("r", accounts, nil, 0, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("zero floor should yield nothing, got %+v", got)
		}
	})

	t.Run("liability excluded", func(t *testing.T) {
		accounts := []domain.Account{mkLiab("cc", 500)} // balance below floor but it's a liability
		got, err := LowBalanceCandidates("r", accounts, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("liability: got %d, want 0", len(got))
		}
	})

	t.Run("archived excluded", func(t *testing.T) {
		a := mkAsset("old", 500)
		a.Archived = true
		got, err := LowBalanceCandidates("r", []domain.Account{a}, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("archived: got %d, want 0", len(got))
		}
	})

	t.Run("transactions applied to balance", func(t *testing.T) {
		accounts := []domain.Account{mkAsset("chk", 20000)} // $200 opening
		// A $150 expense brings it to $50, below $100 floor
		txns := []domain.Transaction{
			{ID: "t1", AccountID: "chk", Amount: money.New(-15000, "USD"), Date: now},
		}
		got, err := LowBalanceCandidates("r", accounts, txns, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("post-txn balance below floor: got %d, want 1", len(got))
		}
		if got[0].Body != "5000" {
			t.Errorf("Body = %q, want 5000 (balance in minor units)", got[0].Body)
		}
	})

	t.Run("multiple accounts each fire once", func(t *testing.T) {
		accounts := []domain.Account{
			mkAsset("a", 5000),  // below
			mkAsset("b", 20000), // above
			mkAsset("c", 3000),  // below
		}
		got, err := LowBalanceCandidates("r", accounts, nil, 10000, now, text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("two below-floor accounts: got %d, want 2", len(got))
		}
	})
}

func TestStaleBalanceCandidatesNoneStale(t *testing.T) {
	now := time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Type: domain.TypeChecking, BalanceAsOf: now},
	}
	got := StaleBalanceCandidates("r", accounts, freshness.DefaultWindows(), now,
		func(string, int) (string, string) { return "", "" })
	if len(got) != 0 {
		t.Errorf("got %d, want 0 (nothing stale)", len(got))
	}
}

func TestPaycheckLandedCandidates(t *testing.T) {
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	text := func(desc string, amount int64) (string, string) {
		return "Paycheck: " + desc, fmt.Sprintf("%d", amount)
	}
	const threshold int64 = 50000 // $500.00
	const window = 3              // last 3 days

	inWindow := now.AddDate(0, 0, -1)                 // yesterday — within window
	onCutoff := now.AddDate(0, 0, -window)            // exactly at cutoff boundary (excluded: Before)
	outsideWindow := now.AddDate(0, 0, -(window + 1)) // 4 days ago — outside window

	t.Run("income within window at threshold fires", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p1", Desc: "Employer", Amount: money.New(threshold, "USD"), Date: inWindow},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 1 {
			t.Fatalf("got %d, want 1: %+v", len(got), got)
		}
		c := got[0]
		if c.Event != notify.EventPaycheckLanded {
			t.Errorf("Event = %q, want paycheck-landed", c.Event)
		}
		if c.Severity != notify.SeverityInfo {
			t.Errorf("Severity = %v, want info", c.Severity)
		}
		if c.OccurrenceKey != "paycheck:p1" {
			t.Errorf("OccurrenceKey = %q, want paycheck:p1", c.OccurrenceKey)
		}
		if c.RuleID != "rule-paycheck" {
			t.Errorf("RuleID = %q", c.RuleID)
		}
	})

	t.Run("income below threshold suppressed", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p2", Desc: "Tip", Amount: money.New(threshold-1, "USD"), Date: inWindow},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 0 {
			t.Errorf("got %d candidates, want 0 (below threshold)", len(got))
		}
	})

	t.Run("expense excluded", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p3", Desc: "Rent", Amount: money.New(-threshold, "USD"), Date: inWindow},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 0 {
			t.Errorf("got %d candidates, want 0 (expense excluded)", len(got))
		}
	})

	t.Run("transfer excluded", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p4", Desc: "Transfer in", Amount: money.New(threshold, "USD"), Date: inWindow, TransferAccountID: "acc-other"},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 0 {
			t.Errorf("got %d candidates, want 0 (transfer excluded)", len(got))
		}
	})

	t.Run("income outside recent window excluded", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p5", Desc: "OldPaycheck", Amount: money.New(threshold, "USD"), Date: outsideWindow},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 0 {
			t.Errorf("got %d candidates, want 0 (outside window)", len(got))
		}
	})

	t.Run("income exactly at cutoff boundary excluded", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p6", Desc: "OldPaycheck", Amount: money.New(threshold, "USD"), Date: onCutoff},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 0 {
			t.Errorf("got %d candidates, want 0 (at cutoff is excluded)", len(got))
		}
	})

	t.Run("multiple paychecks each emit once", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "q1", Desc: "Employer A", Amount: money.New(threshold, "USD"), Date: inWindow},
			{ID: "q2", Desc: "Employer B", Amount: money.New(threshold+10000, "USD"), Date: now.AddDate(0, 0, -2)},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, threshold, window, now, text)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2: %+v", len(got), got)
		}
		keys := map[string]bool{got[0].OccurrenceKey: true, got[1].OccurrenceKey: true}
		if !keys["paycheck:q1"] || !keys["paycheck:q2"] {
			t.Errorf("unexpected keys: %+v", keys)
		}
	})

	t.Run("zero threshold disables", func(t *testing.T) {
		txns := []domain.Transaction{
			{ID: "p7", Desc: "Salary", Amount: money.New(999999, "USD"), Date: inWindow},
		}
		got := PaycheckLandedCandidates("rule-paycheck", txns, 0, window, now, text)
		if got != nil {
			t.Errorf("zero threshold should yield nil, got %+v", got)
		}
	})
}
