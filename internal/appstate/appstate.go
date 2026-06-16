// Package appstate is the seam between the UI and the persistence/logic layers.
// It owns the in-memory SQLite store and a slog logger, validates writes, and
// exposes typed read/write accessors plus JSON import/export.
//
// It is pure Go (no syscall/js): the store builds for js/wasm, and Go's wasm
// runtime already routes os.Stderr to the browser console, so logging needs no
// platform code. This keeps appstate unit-testable on native Go.
package appstate

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/logging"
	"github.com/monstercameron/CashFlux/internal/rules"
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

// TransactionsCSV renders an arbitrary set of transactions as CSV — e.g. a
// filtered subset from the ledger view.
func (a *App) TransactionsCSV(txns []domain.Transaction) ([]byte, error) {
	return store.TransactionsToCSV(txns)
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

// Rules returns every auto-categorization rule.
func (a *App) Rules() []rules.Rule { v, err := a.store.ListRules(); a.logErr("rules", err); return v }

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

// FreshnessWindows returns the staleness windows with the household's per-type
// overrides (from Settings) layered over the built-in defaults.
func (a *App) FreshnessWindows() freshness.Windows {
	overrides := freshness.Windows{}
	for k, v := range a.Settings().FreshnessOverrides {
		overrides[domain.AccountType(k)] = v
	}
	return freshness.DefaultWindows().Merge(overrides)
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
	if err := a.validateCustom("member", m.Custom); err != nil {
		return err
	}
	if err := a.store.PutMember(m); err != nil {
		return err
	}
	a.log.Info("member saved", "id", m.ID)
	return nil
}
func (a *App) DeleteMember(id string) error { return a.del("member", id, a.store.DeleteMember) }

// ReassignOwner moves every account, budget, goal, and transaction owned by oldID
// to newID, returning how many records moved. Scope follows the new owner (shared
// for the group owner, individual otherwise); transactions attributed to the old
// member are re-attributed (cleared when moving to the group). Use it before
// deleting a member who still owns entities. The member itself is not deleted.
func (a *App) ReassignOwner(oldID, newID string) (int, error) {
	scope := domain.ScopeIndividual
	memberID := newID
	if newID == domain.GroupOwnerID {
		scope = domain.ScopeShared
		memberID = ""
	}
	moved := 0
	for _, ac := range a.Accounts() {
		if ac.OwnerID == oldID {
			ac.OwnerID, ac.Scope = newID, scope
			if err := a.store.PutAccount(ac); err != nil {
				return moved, err
			}
			moved++
		}
	}
	for _, b := range a.Budgets() {
		if b.OwnerID == oldID {
			b.OwnerID, b.Scope = newID, scope
			if err := a.store.PutBudget(b); err != nil {
				return moved, err
			}
			moved++
		}
	}
	for _, g := range a.Goals() {
		if g.OwnerID == oldID {
			g.OwnerID, g.Scope = newID, scope
			if err := a.store.PutGoal(g); err != nil {
				return moved, err
			}
			moved++
		}
	}
	for _, t := range a.Transactions() {
		if t.MemberID == oldID {
			t.MemberID = memberID
			if err := a.store.PutTransaction(t); err != nil {
				return moved, err
			}
			moved++
		}
	}
	a.log.Info("reassigned owner", "from", oldID, "to", newID, "moved", moved)
	return moved, nil
}

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

// PutRule saves an auto-categorization rule. A rule needs an ID, a non-empty
// match phrase, and a target category to be useful.
func (a *App) PutRule(r rules.Rule) error {
	if r.ID == "" {
		return fmt.Errorf("appstate: rule needs an id")
	}
	if strings.TrimSpace(r.Match) == "" {
		return fmt.Errorf("appstate: rule needs a match phrase")
	}
	if r.SetCategoryID == "" {
		return fmt.Errorf("appstate: rule needs a category")
	}
	if err := a.store.PutRule(r); err != nil {
		return err
	}
	a.log.Info("rule saved", "id", r.ID)
	return nil
}

// DeleteRule removes an auto-categorization rule.
func (a *App) DeleteRule(id string) error { return a.del("rule", id, a.store.DeleteRule) }

// ApplyRules assigns a category to every currently uncategorized, non-transfer
// transaction whose payee/description matches a saved rule (first match wins),
// also adding the rule's tags when the transaction has none. Already-categorized
// transactions are left untouched. It returns how many transactions were updated.
// This is the retroactive counterpart to applying rules at entry/import — handy
// after adding a rule or importing via a path that doesn't auto-apply (e.g. CSV).
func (a *App) ApplyRules() (int, error) {
	rs := a.Rules()
	if len(rs) == 0 {
		return 0, nil
	}
	updated := 0
	for _, t := range a.Transactions() {
		if t.CategoryID != "" || t.IsTransfer() {
			continue
		}
		r := rules.FirstMatch(rs, t.Payee+" "+t.Desc)
		if r == nil {
			continue
		}
		t.CategoryID = r.SetCategoryID
		if len(t.Tags) == 0 && len(r.SetTags) > 0 {
			t.Tags = r.SetTags
		}
		if err := a.store.PutTransaction(t); err != nil {
			return updated, err
		}
		updated++
	}
	a.log.Info("applied rules to existing transactions", "updated", updated)
	return updated, nil
}

// ReassignCategory moves every transaction and budget referencing oldID to newID,
// returning how many records were moved. Use it before deleting a category that
// is still in use. The category itself is not deleted here.
func (a *App) ReassignCategory(oldID, newID string) (int, error) {
	moved := 0
	for _, t := range a.Transactions() {
		if t.CategoryID == oldID {
			t.CategoryID = newID
			if err := a.store.PutTransaction(t); err != nil {
				return moved, err
			}
			moved++
		}
	}
	for _, b := range a.Budgets() {
		if b.CategoryID == oldID {
			b.CategoryID = newID
			if err := a.store.PutBudget(b); err != nil {
				return moved, err
			}
			moved++
		}
	}
	a.log.Info("reassigned category", "from", oldID, "to", newID, "moved", moved)
	return moved, nil
}

func (a *App) PutTransaction(t domain.Transaction) error {
	if is := validate.ValidateTransaction(t); !is.OK() {
		return is
	}
	if err := a.validateCustom("transaction", t.Custom); err != nil {
		return err
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
	if err := a.validateCustom("budget", b.Custom); err != nil {
		return err
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
	if err := a.validateCustom("goal", g.Custom); err != nil {
		return err
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
