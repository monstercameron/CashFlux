// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the net-worth page's derived figures as engine variables:
// the monthly change, the asset-class composition (cash / invested / property /
// other), and the liquid share — as networth_* variables usable in any formula
// or dashboard widget. Everything is computed from the fundamental Data fields
// already fed to Vars (accounts, transactions, rates, Now) via the same ledger
// calls the /networth surface renders, so a networth_* figure always matches
// the page. (net_worth itself stays the assets−liabilities molecule.)

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
)

// NetWorthVarNames are the fixed net-worth variables addNetWorthVars exposes,
// in a stable order. Money figures are major units of the base currency.
var NetWorthVarNames = []string{
	"networth_change",       // net-worth change this month so far (now vs the month start)
	"networth_change_pct",   // that change as a percent of the month-start figure
	"networth_cash",         // Σ asset balances of cash-type accounts (checking/debit/savings/cash)
	"networth_invested",     // Σ asset balances of investment/retirement/crypto accounts
	"networth_property",     // Σ asset balances of property/vehicle accounts
	"networth_other_assets", // Σ asset balances of every other asset account
	"networth_liquid_pct",   // cash-type assets as a percent of all assets
}

func init() { Names = append(Names, NetWorthVarNames...) }

// netWorthClassBucket maps an asset account's type to its composition bucket
// variable. Liability accounts never reach this (they roll into liabilities).
func netWorthClassBucket(t domain.AccountType) string {
	switch t {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return "networth_cash"
	case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		return "networth_invested"
	case domain.TypeProperty, domain.TypeVehicle:
		return "networth_property"
	}
	return "networth_other_assets"
}

// addNetWorthVars derives the networth_* variables: the month-to-date change
// from a two-point NetWorthSeries, and the asset composition by summing FX-converted
// per-account balances into type buckets (accounts with no exchange rate are
// skipped, matching NetWorthExplained's exclusion semantics).
func addNetWorthVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64, bals map[string]money.Money) {
	for _, name := range NetWorthVarNames {
		out[name] = 0
	}

	// Month-to-date change, read through the app's one net-worth-change seam
	// (C341) so a formula or custom widget bound to networth_change cannot
	// disagree with the dashboard hero that inspired it.
	if c, err := nwchange.MonthToDateChange(d.Accounts, d.Transactions, d.Rates, d.Now); err == nil && c.Known {
		out["networth_change"] = major(c.DeltaMinor())
		if pct, ok := c.PercentChange(); ok {
			out["networth_change_pct"] = float64(pct)
		}
	}

	// Composition: per-account balances bucketed by type (assets only). toBase
	// returns 0 for a currency with no rate, matching NetWorthExplained's
	// exclusion semantics (a rate-less account never fakes a base-currency value).
	var totalAssets, cashAssets int64
	for _, a := range d.Accounts {
		if a.Archived || a.Class == domain.ClassLiability || a.ExcludeFromNetWorth {
			continue
		}
		bal, ok := bals[a.ID]
		if !ok {
			continue
		}
		conv := toBase(bal.Amount, a.Currency)
		bucket := netWorthClassBucket(a.Type)
		out[bucket] += major(conv)
		totalAssets += conv
		if bucket == "networth_cash" {
			cashAssets += conv
		}
	}
	if totalAssets > 0 {
		out["networth_liquid_pct"] = float64(cashAssets*100) / float64(totalAssets)
	}
}
