// SPDX-License-Identifier: MIT

// Package smartengine holds the deterministic ("[rule]") engines for the SMART
// series. Each Free feature is a pure function that reads an Input snapshot and
// returns []smart.Insight; nothing here touches syscall/js, the network, or a
// model, so the whole package unit-tests on native Go.
//
// The engines reuse the app's existing pure engines (ledger, runway, cashflow,
// bills, payoff, goals, subscriptions, forecast) rather than re-deriving their
// math — the SMART layer adds judgment and surfacing, not new arithmetic.
//
// Run is the single entry point: given an Input and the user's opt-in Settings,
// it executes only the enabled Free engines, drops dismissed insights, and
// returns the result sorted for display. AI features have no engine here — they
// run from the wasm layer (this package never makes a model call).
package smartengine

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

// Input is the read-only snapshot the rule engines compute over. It is a plain
// data bundle (no behavior) so engines stay pure and tests construct exactly the
// state they need. All slices are treated as read-only; engines never mutate
// them. Base is the household base-currency code; Rates converts other currencies
// into it. Now is the reference clock (engines never call time.Now themselves, so
// they are deterministic and testable).
type Input struct {
	Now   time.Time
	Base  string
	Rates currency.Rates

	// WeekStart is the household's first day of the week, used to bound weekly
	// budget periods. The zero value (Sunday) is a sensible default.
	WeekStart time.Weekday

	Accounts      []domain.Account
	Transactions  []domain.Transaction
	Categories    []domain.Category
	Budgets       []domain.Budget
	Goals         []domain.Goal
	Recurring     []domain.Recurring
	Members       []domain.Member
	Tasks         []domain.Task
	Subscriptions []domain.SubscriptionCancellation // recorded cancellations

	// Aliases is the learned payee-alias table (TX1). Engines that key on
	// merchant identity resolve raw payees through it (via in.payeeResolver) so
	// processor noise ("AMZN Mktp US*2K4RT0") collapses to one clean merchant and
	// a user rename unifies matching. Empty is valid — resolution then falls back
	// to the built-in normalizer rule pack.
	Aliases []domain.PayeeAlias

	// Subs is the detected subscription set (from subscriptions.Detect), passed in
	// so the subscription engines don't re-run detection per feature.
	Subs []subscriptions.Subscription

	// PaidOccurrences is the set of recurring-occurrence keys (billmatch.Key:
	// "recurringID|YYYY-MM-DD") that carry a durable bill-match link (TX9) — the
	// occurrences already settled by a matched transaction. A missing-payment
	// detector should skip these so a matched bill is not re-flagged as missed.
	// Populated by the wasm adapter from appstate.BillMatchPaidOccurrences().
	PaidOccurrences map[string]bool
}

// OccurrencePaid reports whether the recurring occurrence (recurringID + due
// date) has been settled by a matched transaction (TX9). Detectors call it to
// avoid re-flagging a bill that is already paid. It is safe on a nil map.
func (in Input) OccurrencePaid(recurringID string, due time.Time) bool {
	if in.PaidOccurrences == nil {
		return false
	}
	key := recurringID + "|" + time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	return in.PaidOccurrences[key]
}

// engineFn is one Free feature's rule engine.
type engineFn func(Input) []smart.Insight

// engines maps a feature code to its rule engine. Per-page files register their
// engines in init(), keeping each page's logic self-contained.
var engines = map[string]engineFn{}

// register wires a Free feature's engine. It panics on a duplicate or unknown
// code so a wiring mistake fails loudly at startup (in tests), never silently.
func register(code string, fn engineFn) {
	if _, ok := smart.ByCode(code); !ok {
		panic("smartengine: register unknown feature code " + code)
	}
	if f, _ := smart.ByCode(code); f.Tier != smart.TierFree {
		panic("smartengine: register non-Free feature " + code)
	}
	if _, dup := engines[code]; dup {
		panic("smartengine: duplicate engine for " + code)
	}
	engines[code] = fn
}

