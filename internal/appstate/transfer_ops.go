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
	"github.com/monstercameron/CashFlux/internal/txnclassify"
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
	// FromAccountID is the source account (money leaves here).
	FromAccountID string
	// ToAccountID is the destination account (money arrives here).
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
	writes, err := a.planPairedTransactionEdit(prior, next, receivedMinor)
	if err != nil {
		return err
	}
	if len(writes) == 1 {
		return a.PutTransaction(next)
	}

	// Suppress per-leg firings so the surface re-renders once, with both legs written.
	if err := a.roleGuard(); err != nil {
		return err
	}
	for _, txn := range writes {
		if issues := validate.ValidateTransaction(txn, a.Accounts()...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", txn.Custom); err != nil {
			return err
		}
	}
	if err := a.store.PutTransactionsAtomic(writes); err != nil {
		return fmt.Errorf("save transfer pair: %w", err)
	}
	a.log.Info("transfer pair saved", "ids", expectedIDs(writes))
	a.fireTxnMutated()
	return nil
}

func (a *App) transferPartner(prior domain.Transaction) (domain.Transaction, bool) {
	rates := a.transactionPairRates()
	accounts := a.Accounts()
	for _, txn := range a.Transactions() {
		if txn.ID == prior.ID {
			continue
		}
		if prior.TransferGroupID != "" && txn.TransferGroupID == prior.TransferGroupID && txn.IsTransfer() && txn.AccountID == prior.TransferAccountID && txn.TransferAccountID == prior.AccountID {
			return txn, true
		}
		// Normalize the amounts to the same currency before the account-aware
		// legacy match. Explicit groups never fall back to resemblance.
		if prior.TransferGroupID == "" && txn.TransferGroupID == "" && prior.IsTransfer() &&
			txn.TransferAccountID == prior.AccountID && prior.Date.Equal(txn.Date) {
			left, right := prior, txn
			if left.Amount.Currency != right.Amount.Currency {
				var err error
				left.Amount, err = rates.Convert(left.Amount, rates.Base)
				if err != nil {
					continue
				}
				right.Amount, err = rates.Convert(right.Amount, rates.Base)
				if err != nil {
					continue
				}
			}
			if match, ok := txnclassify.FindReciprocal(left, prior.TransferAccountID, []domain.Transaction{right}, accounts...); ok && match.Exact {
				return txn, true
			}
		}
	}
	return domain.Transaction{}, false
}

// mirroredLegAmount converts the edited magnitude to the partner's currency,
// retaining the partner's booked direction even at zero or after an overpayment.
func (a *App) mirroredLegAmount(edited, pair domain.Transaction, receivedMinor int64) (money.Money, error) {
	magnitude, err := money.Abs(edited.Amount.Amount)
	if err != nil {
		return money.Money{}, fmt.Errorf("mirror transfer amount: %w", err)
	}
	out := money.New(magnitude, pair.Amount.Currency)
	if edited.Amount.Currency != pair.Amount.Currency {
		if receivedMinor > 0 {
			out = money.New(receivedMinor, pair.Amount.Currency)
		} else {
			conv, err := a.transactionPairRates().Convert(money.New(magnitude, edited.Amount.Currency), pair.Amount.Currency)
			if err != nil {
				return money.Money{}, fmt.Errorf("%w: %s to %s", ErrTransferRateUnavailable, edited.Amount.Currency, pair.Amount.Currency)
			}
			out = conv
		}
	}
	if pair.Amount.Amount < 0 {
		out.Amount = -out.Amount
	} else if pair.Amount.Amount == 0 {
		// A rounded-to-zero FX leg has no recorded sign. Derive its direction
		// from the fixed account conventions, never from either live balance.
		var editedAccount, pairAccount domain.Account
		for _, account := range a.Accounts() {
			if account.ID == edited.AccountID {
				editedAccount = account
			}
			if account.ID == pair.AccountID {
				pairAccount = account
			}
		}
		direction := int64(1)
		if edited.Amount.Amount < 0 {
			direction = -1
		}
		out.Amount *= -direction * editedAccount.InflowSign() * pairAccount.InflowSign()
	}
	return out, nil
}

