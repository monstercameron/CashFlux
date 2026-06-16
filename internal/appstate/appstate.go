// Package appstate is the seam between the UI and the persistence/logic layers.
// It owns the in-memory SQLite store and a slog logger, validates writes, and
// exposes typed read/write accessors plus JSON import/export.
//
// It is pure Go (no syscall/js): the store builds for js/wasm, and Go's wasm
// runtime already routes os.Stderr to the browser console, so logging needs no
// platform code. This keeps appstate unit-testable on native Go.
package appstate

import (
	"io"
	"log/slog"
	"os"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/logging"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// App holds the live application state.
type App struct {
	store *store.SQLiteStore
	log   *slog.Logger
	ring  *logging.Ring
}

// Default is the process-wide App, set by Init and read by screens.
var Default *App

// New creates an App backed by a fresh in-memory store. If w is nil, logs go to
// os.Stderr (the browser console under wasm). When seed is true and the store is
// empty, the sample dataset is loaded.
func New(w io.Writer, seed bool) (*App, error) {
	if w == nil {
		w = os.Stderr
	}
	st, err := store.NewMemory()
	if err != nil {
		return nil, err
	}
	logger, ring := logging.New(w, 500, slog.LevelInfo)
	app := &App{store: st, log: logger, ring: ring}
	if seed {
		if err := st.Load(store.SampleDataset()); err != nil {
			return nil, err
		}
		logger.Info("loaded sample dataset")
	}
	return app, nil
}

// Init creates the App and installs it as Default.
func Init(w io.Writer, seed bool) error {
	app, err := New(w, seed)
	if err != nil {
		return err
	}
	Default = app
	return nil
}

// Store exposes the underlying store for export/import and advanced queries.
func (a *App) Store() *store.SQLiteStore { return a.store }

// Log returns the application logger.
func (a *App) Log() *slog.Logger { return a.log }

// LogRing returns the in-app log ring buffer.
func (a *App) LogRing() *logging.Ring { return a.ring }

// ExportJSON serializes the whole dataset.
func (a *App) ExportJSON() ([]byte, error) {
	ds, err := a.store.Snapshot()
	if err != nil {
		return nil, err
	}
	return store.Export(ds)
}

// ImportJSON replaces all data with the given dataset JSON.
func (a *App) ImportJSON(data []byte) error {
	ds, err := store.Import(data)
	if err != nil {
		return err
	}
	if err := a.store.Load(ds); err != nil {
		return err
	}
	a.log.Info("imported dataset", "accounts", len(ds.Accounts), "transactions", len(ds.Transactions))
	return nil
}

// ExportCSV renders all transactions as CSV bytes (human-readable, stable
// columns) — the spreadsheet-friendly export.
func (a *App) ExportCSV() ([]byte, error) {
	return store.TransactionsToCSV(a.Transactions())
}

// ImportTransactionsCSV parses CSV transaction rows and stores each via the
// validated write path (best-effort: invalid rows are skipped), returning how
// many were imported. A parse error (malformed CSV) is returned as-is.
func (a *App) ImportTransactionsCSV(data []byte) (int, error) {
	txns, err := store.TransactionsFromCSV(data)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range txns {
		if err := a.PutTransaction(t); err == nil {
			n++
		}
	}
	a.log.Info("imported transactions from CSV", "imported", n, "rows", len(txns))
	return n, nil
}

// LoadSample replaces all data with the built-in sample dataset (the "load
// sample" action), giving a new household something to explore.
func (a *App) LoadSample() error {
	if err := a.store.Load(store.SampleDataset()); err != nil {
		a.log.Error("load sample", "err", err)
		return err
	}
	a.log.Info("loaded sample data")
	return nil
}

// Wipe removes all data from the store (the "wipe data" action).
func (a *App) Wipe() error {
	if err := a.store.Wipe(); err != nil {
		a.log.Error("wipe", "err", err)
		return err
	}
	a.log.Info("wiped all data")
	return nil
}

// --- read accessors (errors are logged and swallowed; UI shows empty) ---

func (a *App) Members() []domain.Member {
	v, err := a.store.ListMembers()
	a.logErr("members", err)
	return v
}
func (a *App) Accounts() []domain.Account {
	v, err := a.store.ListAccounts()
	a.logErr("accounts", err)
	return v
}
func (a *App) Categories() []domain.Category {
	v, err := a.store.ListCategories()
	a.logErr("categories", err)
	return v
}
func (a *App) Transactions() []domain.Transaction {
	v, err := a.store.ListTransactions()
	a.logErr("transactions", err)
	return v
}
func (a *App) Budgets() []domain.Budget {
	v, err := a.store.ListBudgets()
	a.logErr("budgets", err)
	return v
}
func (a *App) Goals() []domain.Goal { v, err := a.store.ListGoals(); a.logErr("goals", err); return v }
func (a *App) Tasks() []domain.Task { v, err := a.store.ListTasks(); a.logErr("tasks", err); return v }

// CustomFieldDefs returns every registered custom-field definition.
func (a *App) CustomFieldDefs() []customfields.Def {
	v, err := a.store.ListCustomFieldDefs()
	a.logErr("customFieldDefs", err)
	return v
}

// CustomFieldDefsFor returns the custom-field definitions for one entity type
// (e.g. "account", "transaction").
func (a *App) CustomFieldDefsFor(entityType string) []customfields.Def {
	v, err := a.store.CustomFieldDefsByEntity(entityType)
	a.logErr("customFieldDefs", err)
	return v
}

// Settings returns the stored settings.
func (a *App) Settings() store.Settings {
	s, err := a.store.GetSettings()
	a.logErr("settings", err)
	return s
}

func (a *App) logErr(entity string, err error) {
	if err != nil {
		a.log.Error("read failed", "entity", entity, "err", err)
	}
}

// --- validated write-through accessors ---

func (a *App) PutMember(m domain.Member) error {
	if is := validate.ValidateMember(m); !is.OK() {
		return is
	}
	if err := a.store.PutMember(m); err != nil {
		return err
	}
	a.log.Info("member saved", "id", m.ID)
	return nil
}
func (a *App) DeleteMember(id string) error { return a.del("member", id, a.store.DeleteMember) }

func (a *App) PutAccount(ac domain.Account) error {
	if is := validate.ValidateAccount(ac); !is.OK() {
		return is
	}
	if err := a.validateCustom("account", ac.Custom); err != nil {
		return err
	}
	if err := a.store.PutAccount(ac); err != nil {
		return err
	}
	a.log.Info("account saved", "id", ac.ID)
	return nil
}

// validateCustom checks an entity's custom-field values against the definitions
// registered for its type. A definition read error never blocks a save (logged
// and ignored); only genuine value problems are returned, as validate.Issues.
func (a *App) validateCustom(entityType string, custom map[string]any) error {
	defs, err := a.store.CustomFieldDefsByEntity(entityType)
	if err != nil {
		a.log.Error("load custom defs", "entity", entityType, "err", err)
		return nil
	}
	if len(defs) == 0 {
		return nil
	}
	issues := customfields.Validate(defs, custom)
	if len(issues) == 0 {
		return nil
	}
	var is validate.Issues
	for _, m := range issues {
		is = append(is, validate.Issue{Field: "customField", Message: m})
	}
	return is
}
func (a *App) DeleteAccount(id string) error { return a.del("account", id, a.store.DeleteAccount) }

func (a *App) PutCategory(c domain.Category) error {
	if is := validate.ValidateCategory(c); !is.OK() {
		return is
	}
	if err := a.store.PutCategory(c); err != nil {
		return err
	}
	a.log.Info("category saved", "id", c.ID)
	return nil
}
func (a *App) DeleteCategory(id string) error { return a.del("category", id, a.store.DeleteCategory) }

func (a *App) PutTransaction(t domain.Transaction) error {
	if is := validate.ValidateTransaction(t); !is.OK() {
		return is
	}
	if err := a.store.PutTransaction(t); err != nil {
		return err
	}
	a.log.Info("transaction saved", "id", t.ID)
	return nil
}
func (a *App) DeleteTransaction(id string) error {
	return a.del("transaction", id, a.store.DeleteTransaction)
}

func (a *App) PutBudget(b domain.Budget) error {
	if is := validate.ValidateBudget(b); !is.OK() {
		return is
	}
	if err := a.store.PutBudget(b); err != nil {
		return err
	}
	a.log.Info("budget saved", "id", b.ID)
	return nil
}
func (a *App) DeleteBudget(id string) error { return a.del("budget", id, a.store.DeleteBudget) }

func (a *App) PutGoal(g domain.Goal) error {
	if is := validate.ValidateGoal(g); !is.OK() {
		return is
	}
	if err := a.store.PutGoal(g); err != nil {
		return err
	}
	a.log.Info("goal saved", "id", g.ID)
	return nil
}
func (a *App) DeleteGoal(id string) error { return a.del("goal", id, a.store.DeleteGoal) }

func (a *App) PutTask(t domain.Task) error {
	if is := validate.ValidateTask(t); !is.OK() {
		return is
	}
	if err := a.store.PutTask(t); err != nil {
		return err
	}
	a.log.Info("task saved", "id", t.ID)
	return nil
}
func (a *App) DeleteTask(id string) error { return a.del("task", id, a.store.DeleteTask) }

// PutCustomFieldDef validates and saves a custom-field definition. The Def must
// be sound (id, entity type, key, label, known type; choice fields need options).
func (a *App) PutCustomFieldDef(d customfields.Def) error {
	if issues := d.Validate(); len(issues) > 0 {
		var is validate.Issues
		for _, m := range issues {
			is = append(is, validate.Issue{Field: "customField", Message: m})
		}
		return is
	}
	if err := a.store.PutCustomFieldDef(d); err != nil {
		return err
	}
	a.log.Info("custom field saved", "id", d.ID, "entity", d.EntityType)
	return nil
}

// DeleteCustomFieldDef removes a custom-field definition by id.
func (a *App) DeleteCustomFieldDef(id string) error {
	return a.del("customFieldDef", id, a.store.DeleteCustomFieldDef)
}

// PutSettings saves settings.
func (a *App) PutSettings(s store.Settings) error {
	if err := a.store.PutSettings(s); err != nil {
		return err
	}
	a.log.Info("settings saved")
	return nil
}

func (a *App) del(entity, id string, fn func(string) (bool, error)) error {
	ok, err := fn(id)
	if err != nil {
		return err
	}
	a.log.Info("entity deleted", "entity", entity, "id", id, "existed", ok)
	return nil
}
