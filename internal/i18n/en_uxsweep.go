// SPDX-License-Identifier: MIT

package i18n

import "maps"

// uxSweepKeys fills the English keys the 2026-07-03 UX sweep found rendering as
// raw key names in the live app (C335/C336): the /setup rail label + wizard hero,
// the subscriptions net-price-change headline (whose missing key surfaced as a
// raw "%!(EXTRA …)" format error), the dashboard hero widget's registry name,
// and the credential-vault loading line. Defined in their own file and merged
// via init so this does not touch the user-WIP en.go; mirrors en_setup.go.
var uxSweepKeys = Catalog{
	// Rail label for the guided setup wizard (route /setup).
	"nav.setup": "Set up",

	// /setup wizard hero card.
	"setup.welcomeTitle": "Welcome to CashFlux",
	"setup.welcomeBody":  "A few quick steps set up your household: pick a currency, note your income, add a first account, and name who's in the house. Everything can be changed later in Settings.",

	// /subscriptions "Recent price changes" net headline. %s is a formatted
	// monthly amount in the base currency.
	"subs.netPriceUp":   "Recent changes add up to about %s/mo more.",
	"subs.netPriceDown": "Recent changes save you about %s/mo overall.",

	// Dashboard hero widget's display name (widget registry + manager).
	"dashboard.heroTitle": "Overview hero",

	// Shared brief loading line (credential vault form).
	"common.loading": "Loading…",

	// Settings aria-labels that were referenced but never defined — screen
	// readers heard the raw key plus a format error. %s = the row's label /
	// currency codes.
	"settings.freshnessAria": "Days before %s balances count as stale",
	"settings.fxRateAria":    "Exchange rate from %s to %s",
}

func init() {
	maps.Copy(english, uxSweepKeys)
}
