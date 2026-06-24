// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/settle"
)

// SharedExpenses returns every recorded shared expense — the roommate settle-up
// ledger.
func (a *App) SharedExpenses() []domain.SharedExpense {
	v, err := a.store.ListSharedExpenses()
	a.logErr("sharedExpenses", err)
	return v
}

// Settlements returns every recorded settlement payment.
func (a *App) Settlements() []domain.Settlement {
	v, err := a.store.ListSettlements()
	a.logErr("settlements", err)
	return v
}

// PutSharedExpense validates and persists a shared expense (the saved forward
// split): it needs an id, a payer, and at least one share.
func (a *App) PutSharedExpense(e domain.SharedExpense) error {
	if e.ID == "" {
		return fmt.Errorf("appstate: shared expense id is required")
	}
	if e.PayerID == "" {
		return fmt.Errorf("appstate: shared expense needs a payer")
	}
	if len(e.Shares) == 0 {
		return fmt.Errorf("appstate: shared expense needs at least one share")
	}
	if err := a.store.PutSharedExpense(e); err != nil {
		return err
	}
	a.log.Info("shared expense saved", "id", e.ID, "payer", e.PayerID, "total", e.Total().String())
	return nil
}

// DeleteSharedExpense removes a shared expense from the settle-up ledger.
func (a *App) DeleteSharedExpense(id string) error {
	return a.del("sharedExpense", id, a.store.DeleteSharedExpense)
}

// RecordSettlement validates and persists a payment from one member to another
// that squares up the shared ledger.
func (a *App) RecordSettlement(s domain.Settlement) error {
	if s.ID == "" {
		return fmt.Errorf("appstate: settlement id is required")
	}
	if s.FromID == "" || s.ToID == "" {
		return fmt.Errorf("appstate: settlement needs a payer and a recipient")
	}
	if s.FromID == s.ToID {
		return fmt.Errorf("appstate: a settlement must be between two different members")
	}
	if !s.Amount.IsPositive() {
		return fmt.Errorf("appstate: settlement amount must be positive")
	}
	if err := a.store.PutSettlement(s); err != nil {
		return err
	}
	a.log.Info("settlement recorded", "id", s.ID, "from", s.FromID, "to", s.ToID, "amount", s.Amount.String())
	return nil
}

// DeleteSettlement removes a recorded settlement.
func (a *App) DeleteSettlement(id string) error {
	return a.del("settlement", id, a.store.DeleteSettlement)
}

// SettleUp computes, from the recorded shared expenses and settlements, each
// member's net balance (positive = the group owes them) and the minimal set of
// "X pays Y" transfers that zeroes everyone out. currency labels the amounts.
func (a *App) SettleUp(currency string) (map[string]money.Money, []settle.Transfer) {
	expenses := a.SharedExpenses()
	settlements := a.Settlements()

	se := make([]settle.Expense, 0, len(expenses))
	for _, e := range expenses {
		shares := make(map[string]money.Money, len(e.Shares))
		for _, sh := range e.Shares {
			shares[sh.MemberID] = sh.Amount
		}
		se = append(se, settle.Expense{Payer: e.PayerID, Shares: shares})
	}
	ss := make([]settle.Settlement, 0, len(settlements))
	for _, s := range settlements {
		ss = append(ss, settle.Settlement{From: s.FromID, To: s.ToID, Amount: s.Amount})
	}

	net := settle.Net(se, ss, currency)
	return net, settle.Minimize(net)
}
