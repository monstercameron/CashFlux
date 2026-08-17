// SPDX-License-Identifier: MIT

//go:build js && wasm

// The repeats detected on ONE account (SM-6).
//
// The A7 detector already reports "6 recurring charges on Joint Checking" and
// links to /subscriptions, where the account context is gone and the list is the
// household's, not this account's. SM-6 is the other half: the repeats found HERE,
// each with the one action worth having on them — start tracking it as a recurring
// commitment.
//
// The detection is internal/recurdiscover, run once per frame by rhyDiscover and
// filtered to the account, so opening this panel on five accounts costs one sweep,
// not five. Confirming goes through the same ConfirmRecurCandidate the review
// surface uses, back-claim included.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// acctRecurCode gates SM-6. A7 is "Recurring-charge detection" — a Free feature,
// so the panel needs no provider and defaults on.
const acctRecurCode = "SMART-A7"

// acctRecurCap bounds the list. An account with thirty repeats is a list, not a
// finding; the Bills & recurring surface is where a full roster belongs.
const acctRecurCap = 6

// accountRecurProps carries the account whose repeats are listed.
type accountRecurProps struct {
	Account domain.Account
}

// accountRecurringPanel renders the collapsed "N charges here repeat" disclosure.
// Its own component: it owns the disclosure state, and the discovery run stays
// lazy behind it.
func accountRecurringPanel(props accountRecurProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	expanded := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))

	app := appstate.Default
	settings := uistate.LoadSmartSettings()

	if app == nil || props.Account.Archived {
		return Fragment()
	}
	if !settings.IsEnabled(acctRecurCode) || settings.IsMuted(acctRecurCode) {
		return Fragment()
	}
	if !settings.DensityOrDefault().Shows(smart.AffordanceTooltip) {
		return Fragment()
	}

	cands, base := accountRecurCandidates(app, props.Account.ID)
	if len(cands) == 0 {
		return Fragment()
	}

	open := expanded.Get()
	listID := "acct-recur-list-" + props.Account.ID
	head := Button(css.Class("budget-drivers-toggle"), Type("button"),
		Attr("data-testid", "acct-recur-toggle-"+props.Account.ID),
		Attr("aria-expanded", ariaBool(open)), Attr("aria-controls", listID), OnClick(toggle),
		Span(uistate.T("sm6.hint", len(cands))),
		uiw.Icon(icon.ChevronDown, css.Class("budget-drivers-chev", tw.W35, tw.H35)))

	var body ui.Node = Fragment()
	if open {
		rows := make([]ui.Node, 0, len(cands))
		for _, c := range cands {
			rows = append(rows, ui.CreateElement(accountRecurRow, accountRecurRowProps{
				AccountID: props.Account.ID,
				Candidate: c,
				Base:      base,
			}))
		}
		body = Div(css.Class("acct-recur-list"), Attr("id", listID), rows)
	}
	return Div(css.Class("acct-recur"), Attr("data-testid", "acct-recur-"+props.Account.ID), head, body)
}

// accountRecurCandidates returns the account's UNTRACKED repeats, capped, plus
// the base currency their typical amounts are stated in.
//
// It reads rhyDiscover's frame-memoized result rather than running its own sweep:
// the Bills surface, the review strip and this panel would otherwise each sweep
// the whole ledger for the same answer. Candidates whose commitment already exists
// are excluded by Discover itself, so what is left is genuinely untracked.
func accountRecurCandidates(app *appstate.App, accountID string) ([]recurdiscover.Candidate, string) {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	res := rhyDiscover(app, rates, time.Now())
	var out []recurdiscover.Candidate
	for _, c := range res.Candidates {
		if c.AccountID != accountID {
			continue
		}
		out = append(out, c)
		if len(out) >= acctRecurCap {
			break
		}
	}
	return out, base
}

// accountRecurRowProps is one detected repeat.
type accountRecurRowProps struct {
	AccountID string
	Candidate recurdiscover.Candidate
	Base      string
}

// accountRecurRow renders one repeat with its cadence, typical amount, and the
// track action. Its own component — it renders inside a variable-length list and
// owns the click hook.
func accountRecurRow(props accountRecurRowProps) ui.Node {
	c := props.Candidate
	track := ui.UseEvent(Prevent(func() {
		a := appstate.Default
		if a == nil {
			return
		}
		if r, ok := ConfirmRecurCandidate(a, c, time.Now(), props.Base); ok {
			uistate.PostUndoable(uistate.T("sm6.trackedUndo", r.Label))
		}
	}))

	amt := money.New(c.Evidence.Amount.Typical, props.Base)
	cad := recurdiscover.DomainCadence(c.Evidence.Cadence)
	return Div(css.Class("acct-recur-row"),
		Div(css.Class("acct-recur-main"),
			Span(css.Class("acct-recur-name"), c.Payee),
			Span(css.Class("acct-recur-meta", tw.Text12, tw.TextDim),
				uistate.T("sm6.rowMeta", cadenceLabel(cad), fmtMoney(amt))),
		),
		Button(css.Class("btn btn-sm acct-recur-track"), Type("button"),
			Attr("data-testid", "acct-recur-track-"+c.Signature),
			OnClick(track), uistate.T("sm6.makeRecur")),
	)
}
