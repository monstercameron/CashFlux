// SPDX-License-Identifier: MIT

//go:build js && wasm

// The repeats detected on one account.
//
// The recurring-charge detector reports its findings for the whole household on
// /subscriptions. This panel narrows them to one account: the repeats found here,
// each with the one action worth having on them, start tracking it as a recurring
// commitment.
//
// The detection is internal/recurdiscover, run by rhyDiscover and filtered to the
// account. The run is memoised per frame, so every row's panel reads the same result and
// five accounts cost one sweep, not five. Confirming goes through the same ConfirmRecurCandidate the review
// surface uses, back-claim included.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// acctRecurCode is the feature code for recurring-charge detection. It is free, so
// the panel needs no provider and defaults on.
const acctRecurCode = "SMART-A7"

// acctRecurCap bounds how many repeats the panel lists. An account with thirty repeats is a list,
// not a finding; the Bills & recurring surface is where a full roster belongs. The panel's count
// still states every repeat found.
const acctRecurCap = 6

// accountRecurProps carries the account whose repeats are listed.
type accountRecurProps struct {
	Account domain.Account
}

// recurSlot is the empty line a row keeps when it has no recurring charges, so every row
// is the same height whether or not it has some.
func recurSlot() ui.Node {
	return Div(css.Class("acct-recur acct-recur-empty"), Attr("aria-hidden", "true"))
}

// accountRecurringPanel renders the collapsed "N charges here repeat" disclosure. It is its
// own component: it subscribes to the data revision. The discovery result is read on every render
// so the count in the header is current, and it is memoised per frame.
func accountRecurringPanel(props accountRecurProps) ui.Node {
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	settings := uistate.LoadSmartSettings()

	if app == nil || props.Account.Archived {
		return recurSlot()
	}
	if !settings.IsEnabled(acctRecurCode) || settings.IsMuted(acctRecurCode) {
		return recurSlot()
	}
	if !settings.DensityOrDefault().Shows(smart.AffordanceTooltip) {
		return recurSlot()
	}

	cands, base := accountRecurCandidates(app, props.Account.ID)
	if len(cands) == 0 {
		return recurSlot()
	}

	listed := cands[:min(len(cands), acctRecurCap)]
	rows := make([]ui.Node, 0, len(listed))
	for _, c := range listed {
		rows = append(rows, WithKey(ui.CreateElement(accountRecurRow, accountRecurRowProps{
			App:       app,
			Candidate: c,
			Base:      base,
		}), c.Signature))
	}
	body := Fragment(
		Div(css.Class("rows", "acct-recur-list"), rows),
		If(len(cands) > len(listed), uiw.Callout(uiw.CalloutProps{
			TestID: "acct-recur-capnote-" + props.Account.ID,
			Body:   Span(uistate.T("sm6.capNote", len(listed))),
		})),
	)
	return Div(css.Class("acct-recur"), Attr("data-testid", "acct-recur-"+props.Account.ID),
		uiw.Disclosure(uiw.DisclosureProps{
			ID:      "acct-recur-list-" + props.Account.ID,
			Summary: Span(uistate.TN("sm6.hintOne", "sm6.hintMany", len(cands))),
			Body:    body,
			TestID:  "acct-recur-toggle-" + props.Account.ID,
		}))
}

// accountRecurCandidates returns all of the account's UNTRACKED repeats, plus
// the base currency their typical amounts are stated in.
//
// It reads rhyDiscover's frame-memoized result rather than running its own sweep:
// the Bills surface, the review strip and this panel would otherwise each sweep
// the whole ledger for the same answer. Candidates whose commitment already exists
// are excluded by Discover itself, so what is left is genuinely untracked.
func accountRecurCandidates(app *appstate.App, accountID string) ([]recurdiscover.Candidate, string) {
	base, rates := accountBaseAndRates(app)
	return rhyDiscover(app, rates, time.Now()).ForAccount(accountID, 0), base
}

// accountRecurRowProps is one detected repeat.
type accountRecurRowProps struct {
	App       *appstate.App
	Candidate recurdiscover.Candidate
	Base      string
}

// accountRecurRow renders one repeat with its cadence, typical amount, and the
// track action. Its own component — it renders inside a variable-length list and
// owns the click hook.
func accountRecurRow(props accountRecurRowProps) ui.Node {
	c := props.Candidate
	track := ui.UseEvent(Prevent(func() {
		if r, ok := ConfirmRecurCandidate(props.App, c, time.Now(), props.Base); ok {
			uistate.PostUndoable(uistate.T("sm6.trackedUndo", r.Label))
		}
	}))

	amt := money.New(c.Evidence.Amount.Typical, props.Base)
	cad := recurdiscover.DomainCadence(c.Evidence.Cadence)
	return uiw.EntityRow(uiw.EntityRowProps{
		Title: c.Payee,
		Meta:  []string{uistate.T("sm6.rowMeta", cadenceLabel(cad), fmtMoney(amt))},
		Actions: []ui.Node{uiw.ToolButton(uiw.ToolButtonProps{
			Label: uistate.T("sm6.makeRecur"), AriaLabel: uistate.T("sm6.makeRecurAria", c.Payee),
			Class: "acct-recur-track", TestID: "acct-recur-track-" + c.Signature,
		}, OnClick(track))},
	})
}
