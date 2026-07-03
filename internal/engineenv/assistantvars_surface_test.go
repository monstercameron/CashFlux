// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// assistantTxn builds one expense transaction for the assistant-vars tests.
// Amounts are positive minor units of spend (stored as negative outflow).
func assistantTxn(id string, date time.Time, minor int64, catID, payee string) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: "a1", Date: date, Amount: money.New(-minor, "USD"),
		CategoryID: catID, Payee: payee,
	}
}

func TestAddAssistantVars(t *testing.T) {
	// Mid-July reference point: 15 days elapsed, so the pace window is
	// June 1 – June 16 and decreases are suppressed (month < 90% done).
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := Data{
		Accounts: []domain.Account{{ID: "a1", Type: domain.TypeChecking, Class: domain.ClassAsset,
			Currency: "USD", OpeningBalance: money.New(1000000, "USD"), BalanceAsOf: asOf}},
		Categories: []domain.Category{{ID: "c1", Name: "Dining"}},
		Transactions: []domain.Transaction{
			// Baseline months: ~$100/mo Dining in April, May, June.
			assistantTxn("t1", time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 10000, "c1", "Cafe Uno"),
			assistantTxn("t2", time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), 10000, "c1", "Cafe Uno"),
			assistantTxn("t3", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), 10000, "c1", "Cafe Uno"),
			// June also has a late-month charge OUTSIDE the June 1–16 pace window.
			assistantTxn("t4", time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), 5000, "c1", "Cafe Uno"),
			// July so far: $300 — a 3x Dining spike vs the $100 baseline.
			assistantTxn("t5", time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), 20000, "c1", "Trattoria Nove"),
			assistantTxn("t6", time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), 10000, "c1", "Cafe Uno"),
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
	}
	v := Vars(d)

	if got := v["assistant_spend_mtd"]; got != 300 {
		t.Errorf("assistant_spend_mtd = %v, want 300", got)
	}
	if got := v["assistant_spend_prev"]; got != 150 { // $100 + $50 late June
		t.Errorf("assistant_spend_prev = %v, want 150", got)
	}
	if got := v["assistant_spend_pace"]; got != 100 { // June 1–16 only
		t.Errorf("assistant_spend_pace = %v, want 100", got)
	}
	if got := v["assistant_spend_pace_delta"]; got != 200 { // $300 − $100
		t.Errorf("assistant_spend_pace_delta = %v, want 200", got)
	}
	if got := v["assistant_highlights"]; got != 1 { // the Dining spike
		t.Errorf("assistant_highlights = %v, want 1", got)
	}
	// Top payee over the trailing 90 days: Cafe Uno (May $100 + June $100 +
	// late-June $50 + July $100 = $350) beats Trattoria Nove ($200). April's
	// charge sits outside the 90-day window.
	if got := v["assistant_top_merchant"]; got != 350 {
		t.Errorf("assistant_top_merchant = %v, want 350", got)
	}
}

// TestAddAssistantVarsEmpty asserts every assistant_* variable is always
// present so formulas referencing them never hit undefined-variable errors.
func TestAddAssistantVarsEmpty(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range AssistantVarNames {
		if _, ok := v[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
}

// TestAssistantSpendStoryMonthStart asserts the day-one edge: a fresh month
// compares one day of spend against one day of last month, never a zero-width
// window or the whole prior month.
func TestAssistantSpendStoryMonthStart(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		assistantTxn("t1", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), 4000, "", "Day-one shop"),
		assistantTxn("t2", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), 6000, "", "Late-June shop"),
		assistantTxn("t3", time.Date(2026, 7, 1, 6, 0, 0, 0, time.UTC), 2500, "", "Fresh shop"),
	}
	mtd, prev, pace, err := AssistantSpendStory(txns, currency.Rates{Base: "USD"}, now)
	if err != nil {
		t.Fatalf("AssistantSpendStory: %v", err)
	}
	if mtd != 2500 {
		t.Errorf("mtd = %d, want 2500", mtd)
	}
	if prev != 10000 {
		t.Errorf("prev = %d, want 10000", prev)
	}
	if pace != 4000 { // June 1–2 only
		t.Errorf("pace = %d, want 4000", pace)
	}
}
