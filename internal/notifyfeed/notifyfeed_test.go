package notifyfeed

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
)

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
