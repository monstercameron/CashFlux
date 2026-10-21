// SPDX-License-Identifier: MIT

// Package appstate is the seam between the UI and the persistence/logic layers.
// It owns the in-memory SQLite store and a slog logger, validates writes, and
// exposes typed read/write accessors plus JSON import/export.
//
// It is pure Go (no syscall/js): the store builds for js/wasm, and Go's wasm
// runtime already routes os.Stderr to the browser console, so logging needs no
// platform code. This keeps appstate unit-testable on native Go.
package appstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/logging"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeebase"
	"github.com/monstercameron/CashFlux/internal/revalue"
	"github.com/monstercameron/CashFlux/internal/ruleapply"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnmove"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// App holds the live application state.
type App struct {
	store      *store.SQLiteStore
	instanceID uint64
	// transactionDeletes retains relationship before-images for the legacy row restore API.
	// The durable undo journal captures these same records independently.
	transactionDeletes map[string][]store.RecordChange
	log                *slog.Logger
	ring               *logging.Ring
	// blobs is the optional IndexedDB-backed binary artifact store. When non-nil,
	// artifact image bytes are kept here instead of in the main dataset JSON blob,
	// which prevents large uploads from blowing the localStorage quota. Wired in by
	// the wasm entry point after OpenIDB succeeds; nil on native Go (tests) or if
	// IndexedDB is unavailable.
	blobs artifactstore.Store
	// blobUsageCache is the last successfully queried blob-store usage in bytes.
	// It is updated by RefreshBlobUsage (called outside the render path) so that
	// BlobStoreUsage can be called safely from render functions without blocking
	// on the single-threaded wasm runtime.
	blobUsageCache int64
	// reads memoizes the list accessors (Accounts/Transactions/…) by the store's
	// mutation revision, so a screen that reads the same table many times in one render
	// pays the SQL query + JSON unmarshal only once. Each accessor still returns a fresh
	// top-level copy, so callers keep their "you own this slice" contract (safe to sort
	// or filter in place). Single-threaded wasm, so no locking.
	reads    appReadCaches
	settings settingsSnapshotCache
	// triggersSuspended pauses automatic workflow firing from PutTransaction while
	// a bulk operation (import) or a workflow's own effects are running, so a single
	// user-facing "add a transaction" fires triggers but a 500-row import doesn't
	// fire 500 times (and workflow effects can't recursively re-trigger).
	triggersSuspended bool
	// now returns the current time; overridable in tests for deterministic
	// month-scoped figures. Defaults to time.Now.
	now func() time.Time
	// Notifier, if set, surfaces a workflow "notify" message to the user (e.g. a
	// toast). The wasm app wires this; when nil, notices go only to the log.
	Notifier func(string)
	// txnMutatedFns is the set of observers registered via OnTxnMutated, called
	// after any successful transaction add, edit, or delete (C122).
	txnMutatedFns                  []func()
	durableWriter                  func(uint64, func(DurableResult))
	transactionDurabilityObservers []func(TransactionDurability)
	transactionDurabilityOperation uint64
	// suppressTxnObservers, when true, causes fireTxnMutated to return early
	// without calling any observer — used during bulk import so the re-evaluation
	// fires at most once per batch rather than once per row.
	suppressTxnObservers bool
	// activeRoleFn, when non-nil, is called by roleGuard / memberRoleGuard on
	// every mutating operation to determine whether the current operator's role
	// permits the action. Injected via SetActiveRoleFunc; nil ⇒ permissive
	// (RoleOwner). See readonly.go for the seam design.
	activeRoleFn func() domain.MemberRole
}

// Default is the process-wide App, set by Init and read by screens.
var Default *App

var nextInstanceID atomic.Uint64

// InstanceID is a process-local identity for derived-data cache keys. Revisions
// alone are insufficient: multiple app stores share the global mutation counter.
// The identity is stable for this App's lifetime and never exported with data.
func (a *App) InstanceID() uint64 { return a.instanceID }

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
	app := &App{store: st, instanceID: nextInstanceID.Add(1), log: logger, ring: ring, now: time.Now}
	if seed {
		if err := st.Load(store.SampleDataset()); err != nil {
			return nil, err
		}
		logger.Info("loaded sample dataset")
	}
	// Install any persisted refund-pair links into the read-model netting
	// overlays so pre-existing pairs net from the first render (XC2).
	app.SyncTxnLinkNetting()
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

// ExportJSON serializes the whole dataset with device credentials removed.
func (a *App) ExportJSON() ([]byte, error) {
	ds, err := a.store.Snapshot()
	if err != nil {
		return nil, err
	}
	ds.Settings.OpenAIKey = ""
	return store.Export(ds)
}

// ExportJSONRedacted serializes the whole dataset with the device credential
// removed. It is retained for callers that distinguish local snapshots from
// manual exports; both formats omit credentials.
func (a *App) ExportJSONRedacted() ([]byte, error) {
	ds, err := a.store.Snapshot()
	if err != nil {
		return nil, err
	}
	ds.Settings.OpenAIKey = ""
	return store.Export(ds)
}

// ImportJSON replaces all data with the given dataset JSON.
func (a *App) ImportJSON(data []byte) error {
	parsed, err := ParseDataset(data)
	if err != nil {
		return err
	}
	return a.ImportParsedDataset(parsed)
}

// ParsedDataset holds a migrated import without exposing mutable dataset fields.
// Its zero value is invalid; ParseDataset is the only constructor.
type ParsedDataset struct {
	dataset *store.PreparedDataset
}

// ParseDataset validates and migrates JSON once. Recovery may retain the result
// only while its source checkpoint remains unchanged; replayed bytes need a new parse.
func ParseDataset(data []byte) (*ParsedDataset, error) {
	ds, err := store.PrepareDataset(data)
	if err != nil {
		return nil, err
	}
	return &ParsedDataset{dataset: ds}, nil
}

