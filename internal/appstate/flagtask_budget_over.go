// SPDX-License-Identifier: MIT

package appstate

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

const (
	budgetOverTaskNotesKey = "tasks.budgetOverNotes"
	taskTitleKeyField      = "titleKey"
	taskTitleArgsField     = "titleArgs"
	taskNotesKeyField      = "notesKey"
	taskNotesArgsField     = "notesArgs"
)

// newBudgetOverTask returns a localization-ready budget flag task. Key strings
// remain stable in persisted data; presentation resolves them for the active
// language.
func newBudgetOverTask(b domain.Budget, status budgeting.Status) domain.Task {
	titleKey, titleArgs := BudgetOverTaskTitle(b)
	notesArgs := []any{b.Name, status.Percent}
	return domain.Task{
		ID:          id.New(),
		RelatedType: domain.RelatedType("budget"),
		RelatedID:   b.ID,
		Title:       titleKey,
		Notes:       budgetOverTaskNotesKey,
		Status:      domain.StatusOpen,
		Priority:    domain.PriorityMedium,
		Source:      domain.SourceManual,
		Custom: map[string]any{
			taskTitleKeyField:  titleKey,
			taskTitleArgsField: titleArgs,
			taskNotesKeyField:  budgetOverTaskNotesKey,
			taskNotesArgsField: notesArgs,
		},
	}
}

// duplicateBudgetOverTask reports whether this budget already has an open flag.
// Related identity is the stable dedupe key; title copy can change by language.
func duplicateBudgetOverTask(tasks []domain.Task, budgetID string) bool {
	for _, task := range tasks {
		if task.Status == domain.StatusOpen && task.RelatedID == budgetID && task.RelatedType == domain.RelatedType("budget") {
			return true
		}
	}
	return false
}
