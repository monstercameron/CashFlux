// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
// Returns the created commitment and whether the write landed.
func ConfirmRecurCandidate(app *appstate.App, c recurdiscover.Candidate, now time.Time, base string) (domain.Recurring, bool) {
	if app == nil {
		return domain.Recurring{}, false
	}
	m := money.New(c.Evidence.Amount.Typical, base)
	if c.Direction == recurdiscover.Out {
		m = m.Neg()
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
		uistate.PostNotice(err.Error(), true)
		return domain.Recurring{}, false
	}
	// Back-claim the observed cycles so the new commitment starts life with its
	// history attached rather than looking permanently overdue.
	dateByID := map[string]time.Time{}
	for _, t := range app.Transactions() {
		dateByID[t.ID] = t.Date
	}
	for _, tid := range c.Evidence.TxnIDs {
		_ = app.PutTxnLink(domain.TxnLink{
			ID: id.New(), Kind: domain.TxnLinkBillMatch, TxnIDs: []string{tid},
			RecurringID: r.ID, OccurrenceDate: dateByID[tid], CreatedAt: now,
		})
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	return r, true
}
