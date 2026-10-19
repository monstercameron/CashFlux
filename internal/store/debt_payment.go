// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

const debtPaymentSchema = `
CREATE TABLE IF NOT EXISTS debt_payment_links (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS debt_payment_link_txns (link_id TEXT NOT NULL, txn_id TEXT NOT NULL, PRIMARY KEY(link_id, txn_id));
CREATE TABLE IF NOT EXISTS debt_payee_rules (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TRIGGER IF NOT EXISTS debt_payment_link_cleanup AFTER DELETE ON debt_payment_links
BEGIN
  DELETE FROM debt_payment_link_txns WHERE link_id = OLD.id;
END;
`

func (s *SQLiteStore) ensureDebtPaymentSchema() error {
	if _, err := s.db.Exec(debtPaymentSchema); err != nil {
		return fmt.Errorf("store: create debt-payment schema: %w", err)
	}
	return nil
}

// PutDebtPaymentLink atomically persists a debt-payment allocation and its
// transaction membership rows.
func (s *SQLiteStore) PutDebtPaymentLink(link domain.DebtPaymentLink) error {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return err
	}
	if link.ID == "" {
		return fmt.Errorf("store: debt-payment link id is required")
	}
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("store: encode debt-payment link %q: %w", link.ID, err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin debt-payment link write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO debt_payment_links(id, data) VALUES(?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data`, link.ID, string(data)); err != nil {
		return fmt.Errorf("store: write debt-payment link %q: %w", link.ID, err)
	}
	if _, err := tx.Exec("DELETE FROM debt_payment_link_txns WHERE link_id = ?", link.ID); err != nil {
		return fmt.Errorf("store: replace debt-payment members %q: %w", link.ID, err)
	}
	for _, txnID := range link.TransactionIDs {
		if txnID == "" {
			return fmt.Errorf("store: debt-payment link %q has an empty transaction id", link.ID)
		}
		if _, err := tx.Exec("INSERT INTO debt_payment_link_txns(link_id, txn_id) VALUES(?, ?)", link.ID, txnID); err != nil {
			return fmt.Errorf("store: add transaction %q to debt-payment link %q: %w", txnID, link.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit debt-payment link %q: %w", link.ID, err)
	}
	mutationRev.Add(1)
	return nil
}

// ListDebtPaymentLinks returns all debt-payment links ordered by id.
func (s *SQLiteStore) ListDebtPaymentLinks() ([]domain.DebtPaymentLink, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return nil, err
	}
	return queryRows[domain.DebtPaymentLink](s.db, "SELECT data FROM debt_payment_links ORDER BY id")
}

// DeleteDebtPaymentLink removes one persisted debt-payment link.
func (s *SQLiteStore) DeleteDebtPaymentLink(id string) (bool, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return false, err
	}
	return deleteRow(s.db, "debt_payment_links", id)
}

// PutDebtPayeeRule atomically persists a scoped payee-to-debt rule.
func (s *SQLiteStore) PutDebtPayeeRule(rule domain.DebtPayeeRule) error {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return err
	}
	if rule.ID == "" {
		return fmt.Errorf("store: debt payee rule id is required")
	}
	return putJSON(s.db, "debt_payee_rules", rule.ID, rule)
}

// ListDebtPayeeRules returns persisted rules ordered by id.
func (s *SQLiteStore) ListDebtPayeeRules() ([]domain.DebtPayeeRule, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return nil, err
	}
	return queryRows[domain.DebtPayeeRule](s.db, "SELECT data FROM debt_payee_rules ORDER BY id")
}

// DeleteDebtPayeeRule removes one persisted payee rule.
func (s *SQLiteStore) DeleteDebtPayeeRule(id string) (bool, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return false, err
	}
	return deleteRow(s.db, "debt_payee_rules", id)
}

// DeleteTransactionWithDebtPayments removes a transaction and every debt-payment
// link that references it in one SQLite transaction.
func (s *SQLiteStore) DeleteTransactionWithDebtPayments(id string) (bool, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: begin transaction deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM debt_payment_links WHERE id IN (
SELECT link_id FROM debt_payment_link_txns WHERE txn_id = ?
)`, id); err != nil {
		return false, fmt.Errorf("store: delete transaction debt-payment links: %w", err)
	}
	res, err := tx.Exec("DELETE FROM transactions WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("store: delete transaction %q: %w", id, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: count deleted transaction %q: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit transaction deletion %q: %w", id, err)
	}
	if count > 0 {
		mutationRev.Add(1)
	}
	return count > 0, nil
}