// HasEngine reports whether a Free engine is implemented for the given code.
// Used by tests and the settings UI to distinguish "shipped" from "planned".
func HasEngine(code string) bool { _, ok := engines[code]; return ok }

// ImplementedCodes returns the feature codes that have a Free engine, in catalog
// order — the deterministic features the app can actually run today.
func ImplementedCodes() []string {
	var out []string
	for _, f := range smart.Catalog() {
		if _, ok := engines[f.Code]; ok {
			out = append(out, f.Code)
		}
	}
	return out
}

// Run executes the enabled Free engines for the given settings over the Input,
// then filters to active (enabled, non-dismissed) insights and sorts them for
// display. Engines for features the user has not enabled are never called, so an
// off feature costs nothing — the structural guarantee that the SMART layer
// never slows a core flow until asked for.
func Run(in Input, s smart.Settings) []smart.Insight {
	var out []smart.Insight
	for _, code := range s.ActiveCodes() { // enabled AND not muted
		if fn := engines[code]; fn != nil {
			out = append(out, fn(in)...)
		}
	}
	out = s.Active(out)
	smart.SortInsights(out)
	return out
}

// RunPage is Run scoped to a single page — used by per-page Smart panels so a
// page only computes its own engines.
func RunPage(in Input, s smart.Settings, page smart.Page) []smart.Insight {
	var out []smart.Insight
	for _, f := range s.EnabledFeaturesForPage(page) {
		if s.IsMuted(f.Code) { // a muted feature costs nothing and shows nothing
			continue
		}
		if fn := engines[f.Code]; fn != nil {
			out = append(out, fn(in)...)
		}
	}
	out = s.Active(out)
	smart.SortInsights(out)
	return out
}

// --- shared helpers -------------------------------------------------------

// toBaseMinor converts an amount in the given currency to base-currency minor
// units, returning 0 on a missing/invalid rate (engines treat unconvertible
// figures as absent rather than failing the whole run).
func (in Input) toBaseMinor(amt int64, from string) int64 {
	if from == "" || from == in.Base {
		return amt
	}
	v, err := currency.ConvertBetween(amt, from, in.Base, in.Rates)
	if err != nil {
		return 0
	}
	return v
}

// baseMoney wraps a base-currency minor amount as money.Money.
func (in Input) baseMoney(minor int64) money.Money { return money.New(minor, in.Base) }

// txnsForAccount returns the (date-agnostic) transactions belonging to an
// account, preserving input order.
func txnsForAccount(txns []domain.Transaction, accountID string) []domain.Transaction {
	var out []domain.Transaction
	for _, t := range txns {
		if t.AccountID == accountID {
			out = append(out, t)
		}
	}
	return out
}

// lastActivity returns the most recent transaction date touching the account
// (as account or transfer counterpart), and whether any was found.
func lastActivity(txns []domain.Transaction, accountID string) (time.Time, bool) {
	var last time.Time
	found := false
	for _, t := range txns {
		if t.AccountID != accountID && t.TransferAccountID != accountID {
			continue
		}
		if !found || t.Date.After(last) {
			last, found = t.Date, true
		}
	}
	return last, found
}

// activeAssetAccounts returns non-archived asset accounts (the common subject of
// the account-side engines).
func activeAssetAccounts(accounts []domain.Account) []domain.Account {
	var out []domain.Account
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		if a.Class == domain.ClassLiability {
			continue
		}
		out = append(out, a)
	}
	return out
}

// payeeResolver builds a payee-alias resolver over the Input's alias table. When
// Aliases is empty it still resolves through the built-in normalizer rule pack,
// so merchant keying is alias-aware even before the user has learned any names.
func (in Input) payeeResolver() *payeealias.Resolver {
	return payeealias.NewResolver(in.Aliases)
}

// abs64 returns the absolute value of an int64.
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