// ImportParsedDataset replaces all data using a validated parse. Like ImportJSON,
// it preserves this device's credential, loads through the store's clean ingress,
// and synchronizes transaction links. It does not retain the parsed handle.
func (a *App) ImportParsedDataset(parsed *ParsedDataset) error {
	if parsed == nil || parsed.dataset == nil {
		return errors.New("appstate: dataset has not been parsed")
	}
	// OpenAI credentials belong to this device, so every dataset import (sync
	// pulls, restores and user imports) must replace the incoming value even when
	// this device has no credential. Otherwise an imported key would be adopted.
	if err := a.store.LoadPrepared(parsed.dataset, a.Settings().OpenAIKey); err != nil {
		return err
	}
	accounts, transactions := parsed.dataset.Counts()
	a.log.Info("imported dataset", "accounts", accounts, "transactions", transactions)
	a.SyncTxnLinkNetting()
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

// CSVImportResult is what one CSV import actually did, with the outcomes kept
// apart (C687).
//
// The old signature returned only the imported count and the parse failures, and
// computed the duplicate count purely to write it to a log line. So the summary a
// person read could say "imported 12, skipped 3" where the 3 were parse failures,
// or say nothing at all where 40 rows were skipped as duplicates — and the import
// history recorded one conflated number for both. Those are different facts: a
// duplicate is the safeguard working, a parse failure is money missing from the
// ledger that nothing else will notice.
type CSVImportResult struct {
	// Outcomes preserves a result for every staged source row.
	Outcomes []ImportRowOutcome
	// Imported is how many rows were written.
	Imported int
	// Duplicates is how many rows the ledger already had.
	Duplicates int
	// Failed is the rows that could not be parsed, with their reasons.
	Failed []store.CSVRowError
	// PossibleDuplicates are similar rows with a distinct identity that remain
	// imported and need user review.
	PossibleDuplicates []store.CSVRowError
	// DocumentID is the import run these rows are stamped with, empty when the
	// caller did not name one.
	DocumentID string
}

// Presented is how many rows the file offered in total.
func (r CSVImportResult) Presented() int { return r.Imported + r.Duplicates + len(r.Failed) }

// ImportTransactionsCSV parses and writes a CSV of transactions.
//
// docID stamps every written row with the import run that created it
// (domain.Transaction.SourceDocID). Passing it is what makes the import history
// checkable afterwards: without it, "this run added 688 rows" and "the ledger
// holds 565" are two unconnected numbers and the difference has no explanation.
// Empty means the caller is not recording a run.
func (a *App) ImportTransactionsCSV(data []byte, fallbackAccountID, docID string, checkpointID ...string) (CSVImportResult, error) {
	batch, err := a.StageCSV(data, fallbackAccountID)
	if err != nil {
		return CSVImportResult{}, err
	}
	result, err := a.Commit(batch, docID, firstCheckpoint(checkpointID))
	// Compatibility callers render per-row failures rather than an additional
	// storage error; the new Commit API returns both.
	if err != nil && len(result.Failed) > 0 {
		return result, nil
	}
	return result, err
}

func firstCheckpoint(ids []string) string {
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func (a *App) importResolvers() (func(string) string, func(string) string, func(string) string) {
	accPairs := make([][2]string, 0, len(a.Accounts()))
	for _, v := range a.Accounts() {
		accPairs = append(accPairs, [2]string{v.ID, v.Name})
	}
	catPairs := make([][2]string, 0, len(a.Categories()))
	for _, v := range a.Categories() {
		catPairs = append(catPairs, [2]string{v.ID, v.Name})
	}
	memPairs := make([][2]string, 0, len(a.Members()))
	for _, v := range a.Members() {
		memPairs = append(memPairs, [2]string{v.ID, v.Name})
	}
	return idResolver(accPairs), idResolver(catPairs), idResolver(memPairs)
}

func csvSourceLine(parsedIndex int, skipped []store.CSVRowError) int {
	line := parsedIndex + 2
	for _, row := range skipped {
		if row.Line <= line {
			line++
		}
	}
	return line
}

// idResolver builds a function that maps a CSV reference cell to an entity id:
// an exact id passes through, a case-insensitive name match resolves to its id,
// and anything else (including "") is returned unchanged so the validated write
// path can accept or skip it. pairs are the entities' {id, name} tuples.
func idResolver(pairs [][2]string) func(string) string {
	ids := make(map[string]bool, len(pairs))
	byName := make(map[string]string, len(pairs))
	for _, p := range pairs {
		ids[p[0]] = true
		if p[1] != "" {
			byName[strings.ToLower(strings.TrimSpace(p[1]))] = p[0]
		}
	}
	return func(v string) string {
		if v == "" || ids[v] {
			return v
		}
		if id, ok := byName[strings.ToLower(v)]; ok {
			return id
		}
		return v
	}
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

// GetKV returns the persisted app/UI value for key (centralized in SQLite, not
// scattered localStorage), and whether it was present.
func (a *App) GetKV(key string) (string, bool) {
	v, ok, err := a.store.GetKV(key)
	a.logErr("get kv", err)
	return v, ok
}

// SetKV persists an app/UI value into the SQLite dataset (so it survives reloads
// and is cleared by a wipe like every other non-settings table).
func (a *App) SetKV(key, val string) error { return a.store.SetKV(key, val) }

// DeleteKV removes a persisted app/UI value.
func (a *App) DeleteKV(key string) error { return a.store.DeleteKV(key) }

// GetSettingKV/SetSettingKV/DeleteSettingKV persist config & preferences into the
// SQLite dataset's preserved settings KV (survives a wipe, unlike GetKV/SetKV).
func (a *App) GetSettingKV(key string) (string, bool) {
	v, ok, err := a.store.GetSettingKV(key)
	a.logErr("get setting kv", err)
	return v, ok
}
func (a *App) SetSettingKV(key, val string) error { return a.store.SetSettingKV(key, val) }
func (a *App) DeleteSettingKV(key string) error   { return a.store.DeleteSettingKV(key) }

// Wipe removes all data from the store (the "wipe data" action).
func (a *App) Wipe() error {
	if err := a.store.Wipe(); err != nil {
		a.log.Error("wipe", "err", err)
		return err
	}
	a.log.Info("wiped all data")
	a.SyncTxnLinkNetting()
	return nil
}

// --- read accessors (errors are logged and swallowed; UI shows empty) ---

// revCache memoizes one list accessor's result by the store's mutation revision.
// A screen render reads the same table many times; without this, each read pays a
// fresh SQL query + JSON unmarshal. get() re-loads only when the revision advances,
// and always hands back a fresh top-level copy so callers keep their "you own this
// slice" contract (sorting or filtering the result in place can't corrupt the cache).
// wasm is single-threaded, so no locking is needed.
type revCache[T any] struct {
	rev    uint64
	val    []T
	loaded bool
}

func (c *revCache[T]) get(rev uint64, load func() []T) []T {
	if !c.loaded || c.rev != rev {
		c.val, c.rev, c.loaded = load(), rev, true
	}
	if len(c.val) == 0 {
		return nil
	}
	out := make([]T, len(c.val))
	copy(out, c.val)
	return out
}

func (c *revCache[T]) getChecked(rev uint64, load func() ([]T, error)) ([]T, error) {
	if !c.loaded || c.rev != rev {
		value, err := load()
		if err != nil {
			return nil, err
		}
		c.val, c.rev, c.loaded = value, rev, true
	}
	if len(c.val) == 0 {
		return nil, nil
	}
	out := make([]T, len(c.val))
	copy(out, c.val)
	return out, nil
}

// appReadCaches holds one revCache per frequently-read table. It lives inline on
// App (see the reads field) so every accessor shares the same revision key.
type appReadCaches struct {
	members      revCache[domain.Member]
	accounts     revCache[domain.Account]
	categories   revCache[domain.Category]
	transactions ledgerSnapshotCache
	budgets      revCache[domain.Budget]
	goals        revCache[domain.Goal]
	tasks        revCache[domain.Task]
	holdings     revCache[domain.Holding]
	realized     revCache[domain.RealizedSale]
	rules        revCache[rules.Rule]
	documents    revCache[domain.Document]
	recurring    revCache[domain.Recurring]
}

func (a *App) Members() []domain.Member {
	return a.reads.members.get(a.store.Rev(), func() []domain.Member {
		v, err := a.store.ListMembers()
		a.logErr("members", err)
		return v
	})
}
func (a *App) Accounts() []domain.Account {
	rows, err := a.AccountsChecked()
	if err != nil {
		a.logErr("accounts", err)
	}
	return rows
}

// Categories returns every category, ordered naturally by name.
//
// C518/C544: the ordering lives HERE, at the single seam every reader passes
// through, rather than in each screen that draws a list. It was previously in
// store order (i.e. insertion order), and the screens disagreed about what to
// do with that — most sorted nothing, two sorted with a raw byte comparison, so
// "Zoo" came before "apple" and "Item 10" before "Item 9". A long list in an
// arbitrary order does not read as unsorted, it reads as INCOMPLETE, which is
// what produced the "the picker is missing my categories" report.
//
// Sorting at the accessor means a new picker is correct by default rather than
// correct only if its author remembered. Nothing may depend on store order:
// insertion order is arbitrary, and every ordered consumer either wants names
// (this) or the tree (categorytree.Flatten, which re-sorts siblings anyway).
// The result is cached per store revision, so the sort runs once per mutation,
// not once per render.
func (a *App) Categories() []domain.Category {
	return a.reads.categories.get(a.store.Rev(), func() []domain.Category {
		v, err := a.CategoriesChecked()
		a.logErr("categories", err)
		return v
	})
}

// CategoriesChecked reads current categories in the canonical natural order.
// Unlike Categories, it preserves storage failures for callers that must
// distinguish unavailable data from a genuinely empty category collection.
func (a *App) CategoriesChecked() ([]domain.Category, error) {
	if a == nil || a.store == nil {
		return nil, fmt.Errorf("read categories: local dataset is unavailable")
	}
	rows, err := a.store.ListCategories()
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	return catname.Sorted(rows), nil
}

// Transactions returns the ledger snapshot for the current store revision.
// The returned slice and its nested values are shared and read-only; callers must
// not mutate them. Repeated calls within one revision return the same backing array.
func (a *App) Transactions() []domain.Transaction {
	return a.ledgerSnapshot().rows
}

// TransactionsByAccount returns an immutable snapshot of one account's rows.
func (a *App) TransactionsByAccount(accountID string) []domain.Transaction {
	return a.ledgerSnapshot().byAccount[accountID]
}

// TransactionByID returns a shared, read-only transaction and whether it exists.
func (a *App) TransactionByID(id string) (domain.Transaction, bool) {
	snapshot := a.ledgerSnapshot()
	index, ok := snapshot.byID[id]
	if !ok {
		return domain.Transaction{}, false
	}
	return snapshot.rows[index], true
}
func (a *App) Budgets() []domain.Budget {
	return a.reads.budgets.get(a.store.Rev(), func() []domain.Budget {
		v, err := a.store.ListBudgets()
		a.logErr("budgets", err)
		return v
	})
}
func (a *App) Goals() []domain.Goal {
	rows, err := a.GoalsChecked()
	if err != nil {
		a.logErr("goals", err)
	}
	return rows
}
func (a *App) Tasks() []domain.Task {
	return a.reads.tasks.get(a.store.Rev(), func() []domain.Task {
		v, err := a.store.ListTasks()
		a.logErr("tasks", err)
		return v
	})
}

// Holdings returns every persisted investment holding.
func (a *App) Holdings() []domain.Holding {
	return a.reads.holdings.get(a.store.Rev(), func() []domain.Holding {
		v, err := a.store.ListHoldings()
		a.logErr("holdings", err)
		return v
	})
}

// RealizedSales returns every recorded disposal (FP-T1d).
func (a *App) RealizedSales() []domain.RealizedSale {
	return a.reads.realized.get(a.store.Rev(), func() []domain.RealizedSale {
		v, err := a.store.ListRealizedSales()
		a.logErr("realizedsales", err)
		return v
	})
}

// Rules returns every auto-categorization rule.
func (a *App) Rules() []rules.Rule {
	return a.reads.rules.get(a.store.Rev(), func() []rules.Rule {
		v, err := a.store.ListRules()
		a.logErr("rules", err)
		return v
	})
}

// Rev returns the store's monotonic mutation revision — an O(1) value that
// advances on every entity write or delete. It is the correct cache key for
// render-time memoization of derived values (net worth, totals, budget health),
// since it changes exactly when the underlying data does (§1.6).
func (a *App) Rev() uint64 { return a.store.Rev() }

// Documents returns every imported-document record.
func (a *App) Documents() []domain.Document {
	return a.reads.documents.get(a.store.Rev(), func() []domain.Document {
		v, err := a.store.ListDocuments()
		a.logErr("documents", err)
		return v
	})
}

// SavedInsights returns every pinned AI insight.
func (a *App) SavedInsights() []domain.SavedInsight {
	v, err := a.store.ListSavedInsights()
	a.logErr("saved insights", err)
	return v
}

// Recurring returns every scheduled recurring cash flow.
func (a *App) Recurring() []domain.Recurring {
	return a.reads.recurring.get(a.store.Rev(), func() []domain.Recurring {
		v, err := a.store.ListRecurring()
		a.logErr("recurring", err)
		return v
	})
}

// AllocProfiles returns every saved capital-allocation weight profile.
func (a *App) AllocProfiles() []domain.AllocationProfile {
	v, err := a.store.ListAllocProfiles()
	a.logErr("alloc profiles", err)
	return v
}

// Formulas returns every saved custom formula.
func (a *App) Formulas() []domain.Formula {
	v, err := a.store.ListFormulas()
	a.logErr("formulas", err)
	return v
}

// Plans returns every saved what-if plan.
func (a *App) Plans() []domain.Plan {
	v, err := a.store.ListPlans()
	a.logErr("plans", err)
	return v
}

// CustomPages returns every user-authored custom page.
func (a *App) CustomPages() []domain.CustomPage {
	v, err := a.store.ListCustomPages()
	a.logErr("customPages", err)
	return v
}

// Artifacts returns every user-stored artifact (uploaded images, datasets).
func (a *App) Artifacts() []domain.Artifact {
	v, err := a.store.ListArtifacts()
	a.logErr("artifacts", err)
	return v
}

// Workflows returns every user-defined automation.
func (a *App) Workflows() []workflow.Workflow {
	v, err := a.store.ListWorkflows()
	a.logErr("workflows", err)
	return v
}

// WorkflowRuns returns the audit history of workflow executions.
func (a *App) WorkflowRuns() []workflow.Run {
	v, err := a.store.ListWorkflowRuns()
	a.logErr("workflowRuns", err)
	return v
}

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

// FreshnessWindows returns the staleness windows with the revaluation cadences
// for manual-asset types (AC5) and then the household's per-type overrides (from
// Settings) layered over the built-in defaults. Property/vehicle/crypto get their
// revaluation cadence (quarterly / semi-annual / weekly) instead of sharing a cash
// account's clock; a per-account override (Account.RevalueDays) is applied by
// freshness.EffectiveWindowDays at the point of use and wins over all of these.
func (a *App) FreshnessWindows() freshness.Windows {
	overrides := freshness.Windows{}
	for k, v := range a.Settings().FreshnessOverrides {
		overrides[domain.AccountType(k)] = v
	}
	return freshness.DefaultWindows().Merge(revalue.DefaultCadences()).Merge(overrides)
}

// Settings returns the stored settings.
func (a *App) Settings() store.Settings {
	s, err := a.SettingsChecked()
	a.logErr("settings", err)
	return s
}

// MusicState returns the persisted background-music resume point, if one has been
// checkpointed into the dataset.
func (a *App) MusicState() (store.MusicState, bool) {
	s := a.Settings()
	if s.Music == nil {
		return store.MusicState{}, false
	}
	return *s.Music, true
}

// PutMusicState checkpoints the background-music resume point into the dataset so
// it travels with export/import and backups. Called at coarse moments (track
// change, pause, page close, toggle) — never streamed — to avoid re-serializing
// the whole dataset on every position tick.
func (a *App) PutMusicState(m store.MusicState) error {
	s := a.Settings()
	s.Music = &m
	return a.PutSettings(s)
}

func (a *App) logErr(entity string, err error) {
	if err != nil {
		a.log.Error("read failed", "entity", entity, "err", err)
	}
}

// --- validated write-through accessors ---

func (a *App) PutMember(m domain.Member) error {
	if err := a.memberRoleGuard(); err != nil {
		return err
	}
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
func (a *App) DeleteMember(id string) error {
	if err := a.memberRoleGuard(); err != nil {
		return err
	}
	return a.del("member", id, a.store.DeleteMember)
}

// SetDefaultMember marks exactly one member as the default for new member-scoped
// forms.
func (a *App) SetDefaultMember(id string) error {
	found := false
	for _, m := range a.Members() {
		want := m.ID == id
		if want {
			found = true
		}
		if m.IsDefault == want {
			continue
		}
		m.IsDefault = want
		if err := a.store.PutMember(m); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("member %q not found", id)
	}
	a.log.Info("default member set", "id", id)
	return nil
}

// DefaultMemberID returns the configured default member, or the first member as
// a stable fallback when none is marked default.
func (a *App) DefaultMemberID() string {
	members := a.Members()
	for _, m := range members {
		if m.IsDefault {
			return m.ID
		}
	}
	if len(members) > 0 {
		return members[0].ID
	}
	return ""
}

// MemberForNewTransaction returns the member attribution a new transaction form
// should use for an account. Individual accounts use their owner; shared/group
// accounts use the household's default member.
func (a *App) MemberForNewTransaction(account domain.Account) string {
	if account.Scope == domain.ScopeIndividual {
		return account.OwnerID
	}
	return a.DefaultMemberID()
}

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
		changed := false
		if ac.OwnerID == oldID {
			ac.OwnerID, ac.Scope = newID, scope
			changed = true
		}
		// Ghost-member guard: also clean up OwnershipShares when the deleted
		// member appears as a share holder (even if not the OwnerID). Rebuild
		// the map without oldID, redistribute its percentage to newID, then
		// clear the map entirely if fewer than two holders remain so the account
		// reverts to single-owner OwnerID behaviour.
		if oldShare, ok := ac.OwnershipShares[oldID]; ok {
			newShares := make(map[string]int, len(ac.OwnershipShares))
			for k, v := range ac.OwnershipShares {
				if k != oldID {
					newShares[k] = v
				}
			}
			newShares[newID] += oldShare
			if len(newShares) < 2 {
				ac.OwnershipShares = nil
			} else {
				ac.OwnershipShares = newShares
			}
			changed = true
		}
		if changed {
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
	validMembers := make(map[string]bool)
	for _, member := range a.Members() {
		validMembers[member.ID] = true
	}
	transactionUpdates := make([]domain.Transaction, 0)
	for _, t := range a.Transactions() {
		owners := t.Owners()
		updated := make([]string, 0, len(owners))
		for _, owner := range owners {
			if owner == oldID {
				if memberID != "" && validMembers[memberID] {
					updated = append(updated, memberID)
				}
				continue
			}
			if validMembers[owner] {
				updated = append(updated, owner)
			}
		}
		if !sameOwnerIDs(owners, updated) {
			t.SetOwners(updated)
			transactionUpdates = append(transactionUpdates, t)
		}
	}
	if err := a.store.PutTransactionsAtomic(transactionUpdates); err != nil {
		return moved, fmt.Errorf("reassign transaction owners: %w", err)
	}
	moved += len(transactionUpdates)
	a.log.Info("reassigned owner", "from", oldID, "to", newID, "moved", moved)
	return moved, nil
}

func sameOwnerIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DeleteMemberAfterReassign moves every owned record away from oldID, deletes the
// member, and returns the number of records reassigned.
func (a *App) DeleteMemberAfterReassign(oldID, newID string) (int, error) {
	if oldID == newID {
		return 0, fmt.Errorf("new owner must differ from member %q", oldID)
	}
	moved, err := a.ReassignOwner(oldID, newID)
	if err != nil {
		return moved, err
	}
	if err := a.DeleteMember(oldID); err != nil {
		return moved, err
	}
	return moved, nil
}

func (a *App) PutAccount(ac domain.Account) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if is := validate.ValidateAccount(ac); !is.OK() {
		return is
	}
	if err := a.validateCustom("account", ac.Custom); err != nil {
		return err
	}
	// Archiving deliberately keeps saved defaults. The account resolver skips
	// archived IDs without rewriting them, so restoring this account restores the
	// household's prior choice.
	// Record a BalanceSnapshot when the balance changes — compare against the
	// prior persisted value before writing. For new accounts with a nonzero
	// opening balance, treat the prior balance as 0.
	prior, exists, lookupErr := a.store.GetAccount(ac.ID)
	if lookupErr == nil {
		priorBalance := int64(0)
		if exists {
			priorBalance = prior.OpeningBalance.Amount
		}
		newBalance := ac.OpeningBalance.Amount
		if newBalance != priorBalance {
			asOf := ac.BalanceAsOf
			if asOf.IsZero() {
				asOf = time.Now()
			}
			snap := domain.BalanceSnapshot{
				ID:           id.New(),
				AccountID:    ac.ID,
				BalanceMinor: newBalance,
				Currency:     ac.Currency,
				AsOf:         asOf,
			}
			if snapErr := a.store.PutBalanceSnapshot(snap); snapErr != nil {
				a.log.Error("balance snapshot write failed", "accountId", ac.ID, "err", snapErr)
				// Non-fatal: proceed with the account save.
			}
		}
	}
	if err := a.store.PutAccount(ac); err != nil {
		return err
	}
	a.log.Info("account saved", "id", ac.ID)
	return nil
}

// BalanceHistory returns the recorded balance snapshots for the given account,
// sorted by AsOf ascending (oldest first). It is used by the accounts UI to
// render a compact valuation-history panel for illiquid-asset accounts.
func (a *App) BalanceHistory(accountID string) []domain.BalanceSnapshot {
	snaps, err := a.store.ListBalanceSnapshots(accountID)
	if err != nil {
		a.log.Error("load balance history", "accountId", accountID, "err", err)
		return nil
	}
	// Sort ascending by AsOf so callers can rely on chronological order.
	for i := 1; i < len(snaps); i++ {
		for j := i; j > 0 && snaps[j].AsOf.Before(snaps[j-1].AsOf); j-- {
			snaps[j], snaps[j-1] = snaps[j-1], snaps[j]
		}
	}
	return snaps
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
func (a *App) DeleteAccount(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	ds, err := a.store.Snapshot()
	if err != nil {
		return fmt.Errorf("appstate: snapshot before deleting account %q: %w", id, err)
	}
	accounts := ds.Accounts[:0]
	found := false
	for _, account := range ds.Accounts {
		if account.ID == id {
			found = true
			continue
		}
		accounts = append(accounts, account)
	}
	if found {
		for i := range ds.Members {
			if ds.Members[i].Prefs.DefaultAccountID == id {
				ds.Members[i].Prefs.DefaultAccountID = ""
			}
		}
		if ds.Settings.DefaultAccountID == id {
			ds.Settings.DefaultAccountID = ""
		}
		ds.Accounts = accounts
		links := ds.DebtPaymentLinks[:0]
		for _, link := range ds.DebtPaymentLinks {
			if link.DebtAccountID != id {
				links = append(links, link)
			}
		}
		ds.DebtPaymentLinks = links
		rules := ds.DebtPayeeRules[:0]
		for _, rule := range ds.DebtPayeeRules {
			if rule.DebtAccountID != id && rule.SourceAccountID != id {
				rules = append(rules, rule)
			}
		}
		ds.DebtPayeeRules = rules
		// Load replaces the account and its saved defaults inside one SQLite
		// transaction, so references and debt-payment data disappear atomically.
		if err := a.store.Load(ds); err != nil {
			return fmt.Errorf("appstate: delete account %q: %w", id, err)
		}
	}
	a.log.Info("entity deleted", "entity", "account", "id", id, "existed", found)
	return nil
}

func (a *App) PutCategory(c domain.Category) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if is := validate.ValidateCategory(c); !is.OK() {
		return is
	}
	// C495: parent existence, kind agreement and cycles need the whole set, so
	// they are checked at the write seam rather than in the context-free rules.
	if is := validate.ValidateCategoryInTree(c, a.Categories()); !is.OK() {
		return is
	}
	if err := a.store.PutCategory(c); err != nil {
		return err
	}
	a.log.Info("category saved", "id", c.ID)
	return nil
}

// DeleteCategory removes a category, first re-homing any sub-categories that
// pointed to it onto its own parent (the grandparent, or the root for a
// top-level category) so deleting a parent never leaves orphaned children with a
// dangling ParentID (L28).
func (a *App) DeleteCategory(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	refs, err := a.CategoryDeleteReferences(id)
	if err != nil {
		return fmt.Errorf("appstate: check category references before deleting %q: %w", id, err)
	}
	if refs.Total() != 0 {
		return fmt.Errorf("appstate: category %q still has %d live reference(s); reassign them before deleting", id, refs.Total())
	}
	for _, child := range categorytree.ReparentOnDelete(a.Categories(), id) {
		if err := a.store.PutCategory(child); err != nil {
			return fmt.Errorf("appstate: re-home child %q before deleting %q: %w", child.ID, id, err)
		}
	}
	return a.del("category", id, a.store.DeleteCategory)
}

// PutRule saves an auto-categorization rule. A rule needs an ID, a target
// category, and a way to match: a non-empty match phrase OR at least one
// structured condition (C105 — conditions override the phrase at evaluation
// time, so a pure-conditions rule is legitimate).
func (a *App) PutRule(r rules.Rule) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if r.ID == "" {
		return fmt.Errorf("appstate: rule needs an id")
	}
	if strings.TrimSpace(r.Match) == "" && len(r.Conditions) == 0 {
		return fmt.Errorf("appstate: rule needs a match phrase or a condition")
	}
	// A rule must DO something. Category is the common action, but a rule that only
	// links a bill account (or renames, or tags) is valid too.
	if r.SetCategoryID == "" && r.SetBillAccountID == "" && strings.TrimSpace(r.RenameDesc) == "" && len(r.SetTags) == 0 {
		return fmt.Errorf("appstate: rule needs at least one action")
	}
	if err := a.store.PutRule(r); err != nil {
		return err
	}
	a.log.Info("rule saved", "id", r.ID)
	return nil
}

// DeleteRule removes an auto-categorization rule.
func (a *App) DeleteRule(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("rule", id, a.store.DeleteRule)
}

// ReorderRules sets each rule's precedence (Order) from its position in
// orderedIDs (index 0 = highest precedence, runs first). Ids not present keep
// their relative order after the listed ones. Used by drag-to-reorder (C64).
func (a *App) ReorderRules(orderedIDs []string) error {
	rs := a.Rules()
	pos := make(map[string]int, len(orderedIDs))
	for i, id := range orderedIDs {
		pos[id] = i
	}
	next := len(orderedIDs)
	for i := range rs {
		if p, ok := pos[rs[i].ID]; ok {
			rs[i].Order = p
		} else {
			rs[i].Order = next
			next++
		}
		if err := a.store.PutRule(rs[i]); err != nil {
			return err
		}
	}
	a.log.Info("rules reordered", "count", len(rs))
	return nil
}

func (a *App) transactionAutoRules() []rules.Rule {
	userRules := a.Rules()
	categories := a.Categories()
	autoRules := make([]rules.Rule, 0, len(userRules)+len(categories))
	autoRules = append(autoRules, userRules...)
	for _, c := range categories {
		// Short labels are unsafe as substring rules: "Car" matches "Carl's Cafe".
		if len([]rune(strings.TrimSpace(c.Name))) < 4 {
			continue
		}
		autoRules = append(autoRules, rules.Rule{Match: c.Name, SetCategoryID: c.ID})
	}
	return autoRules
}

// SuggestTransactionFields applies the first matching auto-categorization rule
// to draft fields without overriding a manual category or tags. It uses the
// legacy (text-only) match path, so it is appropriate for the quick-add suggestion
// where no amount/account/date context is available yet.
func (a *App) SuggestTransactionFields(text, categoryID string, tags []string) (string, []string) {
	r := rules.FirstMatch(a.transactionAutoRules(), text)
	if r == nil {
		return categoryID, tags
	}
	if strings.TrimSpace(categoryID) == "" && r.SetCategoryID != "" {
		categoryID = r.SetCategoryID
	}
	if len(tags) == 0 && len(r.SetTags) > 0 {
		tags = append([]string(nil), r.SetTags...)
	}
	return categoryID, tags
}

// AutoCategorizeTransaction applies auto-categorization to a transaction using
// the full structured-condition path (C105), which evaluates field/op/value
// conditions when present and falls back to the legacy Match substring otherwise.
// Transfers are excluded from categorization.
func primaryTransactionRule(r rules.Rule) bool {
	return r.SetCategoryID != "" || len(r.SetTags) > 0 || r.RenameDesc != "" || r.SetMemberID != "" || r.SetReviewed || r.SetExcludeFromReports
}

func ruleOwnerForApplyOnce(t domain.Transaction) string {
	owners := t.Owners()
	if len(owners) != 1 {
		return ""
	}
	return owners[0]
}

func (a *App) AutoCategorizeTransaction(t domain.Transaction) domain.Transaction {
	if t.IsTransfer() {
		return t
	}
	rs := a.transactionAutoRules()
	td := rules.NewTxnDate(t.Date)
	// The bill-account action is applied INDEPENDENTLY of the primary (category) match:
	// the first matching rule that sets a bill account wins, so an auto-link rule isn't
	// shadowed by an earlier rule whose only job is a category/tag. Applied only when the
	// transaction has no bill link yet — a manual link is never overwritten.
	if t.BillAccountID == "" {
		if br := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td,
			func(r rules.Rule) bool { return r.SetBillAccountID != "" }); br != nil {
			t.BillAccountID = br.SetBillAccountID
		}
	}
	r := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td, func(rule rules.Rule) bool {
		return rule.SetCategoryID != "" || len(rule.SetTags) > 0 || rule.RenameDesc != "" || rule.SetMemberID != "" || rule.SetReviewed || rule.SetExcludeFromReports
	})
	if r == nil {
		return t
	}
	if strings.TrimSpace(t.CategoryID) == "" {
		if categoryRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td, func(rule rules.Rule) bool {
			return rule.SetCategoryID != ""
		}); categoryRule != nil {
			t.CategoryID = categoryRule.SetCategoryID
		}
	}
	if len(t.Tags) == 0 && len(r.SetTags) > 0 {
		t.Tags = append([]string(nil), r.SetTags...)
	}
	// C373: the apply-once actions (member, reviewed, exclude), evaluated by the
	// one shared predicate so entry-time and the two backfills cannot drift.
	if ap := r.ApplyOnce(ruleOwnerForApplyOnce(t), t.Reviewed, t.ExcludeFromReports); ap.Any() {
		if ap.MemberID != "" {
			t.SetOwners([]string{ap.MemberID}) // TODO(WHO2): rule assignment remains single-owner.
		}
		if ap.Reviewed {
			t.Reviewed = true
		}
		if ap.ExcludeFromReports {
			t.ExcludeFromReports = true
		}
	}
	// C102: the rename action fires on entry too — previously only the
	// "Apply to existing" backfill applied it, so a fresh transaction kept its
	// raw description while an identical historical one got cleaned.
	if r.RenameDesc != "" && t.Desc != r.RenameDesc {
		t.Desc = r.RenameDesc
	}
	return t
}

