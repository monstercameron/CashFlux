// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// ConfirmRecurCandidate turns a discovered candidate into a tracked recurring
// commitment and back-claims the cycles it was detected from.
//
// It was extracted when SM-6 gave the same action a second home (the per-account
// "repeating charges here" panel). The back-claim is the part that must not be
// duplicated: confirming without linking the evidence transactions leaves the new
// commitment looking like it has never been paid, so the bills surface immediately
// reports every past cycle as missed. Two copies of that would have stayed correct
// for exactly as long as nobody edited one of them.
//
// Returns the created commitment and whether the write landed. A failure is reported to the person
// as a notice. When a back-claim link cannot be saved, the links already saved and the new
// commitment are removed again, so no half-claimed commitment is left behind.
func ConfirmRecurCandidate(app *appstate.App, c recurdiscover.Candidate, now time.Time, base string) (domain.Recurring, bool) {
	if app == nil {
		return domain.Recurring{}, false
	}
	m, ok := recurCandidateAmount(app, c, base)
	if !ok {
		return domain.Recurring{}, false
	}
	cad := recurdiscover.DomainCadence(c.Evidence.Cadence)
	r := domain.Recurring{
		ID:        id.New(),
		Label:     c.Payee,
		Amount:    m,
		Cadence:   cad,
		NextDue:   rhyNextDue(cad, c.Evidence.LastSeen, now),
		AccountID: c.AccountID,
	}
	if err := app.PutRecurring(r); err != nil {
		postAcctWriteError("track recurring", err)
		return domain.Recurring{}, false
	}
	// Back-claim the observed cycles so the new commitment starts life with its
	// history attached rather than looking permanently overdue.
	dateByID := map[string]time.Time{}
	for _, t := range app.Transactions() {
		dateByID[t.ID] = t.Date
	}
	var linkIDs []string
	for _, tid := range c.Evidence.TxnIDs {
		link := domain.TxnLink{
			ID: id.New(), Kind: domain.TxnLinkBillMatch, TxnIDs: []string{tid},
			RecurringID: r.ID, OccurrenceDate: dateByID[tid], CreatedAt: now,
		}
		if err := app.PutTxnLink(link); err != nil {
			rollbackRecurCandidate(app, r.ID, linkIDs)
			postAcctWriteError("link recurring history", err)
			uistate.BumpDataRevision()
			return domain.Recurring{}, false
		}
		linkIDs = append(linkIDs, link.ID)
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	return r, true
}

// rollbackRecurCandidate removes a commitment whose back-claim failed, along with the links already
// saved for it. A removal that fails is logged; the person has already been told the tracking failed.
func rollbackRecurCandidate(app *appstate.App, recurringID string, linkIDs []string) {
	for _, linkID := range linkIDs {
		if err := app.DeleteTxnLink(linkID); err != nil {
			slog.Error("recurring: roll back link", "link", linkID, "err", err)
		}
	}
	if err := app.DeleteRecurring(recurringID); err != nil {
		slog.Error("recurring: roll back commitment", "recurring", recurringID, "err", err)
	}
}

// recurCandidateAmount is the candidate's typical amount as a recurring amount, signed by its
// direction. Discovery works in the base currency, but a recurring commitment posts to its account
// and an account only takes its own currency, so the amount is converted to the account's currency.
// When no rate allows the conversion, the person is told and the second result is false.
func recurCandidateAmount(app *appstate.App, c recurdiscover.Candidate, base string) (money.Money, bool) {
	m := money.New(c.Evidence.Amount.Typical, base)
	if c.Direction == recurdiscover.Out {
		m = m.Neg()
	}
	if c.AccountID == "" {
		return m, true
	}
	for _, ac := range app.Accounts() {
		if ac.ID != c.AccountID || ac.Currency == "" || ac.Currency == base {
			continue
		}
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		conv, err := rates.Convert(m, ac.Currency)
		if err != nil {
			uistate.PostNotice(uistate.T("sm6.trackRateUnavailable", ac.Currency), true)
			slog.Warn("recurring: no rate to convert candidate", "account", ac.ID, "err", err)
			return money.Money{}, false
		}
		return conv, true
	}
	return m, true
}
