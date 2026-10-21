// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// AssistantTransactionAmount translates public signed cash flow (negative expense,
// positive income/payment) into the account's fixed booking convention. It never
// uses the current balance to infer a liability's sign.
func AssistantTransactionAmount(account domain.Account, flow money.Money) (money.Money, error) {
	if flow.Currency != account.Currency {
		return money.Money{}, fmt.Errorf("transaction flow currency does not match account")
	}
	if account.InflowSign() < 0 {
		booked, err := flow.NegChecked()
		if err != nil {
			return money.Money{}, fmt.Errorf("transaction booking sign: %w", err)
		}
		return booked, nil
	}
	return flow, nil
}

func creationRecord(table, id string, before, after any) (store.RecordChange, error) {
	c := store.RecordChange{Table: table, ID: id}
	var err error
	if before != nil {
		c.Before, err = json.Marshal(before)
		if err != nil {
			return c, fmt.Errorf("encode expected %s/%s: %w", table, id, err)
		}
	}
	if after != nil {
		c.After, err = json.Marshal(after)
		if err != nil {
			return c, fmt.Errorf("encode intended %s/%s: %w", table, id, err)
		}
	}
	return c, nil
}

// CreateTransactionExpected inserts a plain transaction while checking the
// previewed account/category in the same atomic replay. Canonical duplicates are
// returned without mutation. Addition effects run only after a successful insert.
func (a *App) CreateTransactionExpected(draft domain.Transaction, account domain.Account, category *domain.Category) (domain.Transaction, bool, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Transaction{}, false, err
	}
	if draft.ID == "" || account.ID == "" || draft.AccountID != account.ID || account.Archived || draft.IsTransfer() || draft.TransferGroupID != "" || draft.Amount.Amount == 0 {
		return domain.Transaction{}, false, fmt.Errorf("create transaction: invalid account, transfer, ID or zero amount")
	}
	draft.Date = normalizeTransactionDate(draft.Date)
	if issues := validate.ValidateTransaction(draft, account); !issues.OK() {
		return domain.Transaction{}, false, issues
	}
	if err := a.validateCustom("transaction", draft.Custom); err != nil {
		return domain.Transaction{}, false, err
	}
	liveAccount, found, err := a.store.GetAccount(account.ID)
	if err != nil {
		return domain.Transaction{}, false, err
	}
	if !found || !reflect.DeepEqual(liveAccount, account) {
		return domain.Transaction{}, false, fmt.Errorf("create transaction: account changed since preview")
	}
	guard, err := creationRecord("accounts", account.ID, account, account)
	if err != nil {
		return domain.Transaction{}, false, err
	}
	changes := []store.RecordChange{guard}
	if category != nil {
		if draft.CategoryID != category.ID || category.ID == "" {
			return domain.Transaction{}, false, fmt.Errorf("create transaction: category does not match preview")
		}
		live, found, err := a.store.GetCategory(category.ID)
		if err != nil {
			return domain.Transaction{}, false, err
		}
		if !found || !reflect.DeepEqual(live, *category) {
			return domain.Transaction{}, false, fmt.Errorf("create transaction: category changed since preview")
		}
		if issues := validate.ValidateCategory(*category); !issues.OK() {
			return domain.Transaction{}, false, issues
		}
		guard, err = creationRecord("categories", category.ID, *category, *category)
		if err != nil {
			return domain.Transaction{}, false, err
		}
		changes = append(changes, guard)
	} else if draft.CategoryID != "" {
		return domain.Transaction{}, false, fmt.Errorf("create transaction: missing expected category")
	}
	rows, err := a.store.ListTransactions()
	if err != nil {
		return domain.Transaction{}, false, err
	}
	for _, row := range rows {
		if !row.IsTransfer() && dedupe.Key(row) == dedupe.Key(draft) {
			return row, false, nil
		}
	}
	if _, err := ledger.Balance(account, append(accountTransactions(rows, account.ID), draft)); err != nil {
		return domain.Transaction{}, false, fmt.Errorf("create transaction: resulting balance unavailable: %w", err)
	}
	write, err := creationRecord("transactions", draft.ID, nil, draft)
	if err != nil {
		return domain.Transaction{}, false, err
	}
	changes = append(changes, write)
	var overBefore map[string]bool
	if !a.triggersSuspended {
		overBefore = a.overBudgetIDs()
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return domain.Transaction{}, false, fmt.Errorf("create transaction: %w", err)
	}
	a.afterTransactionSaved(draft, false, overBefore)
	return draft, true, nil
}

// CategorizeTransactionsExpected changes only CategoryID on the exact reviewed
// rows. All before-images and the category are checked together; any failure
// preserves every row, including transfer markers, splits, owners and amounts.
func (a *App) CategorizeTransactionsExpected(expected []domain.Transaction, category domain.Category) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if category.ID == "" {
		return fmt.Errorf("categorize transactions: category ID is required")
	}
	if issues := validate.ValidateCategory(category); !issues.OK() {
		return issues
	}
	if len(expected) == 0 {
		return nil
	}
	if _, err := uniqueExpectedTransactions(expected); err != nil {
		return err
	}
	guard, err := creationRecord("categories", category.ID, category, category)
	if err != nil {
		return err
	}
	changes := []store.RecordChange{guard}
	accounts := a.Accounts()
	for _, before := range expected {
		if before.IsTransfer() || before.TransferGroupID != "" {
			return fmt.Errorf("categorize transactions: transfers must be excluded")
		}
		after := before
		after.CategoryID = category.ID
		if issues := validate.ValidateTransaction(after, accounts...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", after.Custom); err != nil {
			return err
		}
		write, err := creationRecord("transactions", after.ID, before, after)
		if err != nil {
			return err
		}
		changes = append(changes, write)
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("categorize transactions: %w", err)
	}
	a.fireTxnMutated()
	return nil
}

// CreateCategoryExpected creates one top-level category without overwriting an
// existing ID. A canonical sibling-name collision is an explicit no-op.
func (a *App) CreateCategoryExpected(draft domain.Category) (domain.Category, bool, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Category{}, false, err
	}
	draft.Name = strings.TrimSpace(draft.Name)
	if draft.ID == "" || draft.ParentID != "" {
		return domain.Category{}, false, fmt.Errorf("create category: a new top-level ID is required")
	}
	if issues := validate.ValidateCategory(draft); !issues.OK() {
		return domain.Category{}, false, issues
	}
	all, err := a.store.ListCategories()
	if err != nil {
		return domain.Category{}, false, err
	}
	lookup := draft
	lookup.ID = ""
	if existing, collision := catname.Collision(all, lookup); collision {
		return existing, false, nil
	}
	if issues := validate.ValidateCategoryInTree(draft, all); !issues.OK() {
		return domain.Category{}, false, issues
	}
	write, err := creationRecord("categories", draft.ID, nil, draft)
	if err != nil {
		return domain.Category{}, false, err
	}
	if err := a.store.ReplayChangesExpected([]store.RecordChange{write}); err != nil {
		return domain.Category{}, false, fmt.Errorf("create category: %w", err)
	}
	a.log.Info("category saved", "id", draft.ID)
	return draft, true, nil
}