// DocumentImportResult reports the outcome of committing reviewed document rows.
type DocumentImportResult struct {
	Outcomes           []ImportRowOutcome
	Imported           int
	Skipped            int
	Failed             []store.CSVRowError
	PossibleDuplicates []store.CSVRowError
	DocumentID         string
}

// PutDocument saves an imported-document record (needs an ID).
func (a *App) PutDocument(d domain.Document) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if d.ID == "" {
		return fmt.Errorf("appstate: document needs an id")
	}
	if err := a.store.PutDocument(d); err != nil {
		return err
	}
	a.log.Info("document saved", "id", d.ID, "status", string(d.Status))
	return nil
}

// DeleteDocument removes an imported-document record.
func (a *App) DeleteDocument(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("document", id, a.store.DeleteDocument)
}

// ImportReviewedDocumentRows stages and atomically commits reviewed rows. The
// optional checkpoint ID is saved with history in the same transaction.
func (a *App) ImportReviewedDocumentRows(kind domain.DocumentKind, accountID string, rows []extract.Row, checkpointID ...string) (DocumentImportResult, error) {
	batch, err := a.Stage(kind, rows, accountID)
	if err != nil {
		return DocumentImportResult{}, err
	}
	result, err := a.Commit(batch, id.New(), firstCheckpoint(checkpointID))
	out := DocumentImportResult{Imported: result.Imported, Skipped: result.Duplicates,
		Failed: result.Failed, PossibleDuplicates: result.PossibleDuplicates, DocumentID: result.DocumentID, Outcomes: result.Outcomes}
	if err != nil && len(out.Failed) > 0 {
		return out, nil
	}
	return out, err
}

func extractStatusCleared(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "true", "yes", "y", "cleared", "posted", "complete", "completed":
		return true
	}
	return false
}

func (a *App) categoryNameByID(categoryID string) string {
	for _, category := range a.Categories() {
		if category.ID == categoryID {
			return category.Name
		}
	}
	return ""
}
func (a *App) categoryIDForDocumentRow(category string) string {
	aiCat := strings.ToLower(strings.TrimSpace(category))
	if aiCat == "" {
		return ""
	}
	for _, c := range a.Categories() {
		cn := strings.ToLower(c.Name)
		if aiCat == strings.TrimSpace(cn) || aiCat == strings.ToLower(c.ID) {
			return c.ID
		}
	}
	return ""
}

func documentRowsFromExtract(rows []extract.Row) []domain.DocumentRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.DocumentRow, len(rows))
	for i, r := range rows {
		out[i] = domain.DocumentRow{Date: r.Date, Description: r.Description, Amount: r.Amount, Category: r.Category}
	}
	return out
}

// PutSavedInsight pins an AI insight (needs an ID and non-empty text).
func (a *App) PutSavedInsight(si domain.SavedInsight) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if si.ID == "" {
		return fmt.Errorf("appstate: saved insight needs an id")
	}
	if strings.TrimSpace(si.Text) == "" {
		return fmt.Errorf("appstate: saved insight needs text")
	}
	if err := a.store.PutSavedInsight(si); err != nil {
		return err
	}
	a.log.Info("insight pinned", "id", si.ID)
	return nil
}

// DeleteSavedInsight removes a pinned AI insight.
func (a *App) DeleteSavedInsight(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("saved insight", id, a.store.DeleteSavedInsight)
}

// Conversations returns every saved Insights chat (unordered; the caller sorts).
func (a *App) Conversations() []domain.Conversation {
	v, err := a.store.ListConversations()
	if err != nil {
		a.log.Error("list conversations", "err", err)
		return nil
	}
	return v
}

// PutConversation saves (inserts or replaces) an Insights conversation. It needs
// an ID; a blank title falls back to a generic label.
func (a *App) PutConversation(c domain.Conversation) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if c.ID == "" {
		return fmt.Errorf("appstate: conversation needs an id")
	}
	if strings.TrimSpace(c.Title) == "" {
		c.Title = "Untitled chat"
	}
	if err := a.store.PutConversation(c); err != nil {
		return err
	}
	return nil
}

// DeleteConversation removes a saved Insights chat.
func (a *App) DeleteConversation(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("conversation", id, a.store.DeleteConversation)
}

// PutRecurring saves a recurring cash flow. It needs an ID, a label, a currency
// on the amount, and a cadence.
func (a *App) PutRecurring(r domain.Recurring) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if r.ID == "" {
		return fmt.Errorf("appstate: recurring needs an id")
	}
	if strings.TrimSpace(r.Label) == "" {
		return fmt.Errorf("appstate: recurring needs a label")
	}
	if r.Amount.Currency == "" {
		return fmt.Errorf("appstate: recurring needs an amount currency")
	}
	if r.Cadence == "" {
		return fmt.Errorf("appstate: recurring needs a cadence")
	}
	if err := a.store.PutRecurring(r); err != nil {
		return err
	}
	a.log.Info("recurring saved", "id", r.ID)
	// XC3: keep the system-managed sinking-fund goal in sync with the smoothing flag
	// (create/maintain when smoothing is on, dissolve when it is off).
	if err := a.syncSmoothingGoal(r); err != nil {
		a.log.Warn("smoothing goal sync failed", "recurring", r.ID, "err", err)
	}
	return nil
}

// RecordBillPayment logs a payment for an upcoming bill (C57), dated today. For a
// liability-account bill (accountID is a real account), it posts a positive
// transaction that reduces the owed balance. For a recurring bill (accountID is
// "recurring:<id>"), it posts the recurring's amount to its account/category and
// advances the recurring's NextDue so it's no longer shown as due.
func (a *App) RecordBillPayment(accountID, name string, amount money.Money) error {
	now := a.clock()
	if rid, ok := strings.CutPrefix(accountID, "recurring:"); ok {
		for _, r := range a.Recurring() {
			if r.ID != rid {
				continue
			}
			if r.AccountID == "" {
				return fmt.Errorf("appstate: recurring %q has no account to post to", r.Label)
			}
			posted, err := a.postRecurringOccurrence(r, now)
			if err == nil && posted {
				a.fireTxnMutated()
			}
			return err
		}
		return fmt.Errorf("appstate: recurring bill not found")
	}
	// A bill payment posted straight to a liability account must reduce the debt
	// under that account's at-rest sign convention (see liabilityPaymentMinor) —
	// the raw bill amount only did so for negative-stored debts.
	for _, ac := range a.Accounts() {
		if ac.ID == accountID && ac.Class == domain.ClassLiability {
			amount.Amount = liabilityPaymentMinor(ac, a.Transactions(), amount.Amount)
			break
		}
	}
	t := domain.Transaction{
		ID: id.New(), AccountID: accountID, Amount: amount, Date: now,
		Payee: name, Desc: "Bill payment: " + name,
		Source: domain.TxnSourceRecurring,
	}
	return a.PutTransaction(t)
}

// RecurringPaymentSuggestions returns review-only suggestions for expense rows
// that exactly match a recurring payment to a liability account. It never
// changes transaction history.
func (a *App) RecurringPaymentSuggestions() []txnclassify.PaymentSuggestion {
	return txnclassify.FindRecurringPaymentSuggestions(a.Transactions(), a.Accounts(), a.Recurring())
}

// DeleteRecurring removes a recurring cash flow.
func (a *App) DeleteRecurring(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	// XC3: dissolve the system-managed sinking fund (releasing its earmarks) before
	// the recurring itself is removed.
	if err := a.dissolveSmoothingGoal(id); err != nil {
		a.log.Warn("smoothing goal dissolve failed", "recurring", id, "err", err)
	}
	// XC5→XC8: deleting a crept recurring satisfies the "cancel" intent, so
	// auto-resolve any price-creep task watching it.
	a.resolvePriceCreepTasksForDeletedRecurring(id)
	return a.del("recurring", id, a.store.DeleteRecurring)
}

// PutAllocProfile saves a capital-allocation weight profile (needs an ID and a name).
func (a *App) PutAllocProfile(p domain.AllocationProfile) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if p.ID == "" {
		return fmt.Errorf("appstate: allocation profile needs an id")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("appstate: allocation profile needs a name")
	}
	if err := a.store.PutAllocProfile(p); err != nil {
		return err
	}
	a.log.Info("allocation profile saved", "id", p.ID)
	return nil
}

// DeleteAllocProfile removes a saved allocation profile.
func (a *App) DeleteAllocProfile(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("alloc profile", id, a.store.DeleteAllocProfile)
}

// PutFormula saves a custom formula (needs an ID, a name, and an expression).
func (a *App) PutFormula(f domain.Formula) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if f.ID == "" {
		return fmt.Errorf("appstate: formula needs an id")
	}
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("appstate: formula needs a name")
	}
	if strings.TrimSpace(f.Expr) == "" {
		return fmt.Errorf("appstate: formula needs an expression")
	}
	if err := a.store.PutFormula(f); err != nil {
		return err
	}
	a.log.Info("formula saved", "id", f.ID)
	return nil
}

// DeleteFormula removes a saved formula.
func (a *App) DeleteFormula(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("formula", id, a.store.DeleteFormula)
}

// PutPlan saves a what-if plan (needs an ID, a name, and a positive horizon).
func (a *App) PutPlan(p domain.Plan) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if p.ID == "" {
		return fmt.Errorf("appstate: plan needs an id")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("appstate: plan needs a name")
	}
	if p.HorizonMonths <= 0 {
		return fmt.Errorf("appstate: plan needs a positive horizon")
	}
	if err := a.store.PutPlan(p); err != nil {
		return err
	}
	a.log.Info("plan saved", "id", p.ID)
	return nil
}

// DeletePlan removes a saved plan.
func (a *App) DeletePlan(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("plan", id, a.store.DeletePlan)
}

// PutCustomPage saves a user-authored page (needs an ID, a name, and a slug).
func (a *App) PutCustomPage(p domain.CustomPage) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if p.ID == "" {
		return fmt.Errorf("appstate: custom page needs an id")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("appstate: custom page needs a name")
	}
	if strings.TrimSpace(p.Slug) == "" {
		return fmt.Errorf("appstate: custom page needs a slug")
	}
	if err := a.store.PutCustomPage(p); err != nil {
		return err
	}
	a.log.Info("custom page saved", "id", p.ID, "slug", p.Slug)
	return nil
}

// PutPlacements upserts a surface's widget placements (the unified-widget layout
// representation). It is UI layout state — like the layout atoms — so it carries
// no role guard; a viewer arranging their own view persists normally.
func (a *App) PutPlacements(ps []domain.Placement) error {
	for _, p := range ps {
		if err := a.store.PutPlacement(p); err != nil {
			return err
		}
	}
	return nil
}

// Placements returns the persisted widget placements for a surface (e.g.
// "dashboard"), the engine's canonical placement representation.
func (a *App) Placements(surface string) []domain.Placement {
	v, err := a.store.PlacementsForSurface(surface)
	a.logErr("placements", err)
	return v
}

// Molecules returns the active compound-variable definitions: the engine defaults
// (net_worth, safe_to_spend, …) with any persisted in the dataset overriding by
// name or adding new ones. So a user's edit to a formula like net_worth is stored
// in the DB and travels with export, while built-ins remain the seed.
func (a *App) Molecules() []domain.Molecule {
	out := engineenv.DefaultMolecules()
	persisted, err := a.store.ListMolecules()
	a.logErr("molecules", err)
	if len(persisted) == 0 {
		return out
	}
	idx := map[string]int{}
	for i, m := range out {
		idx[m.Name] = i
	}
	for _, p := range persisted {
		if i, ok := idx[p.Name]; ok {
			out[i] = p
		} else {
			idx[p.Name] = len(out)
			out = append(out, p)
		}
	}
	return out
}

// reservedVarNames are the built-in variable names a molecule may not take:
// the full empty-data engine surface (atoms, alloc/planning/recurring/bills/
// smart vars, health & credit factors) minus the default molecules, which ARE
// override-by-name by design. Evaluation has its own atom-shadow guard; this
// check exists so the save path can say no with a reason instead of silently
// losing the fight later.
var reservedVarNames = func() map[string]bool {
	out := map[string]bool{}
	for name := range engineenv.Vars(engineenv.Data{Now: time.Unix(0, 0).UTC()}) {
		out[name] = true
	}
	for _, m := range engineenv.DefaultMolecules() {
		delete(out, m.Name)
	}
	return out
}()

