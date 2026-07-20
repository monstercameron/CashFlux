// SPDX-License-Identifier: MIT

package i18n

// qpassBKeys holds the English strings for the 2026-07-19 v1.2.7 review fixes on
// the investments surface (lane B): the securities-vs-account-balances
// reconciliation shown on the /investments hero, the growth card, and the accounts
// investment banner; plus the honest rename of the misleading "Portfolio metrics"
// toolbar control to a general formula builder. Merged via init so this file does
// not touch en.go.
var qpassBKeys = Catalog{
	// --- reconciliation: tracked securities vs the account balances net worth uses ---
	"investments.reconcileLine":         "%s securities + %s cash & untracked = %s across investment accounts",
	"investments.reconcileBehindLine":   "%s in securities is ahead of %s in recorded balances by %s — update your account balances",
	"investments.reconcileNetWorthNote": "Net worth counts the investment-account balances, not the tracked-securities value.",
	"investments.reconcileTitle":        "Reconciliation by account",
	"investments.reconcileAcctLine":     "%s: %s securities + %s cash & untracked = %s",
	"investments.reconcileAcctBehind":   "%s: %s securities, balance behind by %s — update account balance",
	"investments.reconcileUnnamed":      "Account",
	// --- growth card: clarify it charts account balances, not securities value ---
	"investments.growthCaption": "Charts your investment-account balances over time — the figure net worth uses, not the tracked-securities market value.",
	// --- honest rename of the "Portfolio metrics" toolbar control (it opens the
	//     household-wide formula builder, not a portfolio-scoped tool) ---
	"investments.formulaBuilderShow":  "Formula builder",
	"investments.formulaBuilderHide":  "Hide formula builder",
	"investments.formulaBuilderTitle": "Build custom metrics from any CashFlux figure — not just this portfolio",
	"investments.formulaBuilderHint":  "Build a custom metric from any CashFlux figure — portfolio aggregates, your account custom fields, or any other engine value. This isn't limited to this portfolio.",
}

func init() {
	for k, v := range qpassBKeys {
		english[k] = v
	}
}
