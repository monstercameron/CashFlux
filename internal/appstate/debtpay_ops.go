// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

// DebtPaymentLinks returns persisted debt-payment links, logging read failures.
func (a *App) DebtPaymentLinks() []domain.DebtPaymentLink {
	links, err := a.store.ListDebtPaymentLinks()
	a.logErr("debt-payment links", err)
	return links
}

// PutDebtPaymentLink validates and durably stores one confirmed debt-payment
// allocation. The returned success follows the SQLite transaction commit.
func (a *App) PutDebtPaymentLink(link domain.DebtPaymentLink) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if strings.TrimSpace(link.ID) == "" {
		link.ID = id.New()
	}
	if strings.TrimSpace(link.DebtAccountID) == "" {
		return fmt.Errorf("appstate: debt-payment target account is required")
	}
	target, ok, err := a.store.GetAccount(link.DebtAccountID)
	if err != nil {
		return fmt.Errorf("appstate: read debt-payment target account: %w", err)
	}
	if !ok {
		return fmt.Errorf("appstate: debt-payment target account %q does not exist", link.DebtAccountID)
	}
	if !target.IsLiability() {
		return fmt.Errorf("appstate: debt-payment target account %q is not a liability", link.DebtAccountID)
	}
	if len(link.TransactionIDs) == 0 {
		return fmt.Errorf("appstate: debt-payment link needs at least one transaction")
	}
	seen := make(map[string]struct{}, len(link.TransactionIDs))
	for _, txnID := range link.TransactionIDs {
		if strings.TrimSpace(txnID) == "" {
			return fmt.Errorf("appstate: debt-payment link has an empty transaction id")
		}
		if _, exists := seen[txnID]; exists {
			return fmt.Errorf("appstate: transaction %q appears more than once in debt-payment link", txnID)
		}
		seen[txnID] = struct{}{}
		if _, ok, err := a.store.GetTransaction(txnID); err != nil {
			return fmt.Errorf("appstate: read debt-payment transaction %q: %w", txnID, err)
		} else if !ok {
			return fmt.Errorf("appstate: debt-payment transaction %q does not exist", txnID)
		}
		for _, existing := range a.DebtPaymentLinks() {
			if existing.ID == link.ID {
				continue
			}
			for _, existingTxnID := range existing.TransactionIDs {
				if existingTxnID == txnID {
					return fmt.Errorf("appstate: transaction %q is already linked to debt payment %q", txnID, existing.ID)
				}
			}
		}
	}
	if link.PrincipalMinor < 0 || link.InterestMinor < 0 || link.FeesMinor < 0 || link.EscrowMinor < 0 {
		return fmt.Errorf("appstate: debt-payment allocations must be non-negative")
	}
	if link.Source != domain.DebtPaymentConfirmed && link.Source != domain.DebtPaymentRule {
		return fmt.Errorf("appstate: unknown debt-payment source %q", link.Source)
	}
	switch link.Method {
	case domain.DebtPaymentStatement, domain.DebtPaymentCard, domain.DebtPaymentAmortized:
	default:
		return fmt.Errorf("appstate: unknown debt-payment method %q", link.Method)
	}
	if link.Method == domain.DebtPaymentAmortized && !link.Estimated {
		return fmt.Errorf("appstate: amortization allocation must be marked as an estimate")
	}
	if link.Estimated && link.Method != domain.DebtPaymentAmortized {
		return fmt.Errorf("appstate: estimated allocation must use the amortization method")
	}
	if link.CreatedRevision == 0 {
		link.CreatedRevision = a.store.Rev()
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = a.clock()
	}
	if err := a.store.PutDebtPaymentLink(link); err != nil {
		return fmt.Errorf("appstate: save debt-payment link: %w", err)
	}
	return nil
}

// DeleteDebtPaymentLink removes a debt-payment relationship without changing
// its source transactions.
func (a *App) DeleteDebtPaymentLink(linkID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if strings.TrimSpace(linkID) == "" {
		return fmt.Errorf("appstate: debt-payment link id is required")
	}
	return a.del("debt-payment link", linkID, a.store.DeleteDebtPaymentLink)
}

// DebtPayeeRules returns persisted workspace-scoped debt payee rules.
func (a *App) DebtPayeeRules() []domain.DebtPayeeRule {
	rules, err := a.store.ListDebtPayeeRules()
	a.logErr("debt payee rules", err)
	return rules
}

// PutDebtPayeeRule stores a payee rule created from an explicit confirmation.
func (a *App) PutDebtPayeeRule(rule domain.DebtPayeeRule) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if strings.TrimSpace(rule.ID) == "" {
		rule.ID = id.New()
	}
	if strings.TrimSpace(rule.WorkspaceID) == "" {
		return fmt.Errorf("appstate: debt payee rule workspace is required")
	}
	if strings.TrimSpace(rule.Payee) == "" {
		return fmt.Errorf("appstate: debt payee rule payee is required")
	}
	if strings.TrimSpace(rule.DebtAccountID) == "" {
		return fmt.Errorf("appstate: debt payee rule target account is required")
	}
	target, ok, err := a.store.GetAccount(rule.DebtAccountID)
	if err != nil {
		return fmt.Errorf("appstate: read debt payee target account: %w", err)
	}
	if !ok {
		return fmt.Errorf("appstate: debt payee target account %q does not exist", rule.DebtAccountID)
	}
	if !target.IsLiability() {
		return fmt.Errorf("appstate: debt payee target account %q is not a liability", rule.DebtAccountID)
	}
	if rule.SourceAccountID != "" {
		if _, ok, err := a.store.GetAccount(rule.SourceAccountID); err != nil {
			return fmt.Errorf("appstate: read debt payee source account: %w", err)
		} else if !ok {
			return fmt.Errorf("appstate: debt payee source account %q does not exist", rule.SourceAccountID)
		}
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = a.clock()
	}
	if err := a.store.PutDebtPayeeRule(rule); err != nil {
		return fmt.Errorf("appstate: save debt payee rule: %w", err)
	}
	return nil
}

// DeleteDebtPayeeRule deletes a persisted debt payee rule.
func (a *App) DeleteDebtPayeeRule(ruleID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if strings.TrimSpace(ruleID) == "" {
		return fmt.Errorf("appstate: debt payee rule id is required")
	}
	return a.del("debt payee rule", ruleID, a.store.DeleteDebtPayeeRule)
}