// PutMolecule persists a compound-variable definition (a name + formula over
// the engine atoms). Validated before saving: the formula must parse, the name
// may not shadow a built-in non-molecule variable, and the molecule may not
// reference itself or close a reference cycle with other molecules — a cycle
// never evaluates, so every member would silently read as missing downstream.
func (a *App) PutMolecule(m domain.Molecule) error {
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Formula) == "" {
		return fmt.Errorf("appstate: molecule needs a name and a formula")
	}
	refs, err := formula.References(m.Formula)
	if err != nil {
		return fmt.Errorf("appstate: molecule %q has an invalid formula: %w", m.Name, err)
	}
	if reservedVarNames[m.Name] {
		return fmt.Errorf("appstate: %q is a built-in variable — pick another name", m.Name)
	}
	for _, r := range refs {
		if r == m.Name {
			return fmt.Errorf("appstate: molecule %q references itself", m.Name)
		}
	}
	if cycle := moleculeCycle(a.Molecules(), m); len(cycle) > 0 {
		return fmt.Errorf("appstate: molecule %q would create a reference cycle (%s)", m.Name, strings.Join(cycle, " → "))
	}
	return a.store.PutMolecule(m)
}

// moleculeCycle reports the reference cycle the candidate molecule would
// create, as the name path back to itself — or nil. existing is the current
// set; the candidate replaces its own entry by name. Only molecule names can
// extend the walk (atoms and unknown references terminate it).
func moleculeCycle(existing []domain.Molecule, candidate domain.Molecule) []string {
	refsOf := make(map[string][]string, len(existing)+1)
	for _, m := range existing {
		if m.Name == candidate.Name {
			continue
		}
		if r, err := formula.References(m.Formula); err == nil {
			refsOf[m.Name] = r
		}
	}
	cr, err := formula.References(candidate.Formula)
	if err != nil {
		return nil
	}
	refsOf[candidate.Name] = cr

	seen := map[string]bool{candidate.Name: true}
	var walk func(from string, trail []string) []string
	walk = func(from string, trail []string) []string {
		for _, r := range refsOf[from] {
			if r == candidate.Name {
				return append(append([]string(nil), trail...), r)
			}
			if seen[r] {
				continue
			}
			seen[r] = true
			if _, isMol := refsOf[r]; !isMol {
				continue
			}
			if c := walk(r, append(append([]string(nil), trail...), r)); c != nil {
				return c
			}
		}
		return nil
	}
	return walk(candidate.Name, []string{candidate.Name})
}

// DeleteMolecule removes a persisted compound-variable row by name. For an
// overridden built-in this RESTORES the default definition (Molecules layers
// DefaultMolecules under the persisted set); for a user-created molecule it
// removes it entirely — unless another molecule still references it, which is
// an error naming the dependents, since deleting would silently zero them out
// downstream. Returns whether a persisted row existed.
func (a *App) DeleteMolecule(name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("appstate: molecule name is required")
	}
	// An overridden built-in keeps existing after delete (the default returns),
	// so only a fully-custom molecule can leave dependents dangling.
	isBuiltIn := false
	for _, m := range engineenv.DefaultMolecules() {
		if m.Name == name {
			isBuiltIn = true
			break
		}
	}
	if !isBuiltIn {
		var dependents []string
		for _, m := range a.Molecules() {
			if m.Name == name {
				continue
			}
			refs, err := formula.References(m.Formula)
			if err != nil {
				continue
			}
			for _, r := range refs {
				if r == name {
					dependents = append(dependents, m.Name)
					break
				}
			}
		}
		if len(dependents) > 0 {
			return false, fmt.Errorf("appstate: %q is still used by %s — update or delete those formulas first", name, strings.Join(dependents, ", "))
		}
	}
	return a.store.DeleteMolecule(name)
}

// DeleteCustomPage removes a user-authored page.
func (a *App) DeleteCustomPage(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("customPage", id, a.store.DeleteCustomPage)
}

// PutArtifact saves a user artifact (needs an ID, a name, and a kind).
func (a *App) PutArtifact(art domain.Artifact) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if art.ID == "" {
		return fmt.Errorf("appstate: artifact needs an id")
	}
	if strings.TrimSpace(art.Name) == "" {
		return fmt.Errorf("appstate: artifact needs a name")
	}
	if art.Kind == "" {
		return fmt.Errorf("appstate: artifact needs a kind")
	}
	if err := a.store.PutArtifact(art); err != nil {
		return err
	}
	a.log.Info("artifact saved", "id", art.ID, "kind", art.Kind, "size", art.Size)
	return nil
}

// DeleteArtifact removes a user artifact.
func (a *App) DeleteArtifact(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("artifact", id, a.store.DeleteArtifact)
}

// PutWorkflow saves a user automation (needs an ID and a name).
func (a *App) PutWorkflow(w workflow.Workflow) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if w.ID == "" {
		return fmt.Errorf("appstate: workflow needs an id")
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("appstate: workflow needs a name")
	}
	if errs := workflow.Validate(w); len(errs) > 0 {
		return fmt.Errorf("appstate: invalid workflow: %s", errs[0])
	}
	if err := a.store.PutWorkflow(w); err != nil {
		return err
	}
	a.log.Info("workflow saved", "id", w.ID)
	return nil
}

// DeleteWorkflow removes a workflow.
func (a *App) DeleteWorkflow(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("workflow", id, a.store.DeleteWorkflow)
}

// engineVars builds the workflow condition variable surface from the current
// dataset: the full engine surface the app can compute without the UI layer —
// atoms, the user's molecules (persisted edits and custom compound variables
// included), cf_* custom-field values, and the recurring + category inputs
// that bills_due / safe_to_spend / the health_* factors need. This is the same
// surface widgets read, so a condition like "safe_to_spend < 0" or
// "cf_txn_reimbursable > 0" means the same thing everywhere.
func (a *App) engineVars() map[string]float64 {
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return engineenv.Vars(engineenv.Data{
		Ledger:   budgeting.NewNettedLedger(a.Transactions(), a.TxnLinks()),
		Accounts: a.Accounts(), Transactions: a.Transactions(), Members: a.Members(),
		Budgets: a.Budgets(), Goals: a.Goals(), Tasks: a.Tasks(),
		Recurring: a.Recurring(), Categories: a.Categories(),
		CustomDefs: a.CustomFieldDefs(), Molecules: a.Molecules(),
		Rates: currency.Rates{Base: base, Rates: a.Settings().FXRates}, Now: a.clock(),
	})
}

// WorkflowVariableNames lists the sorted discovery union across all triggers.
// Use WorkflowVariableNamesFor when validating a specific trigger's condition;
// transaction-only names in this union are unavailable to other triggers.
func (a *App) WorkflowVariableNames() []string {
	return a.WorkflowVariableNamesFor(workflow.TriggerTxnAdded)
}

// txnContext builds the workflow context for a transaction-triggered run: the
// engine variable surface plus the triggering transaction's own fields, so a
// condition can reference txn_amount/txn_abs (major units) and txn_payee/txn_desc/
// txn_category/txn_account/txn_tags, and transaction-mutating actions know which
// transaction to change.
func (a *App) txnContext(t domain.Transaction) workflow.Context {
	ctx := workflow.Context{Vars: a.engineVars(), Strs: map[string]string{"txn_account": "", "txn_category": ""}, TxnID: t.ID}
	div := 1.0
	for i := 0; i < currency.Decimals(t.Amount.Currency); i++ {
		div *= 10
	}
	amt := float64(t.Amount.Amount) / div
	ctx.Vars["txn_amount"] = amt
	ctx.Vars["txn_abs"] = math.Abs(amt)
	// What this merchant normally charges, and how this charge compares (C405).
	// Without these, a condition can only ever test a fixed dollar threshold,
	// which cannot express "my subscription went up" — the number that matters is
	// different for every merchant. txn_vs_typical is a multiple: 1.0 is business
	// as usual, 1.2 is twenty percent more than normal.
	//
	// Both read 0 when the merchant has too little history to have a normal. That
	// is deliberate and it is why the composer's help says to pair the ratio with
	// a floor: `txn_vs_typical > 1.15` is false at 0, so a first-ever charge never
	// fires a price-hike workflow.
	if typical, ok := payeebase.Typical(a.Transactions(), t); ok {
		ctx.Vars["txn_payee_typical"] = float64(typical) / div
		ctx.Vars["txn_vs_typical"] = math.Abs(float64(t.Amount.Amount)) / float64(typical)
	} else {
		ctx.Vars["txn_payee_typical"] = 0
		ctx.Vars["txn_vs_typical"] = 0
	}
	ctx.Strs["txn_payee"] = t.Payee
	ctx.Strs["txn_desc"] = t.Desc
	ctx.Strs["txn_tags"] = strings.Join(t.Tags, ",")
	for _, ac := range a.Accounts() {
		if ac.ID == t.AccountID {
			ctx.Strs["txn_account"] = ac.Name
			break
		}
	}
	for _, c := range a.Categories() {
		if c.ID == t.CategoryID {
			ctx.Strs["txn_category"] = c.Name
			break
		}
	}
	// The triggering transaction's OWN custom-field values override the
	// period-wide cf_txn_* sums the engine surface carries (those answer "the
	// household total this period", not "this transaction"): numbers as-is,
	// yes/no as 1/0, text and choices into Strs so contains(cf_txn_project,
	// "x") works. A field the transaction doesn't carry reads 0 / "".
	for _, def := range a.CustomFieldDefsFor("transaction") {
		name := "cf_txn_" + def.Key
		switch def.Type {
		case customfields.TypeNumber:
			ctx.Vars[name] = 0
			switch n := t.Custom[def.Key].(type) {
			case float64:
				ctx.Vars[name] = n
			case int:
				ctx.Vars[name] = float64(n)
			}
		case customfields.TypeBool:
			ctx.Vars[name] = 0
			if b, ok := t.Custom[def.Key].(bool); ok && b {
				ctx.Vars[name] = 1
			}
		default: // text / date / choice
			s, _ := t.Custom[def.Key].(string)
			ctx.Strs[name] = s
		}
	}
	return ctx
}

// RunWorkflow plans a workflow against the current figures and, unless dryRun,
// applies its effects (creating tasks, applying rules, recording notices) and
// saves a Run to the audit history. It returns the Run so the UI can show what
// happened (or would happen). Planning is the pure engine; this is the only place
// the effects actually change state, keeping runs explainable and dry-runnable.
func (a *App) RunWorkflow(w workflow.Workflow, dryRun bool) (workflow.Run, error) {
	return a.runWorkflow(w, workflow.Context{Vars: a.engineVars()}, dryRun)
}

// RunWorkflowOn runs a workflow in the context of a specific transaction, so its
// condition and transaction-mutating actions see that transaction.
func (a *App) RunWorkflowOn(w workflow.Workflow, t domain.Transaction, dryRun bool) (workflow.Run, error) {
	return a.runWorkflow(w, a.txnContext(t), dryRun)
}

func (a *App) runWorkflow(w workflow.Workflow, ctx workflow.Context, dryRun bool) (workflow.Run, error) {
	if errs := workflow.Validate(w); len(errs) > 0 {
		return workflow.Run{}, fmt.Errorf("appstate: invalid workflow: %s", errs[0])
	}
	effects, matched, err := workflow.Plan(w, ctx)
	if err != nil {
		return workflow.Run{}, err
	}
	// Transfer effects: resolve the DedupeKey's {period} placeholder to the
	// CURRENT period (ISO week for weekly cadence, calendar month otherwise) —
	// and re-stamp legacy keys frozen at creation, which matched their own
	// first run forever and silently blocked every later transfer. Also
	// rewrite the engine's raw "N minor units from acc-id" summary into money
	// + account names for the dry-run preview and the run log.
	for i := range effects {
		if effects[i].Kind != workflow.ActionTransfer {
			continue
		}
		if effects[i].DedupeKey != "" {
			effects[i].DedupeKey = stampDedupePeriod(effects[i].DedupeKey, w.Trigger.Cadence, a.clock())
		}
		effects[i].Summary = a.transferSummary(effects[i])
	}
	run := workflow.Run{
		ID: id.New(), WorkflowID: w.ID, At: a.clock().Format(time.RFC3339),
		DryRun: dryRun, Matched: matched, Effects: effects,
	}
	if !dryRun && matched {
		// Suspend triggers while applying so an action that writes data can't
		// recursively re-fire workflows.
		prev := a.triggersSuspended
		a.triggersSuspended = true
		for _, e := range effects {
			a.applyEffect(e)
		}
		a.triggersSuspended = prev
		if err := a.store.PutWorkflowRun(run); err != nil {
			a.logErr("workflowRun", err)
		}
	}
	return run, nil
}

// applyEffect performs one planned effect. Most effects are write-safe and never
// create transactions. ActionTransfer is a sanctioned exception: it is loop-safe
// because the apply path runs inside triggersSuspended=true (so the legs don't
// re-fire RunTriggered) and ValidateTransferAction prevents its use on
// TriggerTxnAdded at save time.
func (a *App) applyEffect(e workflow.Effect) {
	switch e.Kind {
	case workflow.ActionCreateTask:
		// Idempotent: don't pile up duplicate open tasks with the same title when a
		// txn-added workflow fires repeatedly (e.g. across many adds in a month).
		for _, tk := range a.Tasks() {
			if tk.Status == domain.StatusOpen && tk.Title == e.Title {
				return
			}
		}
		task := domain.Task{
			ID: id.New(), Title: e.Title, Notes: e.Notes,
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceManual,
		}
		// XC8: a workflow-created task may carry a self-resolve condition so it
		// auto-completes when the underlying situation resolves.
		if e.ResolveCondition != "" {
			task.Resolve = &domain.TaskResolve{Condition: e.ResolveCondition}
		}
		_ = a.PutTask(task)
	case workflow.ActionApplyRules:
		if _, err := a.ApplyRules(); err != nil {
			a.logErr("workflowApplyRules", err)
		}
	case workflow.ActionSetCategory:
		a.mutateTxn(e.TxnID, func(t *domain.Transaction) { t.CategoryID = e.CategoryID })
	case workflow.ActionAddTag:
		a.mutateTxn(e.TxnID, func(t *domain.Transaction) { t.Tags = addTagUnique(t.Tags, e.Tag) })
	case workflow.ActionFlagReview:
		// Skip the auto review-tag when the user explicitly confirmed the entry
		// (L43): a confident manual add shouldn't be re-flagged for review.
		a.mutateTxn(e.TxnID, func(t *domain.Transaction) {
			if t.Reviewed {
				return
			}
			t.Tags = addTagUnique(t.Tags, e.Tag)
		})
	case workflow.ActionNotify:
		a.log.Info("workflow notice", "message", e.Message)
		if a.Notifier != nil {
			a.Notifier(e.Message)
		}
	case workflow.ActionPostRecurring:
		if _, err := a.PostDueRecurring(a.clock()); err != nil {
			a.logErr("workflowPostRecurring", err)
		}
	case workflow.ActionFlagBudgetOver:
		a.applyFlagBudgetOver()
	case workflow.ActionAgentRun:
		// AG5: the scheduled question lands as a conversation to read, never as an
		// applied change — see agentrun_ops.go for why.
		a.RunScheduledAgentPrompt(e.Prompt)
	case workflow.ActionTransfer:
		// DedupeKey guard: if a prior run already carried this DedupeKey for any
		// effect, the period transfer has already executed — skip to prevent double
		// movement. An empty DedupeKey means the caller didn't set one; allow it
		// (manual / one-shot workflows may omit it intentionally).
		if e.DedupeKey != "" {
			for _, run := range a.WorkflowRuns() {
				for _, prior := range run.Effects {
					if prior.Kind == workflow.ActionTransfer && prior.DedupeKey == e.DedupeKey {
						a.log.Info("workflow transfer skipped (already executed this period)", "dedupeKey", e.DedupeKey)
						return
					}
				}
			}
		}
		_, _, err := a.CreateTransferPair(TransferParams{
			FromAccountID: e.TransferFromAccountID,
			ToAccountID:   e.TransferToAccountID,
			AmountMinor:   e.TransferAmount,
			Desc:          "Automated transfer",
		})
		if err != nil {
			a.logErr("workflowTransfer", err)
		}
	}
}

