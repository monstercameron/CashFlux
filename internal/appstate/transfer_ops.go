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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// ErrTransferRateUnavailable identifies a cross-currency transfer that cannot
// be priced from the saved rates and has no user-supplied received amount.
var ErrTransferRateUnavailable = errors.New("transfer: exchange rate unavailable; enter the amount received")

// TransferParams holds the caller-supplied inputs for a transfer pair.
type TransferParams struct {
	// OperationID identifies one user action and must be reused when retrying it.
	// An empty value is replaced with a fresh ID for backward compatibility.
	OperationID string
	// Source records which product surface created the transfer. Zero defaults to manual.
	Source domain.TxnSource
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

// PutTransactionWithTransferPair writes an edited transaction and, when it is one
// leg of a transfer, mirrors the edit onto the reciprocal leg so the pair stays
// balanced. It takes the transaction as it was BEFORE the edit (prior) as well as
// the edited copy (next), because the reciprocal leg is located by matching the
// pair's old amount and date — after the edit lands they no longer agree.
//
// C629: a transfer is two independent rows, and the edit form only ever wrote the
// one row the user opened. Correcting a transfer's amount from the ledger updated
// one account and not the other: the legs stopped summing to zero, so the net
// total moved by the edit even though no money entered or left the household, and
// every downstream figure that sums transactions inherited the error. It also
// broke delete-both, because isReciprocalTransferLeg matches on equal dates and
// exactly opposite amounts — an edited leg no longer looks reciprocal, so deleting
// it silently orphaned its counterpart.
//
// Only the two fields that define the pairing are mirrored: the amount (through
// the same FX and liability-signing path CreateTransferPair uses, so a
// cross-currency pair converts rather than being copied) and the date. Per-leg
// fields are deliberately left alone — Payee in particular is the OTHER account's
// name on each leg, so copying it across would be wrong.
//
// Observers fire once, after both legs are written.
func (a *App) PutTransactionWithTransferPair(prior, next domain.Transaction) error {
	return a.PutTransactionWithTransferPairReceived(prior, next, 0)
}

// PutTransactionWithTransferPairReceived edits a transfer leg and its partner
// atomically. For a cross-currency amount edit, receivedMinor may explicitly
// supply the partner amount in the partner's currency when no rate is available.
func (a *App) PutTransactionWithTransferPairReceived(prior, next domain.Transaction, receivedMinor int64) error {
	if receivedMinor < 0 {
		return fmt.Errorf("save transfer pair: received amount must not be negative")
	}
	if !next.IsTransfer() {
		if prior.IsTransfer() {
			next.TransferAccountID, next.TransferGroupID = "", ""
			if pair, found := a.transferPartner(prior); found {
				pair.TransferAccountID, pair.TransferGroupID = "", ""
				if err := a.roleGuard(); err != nil {
					return err
				}
				for _, txn := range []domain.Transaction{next, pair} {
					if issues := validate.ValidateTransaction(txn, a.Accounts()...); !issues.OK() {
						return issues
					}
					if err := a.validateCustom("transaction", txn.Custom); err != nil {
						return err
					}
				}
				if err := a.store.PutTransactionsAtomic([]domain.Transaction{next, pair}); err != nil {
					return fmt.Errorf("clear transfer pair: %w", err)
				}
				a.fireTxnMutated()
				return nil
			}
		}
		return a.PutTransaction(next)
	}
	amountChanged := prior.Amount.Amount != next.Amount.Amount
	dateChanged := !prior.Date.Equal(next.Date)
	if !amountChanged && !dateChanged {
		return a.PutTransaction(next)
	}

	// Locate the counterpart against the PRE-edit values, while the pair still matches.
	var pair domain.Transaction
	found := false
	rates := a.transactionPairRates()
	for _, t := range a.Transactions() {
		groupMatch := prior.TransferGroupID != "" && t.TransferGroupID == prior.TransferGroupID && t.ID != prior.ID
		if groupMatch || isReciprocalTransferLegWithRates(prior, t, rates) {
			pair, found = t, true
			break
		}
	}
	if !found {
		// Nothing to keep in step (an already-orphaned leg, or a cross-currency pair
		// that isReciprocalTransferLeg cannot match). Write the one row and let the
		// caller's own validation speak.
		return a.PutTransaction(next)
	}

	if amountChanged {
		mirrored, err := a.mirroredLegAmount(next, pair, receivedMinor)
		if err != nil {
			return err
		}
		pair.Amount = mirrored
	}
	if dateChanged {
		pair.Date = next.Date
	}

	// Suppress per-leg firings so the surface re-renders once, with both legs written.
	if err := a.roleGuard(); err != nil {
		return err
	}
	for _, txn := range []domain.Transaction{next, pair} {
		if issues := validate.ValidateTransaction(txn, a.Accounts()...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", txn.Custom); err != nil {
			return err
		}
	}
	if err := a.store.PutTransactionsAtomic([]domain.Transaction{next, pair}); err != nil {
		return fmt.Errorf("save transfer pair: %w", err)
	}
	a.log.Info("transfer pair saved", "ids", []string{next.ID, pair.ID})
	a.fireTxnMutated()
	return nil
}

func (a *App) transferPartner(prior domain.Transaction) (domain.Transaction, bool) {
	rates := a.transactionPairRates()
	for _, txn := range a.Transactions() {
		if txn.ID == prior.ID {
			continue
		}
		if prior.TransferGroupID != "" && txn.TransferGroupID == prior.TransferGroupID || isReciprocalTransferLegWithRates(prior, txn, rates) {
			return txn, true
		}
	}
	return domain.Transaction{}, false
}

// mirroredLegAmount returns the amount the counterpart leg must carry so the pair
// still represents one move of money: the negation of the edited leg, converted
// into the counterpart's own currency and liability-signed the same way
// CreateTransferPair signs a payment into a debt account.
func (a *App) mirroredLegAmount(edited, pair domain.Transaction, receivedMinor int64) (money.Money, error) {
	abs := edited.Amount.Amount
	if abs < 0 {
		abs = -abs
	}
	out := money.New(abs, pair.Amount.Currency)
	if edited.Amount.Currency != pair.Amount.Currency {
		if receivedMinor > 0 {
			out = money.New(receivedMinor, pair.Amount.Currency)
		} else {
			s := a.Settings()
			rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
			conv, cerr := rates.Convert(money.New(abs, edited.Amount.Currency), pair.Amount.Currency)
			if cerr != nil {
				return money.Money{}, fmt.Errorf("%w: %s to %s", ErrTransferRateUnavailable, edited.Amount.Currency, pair.Amount.Currency)
			}
			out = conv
		}
	}
	accounts := a.Accounts()
	var editedAccount, pairAccount domain.Account
	for _, acc := range accounts {
		switch acc.ID {
		case edited.AccountID:
			editedAccount = acc
		case pair.AccountID:
			pairAccount = acc
		}
	}
	if (editedAccount.Class == domain.ClassLiability) != (pairAccount.Class == domain.ClassLiability) {
		// A liability payment has a sign based on that account's stored balance
		// convention. On the asset side it is always an outgoing (negative) leg;
		// on the liability side liabilityPaymentMinor supplies the signed payment.
		liability := editedAccount
		if pairAccount.Class == domain.ClassLiability {
			liability = pairAccount
		}
		payment := liabilityPaymentMinor(liability, a.Transactions(), out.Amount)
		if editedAccount.Class == domain.ClassLiability {
			editedPayment := liabilityPaymentMinor(editedAccount, a.Transactions(), abs)
			if (edited.Amount.Amount < 0) == (editedPayment < 0) {
				out.Amount = -out.Amount
			}
		} else {
			if edited.Amount.Amount < 0 {
				out.Amount = payment
			} else {
				out.Amount = -payment
			}
		}
	} else {
		// Transfers between accounts of the same class retain the ordinary
		// opposite-sign convention.
		if !edited.Amount.IsNegative() {
			out.Amount = -out.Amount
		}
	}
	return out, nil
}

// CreateTransferPair records both transfer legs and an optional fee in a single
// SQLite transaction. Cross-currency transfers require a valid saved rate or an
// explicit ReceivedMinor. Reuse OperationID when retrying the same action.
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
	if strings.TrimSpace(p.OperationID) == "" {
		p.OperationID = id.New()
	}

	fromAcc, toAcc, ferr := a.transferAccounts(p.FromAccountID, p.ToAccountID)
	if ferr != nil {
		return "", "", ferr
	}

	when := p.Date
	if when.IsZero() {
		when = time.Now()
	}
	when = normalizeTransactionDate(when)
	desc := p.Desc
	if desc == "" {
		desc = "Transfer"
	}
	source := p.Source
	if source == "" {
		source = domain.TxnSourceManual
	}

	fromMoney, toMoney, err := a.transferLegs(fromAcc, toAcc, p.AmountMinor, p.ReceivedMinor)
	if err != nil {
		return "", "", err
	}
	outID, inID = transferID(p.OperationID, "out"), transferID(p.OperationID, "in")
	if p.Date.IsZero() {
		for _, existing := range a.Transactions() {
			if existing.ID == outID {
				when = existing.Date
				break
			}
		}
	}
	groupID := transferGroupID(p, when, desc, source)
	// C680: one id naming the RELATIONSHIP, carried by both legs. Without it every
	// pair operation has to re-derive the partner from date, amount, sign and
	// account — a derivation that fails on the cases it is most needed for: an
	// edited amount stops the legs looking reciprocal, and an FX pair never matches
	// at all because its two legs carry different magnitudes by design.

	// C68: a transfer is an explicit, unambiguous action the user just performed,
	// so both legs are inherently "reviewed" — Reviewed:true keeps the ActionFlagReview
	// workflow from auto-tagging them #needs-review (which read as a false alarm).
	out := domain.Transaction{
		ID: outID, AccountID: fromAcc.ID,
		TransferAccountID: toAcc.ID,
		TransferGroupID:   groupID,
		Amount:            fromMoney,
		Date:              when,
		Desc:              desc,
		Payee:             toAcc.Name,
		Reviewed:          true,
		Source:            source,
	}
	in := domain.Transaction{
		ID: inID, AccountID: toAcc.ID,
		TransferAccountID: fromAcc.ID,
		TransferGroupID:   groupID,
		Amount:            toMoney,
		Date:              when,
		Desc:              desc,
		Payee:             fromAcc.Name,
		Reviewed:          true,
		Source:            source,
	}

	txns := []domain.Transaction{out, in}
	if p.FeeMinor > 0 {
		// The fee is real spending, not part of the moved amount, so it posts
		// as a plain expense (no TransferAccountID — it must count in totals).
		fee := domain.Transaction{
			ID: transferID(p.OperationID, "fee"), AccountID: fromAcc.ID,
			Amount:   money.New(-p.FeeMinor, fromAcc.Currency),
			Date:     when,
			Desc:     desc + " — fee",
			Payee:    toAcc.Name,
			Reviewed: true,
			Source:   source,
		}
		txns = append(txns, fee)
	}
	if err := a.roleGuard(); err != nil {
		return "", "", err
	}
	for _, txn := range txns {
		if issues := validate.ValidateTransaction(txn); !issues.OK() {
			return "", "", issues
		}
		if err := a.validateCustom("transaction", txn.Custom); err != nil {
			return "", "", err
		}
	}
	committed, err := a.store.PutTransferBatchAtomic(p.OperationID, txns)
	if err != nil {
		return "", "", fmt.Errorf("transfer: commit transfer: %w", err)
	}
	if committed {
		a.log.Info("transfer committed", "operation_id", p.OperationID, "ids", []string{outID, inID})
		a.fireTxnMutated()
	}
	return outID, inID, nil
}

