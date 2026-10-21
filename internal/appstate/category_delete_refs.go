// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/catmerge"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// CategoryDeleteReferences returns the live, non-child references that would
// need reassignment before id can be deleted. It uses checked store reads so a
// failed read cannot make a referenced category appear safe to delete.
func (a *App) CategoryDeleteReferences(id string) (catmerge.Counts, error) {
	if a == nil || a.store == nil {
		return catmerge.Counts{}, fmt.Errorf("read category references: local dataset is unavailable")
	}
	transactions, err := a.TransactionsChecked()
	if err != nil {
		return catmerge.Counts{}, err
	}
	budgets, err := a.store.ListBudgets()
	if err != nil {
		return catmerge.Counts{}, fmt.Errorf("read category references: budgets: %w", err)
	}
	goals, err := a.GoalsChecked()
	if err != nil {
		return catmerge.Counts{}, err
	}
	rules, err := a.store.ListRules()
	if err != nil {
		return catmerge.Counts{}, fmt.Errorf("read category references: rules: %w", err)
	}
	recurring, err := a.store.ListRecurring()
	if err != nil {
		return catmerge.Counts{}, fmt.Errorf("read category references: recurring: %w", err)
	}
	pages, err := a.store.ListCustomPages()
	if err != nil {
		return catmerge.Counts{}, fmt.Errorf("read category references: custom pages: %w", err)
	}
	var widgets []domain.WidgetSpec
	for _, page := range pages {
		for _, widget := range page.Widgets {
			if widget.Spec != nil {
				widgets = append(widgets, *widget.Spec)
			}
		}
	}
	refs := catmerge.Refs{
		Transactions: transactions,
		Budgets:      budgets,
		Goals:        goals,
		Rules:        rules,
		Recurring:    recurring,
		Tally:        loadLearnTally(),
		Widgets:      widgets,
	}
	return catmerge.Plan(refs, id, ""), nil
}