// mutateTxn loads the transaction with id, applies fn, and saves it via the
// store (below the trigger layer, so a transaction-mutating action can't re-fire
// the txn-added trigger). A no-op when id is empty or the transaction is gone.
func (a *App) mutateTxn(id string, fn func(*domain.Transaction)) {
	if id == "" {
		return
	}
	t, ok, err := a.store.GetTransaction(id)
	if err != nil || !ok {
		return
	}
	fn(&t)
	if err := a.store.PutTransaction(t); err != nil {
		a.logErr("workflowMutateTxn", err)
	}
}

// addTagUnique appends tag to tags if not already present (case-sensitive).
func addTagUnique(tags []string, tag string) []string {
	if strings.TrimSpace(tag) == "" {
		return tags
	}
	for _, x := range tags {
		if x == tag {
			return tags
		}
	}
	return append(append([]string(nil), tags...), tag)
}

// clock returns the current time via the injectable seam (a.now), defaulting to
// time.Now when unset (e.g. an App built without New).
func (a *App) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

// RunTriggered runs every enabled workflow whose trigger matches the given event.
// When the event concerns a specific transaction (txn-added), pass it so the
// workflow's condition and transaction-mutating actions see it; pass nil for
// aggregate/bulk events (the workflow then runs against the dataset-wide figures
// only, and transaction-mutating actions no-op).
func (a *App) RunTriggered(event workflow.TriggerKind, t *domain.Transaction) {
	for _, w := range a.Workflows() {
		if !w.Enabled || !workflow.Match(w.Trigger, event) {
			continue
		}
		var err error
		if t != nil {
			_, err = a.RunWorkflowOn(w, *t, false)
		} else {
			_, err = a.RunWorkflow(w, false)
		}
		if err != nil {
			a.logErr("workflowTriggered", err)
		}
	}
}

// DatasetBytes reports the serialized size of the whole dataset in bytes — what
// gets written to browser storage. The UI uses it for a storage meter/warning,
// since large artifacts inflate the single autosaved blob toward the localStorage
// quota. Returns 0 on a marshal error (logged).
func (a *App) DatasetBytes() int {
	b, err := a.ExportJSON()
	if err != nil {
		a.logErr("datasetBytes", err)
		return 0
	}
	return len(b)
}

// PostDueRecurring posts a transaction for each autopost recurring whose NextDue
// is on or before asOf, advancing NextDue past asOf — catching up any missed
// periods (bounded). A recurring needs an account to post into; ones without one,
// or without autopost, are skipped. Returns the number of posted occurrences.
// Each occurrence commits its receipt, ledger rows and schedule advance together.
// Failed occurrences remain due; independent recurring items continue posting.
func (a *App) PostDueRecurring(asOf time.Time) (int, error) {
	if err := a.roleGuard(); err != nil {
		return 0, err
	}
	posted := 0
	var failures []error
	through := dateutil.CalendarDayUTC(asOf)
	for _, r := range a.Recurring() {
		if !r.Active() || !r.Autopost || r.AccountID == "" {
			continue
		}
		for guard := 0; !dateutil.CalendarDayUTC(r.NextDue).After(through) && guard < 600; guard++ {
			created, err := a.postRecurringOccurrence(r, r.NextDue)
			if err != nil {
				failure := fmt.Errorf("auto-post recurring %q: %w", r.Label, err)
				failures = append(failures, failure)
				a.log.Error("recurring occurrence was not posted", "recurring", r.ID, "due", dateutil.FormatDate(r.NextDue), "err", err)
				if a.Notifier != nil {
					a.Notifier("Could not post " + r.Label + ": " + err.Error())
				}
				break
			}
			if created {
				posted++
			}
			r = r.Advance()
		}
	}
	if posted > 0 {
		a.fireTxnMutated()
		a.log.Info("posted due recurring", "count", posted)
	}
	return posted, errors.Join(failures...)
}

// postRecurringOccurrence shares one durable identity between manual bill
// payment and automatic posting. Calendar dates, rather than clock instants,
// identify scheduled occurrences and determine when they become due.
func (a *App) postRecurringOccurrence(r domain.Recurring, when time.Time) (bool, error) {
	if err := a.roleGuard(); err != nil {
		return false, err
	}
	if !r.Active() {
		return false, fmt.Errorf("this recurring item is paused")
	}
	if r.NextDue.IsZero() {
		return false, fmt.Errorf("the next due date is missing")
	}
	if _, err := r.Amount.AbsChecked(); err != nil {
		return false, fmt.Errorf("recurring amount: %w", err)
	}
	operationID := fmt.Sprintf("recurring:%q:%s", r.ID, dateutil.FormatDate(r.NextDue))
	when = dateutil.CalendarDayUTC(when)
	txns := []domain.Transaction{{ID: transferID(operationID, "transaction"), AccountID: r.AccountID,
		CategoryID: r.CategoryID, Date: when, Amount: r.Amount, Desc: r.Label, Source: domain.TxnSourceRecurring}}
	accounts := a.Accounts()
	if destination, ok := txnclassify.RecurringLiabilityDestination(r, accounts); ok {
		magnitude, err := r.Amount.AbsChecked()
		if err != nil {
			return false, fmt.Errorf("recurring payment amount: %w", err)
		}
		from, to, err := a.transferAccounts(r.AccountID, destination.ID)
		if err != nil {
			return false, err
		}
		outAmount, inAmount, err := a.transferLegs(from, to, magnitude.Amount, 0)
		if err != nil {
			return false, err
		}
		group := transferID(operationID, "group")
		txns = []domain.Transaction{
			{ID: transferID(operationID, "out"), AccountID: from.ID, TransferAccountID: to.ID, TransferGroupID: group, Amount: outAmount, Date: when, Desc: r.Label, Payee: to.Name, Reviewed: true, Source: domain.TxnSourceRecurring},
			{ID: transferID(operationID, "in"), AccountID: to.ID, TransferAccountID: from.ID, TransferGroupID: group, Amount: inAmount, Date: when, Desc: r.Label, Payee: from.Name, Reviewed: true, Source: domain.TxnSourceRecurring},
		}
	}
	for _, txn := range txns {
		if _, ok := domain.AccountByID(accounts, txn.AccountID); !ok {
			return false, fmt.Errorf("choose an existing account")
		}
		if issues := validate.ValidateTransaction(txn, accounts...); !issues.OK() {
			return false, issues
		}
		if err := a.validateCustom("transaction", txn.Custom); err != nil {
			return false, err
		}
	}
	return a.store.PutRecurringOccurrenceAtomic(operationID, r, r.Advance(), txns)
}

// RunDueScheduledWorkflows finds every enabled scheduled workflow whose NextRun
// is on or before now, runs it, advances NextRun, and persists the updated
// trigger. Returns how many workflows ran. Errors on individual workflows are
// logged and skipped; a store write error aborts and is returned.
func (a *App) RunDueScheduledWorkflows(now time.Time) (int, error) {
	ran := 0
	for _, w := range a.Workflows() {
		if !w.Enabled || !workflow.IsScheduledWorkflowDue(w, now) {
			continue
		}
		if _, err := a.RunWorkflow(w, false); err != nil {
			a.logErr("scheduledWorkflow", err)
			continue
		}
		workflow.AdvanceScheduledNextRun(&w, now)
		if err := a.store.PutWorkflow(w); err != nil {
			return ran, err
		}
		ran++
	}
	if ran > 0 {
		a.log.Info("ran due scheduled workflows", "count", ran)
	}
	return ran, nil
}

// RunDueFundAccruals auto-credits each sinking-fund goal that is due for its
// monthly contribution as of now. It iterates all goals, calls
// goals.FundAccrualDue to determine eligibility and the credit amount, credits
// the goal's CurrentAmount (capped so it never exceeds TargetAmount), records
// the current UTC year-month in Goal.Custom["fundAccrualPeriod"] as the
// once-per-month guard, and persists the updated goal. Errors on individual
// goals are logged and skipped; a store write error aborts and is returned.
// Returns how many goals were credited.
func (a *App) RunDueFundAccruals(now time.Time) (int, error) {
	credited := 0
	periodKey := now.UTC().Format("2006-01")
	for _, g := range a.Goals() {
		due, amountMinor := goals.FundAccrualDue(g, now)
		if !due {
			continue
		}
		credit := money.New(amountMinor, g.CurrentAmount.Currency)
		updated, err := g.CurrentAmount.Add(credit)
		if err != nil {
			a.logErr("fundAccrual: add credit", err)
			continue
		}
		// Final cap: never exceed TargetAmount (FundAccrualDue already constrains
		// amountMinor, but guard here too in case of a currency mismatch edge case).
		if updated.Amount > g.TargetAmount.Amount && g.TargetAmount.Amount > 0 {
			updated = g.TargetAmount
		}
		g.CurrentAmount = updated
		if g.Custom == nil {
			g.Custom = make(map[string]any)
		}
		g.Custom["fundAccrualPeriod"] = periodKey
		if err := a.PutGoal(g); err != nil {
			return credited, fmt.Errorf("appstate: fund accrual put goal %q: %w", g.ID, err)
		}
		credited++
	}
	if credited > 0 {
		a.log.Info("auto-accrued sinking funds", "count", credited)
	}
	return credited, nil
}

// isBudgetOver returns true when the given budget is currently in the StateOver
// state according to the same evaluated carry/boost model as the budget cards.
func (a *App) isBudgetOver(b domain.Budget) bool {
	in, err := a.BudgetViewInput(a.clock())
	if err != nil {
		a.log.Error("evaluate budget flag", "budget", b.ID, "err", err)
		return false
	}
	v := budgeting.EvaluateViewBudget(b, in)
	if v.Error != nil {
		a.log.Error("evaluate budget flag", "budget", b.ID, "err", v.Error)
	}
	return v.Valid && v.Status.State == budgeting.StateOver
}

// applyFlagBudgetOver creates an open task for every budget that is currently
// over its limit, skipping any that already have an open task with the same
// title (deduplication).
func (a *App) applyFlagBudgetOver() {
	in, err := a.BudgetViewInput(a.clock())
	if err != nil {
		a.log.Error("evaluate budget flags", "err", err)
		return
	}
	existing := a.Tasks()
	for _, b := range a.Budgets() {
		v := budgeting.EvaluateViewBudget(b, in)
		if !v.Valid {
			a.log.Error("evaluate budget flag", "budget", b.ID, "err", v.Error)
			continue
		}
		st := v.Status
		if st.State != budgeting.StateOver {
			continue
		}
		titleKey, titleArgs := BudgetOverTaskTitle(b)

		title := "Budget over limit: " + b.Name
		if b.IsSaving() {
			title = "Savings target needs funding: " + b.Name
		}
		dup := false
		for _, tk := range existing {
			if tk.Status == domain.StatusOpen && (tk.RelatedID == b.ID && tk.RelatedType == domain.RelatedType("budget") || tk.RelatedID == "" && tk.Title == title) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		if err := a.PutTask(domain.Task{
			RelatedType: domain.RelatedType("budget"), RelatedID: b.ID,
			Custom:   map[string]any{"titleKey": titleKey, "titleArgs": titleArgs},
			ID:       id.New(),
			Title:    title,
			Notes:    fmt.Sprintf("Budget %q is at %d%% of its limit.", b.Name, st.Percent),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Source:   domain.SourceManual,
		}); err != nil {
			a.log.Error("create budget flag task", "budget", b.ID, "err", err)
		}
	}
}

// FireBillDueTrigger runs every enabled bill-due workflow if any recurring item
// is on or past its due date as of asOf. It is a no-op when triggers are
// suspended.
func (a *App) FireBillDueTrigger(asOf time.Time) {
	if a.triggersSuspended {
		return
	}
	for _, r := range a.Recurring() {
		if r.Active() && !r.NextDue.IsZero() && !dateutil.CalendarDayUTC(r.NextDue).After(dateutil.CalendarDayUTC(asOf)) {
			a.RunTriggered(workflow.TriggerBillDue, nil)
			return
		}
	}
}

// ApplyRulesWithCounts applies every saved categorization rule to all existing,
// non-transfer transactions and returns both the total number of transactions changed
// and a per-rule count (keyed by rule ID) of how many transactions each rule updated.
// Semantics are identical to ApplyRules: categories are overwritten when a rule
// matches, tags are merged additively, and transfers are skipped.
func (a *App) ApplyRulesWithCounts() (total int, perRule map[string]int, err error) {
	if err := a.roleGuard(); err != nil {
		return 0, nil, err
	}
	// #54: recategorizations made by this backfill are audit-stamped "rule" —
	// the trail distinguishes a rule's sweep from a person's edit. Captured
	// before the tag resets, same as the assistant/import seams.
	auditview.SetSessionActor(auditview.ActorRule)
	defer func() {
		auditview.CaptureNow()
		auditview.SetSessionActor("")
	}()
	rs := a.Rules()
	perRule = make(map[string]int, len(rs))
	if len(rs) == 0 {
		return 0, perRule, nil
	}
	for _, t := range a.Transactions() {
		if t.IsTransfer() || t.Reviewed {
			continue
		}
		// C105: use full structured-condition matching so condition-bearing rules
		// are evaluated with amount/account/date context on the backfill path.
		r := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), primaryTransactionRule)
		categoryRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), func(rr rules.Rule) bool {
			return rr.SetCategoryID != ""
		})
		billRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), func(rr rules.Rule) bool { return rr.SetBillAccountID != "" })
		if r == nil {
			r = billRule
		}
		if r == nil && categoryRule == nil {
			continue
		}
		if r == nil {
			r = categoryRule
		}
		changed := false
		if categoryRule != nil && t.CategoryID != categoryRule.SetCategoryID {
			t.CategoryID = categoryRule.SetCategoryID
			changed = true
		}
		for _, tag := range r.SetTags {
			before := len(t.Tags)
			t.Tags = addTagUnique(t.Tags, tag)
			if len(t.Tags) != before {
				changed = true
			}
		}
		// C102: apply the rename action when the rule specifies a new description.
		if r.RenameDesc != "" && t.Desc != r.RenameDesc {
			t.Desc = r.RenameDesc
			changed = true
		}
		// C373: member / reviewed / exclude, through the shared apply-once rule.
		if ap := r.ApplyOnce(ruleOwnerForApplyOnce(t), t.Reviewed, t.ExcludeFromReports); ap.Any() {
			if ap.MemberID != "" {
				t.SetOwners([]string{ap.MemberID}) // TODO(WHO2): rule assignment remains single-owner.
			}
			t.Reviewed = t.Reviewed || ap.Reviewed
			t.ExcludeFromReports = t.ExcludeFromReports || ap.ExcludeFromReports
			changed = true
		}
		// The bill link applies independently of the category winner (first matching rule
		// that sets it), and only onto unlinked transactions — never clobber a manual link.
		if t.BillAccountID == "" {
			if br := billRule; br != nil {
				t.BillAccountID = br.SetBillAccountID
				changed = true
			}
		}
		if !changed {
			continue
		}
		ruleID := ""
		if r != nil {
			ruleID = r.ID
		} else if categoryRule != nil {
			ruleID = categoryRule.ID
		}
		if is := validate.ValidateTransaction(t, a.Accounts()...); !is.OK() {
			return total, perRule, fmt.Errorf("apply rule %q to transaction %q: %w", ruleID, t.ID, is)
		}
		if err = a.validateCustom("transaction", t.Custom); err != nil {
			return total, perRule, fmt.Errorf("apply rule %q to transaction %q: %w", ruleID, t.ID, err)
		}
		if err = a.store.PutTransaction(t); err != nil {
			return total, perRule, fmt.Errorf("apply rules: save transaction %q: %w", t.ID, err)
		}
		total++
		if r != nil {
			perRule[r.ID]++
		} else if categoryRule != nil {
			perRule[categoryRule.ID]++
		}
	}
	// C372: the rule's durable record of what it has actually done.
	if cerr := a.CreditRuleHits(perRule); cerr != nil {
		return total, perRule, cerr
	}
	a.log.Info("applied rules to existing transactions", "updated", total)
	return total, perRule, nil
}

