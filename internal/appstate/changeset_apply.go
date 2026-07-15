// SPDX-License-Identifier: MIT

// Package appstate — changeset_apply.go is the APPLY side of AG1: it maps a
// changeset.Op Kind to a real appstate mutation and runs an enabled subset in
// order, stopping and reporting on the first failure (never a silent partial
// state). The changeset MODEL is the pure, testable value type in
// internal/changeset; keeping dispatch here lets each Kind reuse the validated
// appstate write methods (PutTransaction, PutCategory, …) and lets the applied
// ops land in the audit trail tagged "via assistant" (AG20).
//
// Extensibility: dispatchers live in the package-level changesetApplies registry.
// Additional agent op Kinds register with RegisterChangesetApply from their own
// file, so new tools can propose changeset ops without editing this file.
package appstate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/changeset"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ChangesetApplyFn applies one op's Args against the app and returns a short,
// plain-English confirmation (used in the receipt) or an error that halts the
// changeset.
type ChangesetApplyFn func(a *App, args json.RawMessage) (result string, err error)

// changesetApplies is the Kind → dispatcher registry. It is populated at init by
// the built-in dispatchers below and extended via RegisterChangesetApply.
var changesetApplies = map[string]ChangesetApplyFn{}

// RegisterChangesetApply registers (or replaces) the dispatcher for a Kind. It is
// safe to call from another file's init(); dispatch is single-goroutine (the UI
// tool loop), so no locking is needed.
func RegisterChangesetApply(kind string, fn ChangesetApplyFn) {
	changesetApplies[kind] = fn
}

// ChangesetApplyKinds reports whether a dispatcher is registered for a Kind, so a
// tool can validate a proposed op before showing it in the review card.
func ChangesetApplyKinds(kind string) bool {
	_, ok := changesetApplies[kind]
	return ok
}

// ApplyChangeset runs the enabled ops of cs in order and returns a Receipt.
// Disabled ops are skipped. The first op whose dispatcher errors (or whose Kind
// has no dispatcher) halts application: it is recorded in Receipt.Failed and no
// later op runs, so the user never lands in a silent partial state. Every op that
// applied is recorded in Receipt.Applied (in run order) for the receipt card and
// its one-tap "Undo all".
//
// Mutations made here are tagged "via assistant" in the audit trail (AG20): the
// session actor is set for the duration, and each applied op is captured as its
// own undo/audit point immediately (auditview.CaptureNow) so the tag is live at
// capture time and "Undo all" has one undo point per applied op.
func (a *App) ApplyChangeset(cs changeset.Changeset) changeset.Receipt {
	auditview.SetSessionActor(auditview.ActorAssistant)
	defer auditview.SetSessionActor("")

	var rec changeset.Receipt
	for i, op := range cs.Ops {
		if !op.Enabled {
			continue
		}
		fn := changesetApplies[op.Kind]
		if fn == nil {
			rec.Failed = &changeset.FailedOp{Index: i, Kind: op.Kind, Line: op.Line, Err: "unknown operation: " + op.Kind}
			return rec
		}
		result, err := fn(a, op.Args)
		if err != nil {
			rec.Failed = &changeset.FailedOp{Index: i, Kind: op.Kind, Line: op.Line, Err: err.Error()}
			return rec
		}
		rec.Applied = append(rec.Applied, changeset.AppliedOp{Index: i, Kind: op.Kind, Line: op.Line, Result: result})
		// Capture this op as its own audit + undo point while the assistant actor
		// tag is set. No-op in tests (the slot defaults to a no-op).
		auditview.CaptureNow()
	}
	return rec
}

// baseCur returns the app's base currency, defaulting to USD when unset.
func (a *App) baseCur() string {
	if c := a.Settings().BaseCurrency; c != "" {
		return c
	}
	return "USD"
}

// resolveCategoryByName maps a name to a category: exact (case-insensitive)
// first, then substring.
func (a *App) resolveCategoryByName(name string) (domain.Category, bool) {
	q := strings.ToLower(strings.TrimSpace(name))
	if q == "" {
		return domain.Category{}, false
	}
	cats := a.Categories()
	for _, c := range cats {
		if strings.ToLower(c.Name) == q {
			return c, true
		}
	}
	for _, c := range cats {
		if strings.Contains(strings.ToLower(c.Name), q) {
			return c, true
		}
	}
	return domain.Category{}, false
}

// resolveAccountByName maps a name to an account: exact first, then substring.
func (a *App) resolveAccountByName(name string) (domain.Account, bool) {
	q := strings.ToLower(strings.TrimSpace(name))
	if q == "" {
		return domain.Account{}, false
	}
	accts := a.Accounts()
	for _, ac := range accts {
		if strings.ToLower(ac.Name) == q {
			return ac, true
		}
	}
	for _, ac := range accts {
		if strings.Contains(strings.ToLower(ac.Name), q) {
			return ac, true
		}
	}
	return domain.Account{}, false
}

