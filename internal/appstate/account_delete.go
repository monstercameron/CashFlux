// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/monstercameron/CashFlux/internal/artifactref"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// DeleteAccount removes an account that has no transactions, together with everything that points at
// it, in one commit: the member and household default-account settings, debt-payment links and
// payee rules, sweep rules, goal links and allocations, account-group membership, holdings and
// realized sales, recurring entries, balance history, earmarks, imported-document links, the
// account's attached files and their renewal reminders, the account link on other tasks, bill-payment
// links on other accounts' transactions, rule conditions and bill actions that name it, and
// workflow transfers that use it. A missing account is a no-op. An account that
// still has a transaction, including the far leg of a transfer, returns ErrAccountHasTransactions.
// If any step fails nothing is removed.
func (a *App) DeleteAccount(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	account, found, err := a.store.GetAccount(id)
	if err != nil {
		return fmt.Errorf("appstate: read account %q: %w", id, err)
	}
	txns, err := a.store.ListTransactions()
	if err != nil {
		return fmt.Errorf("appstate: read ledger before deleting account %q: %w", id, err)
	}
	for _, t := range txns {
		if t.AccountID == id || t.TransferAccountID == id {
			return ErrAccountHasTransactions
		}
	}
	if found {
		changes, err := a.accountDeletionChanges(account, txns)
		if err != nil {
			return fmt.Errorf("appstate: plan deleting account %q: %w", id, err)
		}
		if err := a.store.ReplayChanges(changes); err != nil {
			return fmt.Errorf("appstate: delete account %q: %w", id, err)
		}
	}
	a.log.Info("entity deleted", "entity", "account", "id", id, "existed", found)
	return nil
}

// accountChanges collects record changes for accountDeletionChanges.
type accountChanges struct {
	list []store.RecordChange
	err  error
}

// remove queues the deletion of one record.
func (c *accountChanges) remove(table, id string, before any) {
	if c.err != nil {
		return
	}
	var data json.RawMessage
	if before != nil {
		data, c.err = json.Marshal(before)
	}
	c.list = append(c.list, store.RecordChange{Table: table, ID: id, Before: data})
}

// replace queues the rewrite of one record.
func (c *accountChanges) replace(table, id string, before, after any) {
	if c.err != nil {
		return
	}
	var b, af json.RawMessage
	if b, c.err = json.Marshal(before); c.err != nil {
		return
	}
	if af, c.err = json.Marshal(after); c.err != nil {
		return
	}
	c.list = append(c.list, store.RecordChange{Table: table, ID: id, Before: b, After: af})
}