// ApplyOneRule retroactively applies a SINGLE rule to existing transactions —
// the "also apply to what I already have" choice when a rule is created
// (saving alone is future-only). Precedence is honoured: the rule is applied
// only to transactions where it is the FIRST match across the whole chain, so
// a one-rule backfill can never steal a transaction an earlier rule owns.
// Action semantics match ApplyRulesWithCounts (category overwrite, additive
// tags, rename, bill link on unlinked rows).
func (a *App) ApplyOneRule(ruleID string) (int, error) {
	if err := a.roleGuard(); err != nil {
		return 0, err
	}
	rs := a.Rules()
	var target *rules.Rule
	for i := range rs {
		if rs[i].ID == ruleID {
			target = &rs[i]
			break
		}
	}
	if target == nil {
		return 0, fmt.Errorf("apply rule: rule %q not found", ruleID)
	}
	total := 0
	for _, t := range a.Transactions() {
		if t.IsTransfer() || t.Reviewed {
			continue
		}
		td := rules.NewTxnDate(t.Date)
		primary := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td, primaryTransactionRule)
		categoryRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td, func(r rules.Rule) bool { return r.SetCategoryID != "" })
		billRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, td, func(r rules.Rule) bool { return r.SetBillAccountID != "" })
		primaryApplies := primary != nil && primary.ID == ruleID
		if !primaryApplies && (categoryRule == nil || categoryRule.ID != ruleID) && (billRule == nil || billRule.ID != ruleID) {
			continue
		}
		r := target
		changed := false

		if categoryRule != nil && categoryRule.ID == ruleID && t.CategoryID != r.SetCategoryID {
			t.CategoryID = r.SetCategoryID
			changed = true
		}
		if primaryApplies {
			for _, tag := range r.SetTags {
				before := len(t.Tags)
				t.Tags = addTagUnique(t.Tags, tag)
				if len(t.Tags) != before {
					changed = true
				}
			}
			if r.RenameDesc != "" && t.Desc != r.RenameDesc {
				t.Desc = r.RenameDesc
				changed = true
			}
		}

		if t.BillAccountID == "" {
			if billRule != nil && billRule.ID == ruleID {
				t.BillAccountID = billRule.SetBillAccountID
				changed = true
			}
		}
		// C373: member / reviewed / exclude, through the shared apply-once rule.
		if ap := r.ApplyOnce(ruleOwnerForApplyOnce(t), t.Reviewed, t.ExcludeFromReports); primaryApplies && ap.Any() {
			if ap.MemberID != "" {
				t.SetOwners([]string{ap.MemberID}) // TODO(WHO2): rule assignment remains single-owner.
			}
			t.Reviewed = t.Reviewed || ap.Reviewed
			t.ExcludeFromReports = t.ExcludeFromReports || ap.ExcludeFromReports
			changed = true
		}
		if !changed {
			continue
		}
		if is := validate.ValidateTransaction(t, a.Accounts()...); !is.OK() {
			return total, fmt.Errorf("apply rule %q to transaction %q: %w", r.ID, t.ID, is)
		}
		if err := a.validateCustom("transaction", t.Custom); err != nil {
			return total, fmt.Errorf("apply rule %q to transaction %q: %w", r.ID, t.ID, err)
		}
		if err := a.store.PutTransaction(t); err != nil {
			return total, fmt.Errorf("apply rule %q: save transaction %q: %w", ruleID, t.ID, err)
		}
		total++
	}
	// C372: PreviewApplyRules is deliberately NOT a crediting seam — a dry run
	// that inflated the count would make the number mean nothing.
	if cerr := a.creditOneRule(ruleID, total); cerr != nil {
		return total, cerr
	}
	a.log.Info("applied one rule to existing transactions", "rule", ruleID, "updated", total)
	return total, nil
}

// ApplyRuleScoped applies ONE rule to existing transactions within a scope, and
// returns exactly what it changed so the caller can offer a targeted undo (WF7).
//
// Distinct from ApplyRulesWithCounts in two ways that matter. It is scoped —
// history and the future are separate decisions, and "apply to everything ever
// recorded" should be chosen rather than implied. And it RETURNS its changes:
// the existing bulk apply leaves only a dataset-wide checkpoint behind, which
// rolls back everything else that happened alongside it.
func (a *App) ApplyRuleScoped(r rules.Rule, scope ruleapply.Scope, since time.Time) ([]ruleapply.Change, error) {
	if err := a.roleGuard(); err != nil {
		return nil, err
	}
	changes := ruleapply.Plan(r, a.Transactions(), scope, since, a.Rules())
	if err := a.applyRuleChanges(changes); err != nil {
		return nil, err
	}
	a.log.Info("rule applied", "rule", r.ID, "scope", string(scope), "changed", len(changes))
	return changes, nil
}

// UndoRuleChanges puts back what a scoped apply changed.
//
// It works from the RECORD rather than from the rule, because by the time
// somebody undoes, the rule may have been edited or deleted — and re-running it
// would restore something else entirely.
func (a *App) UndoRuleChanges(changes []ruleapply.Change) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.applyRuleChanges(ruleapply.Undo(changes))
}

// applyRuleChanges writes a set of category changes.
//
// Audit-stamped as a rule action, the same as the bulk backfill: the trail has
// to distinguish a rule's sweep from a person's edit, or a later reader cannot
// tell which of their categories they actually chose.
func (a *App) applyRuleChanges(changes []ruleapply.Change) error {
	if len(changes) == 0 {
		return nil
	}
	auditview.SetSessionActor(auditview.ActorRule)
	defer func() { auditview.CaptureNow(); auditview.SetSessionActor("") }()

	byID := make(map[string]domain.Transaction, len(changes))
	for _, t := range a.Transactions() {
		byID[t.ID] = t
	}
	var updated []domain.Transaction
	for _, c := range changes {
		t, ok := byID[c.TxnID]
		if !ok {
			// The transaction was deleted since the plan was made. Skipped rather
			// than failing the whole undo: the rest of the restore is still correct,
			// and a row that no longer exists cannot be put back.
			continue
		}
		if t.CategoryID != c.FromCategoryID {
			return fmt.Errorf("transaction %q changed since this rule application; its category was preserved", t.ID)
		}
		t.CategoryID = c.ToCategoryID
		updated = append(updated, t)
	}
	if err := a.store.PutTransactionsAtomic(updated); err != nil {
		return fmt.Errorf("apply rule changes: %w", err)
	}
	if len(updated) > 0 {
		a.fireTxnMutated()
	}

	return nil
}

// PreviewApplyRules is the dry-run twin of ApplyRulesWithCounts: it reports how
// many transactions WOULD change (total + per rule ID) without writing anything,
// so the UI can show the backfill's blast radius before the user commits to an
// overwrite that has no per-rule undo.
func (a *App) PreviewApplyRules() (total int, perRule map[string]int) {
	rs := a.Rules()
	perRule = make(map[string]int, len(rs))
	if len(rs) == 0 {
		return 0, perRule
	}
	for _, t := range a.Transactions() {
		if t.IsTransfer() || t.Reviewed {
			continue
		}
		r := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), primaryTransactionRule)
		categoryRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), func(rr rules.Rule) bool {
			return rr.SetCategoryID != ""
		})
		billRule := rules.FirstMatchFullWhereCurrency(rs, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), func(rr rules.Rule) bool { return rr.SetBillAccountID != "" })
		if r == nil {
			r = billRule
		}
		if r == nil && categoryRule == nil {
			continue
		}
		if r == nil {
			r = categoryRule
		}
		changed := categoryRule != nil && t.CategoryID != categoryRule.SetCategoryID
		for _, tag := range r.SetTags {
			before := len(t.Tags)
			t.Tags = addTagUnique(t.Tags, tag)
			if len(t.Tags) != before {
				changed = true
			}
		}
		if r.RenameDesc != "" && t.Desc != r.RenameDesc {
			changed = true
		}
		if t.BillAccountID == "" {
			if br := billRule; br != nil {
				changed = true
			}
		}
		// C373: the preview must agree with the apply about what counts as a
		// change, or the blast-radius number it promises is not the one delivered.
		if r.ApplyOnce(ruleOwnerForApplyOnce(t), t.Reviewed, t.ExcludeFromReports).Any() {
			changed = true
		}
		if !changed {
			continue
		}
		total++
		perRule[r.ID]++
	}
	return total, perRule
}

// NextRuleOrder returns a precedence Order value that places a NEW rule after
// every existing one (max existing Order + 1). Without this, new rules default
// to Order 0 and the store's ID tie-break silently jumps them to the TOP of the
// first-match-wins chain — ahead of every rule the user already trusts.
func (a *App) NextRuleOrder() int {
	next := 0
	for _, r := range a.Rules() {
		if r.Order >= next {
			next = r.Order + 1
		}
	}
	return next
}

// ApplyRules applies every saved categorization rule to all existing, non-transfer
// transactions, overwriting any existing category when a rule matches. This is the
// "Apply to existing" backfill path — it intentionally overwrites previously-set
// categories so that correcting or reordering a rule propagates to past transactions.
// Tags are additive (union): each tag from the rule is merged into the transaction's
// existing tags without duplicating any tag already present, so manually-curated tags
// are always preserved. Transfers are always skipped.
// Returns the number of transactions that were changed.
func (a *App) ApplyRules() (int, error) {
	n, _, err := a.ApplyRulesWithCounts()
	return n, err
}

// ReassignCategory moves every transaction and budget referencing oldID to newID,
// returning how many records were moved. Use it before deleting a category that
// is still in use. The category itself is not deleted here.
func (a *App) ReassignCategory(oldID, newID string) (int, error) {
	// C523: this used to rewrite exactly two things — a transaction's CategoryID
	// and a budget's CategoryID — and silently leave behind split lines, a
	// multi-category budget's CategoryIDs list, sinking-fund goals, rules that
	// file into the category, and recurring templates. Those dangling references
	// never fail loudly; they produce a household whose totals quietly stop
	// adding up. It now runs the same complete sweep a merge does.
	//
	// Categories themselves are deliberately excluded from the sweep: the delete
	// path owns retiring the category and re-homing its children
	// (categorytree.ReparentOnDelete), and doing it twice would fight.
	moved, err := a.moveCategoryRefs(oldID, newID)
	if err != nil {
		return moved, err
	}
	a.log.Info("reassigned category", "from", oldID, "to", newID, "moved", moved)
	return moved, nil
}

// OnTxnMutated registers fn to be called after every successful transaction
// add, edit, or delete (C122). Observers are called synchronously in
// registration order. Registering the same function more than once is fine —
// all copies will fire. The observer must not mutate transactions itself (it
// may read state freely).
func (a *App) OnTxnMutated(fn func()) {
	a.txnMutatedFns = append(a.txnMutatedFns, fn)
}

// fireTxnMutated calls every registered OnTxnMutated observer. It is a no-op
// when suppressTxnObservers is true (bulk-import guard: callers fire once after
// the whole batch so observers don't run once per row).
func (a *App) fireTxnMutated() {
	if a.suppressTxnObservers {
		return
	}
	for _, fn := range a.txnMutatedFns {
		fn()
	}
}

func (a *App) PutTransaction(t domain.Transaction) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	t.Date = normalizeTransactionDate(t.Date)
	if is := validate.ValidateTransaction(t, a.Accounts()...); !is.OK() {
		return is
	}
	if err := a.validateCustom("transaction", t.Custom); err != nil {
		return err
	}
	// Detect whether this is a brand-new transaction (vs. an edit) so the
	// "transaction added" trigger fires only on real additions, from every add
	// path (quick-add, inline add, transfer, duplicate, import), not on edits.
	_, existed, _ := a.store.GetTransaction(t.ID)
	// Snapshot which budgets are over BEFORE the write: it's the transaction
	// that pushes a budget over its limit, so the budget-exceeded trigger must
	// fire from here — previously only re-saving the budget document itself
	// checked the transition, leaving the trigger dormant in ordinary use.
	var overBefore map[string]bool
	if !existed && !a.triggersSuspended {
		overBefore = a.overBudgetIDs()
	}
	if err := a.store.PutTransaction(t); err != nil {
		return err
	}
	a.afterTransactionSaved(t, existed, overBefore)
	return nil
}

// afterTransactionSaved runs addition effects and observers only after all records
// belonging to the operation have committed. Edits retain their existing behavior.
func (a *App) afterTransactionSaved(t domain.Transaction, existed bool, overBefore map[string]bool) {
	a.log.Info("transaction saved", "id", t.ID)
	if !existed && !a.triggersSuspended {
		// C192: spending against a sinking fund's linked category draws the fund
		// down. New transactions only (so editing doesn't re-draw).
		a.applySinkingFundDrawdown(t)
	}
	if !existed && !a.triggersSuspended {
		a.RunTriggered(workflow.TriggerTxnAdded, &t)
		for id := range a.overBudgetIDs() {
			if !overBefore[id] {
				a.RunTriggered(workflow.TriggerBudgetExceeded, nil)
				break
			}
		}
	}
	a.fireTxnMutated()
	// XC8: a posting transaction is a data event — evaluate self-resolving tasks
	// and auto-complete any whose rule the transaction satisfies (e.g. a refund
	// that closes a "chase the duplicate charge" task).
	if !existed && !a.triggersSuspended {
		a.resolveTasksForTxn(t)
		// TX9: a posting transaction may settle an expected recurring occurrence —
		// auto-create bill-match links for unambiguous 1:1 pairs (a data event,
		// never a timer). Ambiguous occurrences are left for a manual match.
		a.autoMatchBills(a.clock())
	}
}

func normalizeTransactionDate(date time.Time) time.Time {
	if date.IsZero() {
		return date
	}
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// overBudgetIDs returns the set of budget IDs currently over their limit —
// the before/after snapshot PutTransaction uses to fire budget-exceeded on
// the transition.
func (a *App) overBudgetIDs() map[string]bool {
	out := map[string]bool{}
	for _, b := range a.Budgets() {
		if a.isBudgetOver(b) {
			out[b.ID] = true
		}
	}
	return out
}

// applySinkingFundDrawdown decrements any non-archived sinking-fund goal linked
// (by CategoryID) to the transaction's category when the transaction is an
// expense, so real spending against the linked category draws the fund down
// (C192, R20-drawdown-wire). Income and transfers are ignored. The spend is
// converted to each fund's currency; a fund is skipped (with a warning) when no
// FX rate is available. Persists via the store directly — it's an automatic
// internal balance update, not a user edit, so it bypasses role/validation and
// the milestone-toast side effects of PutGoal.
func (a *App) applySinkingFundDrawdown(t domain.Transaction) {
	if t.IsTransfer() || !t.Amount.IsNegative() || t.CategoryID == "" {
		return
	}
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	spendAbs := t.Amount.Abs()
	for _, g := range a.Goals() {
		if !g.IsSinkingFund || g.Archived || g.CategoryID != t.CategoryID {
			continue
		}
		amt := spendAbs
		if spendAbs.Currency != g.CurrentAmount.Currency {
			conv, err := rates.Convert(spendAbs, g.CurrentAmount.Currency)
			if err != nil {
				a.log.Warn("sinking-fund drawdown skipped: no FX rate",
					"goal", g.ID, "from", spendAbs.Currency, "to", g.CurrentAmount.Currency)
				continue
			}
			amt = conv
		}
		updated, err := goals.DrawDownFund(g, amt.Amount)
		if err != nil {
			a.log.Warn("sinking-fund drawdown failed", "goal", g.ID, "err", err)
			continue
		}
		if err := a.store.PutGoal(updated); err != nil {
			a.log.Error("sinking-fund drawdown persist failed", "goal", g.ID, "err", err)
			continue
		}
		a.log.Info("sinking-fund drawn down", "goal", g.ID, "by", amt.Amount, "now", updated.CurrentAmount.Amount)
	}
}

// WithoutTriggers runs fn with automatic transaction-added workflow firing paused
// (so a bulk add doesn't fire per row), then, if any transactions may have been
// added, fires the trigger once. Used by import paths.
func (a *App) WithoutTriggers(fn func()) {
	prev := a.triggersSuspended
	a.triggersSuspended = true
	fn()
	a.triggersSuspended = prev
	if !a.triggersSuspended {
		// Bulk add: fire aggregate workflows once (no single triggering txn).
		a.RunTriggered(workflow.TriggerTxnAdded, nil)
	}
}

// BulkMutate runs fn with BOTH per-row workflow triggers and per-row txn-mutated
// observers suppressed, then fires the observers exactly once at the end. This is the
// primitive for bulk ledger edits (recategorize / clear / assign / delete / group over
// a selection): without it, every PutTransaction/DeleteTransaction in the loop fires
// fireTxnMutated, re-rendering the whole (thousands-of-rows) surface per row — O(N²)
// work that froze/aborted the app on a large selection. With it, the surface re-renders
// once. Nested calls stay safe (the inner fireTxnMutated no-ops while still suppressed).
func (a *App) BulkMutate(fn func()) {
	prevT, prevO := a.triggersSuspended, a.suppressTxnObservers
	a.triggersSuspended = true
	a.suppressTxnObservers = true
	fn()
	a.triggersSuspended = prevT
	a.suppressTxnObservers = prevO
	a.fireTxnMutated() // once; no-ops if still suppressed (nested)
}

// PutTransactionsAtomic validates the complete edit batch before writing any row.
// It suppresses addition triggers: callers use PutTransaction for new entries.
func (a *App) PutTransactionsAtomic(txns []domain.Transaction) error {
	if len(txns) == 0 {
		return nil
	}
	if err := a.roleGuard(); err != nil {
		return err
	}
	writes := append([]domain.Transaction(nil), txns...)
	accounts := a.Accounts()
	seen := make(map[string]bool, len(writes))
	for i, t := range writes {
		if seen[t.ID] {
			return fmt.Errorf("save transactions: duplicate id %q", t.ID)
		}
		seen[t.ID] = true
		t.Date = normalizeTransactionDate(t.Date)
		if issues := validate.ValidateTransaction(t, accounts...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", t.Custom); err != nil {
			return err
		}
		writes[i] = t
	}
	if err := a.store.PutTransactionsAtomic(writes); err != nil {
		return fmt.Errorf("save transactions: %w", err)
	}
	a.fireTxnMutated()
	return nil
}

// AttachTransactionArtifact attaches a newly uploaded receipt to the current row.
// Artifact bytes and the row reference commit together; neither can survive a
// failure on the other record, and intervening edits are preserved.
func (a *App) AttachTransactionArtifact(transactionID string, artifact domain.Artifact) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if artifact.ID == "" || strings.TrimSpace(artifact.Name) == "" || artifact.Kind == "" {
		return fmt.Errorf("a receipt needs an id, name and kind")
	}
	txn, ok, err := a.store.GetTransaction(transactionID)
	if err != nil {
		return fmt.Errorf("read transaction for receipt: %w", err)
	}
	if !ok {
		return fmt.Errorf("the transaction no longer exists")
	}
	before, err := json.Marshal(txn)
	if err != nil {
		return fmt.Errorf("encode receipt transaction: %w", err)
	}
	txn.Attachments = append(append([]domain.AttachmentRef(nil), txn.Attachments...), domain.AttachmentRef{ArtifactID: artifact.ID, Name: artifact.Name, Kind: artifact.Kind, MIME: artifact.MIME})
	after, err := json.Marshal(txn)
	if err != nil {
		return fmt.Errorf("encode receipt reference: %w", err)
	}
	encodedArtifact, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	changes := []store.RecordChange{{Table: "artifacts", ID: artifact.ID, After: encodedArtifact}, {Table: "transactions", ID: txn.ID, Before: before, After: after}}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("attach receipt: %w", err)
	}
	a.fireTxnMutated()
	return nil
}

