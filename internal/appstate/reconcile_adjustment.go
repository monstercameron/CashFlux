// SPDX-License-Identifier: MIT

package appstate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// PostReconcileAdjustment posts the manual, cleared and reviewed transaction that closes a
// reconciliation difference. diffMinor is the difference in the stated convention (positive is money
// you have). The row is written in the account's own sign, taken from its fixed liability sign, and
// moves the cleared balance toward the statement; conv is ignored and kept only so callers need not
// change. desc is the row's description. It returns the posted transaction. Every call posts a new row; use PostReconcileAdjustmentOnce when
// the same action may be retried.
func (a *App) PostReconcileAdjustment(accountID string, diffMinor int64, conv reconcile.Convention, desc string) (domain.Transaction, error) {
	return a.PostReconcileAdjustmentOnce(accountID, id.New(), diffMinor, conv, desc)
}

// PostReconcileAdjustmentOnce is PostReconcileAdjustment keyed by one user action. Retrying with
// the same actionKey and account returns the row the first call posted instead of posting a second
// one, and retrying with a different difference or description is refused. A zero difference and an
// archived account are refused, and the row is dated like any other transaction, at the start of
// the day. conv is ignored; the account's own liability sign decides the stored sign.
func (a *App) PostReconcileAdjustmentOnce(accountID, actionKey string, diffMinor int64, conv reconcile.Convention, desc string) (domain.Transaction, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Transaction{}, err
	}
	if strings.TrimSpace(actionKey) == "" {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: an action key is required")
	}
	if diffMinor == 0 {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: there is no difference to adjust")
	}
	account, ok, err := a.store.GetAccount(accountID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: read account: %w", err)
	}
	if !ok {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: account %q not found", accountID)
	}
	if account.Archived {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: archived accounts cannot be adjusted")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s|%d:%s", len(accountID), accountID, len(actionKey), actionKey)))
	rowID := "reconcile-adjust-" + hex.EncodeToString(sum[:16])
	stored, err := storedBalance(diffMinor, accountConvention(account))
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: %w", err)
	}
	if existing, found, err := a.store.GetTransaction(rowID); err != nil {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: check earlier attempt: %w", err)
	} else if found {
		if existing.Amount.Amount != stored || existing.Desc != desc {
			return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: this action key already posted a different adjustment")
		}
		return existing, nil
	}
	t := domain.Transaction{
		ID: rowID, AccountID: accountID, Date: normalizeTransactionDate(a.clock()), Desc: desc,
		Amount:  money.New(stored, account.Currency),
		Cleared: true, Reviewed: true, Source: domain.TxnSourceManual,
	}
	if err := a.PutTransaction(t); err != nil {
		return domain.Transaction{}, fmt.Errorf("post reconcile adjustment: %w", err)
	}
	return t, nil
}