// accountDeletionChanges plans every record write that removes account and the references to it.
// txns is the whole ledger, none of whose rows is on the account.
func (a *App) accountDeletionChanges(account domain.Account, txns []domain.Transaction) ([]store.RecordChange, error) {
	id := account.ID
	var c accountChanges

	members, err := a.store.ListMembers()
	if err != nil {
		return nil, fmt.Errorf("read members: %w", err)
	}
	for _, m := range members {
		if m.Prefs.DefaultAccountID == id {
			next := m
			next.Prefs.DefaultAccountID = ""
			c.replace("members", m.ID, m, next)
		}
	}
	settings, err := a.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if settings.DefaultAccountID == id {
		next := settings
		next.DefaultAccountID = ""
		c.replace("settings", "app", settings, next)
	}

	links, err := a.store.ListDebtPaymentLinks()
	if err != nil {
		return nil, fmt.Errorf("read debt-payment links: %w", err)
	}
	for _, l := range links {
		if l.DebtAccountID == id {
			c.remove("debt_payment_links", l.ID, l)
		}
	}
	payeeRules, err := a.store.ListDebtPayeeRules()
	if err != nil {
		return nil, fmt.Errorf("read debt payee rules: %w", err)
	}
	for _, r := range payeeRules {
		if r.DebtAccountID == id || r.SourceAccountID == id {
			c.remove("debt_payee_rules", r.ID, r)
		}
	}

	sweeps, err := a.store.ListSweepRules()
	if err != nil {
		return nil, fmt.Errorf("read sweep rules: %w", err)
	}
	for _, r := range sweeps {
		if r.SourceAccountID == id || r.DestAccountID == id {
			c.remove("sweeprules", r.ID, r)
		}
	}

	goalList, err := a.store.ListGoals()
	if err != nil {
		return nil, fmt.Errorf("read goals: %w", err)
	}
	for _, g := range goalList {
		if !goalReferencesAccount(g, id) {
			continue
		}
		next := g
		if next.AccountID == id {
			next.AccountID = ""
		}
		next.AccountIDs = slices.DeleteFunc(slices.Clone(g.AccountIDs), func(v string) bool { return v == id })
		next.Allocations = slices.DeleteFunc(slices.Clone(g.Allocations), func(al domain.GoalAllocation) bool { return al.AccountID == id })
		c.replace("goals", g.ID, g, next)
	}

	groups, err := a.store.ListAccountGroups()
	if err != nil {
		return nil, fmt.Errorf("read account groups: %w", err)
	}
	for _, g := range groups {
		if slices.Contains(g.AccountIDs, id) {
			next := g
			next.AccountIDs = slices.DeleteFunc(slices.Clone(g.AccountIDs), func(v string) bool { return v == id })
			c.replace("accountgroups", g.ID, g, next)
		}
	}

	holdings, err := a.store.ListHoldings()
	if err != nil {
		return nil, fmt.Errorf("read holdings: %w", err)
	}
	for _, h := range holdings {
		if h.AccountID == id {
			c.remove("holdings", h.ID, h)
		}
	}
	sales, err := a.store.ListRealizedSales()
	if err != nil {
		return nil, fmt.Errorf("read realized sales: %w", err)
	}
	for _, s := range sales {
		if s.AccountID == id {
			c.remove("realizedsales", s.ID, s)
		}
	}

	recurring, err := a.store.ListRecurring()
	if err != nil {
		return nil, fmt.Errorf("read recurring entries: %w", err)
	}
	for _, r := range recurring {
		if r.AccountID == id {
			c.remove("recurring", r.ID, r)
		}
	}

	snapshots, err := a.store.ListBalanceSnapshots(id)
	if err != nil {
		return nil, fmt.Errorf("read balance history: %w", err)
	}
	for _, s := range snapshots {
		c.remove("balance_snapshots", s.ID, s)
	}

	earmarks, err := a.store.ListEarmarks()
	if err != nil {
		return nil, fmt.Errorf("read earmarks: %w", err)
	}
	for _, e := range earmarks {
		if e.DestinationID == id {
			c.remove("earmarks", e.ID, e)
		}
	}

	documents, err := a.store.ListDocuments()
	if err != nil {
		return nil, fmt.Errorf("read documents: %w", err)
	}
	for _, d := range documents {
		if d.AccountID == id {
			next := d
			next.AccountID = ""
			c.replace("documents", d.ID, d, next)
		}
	}

	tasks, err := a.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("read tasks: %w", err)
	}
	for _, t := range tasks {
		if t.RelatedType != domain.RelatedAccount || t.RelatedID != id {
			continue
		}
		if key, _ := t.Custom[docExpiryTaskKey].(string); key != "" {
			c.remove("tasks", t.ID, t)
			continue
		}
		next := t
		next.RelatedType, next.RelatedID = domain.RelatedNone, ""
		c.replace("tasks", t.ID, t, next)
	}

	for _, row := range txns {
		if row.BillAccountID == id {
			next := row
			next.BillAccountID = ""
			c.replace("transactions", row.ID, row, next)
		}
	}

	ruleList, err := a.store.ListRules()
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	for _, r := range ruleList {
		next, keep, changed := ruleWithoutAccount(r, id)
		switch {
		case !keep:
			c.remove("rules", r.ID, r)
		case changed:
			c.replace("rules", r.ID, r, next)
		}
	}

	workflows, err := a.store.ListWorkflows()
	if err != nil {
		return nil, fmt.Errorf("read workflows: %w", err)
	}
	for _, w := range workflows {
		next, changed := workflowWithoutAccount(w, id)
		if changed {
			c.replace("workflows", w.ID, w, next)
		}
	}

	if len(account.DocRefs) > 0 {
		accounts, err := a.store.ListAccounts()
		if err != nil {
			return nil, fmt.Errorf("read accounts: %w", err)
		}
		pages, err := a.store.ListCustomPages()
		if err != nil {
			return nil, fmt.Errorf("read custom pages: %w", err)
		}
		remaining := slices.DeleteFunc(slices.Clone(accounts), func(other domain.Account) bool { return other.ID == id })
		// artifactref.Referenced is the one definition of "still in use": it also counts
		// transaction attachments, custom-page widget bindings and goal vision images.
		stillUsed := artifactref.Referenced(txns, pages, goalList, remaining)
		for _, other := range accounts {
			if other.ID == id {
				continue
			}
			for _, ref := range other.DocRefs {
				stillUsed[ref.ArtifactID] = true
			}
		}
		for _, ref := range account.DocRefs {
			if !stillUsed[ref.ArtifactID] {
				c.remove("artifacts", ref.ArtifactID, nil)
				stillUsed[ref.ArtifactID] = true
			}
		}
	}

	c.remove("accounts", id, account)
	if c.err != nil {
		return nil, c.err
	}
	return c.list, nil
}