// EditTransaction commits a row's edits, account move and mirrored transfer
// amount together. A stale draft is refused, as is silently unclassifying its
// partner. Use EditTransactionRemovingPartner after a named confirmation instead.
func (a *App) EditTransaction(prior, draft domain.Transaction) error {
	return a.editTransaction(prior, draft, false)
}

// EditTransactionRemovingPartner turns a paired row into ordinary spending or
// income and deletes the other leg, including its relationships, atomically.
// The caller must first confirm the named partner deletion with the user.
func (a *App) EditTransactionRemovingPartner(prior, draft domain.Transaction) error {
	return a.editTransaction(prior, draft, true)
}

func (a *App) editTransaction(prior, draft domain.Transaction, removePartner bool) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	current, ok, err := a.store.GetTransaction(prior.ID)
	if err != nil {
		return fmt.Errorf("edit transaction: %w", err)
	}
	prior.Date = normalizeTransactionDate(prior.Date)
	if !ok || draft.ID != prior.ID || !reflect.DeepEqual(current, prior) {
		return fmt.Errorf("this transaction changed; reopen it before saving")
	}
	draft.Date = normalizeTransactionDate(draft.Date)
	if draft.AccountID != prior.AccountID {
		// The draft is expressed in the original account's booking convention.
		// Apply the pure move before mirroring, including its checked split signing.
		target := draft.AccountID
		draft.AccountID = prior.AccountID
		moved, _, _, err := txnmove.Apply(draft, target, a.Accounts(), a.Transactions())
		if err != nil {
			return fmt.Errorf("move edited transaction: %w", err)
		}
		draft = moved
	}

	var pair domain.Transaction
	paired := false
	if prior.IsTransfer() {
		pair, paired = a.transferPartner(prior)
	}
	writes := []domain.Transaction{draft}
	if prior.IsTransfer() && paired {
		if !draft.IsTransfer() {
			if !removePartner {
				return fmt.Errorf("confirm deleting the other transfer leg before changing this to income or spending")
			}
			draft.TransferAccountID, draft.TransferGroupID = "", ""
			if issues := validate.ValidateTransaction(draft, a.Accounts()...); !issues.OK() {
				return issues
			}
			if err := a.validateCustom("transaction", draft.Custom); err != nil {
				return err
			}
			changes, _, err := a.transactionDeleteChanges([]string{pair.ID})
			if err != nil {
				return err
			}
			before, _ := json.Marshal(current)
			after, err := json.Marshal(draft)
			if err != nil {
				return fmt.Errorf("encode edited transaction: %w", err)
			}
			changes = append(changes, store.RecordChange{Table: "transactions", ID: draft.ID, Before: before, After: after})
			if err := a.store.ReplayChangesExpected(changes); err != nil {
				return fmt.Errorf("remove transfer partner: %w", err)
			}
			a.SyncTxnLinkNetting()
			a.fireTxnMutated()
			return nil
		}
		if draft.TransferAccountID != prior.TransferAccountID {
			return fmt.Errorf("change the transfer destination separately before editing this pair")
		}
		if draft.AccountID != prior.AccountID {
			if draft.AccountID == pair.AccountID {
				return fmt.Errorf("the two transfer accounts must differ")
			}
			pair.TransferAccountID = draft.AccountID
		}
		if draft.Amount != prior.Amount {
			mirrored, err := a.mirroredLegAmount(draft, pair, 0)
			if err != nil {
				return err
			}
			pair.Amount = mirrored
		}
		pair.Date = draft.Date
		writes = append(writes, pair)
	}
	return a.PutTransactionsAtomic(writes)
}

// DeleteTransaction removes one row and its relationships in one SQL commit.
func (a *App) DeleteTransaction(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	_, err := a.deleteTransactionsAtomic([]string{id})
	return err
}

// transactionDeleteChanges plans row and relationship changes without touching
// the store. Checked replay commits the entire plan or preserves every record.
func (a *App) transactionDeleteChanges(ids []string) ([]store.RecordChange, []string, error) {
	deleted := make(map[string]bool, len(ids))
	var changes []store.RecordChange
	var actual []string
	for _, id := range ids {
		if deleted[id] {
			continue
		}
		t, ok, err := a.store.GetTransaction(id)
		if err != nil {
			return nil, nil, fmt.Errorf("read transaction for deletion: %w", err)
		}
		if !ok {
			continue
		}
		before, err := json.Marshal(t)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: id, Before: before})
		actual = append(actual, id)
		deleted[id] = true
	}
	links, err := a.store.ListTxnLinks()
	if err != nil {
		return nil, nil, err
	}
	for _, link := range links {
		members := make([]string, 0, len(link.TxnIDs))
		for _, id := range link.TxnIDs {
			if !deleted[id] {
				members = append(members, id)
			}
		}
		if len(members) == len(link.TxnIDs) {
			continue
		}
		before, err := json.Marshal(link)
		if err != nil {
			return nil, nil, err
		}
		var after json.RawMessage
		if link.Kind == domain.TxnLinkOrderGroup && len(members) >= 2 {
			link.TxnIDs = members
			after, err = json.Marshal(link)
			if err != nil {
				return nil, nil, err
			}
		}
		changes = append(changes, store.RecordChange{Table: "txnlinks", ID: link.ID, Before: before, After: after})
	}
	debtLinks, err := a.store.ListDebtPaymentLinks()
	if err != nil {
		return nil, nil, err
	}
	for _, link := range debtLinks {
		for _, id := range link.TransactionIDs {
			if !deleted[id] {
				continue
			}
			before, err := json.Marshal(link)
			if err != nil {
				return nil, nil, err
			}
			changes = append(changes, store.RecordChange{Table: "debt_payment_links", ID: link.ID, Before: before})
			break
		}
	}
	return changes, actual, nil
}

// RestoreTransactions restores the rows and relationships carried by preceding
// deletes on this App. It never runs addition triggers or draws sinking funds
// again. Relationships with intervening edits are refused, never overwritten.
func (a *App) RestoreTransactions(txns []domain.Transaction) error {
	if len(txns) == 0 {
		return nil
	}
	if err := a.roleGuard(); err != nil {
		return err
	}
	var changes []store.RecordChange
	restored := make(map[string]bool, len(txns))
	for _, t := range txns {
		t.Date = normalizeTransactionDate(t.Date)
		if issues := validate.ValidateTransaction(t, a.Accounts()...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", t.Custom); err != nil {
			return err
		}
		current, ok, err := a.store.GetTransaction(t.ID)
		if err != nil {
			return err
		}
		var before json.RawMessage
		if ok {
			before, err = json.Marshal(current)
			if err != nil {
				return err
			}
		}
		after, err := json.Marshal(t)
		if err != nil {
			return err
		}
		changes = append(changes, store.RecordChange{Table: "transactions", ID: t.ID, Before: before, After: after})
		restored[t.ID] = true
	}
	seen := map[string]bool{}
	for _, t := range txns {
		for _, c := range a.transactionDeletes[t.ID] {
			key := c.Table + "/" + c.ID
			if seen[key] {
				continue
			}
			var members []string
			if c.Table == "txnlinks" {
				var link domain.TxnLink
				if err := json.Unmarshal(c.Before, &link); err != nil {
					return err
				}
				members = link.TxnIDs
			} else {
				var link domain.DebtPaymentLink
				if err := json.Unmarshal(c.Before, &link); err != nil {
					return err
				}
				members = link.TransactionIDs
			}
			complete := true
			for _, id := range members {
				if !restored[id] {
					if _, ok := a.TransactionByID(id); !ok {
						complete = false
						break
					}
				}
			}
			if !complete {
				continue
			}
			seen[key] = true
			changes = append(changes, store.RecordChange{Table: c.Table, ID: c.ID, Before: c.After, After: c.Before})
		}
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("restore transactions: %w", err)
	}
	for id := range restored {
		delete(a.transactionDeletes, id)
	}
	a.SyncTxnLinkNetting()
	a.fireTxnMutated()
	return nil
}

// DeleteTransactionWithTransferPair removes a transaction and, when it is one
// leg of a transfer, also removes the reciprocal leg so balances stay paired.
// The txnMutated observers fire exactly once after the full pair is removed.
// DeleteTransactionsBulk deletes many transactions efficiently: it resolves reciprocal
// transfer legs ONCE from a single ledger snapshot (instead of re-fetching the whole
// ledger per id, which made a large bulk delete O(N²) and appear frozen), then deletes
// them all inside BulkMutate so the surface re-renders once.
// ExpandTransferPairs returns ids plus the reciprocal leg of every transfer among
// them — exactly the set DeleteTransactionsBulk will remove. Callers that snapshot
// before a bulk delete must snapshot THIS set, not the ids they selected: a
// selection holding one leg of a transfer takes its counterpart down with it, and
// an undo built from the selection alone silently restores half a transfer,
// leaving the other account permanently short.
func (a *App) ExpandTransferPairs(ids []string) []string {
	all := a.Transactions()
	return expandTransferPairs(all, ids, a.transactionPairRates(), a.Accounts()...)
}

func (a *App) transactionPairRates() currency.Rates {
	s := a.Settings()
	base := s.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: s.FXRates}
}

func expandTransferPairs(all []domain.Transaction, ids []string, rates currency.Rates, accounts ...domain.Account) []string {
	byID := make(map[string]domain.Transaction, len(all))
	grouped := make(map[string]map[transferDirection]domain.Transaction)
	legacy := make(map[legacyTransferIndexKey][]domain.Transaction)
	for _, t := range all {
		byID[t.ID] = t
		if !t.IsTransfer() {
			continue
		}
		if t.TransferGroupID != "" {
			if grouped[t.TransferGroupID] == nil {
				grouped[t.TransferGroupID] = make(map[transferDirection]domain.Transaction)
			}
			grouped[t.TransferGroupID][transferDirectionKey(t)] = t
			continue
		}
		if key, ok := legacyTransferPairKey(t, rates, accounts...); ok {
			legacy[key] = append(legacy[key], t)
		}
	}
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		add(id)
		if t, ok := byID[id]; ok && t.IsTransfer() {
			var candidates []domain.Transaction
			if t.TransferGroupID != "" {
				if candidate, ok := grouped[t.TransferGroupID][transferDirectionKey(t).reverse()]; ok {
					candidates = append(candidates, candidate)
				}
			} else if key, ok := legacyTransferPairKey(t, rates, accounts...); ok {
				candidates = legacy[reverseLegacyTransferKey(key)]
			}
			for i, u := range candidates {
				if !seen[u.ID] && isReciprocalTransferLegWithRates(t, u, rates, accounts...) {
					add(u.ID)
					if t.TransferGroupID == "" {
						if key, ok := legacyTransferPairKey(t, rates, accounts...); ok {
							legacy[reverseLegacyTransferKey(key)] = candidates[i+1:]
						}
					}
					break
				}
			}
		}
	}
	return out
}

// DeleteTransactionsBulk removes the selected transactions and any transfer
// partners atomically. It returns only the IDs that were actually deleted.
func (a *App) DeleteTransactionsBulk(ids []string) ([]string, error) {
	if err := a.roleGuard(); err != nil {
		return nil, err
	}
	all := a.Transactions()
	toDel := expandTransferPairs(all, ids, a.transactionPairRates(), a.Accounts()...)
	return a.deleteTransactionsAtomic(toDel)
}

// DeleteTransactionsBulkExpanded deletes an already-expanded transaction set
// atomically and returns the IDs that were actually deleted. It lets callers
// use one pre-delete snapshot for both undo capture and deletion.
func (a *App) DeleteTransactionsBulkExpanded(ids []string) ([]string, error) {
	if err := a.roleGuard(); err != nil {
		return nil, err
	}
	return a.deleteTransactionsAtomic(ids)
}

func (a *App) deleteTransactionsAtomic(ids []string) ([]string, error) {
	changes, deleted, err := a.transactionDeleteChanges(ids)
	if err != nil {
		return nil, fmt.Errorf("plan transaction deletion: %w", err)
	}
	if len(deleted) == 0 {
		return nil, nil
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return nil, fmt.Errorf("delete transactions: %w", err)
	}
	a.afterTransactionsDeleted(changes, deleted)
	return deleted, nil
}

// afterTransactionsDeleted retains relationship restoration metadata and notifies
// observers after the transaction rows and their related changes commit together.
func (a *App) afterTransactionsDeleted(changes []store.RecordChange, deleted []string) {
	var relations []store.RecordChange
	for _, c := range changes {
		if c.Table != "transactions" {
			relations = append(relations, c)
		}
	}
	if len(relations) > 0 {
		if a.transactionDeletes == nil {
			a.transactionDeletes = map[string][]store.RecordChange{}
		}
		for _, id := range deleted {
			a.transactionDeletes[id] = relations
		}
	}
	a.SyncTxnLinkNetting()
	a.fireTxnMutated()
}

// DeleteTransactionWithTransferPair deletes both legs and their relationships.
func (a *App) DeleteTransactionWithTransferPair(id string) error {
	_, err := a.DeleteTransactionsBulk([]string{id})
	return err
}

// isReciprocalTransferLeg reports whether candidate is the far side of target.
//
// C680: when both legs carry a TransferGroupID, identity settles it and nothing
// else is consulted. That is the whole point of the id — the resemblance test
// below fails on exactly the cases that matter most. An edited amount stops two
// real legs looking reciprocal (that was C629, where a one-sided edit silently
// orphaned its partner), and a cross-currency pair NEVER matches it, because the
// two legs carry different magnitudes by design.
//
// The legacy fallback remains for rows written before the id existed and for a
// leg someone classified by hand from an import. It requires exact amounts in
// one currency, or equal FX-converted base amounts when rates are available. A
// group id that disagrees with it wins.
func isReciprocalTransferLeg(target, candidate domain.Transaction) bool {
	return isReciprocalTransferLegWithRates(target, candidate, currency.Rates{})
}