// DeleteTransactionsWithDebtPaymentsAtomic deletes the selected transactions
// and every debt-payment link that references any of them in one transaction.
// It returns only transaction IDs that existed and were deleted.
func (s *SQLiteStore) DeleteTransactionsWithDebtPaymentsAtomic(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("store: begin debt-payment transaction deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	deleteLinks, err := tx.Prepare(`DELETE FROM debt_payment_links WHERE id IN (
SELECT link_id FROM debt_payment_link_txns WHERE txn_id = ?
)`)
	if err != nil {
		return nil, fmt.Errorf("store: prepare debt-payment link cleanup: %w", err)
	}
	defer deleteLinks.Close()
	deleteTransactions, err := tx.Prepare("DELETE FROM transactions WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("store: prepare transaction delete: %w", err)
	}
	defer deleteTransactions.Close()
	deleted := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := deleteLinks.Exec(id); err != nil {
			return nil, fmt.Errorf("store: delete transaction %q debt-payment links: %w", id, err)
		}
		result, err := deleteTransactions.Exec(id)
		if err != nil {
			return nil, fmt.Errorf("store: delete transaction %q: %w", id, err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("store: count deleted transaction %q: %w", id, err)
		}
		if count > 0 {
			deleted = append(deleted, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit debt-payment transaction deletion: %w", err)
	}
	if len(deleted) > 0 {
		mutationRev.Add(1)
	}
	return deleted, nil
}

// DeleteAccountWithDebtPayments removes an account, links targeting it, and
// rules that use it as their debt or source account in one SQLite transaction.
func (s *SQLiteStore) DeleteAccountWithDebtPayments(id string) (bool, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: begin account deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM debt_payment_links WHERE json_extract(data, '$.debtAccountId') = ?", id); err != nil {
		return false, fmt.Errorf("store: delete account debt-payment links: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM debt_payee_rules WHERE json_extract(data, '$.debtAccountId') = ? OR json_extract(data, '$.sourceAccountId') = ?", id, id); err != nil {
		return false, fmt.Errorf("store: delete account debt payee rules: %w", err)
	}
	res, err := tx.Exec("DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("store: delete account %q: %w", id, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: count deleted account %q: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit account deletion %q: %w", id, err)
	}
	if count > 0 {
		mutationRev.Add(1)
	}
	return count > 0, nil
}

// DebtPaymentSnapshot is the standalone export shape for the records owned by
// this lane. The full Dataset integration is kept in Dataset's owning file.
type DebtPaymentSnapshot struct {
	Links []domain.DebtPaymentLink `json:"debtPaymentLinks,omitempty"`
	Rules []domain.DebtPayeeRule   `json:"debtPayeeRules,omitempty"`
}

// SnapshotDebtPayments reads links and rules in one SQLite read transaction.
func (s *SQLiteStore) SnapshotDebtPayments() (DebtPaymentSnapshot, error) {
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return DebtPaymentSnapshot{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return DebtPaymentSnapshot{}, fmt.Errorf("store: begin debt-payment snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	links, err := queryRowsTx[domain.DebtPaymentLink](tx, "SELECT data FROM debt_payment_links ORDER BY id")
	if err != nil {
		return DebtPaymentSnapshot{}, fmt.Errorf("store: read debt-payment links: %w", err)
	}
	rules, err := queryRowsTx[domain.DebtPayeeRule](tx, "SELECT data FROM debt_payee_rules ORDER BY id")
	if err != nil {
		return DebtPaymentSnapshot{}, fmt.Errorf("store: read debt payee rules: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DebtPaymentSnapshot{}, fmt.Errorf("store: finish debt-payment snapshot: %w", err)
	}
	return DebtPaymentSnapshot{Links: links, Rules: rules}, nil
}

func queryRowsTx[T any](tx *sql.Tx, query string) ([]T, error) {
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
