package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
	_ "github.com/ncruces/go-sqlite3/driver" // registers the pure-Go "sqlite3" driver (embeds SQLite via wazero)
)

// SQLiteStore is an in-memory, pure-Go SQLite-backed store. Each entity is kept
// as a JSON document keyed by id, which keeps ingress (Load) and egress
// (Snapshot) clean and round-trippable while still giving us SQL on top.
type SQLiteStore struct {
	db *sql.DB
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS members      (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS accounts     (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS categories   (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS transactions (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS budgets      (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS goals        (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tasks        (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS customfielddefs (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS rules        (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS documents    (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS savedinsights (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS conversations (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recurring    (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS allocprofiles (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS formulas     (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS plans        (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS custompages  (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS artifacts    (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS workflows    (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS workflowruns (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sharedexpenses (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settlements  (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settings     (id TEXT PRIMARY KEY, data TEXT NOT NULL);
`

// NewMemory opens a fresh in-memory SQLite database and creates the schema. A
// single connection is pinned so the in-memory database is shared across calls.
func NewMemory() (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close releases the database.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func replaceRows[T any](tx *sql.Tx, table string, items []T, idOf func(T) string) error {
	if _, err := tx.Exec("DELETE FROM " + table); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO " + table + "(id, data) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		data, err := json.Marshal(it)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(idOf(it), string(data)); err != nil {
			return err
		}
	}
	return nil
}

func loadRows[T any](db *sql.DB, table string) ([]T, error) {
	return queryRows[T](db, "SELECT data FROM "+table+" ORDER BY id")
}

// Load replaces all stored data with the given dataset (clean ingress).
func (s *SQLiteStore) Load(ds Dataset) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := replaceRows(tx, "members", ds.Members, func(m domain.Member) string { return m.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "accounts", ds.Accounts, func(a domain.Account) string { return a.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "categories", ds.Categories, func(c domain.Category) string { return c.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "transactions", ds.Transactions, func(t domain.Transaction) string { return t.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "budgets", ds.Budgets, func(b domain.Budget) string { return b.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "goals", ds.Goals, func(g domain.Goal) string { return g.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "tasks", ds.Tasks, func(t domain.Task) string { return t.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "customfielddefs", ds.CustomFields, func(d customfields.Def) string { return d.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "rules", ds.Rules, func(r rules.Rule) string { return r.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "documents", ds.Documents, func(d domain.Document) string { return d.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "savedinsights", ds.SavedInsights, func(s domain.SavedInsight) string { return s.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "conversations", ds.Conversations, func(c domain.Conversation) string { return c.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "recurring", ds.Recurring, func(r domain.Recurring) string { return r.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "allocprofiles", ds.AllocProfiles, func(p domain.AllocationProfile) string { return p.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "formulas", ds.Formulas, func(f domain.Formula) string { return f.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "plans", ds.Plans, func(p domain.Plan) string { return p.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "custompages", ds.CustomPages, func(p domain.CustomPage) string { return p.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "artifacts", ds.Artifacts, func(a domain.Artifact) string { return a.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "workflows", ds.Workflows, func(w workflow.Workflow) string { return w.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "workflowruns", ds.WorkflowRuns, func(r workflow.Run) string { return r.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "sharedexpenses", ds.SharedExpenses, func(e domain.SharedExpense) string { return e.ID }); err != nil {
		return err
	}
	if err := replaceRows(tx, "settlements", ds.Settlements, func(s domain.Settlement) string { return s.ID }); err != nil {
		return err
	}

	settingsData, err := json.Marshal(ds.Settings)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM settings"); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO settings(id, data) VALUES('app', ?)", string(settingsData)); err != nil {
		return err
	}
	return tx.Commit()
}

// Snapshot reads the entire dataset back out (clean egress).
func (s *SQLiteStore) Snapshot() (Dataset, error) {
	var ds Dataset
	var err error
	if ds.Members, err = loadRows[domain.Member](s.db, "members"); err != nil {
		return Dataset{}, err
	}
	if ds.Accounts, err = loadRows[domain.Account](s.db, "accounts"); err != nil {
		return Dataset{}, err
	}
	if ds.Categories, err = loadRows[domain.Category](s.db, "categories"); err != nil {
		return Dataset{}, err
	}
	if ds.Transactions, err = loadRows[domain.Transaction](s.db, "transactions"); err != nil {
		return Dataset{}, err
	}
	if ds.Budgets, err = loadRows[domain.Budget](s.db, "budgets"); err != nil {
		return Dataset{}, err
	}
	if ds.Goals, err = loadRows[domain.Goal](s.db, "goals"); err != nil {
		return Dataset{}, err
	}
	if ds.Tasks, err = loadRows[domain.Task](s.db, "tasks"); err != nil {
		return Dataset{}, err
	}
	if ds.CustomFields, err = loadRows[customfields.Def](s.db, "customfielddefs"); err != nil {
		return Dataset{}, err
	}
	if ds.Rules, err = loadRows[rules.Rule](s.db, "rules"); err != nil {
		return Dataset{}, err
	}
	if ds.Documents, err = loadRows[domain.Document](s.db, "documents"); err != nil {
		return Dataset{}, err
	}
	if ds.SavedInsights, err = loadRows[domain.SavedInsight](s.db, "savedinsights"); err != nil {
		return Dataset{}, err
	}
	if ds.Conversations, err = loadRows[domain.Conversation](s.db, "conversations"); err != nil {
		return Dataset{}, err
	}
	if ds.Recurring, err = loadRows[domain.Recurring](s.db, "recurring"); err != nil {
		return Dataset{}, err
	}
	if ds.AllocProfiles, err = loadRows[domain.AllocationProfile](s.db, "allocprofiles"); err != nil {
		return Dataset{}, err
	}
	if ds.Formulas, err = loadRows[domain.Formula](s.db, "formulas"); err != nil {
		return Dataset{}, err
	}
	if ds.Plans, err = loadRows[domain.Plan](s.db, "plans"); err != nil {
		return Dataset{}, err
	}
	if ds.CustomPages, err = loadRows[domain.CustomPage](s.db, "custompages"); err != nil {
		return Dataset{}, err
	}
	if ds.Artifacts, err = loadRows[domain.Artifact](s.db, "artifacts"); err != nil {
		return Dataset{}, err
	}
	if ds.Workflows, err = loadRows[workflow.Workflow](s.db, "workflows"); err != nil {
		return Dataset{}, err
	}
	if ds.WorkflowRuns, err = loadRows[workflow.Run](s.db, "workflowruns"); err != nil {
		return Dataset{}, err
	}
	if ds.SharedExpenses, err = loadRows[domain.SharedExpense](s.db, "sharedexpenses"); err != nil {
		return Dataset{}, err
	}
	if ds.Settlements, err = loadRows[domain.Settlement](s.db, "settlements"); err != nil {
		return Dataset{}, err
	}

	var settingsData string
	row := s.db.QueryRow("SELECT data FROM settings WHERE id = 'app'")
	if err := row.Scan(&settingsData); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Dataset{}, err
		}
	} else if err := json.Unmarshal([]byte(settingsData), &ds.Settings); err != nil {
		return Dataset{}, err
	}

	ds.SchemaVersion = SchemaVersion
	return ds, nil
}
