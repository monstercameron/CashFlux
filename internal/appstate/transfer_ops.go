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
	// ReceivedMinor optionally overrides the in-leg amount, in the DESTINATION
	// account's minor units — for cross-currency transfers where the user knows
	// the exact amount that landed (their bank's rate, not the saved table's).
	// Zero means "convert at the saved rate" (the default). Must not be
	// negative; magnitude only (liability payment signing still applies).
	ReceivedMinor int64
	// FeeMinor optionally records a transfer fee, in the SOURCE account's minor
	// units. It posts as a third, real expense transaction on the source
	// account (fees are spending, not part of the moved amount). Zero means no
	// fee. Must not be negative.
	FeeMinor int64
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
	if p.ReceivedMinor < 0 {
		return "", "", fmt.Errorf("transfer: received amount must not be negative")
	}
	if p.FeeMinor < 0 {
		return "", "", fmt.Errorf("transfer: fee must not be negative")
	}

	fromAcc, toAcc, ferr := a.transferAccounts(p.FromAccountID, p.ToAccountID)
	if ferr != nil {
		return "", "", ferr
	}

	when := p.Date
	if when.IsZero() {
		when = time.Now()
	}
	desc := p.Desc
	if desc == "" {
		desc = "Transfer"
	}

	fromMoney, toMoney := a.transferLegs(fromAcc, toAcc, p.AmountMinor, p.ReceivedMinor)

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
	if p.FeeMinor > 0 {
		// The fee is real spending, not part of the moved amount, so it posts
		// as a plain expense (no TransferAccountID — it must count in totals).
		fee := domain.Transaction{
			ID: id.New(), AccountID: fromAcc.ID,
			Amount:   money.New(-p.FeeMinor, fromAcc.Currency),
			Date:     when,
			Desc:     desc + " — fee",
			Payee:    toAcc.Name,
			Reviewed: true,
			Source:   domain.TxnSourceManual,
		}
		if err := a.PutTransaction(fee); err != nil {
			return "", "", fmt.Errorf("transfer: record fee: %w", err)
		}
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

// transferAccounts resolves and validates the two accounts of a transfer pair.
func (a *App) transferAccounts(fromID, toID string) (fromAcc, toAcc domain.Account, err error) {
	var foundFrom, foundTo bool
	for _, ac := range a.Accounts() {
		switch ac.ID {
		case fromID:
			fromAcc, foundFrom = ac, true
		case toID:
			toAcc, foundTo = ac, true
		}
	}
	if !foundFrom {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: source account %q not found", fromID)
	}
	if !foundTo {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: destination account %q not found", toID)
	}
	return fromAcc, toAcc, nil
}

// transferLegs computes the signed out/in leg amounts for a transfer of
// amountMinor (source minor units, > 0) between the two accounts: a negative
// out leg in the source currency, and an in leg FX-converted to the destination
// currency when the currencies differ (copied as-is when no rate exists).
// receivedMinor > 0 overrides the cross-currency in-leg magnitude (destination
// minor units) — the user's actual landed amount beats the saved-rate estimate;
// it is ignored for same-currency pairs, where the moved amount IS the amount.
//
// A transfer into a liability is a payment and must REDUCE the debt. Liability
// balances carry two at-rest sign conventions (the sample data stores debts
// negative; the "amount you owe" add form stores them positive), so a blanket
// positive in-leg grew positive-stored debts instead of paying them down. Pick
// the sign per account: whichever moves its booked balance toward zero.
func (a *App) transferLegs(fromAcc, toAcc domain.Account, amountMinor, receivedMinor int64) (fromMoney, toMoney money.Money) {
	fromMoney = money.New(-amountMinor, fromAcc.Currency)
	toMoney = money.New(amountMinor, toAcc.Currency) // fallback: same minor units
	if fromAcc.Currency != toAcc.Currency {
		if receivedMinor > 0 {
			toMoney = money.New(receivedMinor, toAcc.Currency)
		} else {
			s := a.Settings()
			rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
			srcAbs := money.New(amountMinor, fromAcc.Currency)
			if conv, cerr := rates.Convert(srcAbs, toAcc.Currency); cerr == nil {
				toMoney = conv
			}
		}
	}
	if toAcc.Class == domain.ClassLiability {
		toMoney.Amount = liabilityPaymentMinor(toAcc, a.Transactions(), toMoney.Amount)
	}
	return fromMoney, toMoney
}

// TransferPreview reports what a transfer pair would do to both booked
// balances, before anything posts. Amounts are in each account's own currency.
type TransferPreview struct {
	// FromBefore/FromAfter are the source account's booked balance now and
	// after the out leg.
	FromBefore, FromAfter money.Money
	// ToBefore/ToAfter are the destination account's booked balance now and
	// after the in leg (FX-converted and liability-signed like the real post).
	ToBefore, ToAfter money.Money
}

// PreviewTransferPair computes the before/after balances a CreateTransferPair
// call with the same params would produce, without writing anything. It shares
// the exact leg math (transferLegs) with CreateTransferPair so the preview can
// never drift from what actually posts.
func (a *App) PreviewTransferPair(p TransferParams) (TransferPreview, error) {
	if p.FromAccountID == "" || p.ToAccountID == "" {
		return TransferPreview{}, fmt.Errorf("transfer preview: both accounts are required")
	}
	if p.FromAccountID == p.ToAccountID {
		return TransferPreview{}, fmt.Errorf("transfer preview: source and destination accounts must differ")
	}
	if p.AmountMinor <= 0 {
		return TransferPreview{}, fmt.Errorf("transfer preview: amount must be greater than zero")
	}
	if p.ReceivedMinor < 0 || p.FeeMinor < 0 {
		return TransferPreview{}, fmt.Errorf("transfer preview: received amount and fee must not be negative")
	}
	fromAcc, toAcc, err := a.transferAccounts(p.FromAccountID, p.ToAccountID)
	if err != nil {
		return TransferPreview{}, err
	}
	txns := a.Transactions()
	fromBal, err := ledger.Balance(fromAcc, txns)
	if err != nil {
		return TransferPreview{}, fmt.Errorf("transfer preview: source balance: %w", err)
	}
	toBal, err := ledger.Balance(toAcc, txns)
	if err != nil {
		return TransferPreview{}, fmt.Errorf("transfer preview: destination balance: %w", err)
	}
	fromMoney, toMoney := a.transferLegs(fromAcc, toAcc, p.AmountMinor, p.ReceivedMinor)
	return TransferPreview{
		FromBefore: fromBal,
		FromAfter:  money.New(fromBal.Amount+fromMoney.Amount-p.FeeMinor, fromAcc.Currency),
		ToBefore:   toBal,
		ToAfter:    money.New(toBal.Amount+toMoney.Amount, toAcc.Currency),
	}, nil
}
