// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/networthview"
	"github.com/monstercameron/CashFlux/internal/periodview"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// netWorthCache lets useNetWorth share one computation across components in a single
// render. The dashboard computes net worth twice per mount — once for the widget
// context, once for the hero — and (with no active scope) they're identical; this
// collapses them to one full-ledger pass. Keyed on the store revision PLUS the account
// set, so a scoped view and the household total never share an entry, and a scope
// change recomputes (which the previous rev-only memo key missed).
var netWorthCache = newViewCache[networthview.Snapshot]()

// acctSig is a cheap, order-independent signature of an account set for the cache key.
func acctSig(accounts []domain.Account) string {
	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	sort.Strings(ids)
	var signature strings.Builder
	for _, id := range ids {
		signature.WriteString(strconv.Quote(id))
	}
	return signature.String()
}

// This file holds the dashboard's render-time *derived selectors* — net worth,
// period totals — wrapped in ui.UseMemo so each recomputes
// only when the underlying data actually changes, not on every re-render (§1.6).
// (state.UseComputed until GWC v4 soft-deprecated it in favor of UseMemo, which
// returns the value directly instead of wrapping it in an atom.)
//
// Correctness hinges on the memo key: app.Rev() is an O(1) monotonic counter the
// store advances on every entity write/delete AND on settings (base currency / FX)
// writes. Persisted UI settings can also advance that revision. UI-only inputs
// that affect a result (the active period, the
// active-member filter) are added as extra deps so switching them recomputes too.

func netWorthCacheKey(app *appstate.App, accounts []domain.Account, rates currency.Rates) string {
	return revKey(app) + "|" + acctSig(accounts) + "|rates=" + fmt.Sprintf("%#v", rates)
}

// netWorthSnapshot is the hook-free shared path for household comparisons and
// notifications. Inputs must represent this app revision's immutable ledger.
func netWorthSnapshot(app *appstate.App, accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) networthview.Snapshot {
	return memoByRev(netWorthCache, netWorthCacheKey(app, accounts, rates), func() networthview.Snapshot {
		return networthview.Build(accounts, txns, rates)
	})
}

// netWorthBalanceSnapshot lets Accounts reuse the exact balances it already
// prepared, while sharing the same complete key and result with other surfaces.
func netWorthBalanceSnapshot(app *appstate.App, accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) networthview.Snapshot {
	return memoByRev(netWorthCache, netWorthCacheKey(app, accounts, rates), func() networthview.Snapshot {
		return networthview.FromBalances(accounts, balances, rates)
	})
}

// useNetWorth preserves fatal errors and intentional missing-rate subtotals.
func useNetWorth(app *appstate.App, accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) networthview.Snapshot {
	key := netWorthCacheKey(app, accounts, rates)
	return ui.UseMemo(func() networthview.Snapshot {
		return netWorthSnapshot(app, accounts, txns, rates)
	}, key)
}

// usePeriodTotals returns memoized income/expense for the period over the given
// (already member-filtered) transactions. Period bounds and a member signature are
// extra deps so changing the active period or member recomputes without a data
// change.
func usePeriodTotals(app *appstate.App, txns []domain.Transaction, start, end time.Time, rates currency.Rates, memberSig string) periodview.Totals {
	return ui.UseMemo(func() periodview.Totals {
		return periodview.BuildTotals(txns, start, end, rates)
	}, revKey(app), start, end, memberSig, fmt.Sprintf("%#v", rates))
}