func isReciprocalTransferLegWithRates(target, candidate domain.Transaction, rates currency.Rates, accounts ...domain.Account) bool {
	if candidate.ID == target.ID || !candidate.IsTransfer() {
		return false
	}
	if target.TransferGroupID != "" || candidate.TransferGroupID != "" {
		// One of them is grouped. Only an exact group match counts: a grouped leg
		// whose partner is elsewhere must NOT fall through to the heuristic and
		// adopt a look-alike, which is how an unrelated row gets mutated.
		return target.TransferGroupID != "" && target.TransferGroupID == candidate.TransferGroupID
	}
	if candidate.AccountID != target.TransferAccountID ||
		candidate.TransferAccountID != target.AccountID || !candidate.Date.Equal(target.Date) ||
		transferNegative(candidate, accounts) == transferNegative(target, accounts) {
		return false
	}
	if candidate.Amount.Currency == target.Amount.Currency {
		left, e1 := money.Abs(candidate.Amount.Amount)
		right, e2 := money.Abs(target.Amount.Amount)
		return e1 == nil && e2 == nil && left == right
	}
	if rates.Base == "" {
		return false
	}
	targetBase, err := rates.Convert(target.Amount.Abs(), rates.Base)
	if err != nil {
		return false
	}
	candidateBase, err := rates.Convert(candidate.Amount.Abs(), rates.Base)
	return err == nil && candidateBase.Amount == targetBase.Amount
}

func legacyTransferPairKey(t domain.Transaction, rates currency.Rates, accounts ...domain.Account) (legacyTransferIndexKey, bool) {
	if t.TransferGroupID != "" || !t.IsTransfer() || t.TransferAccountID == "" {
		return legacyTransferIndexKey{}, false
	}
	abs := t.Amount.Abs()
	amount, currencyCode := abs.Amount, abs.Currency
	if rates.Base != "" {
		if converted, err := rates.Convert(abs, rates.Base); err == nil {
			amount, currencyCode = converted.Amount, rates.Base
		}
	}
	key := legacyTransferIndexKey{AccountID: t.AccountID, TransferAccountID: t.TransferAccountID,
		Date: t.Date.UnixNano(), Currency: currencyCode, Amount: amount, Negative: transferNegative(t, accounts)}
	return key, true
}

// transferNegative describes economic direction rather than a liability's
// booking sign. TCS1 owns the account convention; pairing consumes that API.
func transferNegative(t domain.Transaction, accounts []domain.Account) bool {
	negative := t.Amount.IsNegative()
	if account, ok := domain.AccountByID(accounts, t.AccountID); ok && account.InflowSign() < 0 {
		return !negative
	}
	return negative
}

type legacyTransferIndexKey struct {
	AccountID, TransferAccountID string
	Date                         int64
	Currency                     string
	Amount                       int64
	Negative                     bool
}

func reverseLegacyTransferKey(key legacyTransferIndexKey) legacyTransferIndexKey {
	key.AccountID, key.TransferAccountID = key.TransferAccountID, key.AccountID
	key.Negative = !key.Negative
	return key
}

type transferDirection struct {
	AccountID, TransferAccountID string
}

func transferDirectionKey(t domain.Transaction) transferDirection {
	return transferDirection{AccountID: t.AccountID, TransferAccountID: t.TransferAccountID}
}

func (k transferDirection) reverse() transferDirection {
	return transferDirection{AccountID: k.TransferAccountID, TransferAccountID: k.AccountID}
}

// PutBudget saves a budget and records any permanent limit change from the
// application clock's period. All writers share the history-aware boundary.
func (a *App) PutBudget(b domain.Budget) error { return a.PutBudgetAt(b, a.clock()) }
func (a *App) DeleteBudget(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("budget", id, a.store.DeleteBudget)
}

// CoverBudget moves amt between the budgets' current-period boosts atomically.
// Standing limits and their history are unchanged. Each leg uses its own grid;
// the source must retain positive available room after the cover.
func (a *App) CoverBudget(fromID, toID string, amt money.Money) error {
	return a.coverBudgetsAt(toID, map[string]money.Money{fromID: amt}, a.clock(), false)
}

func (a *App) PutGoal(g domain.Goal) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if is := validate.ValidateGoal(g); !is.OK() {
		return is
	}
	if err := a.validateCustom("goal", g.Custom); err != nil {
		return err
	}
	wasComplete, beforeKnown := false, true
	rates := a.goalEvaluationRates()
	for _, existing := range a.Goals() {
		if existing.ID == g.ID {
			wasComplete, beforeKnown = goalCompletionForWrite(existing, rates)
			break
		}
	}
	if err := a.store.PutGoal(g); err != nil {
		return err
	}
	a.afterGoalSaved(g, wasComplete, beforeKnown, rates)
	return nil
}

// afterGoalSaved fires the known completion transition after its enclosing write
// has committed, including operations that also change the ledger.
func (a *App) afterGoalSaved(g domain.Goal, wasComplete, beforeKnown bool, rates currency.Rates) {
	a.log.Info("goal saved", "id", g.ID)
	isComplete, afterKnown := goalCompletionForWrite(g, rates)
	if beforeKnown && afterKnown && !wasComplete && isComplete && !a.triggersSuspended {
		a.RunTriggered(workflow.TriggerGoalReached, nil)
	}
}
func (a *App) DeleteGoal(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("goal", id, a.store.DeleteGoal)
}

// PutHolding persists an investment holding (insert or replace by ID).
func (a *App) PutHolding(h domain.Holding) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if err := a.store.PutHolding(h); err != nil {
		return fmt.Errorf("appstate: put holding: %w", err)
	}
	a.log.Info("holding saved", "id", h.ID, "ticker", h.Ticker)
	return nil
}

// PutRealizedSale records a disposal (insert or replace by ID).
func (a *App) PutRealizedSale(r domain.RealizedSale) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if err := a.store.PutRealizedSale(r); err != nil {
		return fmt.Errorf("appstate: put realized sale: %w", err)
	}
	a.log.Info("realized sale recorded", "id", r.ID, "name", r.Name, "gain", r.GainMinor)
	return nil
}

// DeleteRealizedSale removes a recorded disposal by ID.
func (a *App) DeleteRealizedSale(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("realizedsale", id, a.store.DeleteRealizedSale)
}

// DeleteHolding removes an investment holding by ID.
func (a *App) DeleteHolding(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("holding", id, a.store.DeleteHolding)
}

// ArchiveGoal sets the Archived flag on a goal to archive (true) or restore
// (false) it. The goal must already exist; a missing ID returns an error.
func (a *App) ArchiveGoal(goalID string, archive bool) error {
	goals, err := a.store.ListGoals()
	if err != nil {
		return fmt.Errorf("appstate: archive goal: %w", err)
	}
	for _, g := range goals {
		if g.ID != goalID {
			continue
		}
		g.Archived = archive
		return a.PutGoal(g)
	}
	return fmt.Errorf("appstate: archive goal: id %q not found", goalID)
}

func (a *App) PutTask(t domain.Task) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if is := validate.ValidateTask(t); !is.OK() {
		return is
	}
	if err := a.store.PutTask(t); err != nil {
		return err
	}
	a.log.Info("task saved", "id", t.ID)
	return nil
}

// ReorderTask moves a task to another sibling's position (drag-and-drop in the "Custom
// order" view), persisting only the tasks whose manual Order changed. A drop onto a
// non-sibling (different parent), onto itself, or onto a missing task is a safe no-op.
func (a *App) ReorderTask(srcID, targetID string) error {
	changed, ok := tasksort.Reorder(a.Tasks(), srcID, targetID)
	if !ok {
		return nil
	}
	for _, t := range changed {
		if err := a.PutTask(t); err != nil {
			return fmt.Errorf("appstate: reorder task: %w", err)
		}
	}
	a.log.Info("tasks reordered", "src", srcID, "target", targetID, "changed", len(changed))
	return nil
}

// CreateFreshnessReminderTask creates the to-do generated from the dashboard's
// stale-balance nudge.
func (a *App) CreateFreshnessReminderTask(title string) (domain.Task, error) {
	t := domain.Task{
		ID:       id.New(),
		Title:    title,
		Status:   domain.StatusOpen,
		Priority: domain.PriorityMedium,
		Source:   domain.SourceNudge,
	}
	if err := a.PutTask(t); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}
func (a *App) DeleteTask(id string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	return a.del("task", id, a.store.DeleteTask)
}

// CompleteTask marks the task identified by id as done and, if the task has a
// non-empty Recurrence, atomically saves a fresh open successor via
// taskrecur.NextOccurrence. nextID must be a freshly-generated ID (e.g.
// id.New()); now is the reference time used when the task has no Due date.
//
// Re-opening a done task (StatusDone → StatusOpen) must go through PutTask
// directly; only the open→done transition spawns a successor.
func (a *App) CompleteTask(taskID, nextID string, now time.Time) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	tasks := a.Tasks()
	var found domain.Task
	var ok bool
	for _, t := range tasks {
		if t.ID == taskID {
			found = t
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	found.Status = domain.StatusDone
	if next, spawn := taskrecur.NextOccurrence(found, nextID, now); spawn {
		if is := validate.ValidateTask(next); !is.OK() {
			return fmt.Errorf("spawn next occurrence: %w", is)
		}
		if err := a.store.PutTasksAtomic(found, next); err != nil {
			return fmt.Errorf("complete recurring task: %w", err)
		}
		a.log.Info("spawned recurring task", "from", taskID, "next", next.ID, "due", next.Due)
	} else if err := a.PutTask(found); err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	return nil
}

// PutCustomFieldDef validates and saves a custom-field definition. The Def must
// be sound (id, entity type, key, label, known type; choice fields need options).
func (a *App) PutCustomFieldDef(d customfields.Def) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
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
	if err := a.roleGuard(); err != nil {
		return err
	}
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

// --- allocation application (L17) ---

// AllocationResult summarises the outcome of ApplyAllocation so the UI can
// show exactly what happened.
type AllocationResult struct {
	GoalsFunded    int   // number of goals that received a contribution
	GoalDollars    int64 // total minor units added to goals
	EarmarksMade   int   // number of earmarks created
	EarmarkDollars int64 // total minor units earmarked
	Overflow       int64 // minor units that could not be added to goals because they are already complete
}

// undoSnapshot holds the pre-apply dataset for single-step undo.
var undoSnapshot *store.Dataset

// ApplyAllocation commits a set of Actions returned by allocate.PlanActions.
// Goals receive contributions (CurrentAmount is bumped, capped at TargetAmount;
// any excess is reported in AllocationResult.Overflow but never silently lost).
// Account and debt earmarks are persisted as domain.Earmark records.
//
// The operation is atomic: the full dataset is snapshotted before any writes,
// and on any error the snapshot is restored so the store is left unchanged.
// On success the pre-apply snapshot is stored for UndoLastAllocation.
func (a *App) ApplyAllocation(actions []allocate.Action) (AllocationResult, error) {
	snap, err := a.store.Snapshot()
	if err != nil {
		return AllocationResult{}, fmt.Errorf("appstate: apply allocation: snapshot: %w", err)
	}

	var result AllocationResult
	for _, act := range actions {
		if err := a.applyAllocationAction(act, &result); err != nil {
			// Atomically roll back to the pre-apply snapshot.
			if loadErr := a.store.Load(snap); loadErr != nil {
				a.log.Error("apply allocation rollback failed", "rollbackErr", loadErr, "originalErr", err)
			}
			return AllocationResult{}, fmt.Errorf("appstate: apply allocation: %w", err)
		}
	}

	undoSnapshot = &snap
	a.log.Info("allocation applied",
		"goals", result.GoalsFunded, "goalDollars", result.GoalDollars,
		"earmarks", result.EarmarksMade, "earmarkDollars", result.EarmarkDollars,
		"overflow", result.Overflow)
	return result, nil
}

func (a *App) applyAllocationAction(act allocate.Action, result *AllocationResult) error {
	switch act.Kind {
	case allocate.GoalContribution:
		g, ok, err := a.store.GetGoal(act.DestinationID)
		if err != nil {
			return fmt.Errorf("load goal %q: %w", act.DestinationID, err)
		}
		if !ok {
			return fmt.Errorf("goal %q not found", act.DestinationID)
		}
		headroom := g.TargetAmount.Amount - g.CurrentAmount.Amount
		if headroom < 0 {
			headroom = 0
		}
		credit := act.Amount
		if credit > headroom {
			result.Overflow += credit - headroom
			credit = headroom
		}
		if credit > 0 {
			g.CurrentAmount.Amount += credit
			if err := a.PutGoal(g); err != nil {
				return fmt.Errorf("save goal %q: %w", act.DestinationID, err)
			}
		}
		result.GoalsFunded++
		result.GoalDollars += credit

	case allocate.AccountEarmark, allocate.DebtPaydownEarmark:
		kind := domain.EarmarkKindAccount
		if act.Kind == allocate.DebtPaydownEarmark {
			kind = domain.EarmarkKindDebt
		}
		cur := a.Settings().BaseCurrency
		if cur == "" {
			cur = "USD"
		}
		em := domain.Earmark{
			ID:              id.New(),
			DestinationID:   act.DestinationID,
			DestinationKind: kind,
			Amount:          money.New(act.Amount, cur),
			Currency:        cur,
			CreatedAt:       a.clock(),
		}
		if err := a.store.PutEarmark(em); err != nil {
			return fmt.Errorf("save earmark for %q: %w", act.DestinationID, err)
		}
		result.EarmarksMade++
		result.EarmarkDollars += act.Amount
	}
	return nil
}

// UndoLastAllocation restores the dataset to the state before the last
// ApplyAllocation call. It is a no-op (returns an error) when there is no
// snapshot to restore.
func (a *App) UndoLastAllocation() error {
	if undoSnapshot == nil {
		return fmt.Errorf("appstate: no allocation to undo")
	}
	snap := *undoSnapshot
	undoSnapshot = nil
	if err := a.store.Load(snap); err != nil {
		return fmt.Errorf("appstate: undo allocation: %w", err)
	}
	a.log.Info("allocation undone")
	return nil
}

// Earmarks returns all persisted earmark records.
func (a *App) Earmarks() []domain.Earmark {
	v, err := a.store.ListEarmarks()
	a.logErr("earmarks", err)
	return v
}

// Cancellations returns every persisted subscription cancellation record.
func (a *App) Cancellations() []domain.SubscriptionCancellation {
	v, err := a.store.ListSubscriptionCancellations()
	a.logErr("subscription cancellations", err)
	return v
}

// MarkSubscriptionCancelled records that the subscription identified by subName
// was cancelled on the given date. If a cancellation record already exists for
// this subscription name, it is updated rather than duplicated (dedupe by
// SubName). subName must not be empty.
func (a *App) MarkSubscriptionCancelled(subName string, on time.Time) error {
	subName = strings.TrimSpace(subName)
	if subName == "" {
		return fmt.Errorf("appstate: subscription name is required")
	}
	// Dedupe: look for an existing record with the same SubName.
	existing, err := a.store.ListSubscriptionCancellations()
	if err != nil {
		return err
	}
	for _, c := range existing {
		if strings.EqualFold(strings.TrimSpace(c.SubName), subName) {
			// Update the existing record in place.
			c.CancelledOn = on
			if err := a.store.PutSubscriptionCancellation(c); err != nil {
				return err
			}
			a.log.Info("subscription cancellation updated", "subName", subName, "cancelledOn", on)
			return nil
		}
	}
	sc := domain.SubscriptionCancellation{
		ID:          id.New(),
		SubName:     subName,
		CancelledOn: on,
	}
	if err := a.store.PutSubscriptionCancellation(sc); err != nil {
		return err
	}
	a.log.Info("subscription marked cancelled", "subName", subName, "cancelledOn", on)
	return nil
}

// UnmarkSubscriptionCancelled removes the cancellation record for subName. It
// is a no-op (and returns nil) if no record exists for that name.
func (a *App) UnmarkSubscriptionCancelled(subName string) error {
	subName = strings.TrimSpace(subName)
	existing, err := a.store.ListSubscriptionCancellations()
	if err != nil {
		return err
	}
	for _, c := range existing {
		if strings.EqualFold(strings.TrimSpace(c.SubName), subName) {
			if _, err := a.store.DeleteSubscriptionCancellation(c.ID); err != nil {
				return err
			}
			a.log.Info("subscription cancellation removed", "subName", subName)
			return nil
		}
	}
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
