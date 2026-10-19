// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// PutCategoriesAtomic validates and saves existing categories as one undoable
// store transition. If any category is invalid or has changed since it was
// read, no category in the batch is written.
func (a *App) PutCategoriesAtomic(writes []domain.Category) error {
	if len(writes) == 0 {
		return nil
	}
	if err := a.roleGuard(); err != nil {
		return err
	}

	categories := a.Categories()
	current := make(map[string]domain.Category, len(writes))
	for _, category := range categories {
		current[category.ID] = category
	}
	seen := make(map[string]bool, len(writes))
	changes := make([]store.RecordChange, 0, len(writes))
	for _, category := range writes {
		if category.ID == "" || seen[category.ID] {
			return fmt.Errorf("appstate: category batch has an empty or duplicate id %q", category.ID)
		}
		seen[category.ID] = true
		beforeCategory, ok := current[category.ID]
		if !ok {
			return fmt.Errorf("appstate: category %q does not exist", category.ID)
		}
		if issues := validate.ValidateCategory(category); !issues.OK() {
			return fmt.Errorf("appstate: validate category %q: %w", category.ID, issues)
		}
		if !category.CategoryClass.Valid() {
			return fmt.Errorf("appstate: category %q has an invalid classification %q", category.ID, category.CategoryClass)
		}
		if issues := validate.ValidateCategoryInTree(category, categories); !issues.OK() {
			return fmt.Errorf("appstate: validate category %q in tree: %w", category.ID, issues)
		}
		before, err := json.Marshal(beforeCategory)
		if err != nil {
			return fmt.Errorf("appstate: encode category %q before image: %w", category.ID, err)
		}
		after, err := json.Marshal(category)
		if err != nil {
			return fmt.Errorf("appstate: encode category %q update: %w", category.ID, err)
		}
		changes = append(changes, store.RecordChange{Table: "categories", ID: category.ID, Before: before, After: after})
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("appstate: commit category batch: %w", err)
	}
	for _, category := range writes {
		a.log.Info("category saved", "id", category.ID)
	}
	return nil
}
