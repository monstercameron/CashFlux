package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// --- generic helpers ---

func putJSON[T any](db *sql.DB, table, id string, item T) error {
	if id == "" {
		return fmt.Errorf("store: %s: id is required", table)
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO "+table+"(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data",
		id, string(data),
	)
	if err != nil {
		return fmt.Errorf("store: put %s: %w", table, err)
	}
	return nil
}

func getJSON[T any](db *sql.DB, table, id string) (T, bool, error) {
	var zero T
	var data string
	err := db.QueryRow("SELECT data FROM "+table+" WHERE id = ?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	var item T
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return zero, false, err
	}
	return item, true, nil
}

func deleteRow(db *sql.DB, table, id string) (bool, error) {
	res, err := db.Exec("DELETE FROM "+table+" WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func queryRows[T any](db *sql.DB, query string, args ...any) ([]T, error) {
	rows, err := db.Query(query, args...)
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

// --- Members ---

func (s *SQLiteStore) PutMember(m domain.Member) error { return putJSON(s.db, "members", m.ID, m) }
func (s *SQLiteStore) GetMember(id string) (domain.Member, bool, error) {
	return getJSON[domain.Member](s.db, "members", id)
}
func (s *SQLiteStore) DeleteMember(id string) (bool, error) { return deleteRow(s.db, "members", id) }
func (s *SQLiteStore) ListMembers() ([]domain.Member, error) {
	return loadRows[domain.Member](s.db, "members")
}

// --- Accounts ---

func (s *SQLiteStore) PutAccount(a domain.Account) error { return putJSON(s.db, "accounts", a.ID, a) }
func (s *SQLiteStore) GetAccount(id string) (domain.Account, bool, error) {
	return getJSON[domain.Account](s.db, "accounts", id)
}
func (s *SQLiteStore) DeleteAccount(id string) (bool, error) { return deleteRow(s.db, "accounts", id) }
func (s *SQLiteStore) ListAccounts() ([]domain.Account, error) {
	return loadRows[domain.Account](s.db, "accounts")
}

// --- Categories ---

func (s *SQLiteStore) PutCategory(c domain.Category) error {
	return putJSON(s.db, "categories", c.ID, c)
}
func (s *SQLiteStore) GetCategory(id string) (domain.Category, bool, error) {
	return getJSON[domain.Category](s.db, "categories", id)
}
func (s *SQLiteStore) DeleteCategory(id string) (bool, error) {
	return deleteRow(s.db, "categories", id)
}
func (s *SQLiteStore) ListCategories() ([]domain.Category, error) {
	return loadRows[domain.Category](s.db, "categories")
}

// --- Transactions ---

func (s *SQLiteStore) PutTransaction(t domain.Transaction) error {
	return putJSON(s.db, "transactions", t.ID, t)
}
func (s *SQLiteStore) GetTransaction(id string) (domain.Transaction, bool, error) {
	return getJSON[domain.Transaction](s.db, "transactions", id)
}
func (s *SQLiteStore) DeleteTransaction(id string) (bool, error) {
	return deleteRow(s.db, "transactions", id)
}
func (s *SQLiteStore) ListTransactions() ([]domain.Transaction, error) {
	return loadRows[domain.Transaction](s.db, "transactions")
}

// TransactionsByAccount returns transactions for a single account.
func (s *SQLiteStore) TransactionsByAccount(accountID string) ([]domain.Transaction, error) {
	return queryRows[domain.Transaction](s.db,
		"SELECT data FROM transactions WHERE json_extract(data, '$.accountId') = ? ORDER BY id", accountID)
}

// TransactionsByCategory returns transactions in a single category.
func (s *SQLiteStore) TransactionsByCategory(categoryID string) ([]domain.Transaction, error) {
	return queryRows[domain.Transaction](s.db,
		"SELECT data FROM transactions WHERE json_extract(data, '$.categoryId') = ? ORDER BY id", categoryID)
}

// TransactionsByMember returns transactions attributed to a member.
func (s *SQLiteStore) TransactionsByMember(memberID string) ([]domain.Transaction, error) {
	return queryRows[domain.Transaction](s.db,
		"SELECT data FROM transactions WHERE json_extract(data, '$.memberId') = ? ORDER BY id", memberID)
}

// TransactionsByDateRange returns transactions whose date falls in [start, end).
func (s *SQLiteStore) TransactionsByDateRange(start, end time.Time) ([]domain.Transaction, error) {
	all, err := s.ListTransactions()
	if err != nil {
		return nil, err
	}
	var out []domain.Transaction
	for _, t := range all {
		if dateutil.InRange(t.Date, start, end) {
			out = append(out, t)
		}
	}
	return out, nil
}

// --- Budgets ---

func (s *SQLiteStore) PutBudget(b domain.Budget) error { return putJSON(s.db, "budgets", b.ID, b) }
func (s *SQLiteStore) GetBudget(id string) (domain.Budget, bool, error) {
	return getJSON[domain.Budget](s.db, "budgets", id)
}
func (s *SQLiteStore) DeleteBudget(id string) (bool, error) { return deleteRow(s.db, "budgets", id) }
func (s *SQLiteStore) ListBudgets() ([]domain.Budget, error) {
	return loadRows[domain.Budget](s.db, "budgets")
}

// --- Goals ---

func (s *SQLiteStore) PutGoal(g domain.Goal) error { return putJSON(s.db, "goals", g.ID, g) }
func (s *SQLiteStore) GetGoal(id string) (domain.Goal, bool, error) {
	return getJSON[domain.Goal](s.db, "goals", id)
}
func (s *SQLiteStore) DeleteGoal(id string) (bool, error) { return deleteRow(s.db, "goals", id) }
func (s *SQLiteStore) ListGoals() ([]domain.Goal, error) {
	return loadRows[domain.Goal](s.db, "goals")
}

// --- Tasks ---

func (s *SQLiteStore) PutTask(t domain.Task) error { return putJSON(s.db, "tasks", t.ID, t) }
func (s *SQLiteStore) GetTask(id string) (domain.Task, bool, error) {
	return getJSON[domain.Task](s.db, "tasks", id)
}
func (s *SQLiteStore) DeleteTask(id string) (bool, error) { return deleteRow(s.db, "tasks", id) }
func (s *SQLiteStore) ListTasks() ([]domain.Task, error) {
	return loadRows[domain.Task](s.db, "tasks")
}

// TasksByStatus returns tasks in a given status.
func (s *SQLiteStore) TasksByStatus(status domain.TaskStatus) ([]domain.Task, error) {
	return queryRows[domain.Task](s.db,
		"SELECT data FROM tasks WHERE json_extract(data, '$.status') = ? ORDER BY id", string(status))
}

// --- Custom field definitions ---

func (s *SQLiteStore) PutCustomFieldDef(d customfields.Def) error {
	return putJSON(s.db, "customfielddefs", d.ID, d)
}
func (s *SQLiteStore) GetCustomFieldDef(id string) (customfields.Def, bool, error) {
	return getJSON[customfields.Def](s.db, "customfielddefs", id)
}
func (s *SQLiteStore) DeleteCustomFieldDef(id string) (bool, error) {
	return deleteRow(s.db, "customfielddefs", id)
}
func (s *SQLiteStore) ListCustomFieldDefs() ([]customfields.Def, error) {
	return loadRows[customfields.Def](s.db, "customfielddefs")
}

// CustomFieldDefsByEntity returns the definitions registered for one entity type.
func (s *SQLiteStore) CustomFieldDefsByEntity(entityType string) ([]customfields.Def, error) {
	return queryRows[customfields.Def](s.db,
		"SELECT data FROM customfielddefs WHERE json_extract(data, '$.entityType') = ? ORDER BY id", entityType)
}

// --- Auto-categorization rules ---

func (s *SQLiteStore) PutRule(r rules.Rule) error {
	return putJSON(s.db, "rules", r.ID, r)
}
func (s *SQLiteStore) GetRule(id string) (rules.Rule, bool, error) {
	return getJSON[rules.Rule](s.db, "rules", id)
}
func (s *SQLiteStore) DeleteRule(id string) (bool, error) {
	return deleteRow(s.db, "rules", id)
}
func (s *SQLiteStore) ListRules() ([]rules.Rule, error) {
	return loadRows[rules.Rule](s.db, "rules")
}

// --- Documents (imported statements/receipts) ---

func (s *SQLiteStore) PutDocument(d domain.Document) error {
	return putJSON(s.db, "documents", d.ID, d)
}
func (s *SQLiteStore) GetDocument(id string) (domain.Document, bool, error) {
	return getJSON[domain.Document](s.db, "documents", id)
}
func (s *SQLiteStore) DeleteDocument(id string) (bool, error) {
	return deleteRow(s.db, "documents", id)
}
func (s *SQLiteStore) ListDocuments() ([]domain.Document, error) {
	return loadRows[domain.Document](s.db, "documents")
}

// --- Saved insights (pinned AI insights) ---

func (s *SQLiteStore) PutSavedInsight(si domain.SavedInsight) error {
	return putJSON(s.db, "savedinsights", si.ID, si)
}
func (s *SQLiteStore) GetSavedInsight(id string) (domain.SavedInsight, bool, error) {
	return getJSON[domain.SavedInsight](s.db, "savedinsights", id)
}
func (s *SQLiteStore) DeleteSavedInsight(id string) (bool, error) {
	return deleteRow(s.db, "savedinsights", id)
}
func (s *SQLiteStore) ListSavedInsights() ([]domain.SavedInsight, error) {
	return loadRows[domain.SavedInsight](s.db, "savedinsights")
}

// --- Recurring cash flows (scheduled bills / income) ---

func (s *SQLiteStore) PutRecurring(r domain.Recurring) error {
	return putJSON(s.db, "recurring", r.ID, r)
}
func (s *SQLiteStore) GetRecurring(id string) (domain.Recurring, bool, error) {
	return getJSON[domain.Recurring](s.db, "recurring", id)
}
func (s *SQLiteStore) DeleteRecurring(id string) (bool, error) {
	return deleteRow(s.db, "recurring", id)
}
func (s *SQLiteStore) ListRecurring() ([]domain.Recurring, error) {
	return loadRows[domain.Recurring](s.db, "recurring")
}