// CreateTransferPair records both transfer legs and an optional fee in a single
// SQLite transaction. Cross-currency transfers require a valid saved rate or an
// explicit ReceivedMinor. Both accounts must exist and be active. Reuse OperationID
// when retrying the same action.
//
// Returns the out-leg and in-leg transaction IDs on success so callers can
// navigate to the ledger or construct an undo snapshot.
func (a *App) CreateTransferPair(p TransferParams) (outID, inID string, err error) {
	if err := a.ready(); err != nil {
		return "", "", err
	}
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
	if fromAcc.Archived || toAcc.Archived {
		return "", "", fmt.Errorf("transfer: archived accounts cannot be used")
	}

	if p.Date.IsZero() {
		p.Date = a.clock()
		// A retry without a date keeps the date its first attempt booked.
		if existing, found, gerr := a.store.GetTransaction(transferID(p.OperationID, "out")); gerr == nil && found {
			p.Date = existing.Date
		}
	}
	txns, outID, inID, err := buildTransferRows(p, fromAcc, toAcc, a.transactionPairRates())
	if err != nil {
		return "", "", err
	}
	if err := a.roleGuard(); err != nil {
		return "", "", err
	}
	accounts := a.Accounts()
	for _, txn := range txns {
		if issues := validate.ValidateTransaction(txn, accounts...); !issues.OK() {
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

// liabilityPaymentMinor signs a payment using the account's fixed convention, so
// live balances never decide the direction, including at zero and after an
// overpayment.
func liabilityPaymentMinor(acc domain.Account, amountMinor int64) int64 {
	if amountMinor < 0 {
		amountMinor = -amountMinor
	}
	return acc.InflowSign() * amountMinor
}

// transferAccounts resolves the two accounts of a transfer pair.
func (a *App) transferAccounts(fromID, toID string) (fromAcc, toAcc domain.Account, err error) {
	fromAcc, foundFrom := a.findAccount(fromID)
	if !foundFrom {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: source account %q not found", fromID)
	}
	toAcc, foundTo := a.findAccount(toID)
	if !foundTo {
		return domain.Account{}, domain.Account{}, fmt.Errorf("transfer: destination account %q not found", toID)
	}
	return fromAcc, toAcc, nil
}

// transferLegs books outgoing and incoming money using each account's fixed
// convention. Liability sources increase debt (cash advances); destinations
// reduce debt (payments). Cross-currency legs require saved rates or an explicit
// receivedMinor magnitude; same-currency legs always share a magnitude.
func (a *App) transferLegs(fromAcc, toAcc domain.Account, amountMinor, receivedMinor int64) (fromMoney, toMoney money.Money, err error) {
	return transferLegsWithRates(fromAcc, toAcc, amountMinor, receivedMinor, a.transactionPairRates())
}

func transferLegsWithRates(fromAcc, toAcc domain.Account, amountMinor, receivedMinor int64, rates currency.Rates) (fromMoney, toMoney money.Money, err error) {
	fromMoney = money.New(-fromAcc.InflowSign()*amountMinor, fromAcc.Currency)
	toMoney = money.New(amountMinor, toAcc.Currency)
	if fromAcc.Currency != toAcc.Currency {
		if receivedMinor > 0 {
			toMoney = money.New(receivedMinor, toAcc.Currency)
		} else {
			srcAbs := money.New(amountMinor, fromAcc.Currency)
			conv, cerr := rates.Convert(srcAbs, toAcc.Currency)
			if cerr != nil {
				return money.Money{}, money.Money{}, fmt.Errorf("%w: %s to %s", ErrTransferRateUnavailable, fromAcc.Currency, toAcc.Currency)
			}
			toMoney = conv
		}
	}
	if toAcc.Class == domain.ClassLiability {
		toMoney.Amount = liabilityPaymentMinor(toAcc, toMoney.Amount)
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
// call with the same params would produce, without writing anything. It builds the
// legs with the same function CreateTransferPair uses and applies them with the same
// signed fee, so the figures match what posts for the same ledger and rates.
func (a *App) PreviewTransferPair(p TransferParams) (TransferPreview, error) {
	if err := a.ready(); err != nil {
		return TransferPreview{}, err
	}
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
	if fromAcc.Archived || toAcc.Archived {
		return TransferPreview{}, fmt.Errorf("transfer preview: archived accounts cannot be used")
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
	return transferPreviewBalances(fromAcc, toAcc, fromBal, toBal, fromMoney, toMoney, p.FeeMinor)
}

// transferPreviewBalances applies a transfer's two legs and its fee to the booked balances of
// both accounts. The fee is the signed row the transfer posts on the source account, and every
// addition is checked for overflow.
func transferPreviewBalances(from, to domain.Account, fromBal, toBal, fromLeg, toLeg money.Money, feeMinor int64) (TransferPreview, error) {
	feeDelta := int64(0)
	if feeMinor > 0 {
		feeDelta = -from.InflowSign() * feeMinor
	}
	fromAfter, err := checkedAdd(fromBal.Amount, fromLeg.Amount)
	if err == nil {
		fromAfter, err = checkedAdd(fromAfter, feeDelta)
	}
	if err != nil {
		return TransferPreview{}, fmt.Errorf("transfer preview: source balance overflows")
	}
	toAfter, err := checkedAdd(toBal.Amount, toLeg.Amount)
	if err != nil {
		return TransferPreview{}, fmt.Errorf("transfer preview: destination balance overflows")
	}
	return TransferPreview{
		FromBefore: fromBal, FromAfter: money.New(fromAfter, from.Currency),
		ToBefore: toBal, ToAfter: money.New(toAfter, to.Currency),
	}, nil
}