// ruleWithoutAccount removes a deleted account from an auto-categorization rule. The bill-account
// action is cleared, and the rule is dropped (keep false) when that leaves it with nothing to do.
// A condition "account is X" can never match again, so a rule carrying one is dropped too; an
// "account is not X" condition is always true now and is removed while other conditions or a match
// phrase remain.
func ruleWithoutAccount(r rules.Rule, accountID string) (next rules.Rule, keep, changed bool) {
	next = r
	if r.SetBillAccountID == accountID {
		next.SetBillAccountID = ""
		changed = true
	}
	var conditions []rules.RuleCondition
	for _, cond := range r.Conditions {
		if cond.Field == rules.ConditionFieldAccount && strings.TrimSpace(cond.Value) == accountID {
			if cond.Op == rules.ConditionOpIs {
				return r, false, true
			}
			if cond.Op == rules.ConditionOpIsNot && (len(r.Conditions) > 1 || strings.TrimSpace(r.Match) != "") {
				changed = true
				continue
			}
		}
		conditions = append(conditions, cond)
	}
	if changed {
		next.Conditions = conditions
	}
	if changed && next.SetCategoryID == "" && next.SetBillAccountID == "" && strings.TrimSpace(next.RenameDesc) == "" &&
		len(next.SetTags) == 0 && !next.SetReviewed && !next.SetExcludeFromReports && next.SetMemberID == "" {
		return r, false, true
	}
	return next, true, changed
}

// workflowWithoutAccount drops the transfer actions that move money to or from a deleted account.
// A workflow left with no actions is turned off rather than deleted.
func workflowWithoutAccount(w workflow.Workflow, accountID string) (next workflow.Workflow, changed bool) {
	next = w
	next.Actions = nil
	for _, act := range w.Actions {
		if act.Kind == workflow.ActionTransfer && (act.TransferFromAccountID == accountID || act.TransferToAccountID == accountID) {
			changed = true
			continue
		}
		next.Actions = append(next.Actions, act)
	}
	if !changed {
		return w, false
	}
	if len(next.Actions) == 0 {
		next.Enabled = false
	}
	return next, true
}

// goalReferencesAccount reports whether the goal links to, is funded from, or earmarks the account.
func goalReferencesAccount(g domain.Goal, accountID string) bool {
	if g.AccountID == accountID || slices.Contains(g.AccountIDs, accountID) {
		return true
	}
	return slices.ContainsFunc(g.Allocations, func(al domain.GoalAllocation) bool { return al.AccountID == accountID })
}
