// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

const regularSpendOverridesKey = "regularSpendOverrides.v1"

// GetRegularSpendOverrides returns the baseline corrections stored in this
// household dataset. An absent value is the zero-value set of corrections.
func (s *SQLiteStore) GetRegularSpendOverrides() (domain.RegularSpendOverrides, error) {
	var raw string
	err := s.db.QueryRow("SELECT v FROM appkv WHERE k = ?", regularSpendOverridesKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return domain.RegularSpendOverrides{}, nil
	}
	if err != nil {
		return domain.RegularSpendOverrides{}, fmt.Errorf("store: read regular-spend overrides: %w", err)
	}
	var overrides domain.RegularSpendOverrides
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		return domain.RegularSpendOverrides{}, fmt.Errorf("store: decode regular-spend overrides: %w", err)
	}
	return normalizeRegularSpendOverrides(overrides), nil
}

// PutRegularSpendOverrides durably replaces this household's baseline
// corrections in SQLite. It does not alter transaction or report classification.
func (s *SQLiteStore) PutRegularSpendOverrides(overrides domain.RegularSpendOverrides) error {
	overrides = normalizeRegularSpendOverrides(overrides)
	data, err := json.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("store: encode regular-spend overrides: %w", err)
	}
	if _, err := s.db.Exec("INSERT INTO appkv(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v", regularSpendOverridesKey, string(data)); err != nil {
		return fmt.Errorf("store: save regular-spend overrides: %w", err)
	}
	mutationRev.Add(1)
	return nil
}

// DeleteCategoryWithRegularSpendCleanup removes a category and its essential
// override in one SQLite transaction. Callers remain responsible for any
// category reparenting or reassignment required by the product flow.
func (s *SQLiteStore) DeleteCategoryWithRegularSpendCleanup(id string) (bool, error) {
	return s.deleteWithRegularSpendCleanup("categories", id, func(o *domain.RegularSpendOverrides) {
		o.EssentialCategoryIDs = removeString(o.EssentialCategoryIDs, id)
	})
}

// DeleteTransactionWithRegularSpendCleanup removes a transaction and its
// one-off override in one SQLite transaction.
func (s *SQLiteStore) DeleteTransactionWithRegularSpendCleanup(id string) (bool, error) {
	return s.deleteWithRegularSpendCleanup("transactions", id, func(o *domain.RegularSpendOverrides) {
		o.OneOffTransactionIDs = removeString(o.OneOffTransactionIDs, id)
	})
}

func (s *SQLiteStore) deleteWithRegularSpendCleanup(table, id string, clean func(*domain.RegularSpendOverrides)) (bool, error) {
	if strings.TrimSpace(id) == "" {
		return false, fmt.Errorf("store: delete %s: id is required", table)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: begin %s deletion: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	var raw string
	err = tx.QueryRow("SELECT v FROM appkv WHERE k = ?", regularSpendOverridesKey).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("store: read regular-spend overrides for %s deletion: %w", table, err)
	}
	overrides := domain.RegularSpendOverrides{}
	if err == nil {
		if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
			return false, fmt.Errorf("store: decode regular-spend overrides for %s deletion: %w", table, err)
		}
	}
	clean(&overrides)
	overrides = normalizeRegularSpendOverrides(overrides)
	data, err := json.Marshal(overrides)
	if err != nil {
		return false, fmt.Errorf("store: encode regular-spend overrides for %s deletion: %w", table, err)
	}
	if _, err := tx.Exec("INSERT INTO appkv(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v", regularSpendOverridesKey, string(data)); err != nil {
		return false, fmt.Errorf("store: save regular-spend overrides for %s deletion: %w", table, err)
	}
	result, err := tx.Exec("DELETE FROM "+table+" WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("store: delete %s: %w", table, err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: count deleted %s: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit %s deletion and override cleanup: %w", table, err)
	}
	mutationRev.Add(1)
	return deleted > 0, nil
}

func normalizeRegularSpendOverrides(overrides domain.RegularSpendOverrides) domain.RegularSpendOverrides {
	overrides.EssentialCategoryIDs = cleanIdentifiers(overrides.EssentialCategoryIDs)
	overrides.OneOffTransactionIDs = cleanIdentifiers(overrides.OneOffTransactionIDs)
	merchants := make([]string, 0, len(overrides.TreatMerchantRegular))
	for _, merchant := range overrides.TreatMerchantRegular {
		merchant = strings.ToLower(strings.Join(strings.Fields(merchant), " "))
		if merchant != "" {
			merchants = append(merchants, merchant)
		}
	}
	overrides.TreatMerchantRegular = uniqueSorted(merchants)
	if len(overrides.TreatMerchantRegular) == 0 {
		overrides.TreatMerchantRegular = nil
	}
	return overrides
}

func cleanIdentifiers(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return uniqueSorted(cleaned)
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func uniqueSorted(values []string) []string {
	sort.Strings(values)
	if len(values) < 2 {
		return values
	}
	unique := values[:1]
	for _, value := range values[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}
