// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// accounts.go holds the shared helpers for the accounts surface. The page itself is
// a widgetized SURFACE HOST in accounts_widget.go (the Accounts() entry point), whose
// tiles live in accounts_tiles.go; the rich per-account row is AccountRow in
// accounts_row.go. These helpers are used across those files (and the add form).

// labeledField wraps a form control in a <label> with persistent visible text, so
// the field stays self-describing after a placeholder would have vanished (C49).
// The wrapping <label> also associates the text with the control for a11y. Styled
// inline (stacked text-over-control) to avoid a stylesheet dependency.
func labeledField(label string, control ui.Node) ui.Node {
	return Label(css.Class("labeled-field"),
		Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
		Span(css.Class("t-caption", tw.TextDim), label),
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
// option reads "CODE — Name". A validated picker (vs the old free-text input) keeps
// typos from silently breaking FX.
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
// subtitle under the hero figure: a colored ↑/↓ glyph + the signed amount + "this
// month" (G3 §3). A zero or unknown delta reads as a calm "no change" caption.
func netWorthDeltaLine(delta money.Money, have bool) ui.Node {
	if !have || delta.Amount == 0 {
		return Span(css.Class("stat-sub", tw.TextDim), uistate.T("accounts.noChangeMonth"))
	}
	up := delta.Amount > 0
	tone, glyph := tw.TextUp, icon.TrendingUp
	if !up {
		tone, glyph = tw.TextDown, icon.TrendingDown
	}
	abs := delta
	if !up {
		abs = money.New(-delta.Amount, delta.Currency)
	}
	sign := "+"
	if !up {
		sign = "−"
	}
	return Span(css.Class("stat-sub", tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tone),
		uiw.Icon(glyph, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("accounts.deltaThisMonth", sign+fmtMoney(abs))))
}

// accountTypeIcon maps an account type to a small leading glyph so Checking /
// Investment / Credit Card are distinguishable at a glance without reading the
// meta-line (G3 §5). Unknown types fall back to the generic accounts glyph.
func accountTypeIcon(t domain.AccountType) icon.Name {
	switch t {
	case domain.TypeCreditCard, domain.TypeLineOfCredit:
		return icon.CreditCard
	case domain.TypeLoan, domain.TypePersonalLoan, domain.TypeMortgage:
		return icon.Landmark
	case domain.TypeInvestment:
		return icon.Reports
	case domain.TypeRetirement:
		return icon.TrendingUp
	case domain.TypeCrypto:
		return icon.Scale
	case domain.TypeProperty:
		return icon.Box // closest available glyph for a real-estate/building asset (C224)
	case domain.TypeVehicle:
		return icon.Calculator // closest available glyph for a vehicle asset (C224)
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings:
		return icon.Landmark
	default:
		return icon.Accounts
	}
}

// accountMeta builds an account row's subtitle: type · currency, plus credit
// utilization for liability accounts that have a credit limit.
func accountMeta(a domain.Account, bal money.Money) string {
	meta := humanizeType(string(a.Type)) + " · " + a.Currency
	if a.Class == domain.ClassLiability {
		if pct, ok := ledger.Utilization(bal.Amount, a.CreditLimit.Amount); ok {
			meta += fmt.Sprintf(" · %d%% of limit used", pct)
		}
	}
	return meta
}
