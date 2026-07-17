// SPDX-License-Identifier: MIT

// Package appstate — transfer orchestration helpers.
//
// This file holds CreateTransferPair, which records the two transaction legs
// that make up an inter-account transfer. It lives here (rather than in the
// accounts or transactions screen) so both the accounts Transfer button and any
// future caller (AI agent, CSV import reconciliation) share the same logic path.
//
// No syscall/js dependency; the file may be unit-tested on native Go.
package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TransferParams holds the caller-supplied inputs for a transfer pair.
type TransferParams struct {
	// FromAccountID is the source account (money leaves here, amount is negative).
	FromAccountID string
	// ToAccountID is the destination account (money arrives here, amount is positive).
	ToAccountID string
	// AmountMinor is the transfer amount in the source account's minor units (must be > 0).
	AmountMinor int64
	// Date is the effective date of both legs. Zero is treated as time.Now().
	Date time.Time
	// Desc is the human-readable description shared by both legs. When empty,
	// "Transfer" is used.
	Desc string
}

// CreateTransferPair records a paired inter-account transfer: a negative "out"
// leg on the source account and a positive "in" leg on the destination account.
// The in-leg amount is FX-converted via the app's current exchange-rate table
// when the two accounts use different currencies; if no rate is available the
// amount is copied as-is (same minor units, destination currency) so the caller
// always gets two legs even without a configured FX rate.
//
// Both legs are written atomically via PutTransaction; if the second write fails
// the first is not rolled back (callers should treat a non-nil error as an
// inconsistent state and surface it to the user).
//
// Returns the out-leg and in-leg transaction IDs on success so callers can
// navigate to the ledger or construct an undo snapshot.
func (a *App) CreateTransferPair(p TransferParams) (outID, inID string, err error) {
	if p.FromAccountID == "" {
		return "", "", fmt.Errorf("transfer: FromAccountID is required")
	}
	if p.ToAccountID == "" {
		return "", "", fmt.Errorf("transfer: ToAccountID is required")
	}
	if p.FromAccountID == p.ToAccountID {
		return "", "", fmt.Errorf("transfer: source and destination accounts must differ")
	}
	if p.AmountMinor <= 0 {
		return "", "", fmt.Errorf("transfer: amount must be greater than zero")
	}

	var fromAcc, toAcc domain.Account
	var foundFrom, foundTo bool
	for _, ac := range a.Accounts() {
		switch ac.ID {
		case p.FromAccountID:
			fromAcc, foundFrom = ac, true
		case p.ToAccountID:
			toAcc, foundTo = ac, true
		}
	}
	if !foundFrom {
		return "", "", fmt.Errorf("transfer: source account %q not found", p.FromAccountID)
	}
	if !foundTo {
		return "", "", fmt.Errorf("transfer: destination account %q not found", p.ToAccountID)
	}

	when := p.Date
	if when.IsZero() {
		when = time.Now()
	}
	desc := p.Desc
	if desc == "" {
		desc = "Transfer"
	}

	fromMoney := money.New(-p.AmountMinor, fromAcc.Currency)
	toMoney := money.New(p.AmountMinor, toAcc.Currency) // fallback: same minor units
	if fromAcc.Currency != toAcc.Currency {
		s := a.Settings()
		rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
		srcAbs := money.New(p.AmountMinor, fromAcc.Currency)
		if conv, cerr := rates.Convert(srcAbs, toAcc.Currency); cerr == nil {
			toMoney = conv
		}
	}
	// A transfer into a liability is a payment and must REDUCE the debt. Liability
	// balances carry two at-rest sign conventions (the sample data stores debts
	// negative; the "amount you owe" add form stores them positive), so a blanket
	// positive in-leg grew positive-stored debts instead of paying them down. Pick
	// the sign per account: whichever moves its booked balance toward zero.
	if toAcc.Class == domain.ClassLiability {
		toMoney.Amount = liabilityPaymentMinor(toAcc, a.Transactions(), toMoney.Amount)
	}

	outID = id.New()
	inID = id.New()

	// C68: a transfer is an explicit, unambiguous action the user just performed,
	// so both legs are inherently "reviewed" — Reviewed:true keeps the ActionFlagReview
	// workflow from auto-tagging them #needs-review (which read as a false alarm).
	out := domain.Transaction{
		ID: outID, AccountID: fromAcc.ID,
		TransferAccountID: toAcc.ID,
		Amount:            fromMoney,
		Date:              when,
		Desc:              desc,
		Payee:             toAcc.Name,
		Reviewed:          true,
		Source:            domain.TxnSourceManual,
	}
	in := domain.Transaction{
		ID: inID, AccountID: toAcc.ID,
		TransferAccountID: fromAcc.ID,
		Amount:            toMoney,
		Date:              when,
		Desc:              desc,
		Payee:             fromAcc.Name,
		Reviewed:          true,
		Source:            domain.TxnSourceManual,
	}

	if err := a.PutTransaction(out); err != nil {
		return "", "", fmt.Errorf("transfer: record out-leg: %w", err)
	}
	if err := a.PutTransaction(in); err != nil {
		return "", "", fmt.Errorf("transfer: record in-leg: %w", err)
	}
	return outID, inID, nil
}

// liabilityPaymentMinor returns the signed minor-unit amount for a payment leg
// posted to a liability account: the sign that moves the account's booked
// balance toward zero under its own at-rest convention. A zero balance falls
// back to the opening balance's sign; when both are zero the debt is settled
// and the leg posts positive (the sample-data convention). amountMinor may be
// passed with either sign — only its magnitude is used.
func liabilityPaymentMinor(acc domain.Account, all []domain.Transaction, amountMinor int64) int64 {
	if amountMinor < 0 {
		amountMinor = -amountMinor
	}
	ref := acc.OpeningBalance.Amount
	if bal, err := ledger.Balance(acc, all); err == nil && bal.Amount != 0 {
		ref = bal.Amount
	}
	if ref > 0 {
		return -amountMinor // debt stored positive-owed: a payment subtracts
	}
	return amountMinor // debt stored negative (or settled): a payment adds toward zero
}
