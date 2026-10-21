// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// ClockNow exposes the app's injected clock for previewed assistant dates.
func (a *App) ClockNow() time.Time { return a.clock() }

// CreateTaskExpected creates a task only while the task collection still matches
// the previewed collection. The task is inserted through the store's atomic
// expected-change path, so a reused ID cannot overwrite another task.
func (a *App) CreateTaskExpected(draft domain.Task, expectedTasks []domain.Task) (domain.Task, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Task{}, err
	}
	if draft.ID == "" {
		return domain.Task{}, fmt.Errorf("create task: a new ID is required")
	}
	if issues := validate.ValidateTask(draft); !issues.OK() {
		return domain.Task{}, issues
	}
	live, err := a.store.ListTasks()
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: read tasks: %w", err)
	}
	if !reflect.DeepEqual(live, expectedTasks) {
		return domain.Task{}, fmt.Errorf("create task: tasks changed since preview")
	}
	write, err := creationRecord("tasks", draft.ID, nil, draft)
	if err != nil {
		return domain.Task{}, err
	}
	if err := a.store.ReplayChangesExpected([]store.RecordChange{write}); err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	a.log.Info("task saved", "id", draft.ID)
	return draft, nil
}

// CreateAccountExpected creates an account only while the account collection
// and saved base currency still match the approval preview. It validates the
// same canonical account rules as PutAccount and inserts without replacing an
// existing account ID.
func (a *App) CreateAccountExpected(draft domain.Account, expectedAccounts []domain.Account, expectedBaseCurrency string) (domain.Account, error) {
	if err := a.roleGuard(); err != nil {
		return domain.Account{}, err
	}
	if draft.ID == "" {
		return domain.Account{}, fmt.Errorf("create account: a new ID is required")
	}
	draft = draft.WithLiabilitySign()
	if issues := validate.ValidateAccount(draft); !issues.OK() {
		return domain.Account{}, issues
	}
	if err := a.validateCustom("account", draft.Custom); err != nil {
		return domain.Account{}, err
	}
	live, err := a.store.ListAccounts()
	if err != nil {
		return domain.Account{}, fmt.Errorf("create account: read accounts: %w", err)
	}
	if !reflect.DeepEqual(live, expectedAccounts) {
		return domain.Account{}, fmt.Errorf("create account: accounts changed since preview")
	}
	settings, err := a.store.GetSettings()
	if err != nil {
		return domain.Account{}, fmt.Errorf("create account: read settings: %w", err)
	}
	if settings.BaseCurrency != expectedBaseCurrency {
		return domain.Account{}, fmt.Errorf("create account: base currency changed since preview")
	}
	for _, existing := range live {
		if !existing.Archived && strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(draft.Name)) {
			return domain.Account{}, fmt.Errorf("create account: an account named %q already exists", existing.Name)
		}
	}
	write, err := creationRecord("accounts", draft.ID, nil, draft)
	if err != nil {
		return domain.Account{}, err
	}
	if err := a.store.ReplayChangesExpected([]store.RecordChange{write}); err != nil {
		return domain.Account{}, fmt.Errorf("create account: %w", err)
	}
	if draft.OpeningBalance.Amount != 0 {
		asOf := draft.BalanceAsOf
		if asOf.IsZero() {
			asOf = a.clock()
		}
		snapshot := domain.BalanceSnapshot{
			ID: id.New(), AccountID: draft.ID, BalanceMinor: draft.OpeningBalance.Amount,
			Currency: draft.Currency, AsOf: asOf,
		}
		if err := a.store.PutBalanceSnapshot(snapshot); err != nil {
			a.log.Error("balance snapshot write failed", "accountId", draft.ID, "err", err)
		}
	}
	a.log.Info("account saved", "id", draft.ID)
	return draft, nil
}