func init() {
	// add_task: create a to-do.
	RegisterChangesetApply("add_task", func(a *App, raw json.RawMessage) (string, error) {
		var p struct {
			Title    string `json:"title"`
			Notes    string `json:"notes"`
			Priority string `json:"priority"`
			Due      string `json:"due"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || strings.TrimSpace(p.Title) == "" {
			return "", fmt.Errorf("a task needs a title")
		}
		t := domain.Task{ID: id.New(), Title: strings.TrimSpace(p.Title), Notes: p.Notes, Status: domain.StatusOpen, Priority: parsePriority(p.Priority), Source: domain.SourceAI}
		if d, err := dateutil.ParseDate(p.Due); err == nil && p.Due != "" {
			t.Due = d
		}
		if err := a.PutTask(t); err != nil {
			return "", err
		}
		return "Added to-do: " + t.Title, nil
	})

	// create_category: create a spending/income category (returns the existing
	// one if the name already exists, so a changeset re-run is idempotent).
	RegisterChangesetApply("create_category", func(a *App, raw json.RawMessage) (string, error) {
		var p struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", fmt.Errorf("couldn't read the category")
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return "", fmt.Errorf("a category needs a name")
		}
		for _, c := range a.Categories() {
			if strings.EqualFold(c.Name, name) {
				return "Category already exists: " + c.Name, nil
			}
		}
		kind := domain.KindExpense
		if strings.EqualFold(p.Kind, "income") {
			kind = domain.KindIncome
		}
		c := domain.Category{ID: id.New(), Name: name, Kind: kind}
		if err := a.PutCategory(c); err != nil {
			return "", err
		}
		return "Created category: " + c.Name, nil
	})

	// add_transaction: record an expense (negative) or income (positive).
	RegisterChangesetApply("add_transaction", func(a *App, raw json.RawMessage) (string, error) {
		var p struct {
			Amount      float64 `json:"amount"`
			Account     string  `json:"account"`
			Category    string  `json:"category"`
			Payee       string  `json:"payee"`
			Description string  `json:"description"`
			Date        string  `json:"date"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", fmt.Errorf("couldn't read the transaction")
		}
		acc, ok := a.resolveAccountByName(p.Account)
		if !ok {
			return "", fmt.Errorf("no account matching %q", p.Account)
		}
		desc := strings.TrimSpace(p.Description)
		if desc == "" {
			desc = strings.TrimSpace(p.Payee)
		}
		if desc == "" {
			if p.Amount >= 0 {
				desc = "Income"
			} else {
				desc = "Expense"
			}
		}
		amt := money.New(currency.MinorFromMajor(p.Amount, acc.Currency), acc.Currency)
		t := domain.Transaction{ID: id.New(), AccountID: acc.ID, Amount: amt, Payee: strings.TrimSpace(p.Payee), Desc: desc, Date: time.Now(), Source: domain.TxnSourceAssistant}
		if d, err := dateutil.ParseDate(p.Date); err == nil && p.Date != "" {
			t.Date = d
		}
		if c, ok := a.resolveCategoryByName(p.Category); ok {
			t.CategoryID = c.ID
		}
		if err := a.PutTransaction(t); err != nil {
			return "", err
		}
		return "Recorded " + t.Amount.String() + " in " + acc.Name, nil
	})

	// categorize_transactions: assign a category to matching transactions.
	RegisterChangesetApply("categorize_transactions", func(a *App, raw json.RawMessage) (string, error) {
		var p struct {
			Match             string `json:"match"`
			Category          string `json:"category"`
			OnlyUncategorized *bool  `json:"only_uncategorized"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", fmt.Errorf("couldn't read the details")
		}
		q := strings.ToLower(strings.TrimSpace(p.Match))
		if q == "" {
			return "", fmt.Errorf("give a phrase to match on")
		}
		c, ok := a.resolveCategoryByName(p.Category)
		if !ok {
			return "", fmt.Errorf("no category matching %q", p.Category)
		}
		onlyUncat := p.OnlyUncategorized == nil || *p.OnlyUncategorized
		changed := 0
		for _, t := range a.Transactions() {
			if t.IsTransfer() || t.CategoryID == c.ID {
				continue
			}
			if onlyUncat && t.CategoryID != "" {
				continue
			}
			if !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), q) {
				continue
			}
			t.CategoryID = c.ID
			if err := a.PutTransaction(t); err != nil {
				return "", fmt.Errorf("categorized %d, then: %w", changed, err)
			}
			changed++
		}
		if changed == 0 {
			return "", fmt.Errorf("no transactions matched %q", p.Match)
		}
		return fmt.Sprintf("Categorized %d transaction(s) as %s", changed, c.Name), nil
	})

	// add_goal_contribution: add money toward a savings goal.
	RegisterChangesetApply("add_goal_contribution", func(a *App, raw json.RawMessage) (string, error) {
		var p struct {
			Goal   string  `json:"goal"`
			Amount float64 `json:"amount"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || p.Amount == 0 {
			return "", fmt.Errorf("provide a goal and a non-zero amount")
		}
		q := strings.ToLower(strings.TrimSpace(p.Goal))
		for _, g := range a.Goals() {
			if q != "" && strings.Contains(strings.ToLower(g.Name), q) {
				g.CurrentAmount = money.New(g.CurrentAmount.Amount+currency.MinorFromMajor(p.Amount, g.CurrentAmount.Currency), g.CurrentAmount.Currency)
				if err := a.PutGoal(g); err != nil {
					return "", err
				}
				return "Added to goal " + g.Name + " — now " + g.CurrentAmount.String(), nil
			}
		}
		return "", fmt.Errorf("no goal matching %q", p.Goal)
	})
}

// parsePriority maps a string to a TaskPriority (default medium). Named to avoid
// clashing with the screens package's parseTaskPriority.
func parsePriority(s string) domain.TaskPriority {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return domain.PriorityLow
	case "high":
		return domain.PriorityHigh
	default:
		return domain.PriorityMedium
	}
}