func transferID(operationID, leg string) string {
	sum := sha256.Sum256([]byte(operationID))
	return "transfer-" + hex.EncodeToString(sum[:16]) + "-" + leg
}

func transferGroupID(p TransferParams, when time.Time, desc string, source domain.TxnSource) string {
	payload := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%s|%s|%s", p.OperationID, p.FromAccountID, p.ToAccountID,
		p.AmountMinor, p.ReceivedMinor, p.FeeMinor, when.UTC().Format(time.RFC3339Nano), desc, source)
	sum := sha256.Sum256([]byte(payload))
	return "transfer-group-" + hex.EncodeToString(sum[:16])
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
// currency when the currencies differ. Missing FX returns an error.
// receivedMinor > 0 overrides the cross-currency in-leg magnitude (destination
// minor units) — the user's actual landed amount beats the saved-rate estimate;
// it is ignored for same-currency pairs, where the moved amount IS the amount.
//
// A transfer into a liability is a payment and must REDUCE the debt. Liability
// balances carry two at-rest sign conventions (the sample data stores debts
// negative; the "amount you owe" add form stores them positive), so a blanket
// positive in-leg grew positive-stored debts instead of paying them down. Pick
// the sign per account: whichever moves its booked balance toward zero.
func (a *App) transferLegs(fromAcc, toAcc domain.Account, amountMinor, receivedMinor int64) (fromMoney, toMoney money.Money, err error) {
	fromMoney = money.New(-amountMinor, fromAcc.Currency)
	toMoney = money.New(amountMinor, toAcc.Currency)
	if fromAcc.Currency != toAcc.Currency {
		if receivedMinor > 0 {
			toMoney = money.New(receivedMinor, toAcc.Currency)
		} else {
			s := a.Settings()
			rates := currency.Rates{Base: s.BaseCurrency, Rates: s.FXRates}
			srcAbs := money.New(amountMinor, fromAcc.Currency)
			conv, cerr := rates.Convert(srcAbs, toAcc.Currency)
			if cerr != nil {
				return money.Money{}, money.Money{}, fmt.Errorf("%w: %s to %s", ErrTransferRateUnavailable, fromAcc.Currency, toAcc.Currency)
			}
			toMoney = conv
		}
	}
	if toAcc.Class == domain.ClassLiability {
		toMoney.Amount = liabilityPaymentMinor(toAcc, a.Transactions(), toMoney.Amount)
	}
	return fromMoney, toMoney, nil
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
	fromMoney, toMoney, err := a.transferLegs(fromAcc, toAcc, p.AmountMinor, p.ReceivedMinor)
	if err != nil {
		return TransferPreview{}, err
	}
	return TransferPreview{
		FromBefore: fromBal,
		FromAfter:  money.New(fromBal.Amount+fromMoney.Amount-p.FeeMinor, fromAcc.Currency),
		ToBefore:   toBal,
		ToAfter:    money.New(toBal.Amount+toMoney.Amount, toAcc.Currency),
	}, nil
}
