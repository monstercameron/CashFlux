// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// newTestAppAt returns an unseeded app with a fixed clock.
func newTestAppAt(t *testing.T, at time.Time) *App {
	t.Helper()
	a, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.now = func() time.Time { return at }
	return a
}

func putAccount(t *testing.T, a *App, id, name string) {
	t.Helper()
	if err := a.PutAccount(domain.Account{
		ID: id, Name: name, OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"),
	}); err != nil {
		t.Fatalf("PutAccount %s: %v", id, err)
	}
}

// TestScheduledTransferDedupeSpansPeriods proves the fix for the frozen
// DedupeKey bug: a pay-yourself-first transfer must execute once per period —
// a second run in the SAME month is skipped, but a run in the NEXT month
// transfers again (the old creation-stamped key blocked it forever).
func TestScheduledTransferDedupeSpansPeriods(t *testing.T) {
	june := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	a := newTestAppAt(t, june)
	putAccount(t, a, "acc-from", "Checking")
	putAccount(t, a, "acc-to", "Savings")

	wf, err := a.CreatePayYourselfFirstWorkflow("acc-from", "acc-to", 5000, domain.CadenceMonthly)
	if err != nil {
		t.Fatalf("CreatePayYourselfFirstWorkflow: %v", err)
	}
	if !strings.Contains(wf.Actions[0].DedupeKey, "{period}") {
		t.Fatalf("DedupeKey should carry the {period} placeholder, got %q", wf.Actions[0].DedupeKey)
	}

	countTransfers := func() int {
		n := 0
		for _, tx := range a.Transactions() {
			if tx.IsTransfer() {
				n++
			}
		}
		return n
	}

	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if got := countTransfers(); got != 2 { // a transfer is a two-leg pair
		t.Fatalf("after first run: %d transfer legs, want 2", got)
	}
	// Same month again → deduped, no new legs.
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("run 2 (same month): %v", err)
	}
	if got := countTransfers(); got != 2 {
		t.Fatalf("same-month re-run must dedupe: %d legs, want 2", got)
	}
	// Next month → a fresh period key → transfers again.
	a.now = func() time.Time { return june.AddDate(0, 1, 0) }
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("run 3 (next month): %v", err)
	}
	if got := countTransfers(); got != 4 {
		t.Fatalf("next-month run must transfer again: %d legs, want 4", got)
	}
}

// TestLegacyDedupeKeyIsRestamped proves an OLD workflow whose key was frozen
// at creation (":YYYY-MM" suffix) is repaired transparently: the next period
// still transfers.
func TestLegacyDedupeKeyIsRestamped(t *testing.T) {
	june := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	a := newTestAppAt(t, june)
	putAccount(t, a, "acc-from", "Checking")
	putAccount(t, a, "acc-to", "Savings")

	wf := workflow.Workflow{
		ID: "legacy-pyf", Name: "Legacy PYF", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled, Cadence: domain.CadenceMonthly},
		Actions: []workflow.Action{{
			Kind: workflow.ActionTransfer, TransferFromAccountID: "acc-from",
			TransferToAccountID: "acc-to", TransferAmount: 2500,
			DedupeKey: "pyf:legacy-pyf:2026-05", // frozen at creation, old format
		}},
	}
	if err := a.PutWorkflow(wf); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("june run: %v", err)
	}
	a.now = func() time.Time { return june.AddDate(0, 1, 0) }
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("july run: %v", err)
	}
	legs := 0
	for _, tx := range a.Transactions() {
		if tx.IsTransfer() {
			legs++
		}
	}
	if legs != 4 {
		t.Fatalf("legacy key must re-stamp per period: %d legs, want 4 (two monthly transfers)", legs)
	}
}

