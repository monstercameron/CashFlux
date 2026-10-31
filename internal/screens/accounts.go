// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/revalue"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// accounts.go holds the shared helpers for the accounts surface. The page itself is
// a widgetized SURFACE HOST in accounts_widget.go (the Accounts() entry point), whose
// tiles live in accounts_tiles.go; the rich per-account row is AccountRow in
// accounts_row.go. These helpers are used across those files (and the add form).

// condName returns name when ok is true and "" otherwise, so a conditional
// modifier class can be passed as one more argument of a single css.Class call
// (an element takes exactly one class option; a second one would replace the first).
func condName(ok bool, name string) string {
	if ok {
		return name
	}
	return ""
}

// labeledField wraps a form control in a <label> with persistent visible text, so
// the field stays self-describing after a placeholder would have vanished.
// The wrapping <label> also associates the text with the control for a11y. The
// labeled-field-tight class stacks the text over the control.
func labeledField(label string, control ui.Node) ui.Node {
	return Label(css.Class("labeled-field", "labeled-field-tight"),
		Span(css.Class("t-caption", tw.TextDim), label),
		control,
	)
}

// groupedField mirrors labeledField for a GROUP of controls (a checklist).
// It must be a fieldset+legend, never a <label> wrapper: wrapping a group in
// <label> makes the FIRST checkbox inside the label's implicit control, so its
// accessible name becomes the entire group's text ("Linked accounts Asset
// Account Priya's Business Checking …").
func groupedField(label string, control ui.Node) ui.Node {
	return Fieldset(css.Class("labeled-field", "labeled-group"),
		Legend(css.Class("t-caption", tw.TextDim), label),
		control,
	)
}

// ariaBool renders a Go bool as the "true"/"false" string an ARIA state attribute
// (e.g. aria-expanded) expects, keeping disclosure toggles screen-reader-correct.
func ariaBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// currencyOptions builds the account-currency picker's SelectOptions: every known
// registry currency, plus any code already in play (the base currency, the FX-table
// currencies, and the current selection) so an in-use code is never dropped. Each
// option reads "CODE — Name". Picking from a fixed list keeps typos from
// silently breaking FX.
func currencyOptions(app *appstate.App, selected string) []uiw.SelectOption {
	seen := map[string]bool{}
	var codes []string
	add := func(c string) {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		codes = append(codes, c)
	}
	for _, c := range currency.List() {
		add(c.Code)
	}
	add(app.Settings().BaseCurrency)
	for code := range app.Settings().FXRates {
		add(code)
	}
	add(selected)
	sort.Strings(codes)

	opts := make([]uiw.SelectOption, 0, len(codes))
	for _, c := range codes {
		label := c
		if cur, ok := currency.Lookup(c); ok {
			label = c + " — " + cur.Name
		}
		opts = append(opts, uiw.SelectOption{Value: c, Label: label})
	}
	return opts
}

// netWorthDeltaLine renders the month-to-date net-worth change as a small trend
// subtitle under the hero figure: a colored ↑/↓ glyph, the signed amount and "this
// month". An unknown or unrepresentable change gets an unavailable caption, distinct
// from a known zero change.
func netWorthDeltaLine(delta money.Money, have bool) ui.Node {
	abs, err := delta.AbsChecked()
	if !have || err != nil {
		return Span(css.Class("stat-sub", tw.TextDim), Attr("data-testid", "acct-networth-delta-unavailable"), uistate.T("accounts.deltaUnavailable"))
	}
	if delta.Amount == 0 {
		return Span(css.Class("stat-sub", tw.TextDim), Attr("data-testid", "acct-networth-delta-zero"), uistate.T("accounts.noChangeMonth"))
	}
	tone, glyph, sign := tw.TextUp, icon.TrendingUp, "+"
	if delta.Amount < 0 {
		tone, glyph, sign = tw.TextDown, icon.TrendingDown, "−"
	}
	return Span(css.Class("stat-sub", tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tone),
		uiw.Icon(glyph, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("accounts.deltaThisMonth", sign+fmtMoney(abs))))
}

// accountTypeIcon maps an account type to a small leading glyph so Checking /
// Investment / Credit Card are distinguishable at a glance without reading the
// meta-line. Unknown types fall back to the generic accounts glyph.
func accountTypeIcon(t domain.AccountType) icon.Name {
	switch t {
	case domain.TypeCreditCard, domain.TypeLineOfCredit:
		return icon.CreditCard
	case domain.TypeLoan, domain.TypePersonalLoan, domain.TypeMortgage:
		return icon.Landmark
	case domain.TypeUtilities:
		return icon.Receipt // a utility/HOA account is a recurring bill
	case domain.TypeInvestment:
		return icon.Reports
	case domain.TypeRetirement:
		return icon.TrendingUp
	case domain.TypeCrypto:
		return icon.Scale
	case domain.TypeProperty:
		return icon.Box // closest available glyph for a real-estate/building asset
	case domain.TypeVehicle:
		return icon.Calculator // closest available glyph for a vehicle asset
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings:
		return icon.Landmark
	default:
		return icon.Accounts
	}
}

// isRevaluableType reports whether an account type is periodically ESTIMATED rather
// than reconciled: property, vehicle, crypto (internal/revalue's cadence-driven
// types) plus "Other", which a household may also be using for a manually-valued
// asset. Gates the RevalueDays override field on the edit form's advanced section.
func isRevaluableType(t domain.AccountType) bool {
	return revalue.IsRevaluable(t) || t == domain.TypeOther
}

// accountMeta builds an account row's subtitle: type and currency, plus credit
// utilization for liability accounts that have a credit limit. Utilization is read
// from the balance as the row shows it, so an overpaid card, which holds money the
// lender owes, uses none of its limit.
func accountMeta(a domain.Account, bal money.Money) string {
	typ := humanizeType(string(a.Type))
	if a.Class == domain.ClassLiability {
		owed := reconcile.OwedMinor(reconcile.DisplayBalance(a, bal, bal).Balance.Amount)
		if pct, ok := ledger.Utilization(owed, a.CreditLimit.Amount); ok {
			return uistate.T("accounts.metaLineLimit", typ, a.Currency, pct)
		}
	}
	return uistate.T("accounts.metaLine", typ, a.Currency)
}

// accountBaseAndRates returns the household base currency, the default one when none is set,
// together with the saved exchange rates anchored on it.
func accountBaseAndRates(app *appstate.App) (string, currency.Rates) {
	settings := app.Settings()
	base := accountedit.BaseCurrencyOrDefault(settings.BaseCurrency)
	return base, currency.Rates{Base: base, Rates: settings.FXRates}
}

// accountSnapshot shares the ledger index between account page tiles. Store and
// app identity are captured by revKey; filters never mutate this snapshot.
var accountSnapshotCache = newViewCache[accountview.Snapshot](1)

func accountSnapshot(app *appstate.App) accountview.Snapshot {
	return memoByRev(accountSnapshotCache, revKey(app), func() accountview.Snapshot {
		accounts := app.Accounts()
		byAccount := make(map[string][]domain.Transaction, len(accounts))
		for _, account := range accounts {
			byAccount[account.ID] = app.TransactionsByAccount(account.ID)
		}
		return accountview.BuildIndexed(accounts, app.Transactions(), byAccount)
	})
}