// TestTxnContextCustomFieldOverrides proves a condition can read THIS
// transaction's custom-field values: numbers and yes/no fields as cf_txn_*
// numbers, text via contains().
func TestTxnContextCustomFieldOverrides(t *testing.T) {
	a := newTestAppAt(t, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC))
	putAccount(t, a, "acc-1", "Checking")
	for _, def := range []customfields.Def{
		{ID: "d1", EntityType: "transaction", Key: "reimbursable", Label: "Reimbursable", Type: customfields.TypeBool},
		{ID: "d2", EntityType: "transaction", Key: "tip", Label: "Tip", Type: customfields.TypeNumber},
		{ID: "d3", EntityType: "transaction", Key: "project", Label: "Project", Type: customfields.TypeSelect, Options: []string{"Side hustle", "Personal"}},
	} {
		if err := a.PutCustomFieldDef(def); err != nil {
			t.Fatalf("PutCustomFieldDef: %v", err)
		}
	}
	wf := workflow.Workflow{
		ID: "wf-cf", Name: "Tag reimbursable", Enabled: true,
		Trigger:   workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Condition: `and(cf_txn_reimbursable > 0, cf_txn_tip >= 2, contains(cf_txn_project, "hustle"))`,
		Actions:   []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "reimburse-me"}},
	}
	if err := a.PutWorkflow(wf); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}

	put := func(id string, custom map[string]any) {
		t.Helper()
		if err := a.PutTransaction(domain.Transaction{
			ID: id, AccountID: "acc-1", Date: a.clock(), Desc: "d", Payee: "p",
			Amount: money.New(-1000, "USD"), Custom: custom,
		}); err != nil {
			t.Fatalf("PutTransaction %s: %v", id, err)
		}
	}
	put("t-match", map[string]any{"reimbursable": true, "tip": float64(3), "project": "Side hustle"})
	put("t-nomatch", map[string]any{"reimbursable": false, "tip": float64(3), "project": "Side hustle"})

	tagOf := func(id string) []string {
		for _, tx := range a.Transactions() {
			if tx.ID == id {
				return tx.Tags
			}
		}
		return nil
	}
	if tags := tagOf("t-match"); len(tags) != 1 || tags[0] != "reimburse-me" {
		t.Fatalf("matching txn tags = %v, want [reimburse-me]", tags)
	}
	if tags := tagOf("t-nomatch"); len(tags) != 0 {
		t.Fatalf("non-matching txn should be untagged, got %v", tags)
	}
}

// TestBudgetExceededFiresFromTransaction proves the trigger fires from the
// path that actually pushes a budget over — adding a transaction — not just
// from re-saving the budget document.
func TestBudgetExceededFiresFromTransaction(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	a := newTestAppAt(t, now)
	putAccount(t, a, "acc-1", "Checking")
	if err := a.PutCategory(domain.Category{ID: "cat-1", Name: "Dining", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := a.PutBudget(domain.Budget{
		ID: "b-1", Name: "Dining", CategoryID: "cat-1", Limit: money.New(10000, "USD"),
		OwnerID: "m1", Scope: domain.ScopeIndividual, Period: domain.PeriodMonthly,
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	wf := workflow.Workflow{
		ID: "wf-over", Name: "Budget watch", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerBudgetExceeded},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Budget went over"}},
	}
	if err := a.PutWorkflow(wf); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}

	put := func(id string, amountMinor int64) {
		t.Helper()
		if err := a.PutTransaction(domain.Transaction{
			ID: id, AccountID: "acc-1", CategoryID: "cat-1", MemberID: "m1",
			Date: now, Desc: "meal", Payee: "p",
			Amount: money.New(amountMinor, "USD"),
		}); err != nil {
			t.Fatalf("PutTransaction: %v", err)
		}
	}
	hasTask := func() bool {
		for _, tk := range a.Tasks() {
			if tk.Title == "Budget went over" {
				return true
			}
		}
		return false
	}

	put("t-under", -6000) // 60 of 100 — still under
	if hasTask() {
		t.Fatal("budget-exceeded fired while still under the limit")
	}
	put("t-over", -5000) // 110 of 100 — transition to over
	if !hasTask() {
		t.Fatal("budget-exceeded did not fire from the transaction that pushed the budget over")
	}
}
