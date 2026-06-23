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
	"math"
	"os"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/logging"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// App holds the live application state.
type App struct {
	store *store.SQLiteStore
	log   *slog.Logger
	ring  *logging.Ring
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
	app := &App{store: st, log: logger, ring: ring, now: time.Now}
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

// ExportJSONRedacted serializes the whole dataset for local persistence with the
// OpenAI key removed, so the secret is never written to localStorage (it stays
// session-only). The manual ExportJSON keeps the key, so a user's own backup is
// complete.
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
// validated write path, returning how many were imported, which rows were
// skipped (per-row parse failures), and any structural error (malformed CSV).
// Missing currencies default to the household base currency, and
// account/category/member cells given as names (a hand-written CSV) are
// resolved to ids (C27). fallbackAccountID is applied to any row whose account
// column is blank or unresolvable — pass "" to keep the previous behavior
// (rows without an account are rejected by the validated write path).
func (a *App) ImportTransactionsCSV(data []byte, fallbackAccountID string) (imported int, skipped []store.CSVRowError, err error) {
	base := "USD"
	if s := a.Settings(); s.BaseCurrency != "" {
		base = s.BaseCurrency
	}
	txns, skipped, err := store.TransactionsFromCSVResilient(data, base)
	if err != nil {
		return 0, nil, err
	}

	accPairs := make([][2]string, 0, len(a.Accounts()))
	for _, ac := range a.Accounts() {
		accPairs = append(accPairs, [2]string{ac.ID, ac.Name})
	}
	catPairs := make([][2]string, 0, len(a.Categories()))
	for _, c := range a.Categories() {
		catPairs = append(catPairs, [2]string{c.ID, c.Name})
	}
	memPairs := make([][2]string, 0, len(a.Members()))
	for _, m := range a.Members() {
		memPairs = append(memPairs, [2]string{m.ID, m.Name})
	}
	resolveAcc, resolveCat, resolveMem := idResolver(accPairs), idResolver(catPairs), idResolver(memPairs)
	for i := range txns {
		txns[i].AccountID = resolveAcc(txns[i].AccountID)
		if txns[i].AccountID == "" && fallbackAccountID != "" {
			txns[i].AccountID = fallbackAccountID
		}
		txns[i].TransferAccountID = resolveAcc(txns[i].TransferAccountID)
		txns[i].CategoryID = resolveCat(txns[i].CategoryID)
		txns[i].MemberID = resolveMem(txns[i].MemberID)
		txns[i] = a.AutoCategorizeTransaction(txns[i])
	}

	n := 0
	// Suspend per-row trigger firing during the bulk import; WithoutTriggers fires
	// the txn-added trigger once afterward instead of once per imported row.
	a.WithoutTriggers(func() {
		for _, t := range txns {
			if putErr := a.PutTransaction(t); putErr == nil {
				n++
			}
		}
	})
	a.log.Info("imported transactions from CSV", "imported", n, "parsed", len(txns), "skipped", len(skipped))
	return n, skipped, nil
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

// Documents returns every imported-document record.
func (a *App) Documents() []domain.Document {
	v, err := a.store.ListDocuments()
	a.logErr("documents", err)
	return v
}

// SavedInsights returns every pinned AI insight.
func (a *App) SavedInsights() []domain.SavedInsight {
	v, err := a.store.ListSavedInsights()
	a.logErr("saved insights", err)
	return v
}

// Recurring returns every scheduled recurring cash flow.
func (a *App) Recurring() []domain.Recurring {
	v, err := a.store.ListRecurring()
	a.logErr("recurring", err)
	return v
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

// DeleteCategory removes a category, first re-homing any sub-categories that
// pointed to it onto its own parent (the grandparent, or the root for a
// top-level category) so deleting a parent never leaves orphaned children with a
// dangling ParentID (L28).
func (a *App) DeleteCategory(id string) error {
	for _, child := range categorytree.ReparentOnDelete(a.Categories(), id) {
		if err := a.store.PutCategory(child); err != nil {
			return fmt.Errorf("appstate: re-home child %q before deleting %q: %w", child.ID, id, err)
		}
	}
	return a.del("category", id, a.store.DeleteCategory)
}

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
		autoRules = append(autoRules, rules.Rule{Match: c.Name, SetCategoryID: c.ID})
	}
	return autoRules
}

// SuggestTransactionFields applies the first matching auto-categorization rule
// to draft fields without overriding a manual category or tags.
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

// AutoCategorizeTransaction applies auto-categorization to a transaction without
// overwriting manual category/tags. Transfers are excluded from categorization.
func (a *App) AutoCategorizeTransaction(t domain.Transaction) domain.Transaction {
	if t.IsTransfer() {
		return t
	}
	t.CategoryID, t.Tags = a.SuggestTransactionFields(t.Payee+" "+t.Desc, t.CategoryID, t.Tags)
	return t
}

// DocumentImportResult reports how many reviewed rows were committed to the
// ledger and how many were skipped as duplicates.
type DocumentImportResult struct {
	Imported   int
	Skipped    int
	DocumentID string
}

// PutDocument saves an imported-document record (needs an ID).
func (a *App) PutDocument(d domain.Document) error {
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
func (a *App) DeleteDocument(id string) error { return a.del("document", id, a.store.DeleteDocument) }

// ImportReviewedDocumentRows commits reviewed document-extraction rows into the
// ledger for an account, skipping same-date/same-amount duplicates and recording
// an import-history document when at least one row is imported.
func (a *App) ImportReviewedDocumentRows(kind domain.DocumentKind, accountID string, rows []extract.Row) (DocumentImportResult, error) {
	var result DocumentImportResult
	acc, ok := domain.AccountByID(a.Accounts(), accountID)
	if !ok {
		return result, fmt.Errorf("appstate: choose an account for document import")
	}
	dec := currency.Decimals(acc.Currency)
	seen := map[string]bool{}
	for _, t := range a.Transactions() {
		if t.AccountID != acc.ID {
			continue
		}
		sig := extract.Row{Date: dateutil.FormatDate(t.Date), Amount: money.FormatMinor(t.Amount.Amount, dec)}.Signature()
		seen[sig] = true
	}
	fresh := extract.FilterNew(rows, seen)
	result.Skipped = len(rows) - len(fresh)

	importedRows := make([]extract.Row, 0, len(fresh))
	a.WithoutTriggers(func() {
		for _, r := range fresh {
			t, ok := a.transactionFromDocumentRow(acc, dec, r)
			if !ok {
				continue
			}
			if err := a.PutTransaction(t); err == nil {
				result.Imported++
				importedRows = append(importedRows, r)
			}
		}
	})
	if result.Imported == 0 {
		return result, nil
	}

	docID := id.New()
	result.DocumentID = docID
	if err := a.PutDocument(domain.Document{
		ID: docID, Kind: kind, UploadedAt: time.Now(), AccountID: acc.ID,
		Status: domain.DocImported, Extracted: documentRowsFromExtract(importedRows),
	}); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) transactionFromDocumentRow(acc domain.Account, decimals int, r extract.Row) (domain.Transaction, bool) {
	amt, err := money.ParseMinor(strings.TrimSpace(r.Amount), decimals)
	if err != nil || amt == 0 {
		return domain.Transaction{}, false
	}
	date, err := dateutil.ParseDate(strings.TrimSpace(r.Date))
	if err != nil {
		date = time.Now()
	}
	desc := strings.TrimSpace(r.Description)
	t := domain.Transaction{
		ID: id.New(), AccountID: acc.ID, Date: date, Desc: desc,
		CategoryID: a.categoryIDForDocumentRow(r.Category), Amount: money.New(amt, acc.Currency),
	}
	return a.AutoCategorizeTransaction(t), true
}

func (a *App) categoryIDForDocumentRow(category string) string {
	aiCat := strings.ToLower(strings.TrimSpace(category))
	if aiCat == "" {
		return ""
	}
	for _, c := range a.Categories() {
		cn := strings.ToLower(c.Name)
		if aiCat == cn || len(cn) >= 3 && (strings.Contains(aiCat, cn) || strings.Contains(cn, aiCat)) {
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
	return a.del("conversation", id, a.store.DeleteConversation)
}

// PutRecurring saves a recurring cash flow. It needs an ID, a label, a currency
// on the amount, and a cadence.
func (a *App) PutRecurring(r domain.Recurring) error {
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
	return nil
}

// RecordBillPayment logs a payment for an upcoming bill (C57), dated today. For a
// liability-account bill (accountID is a real account), it posts a positive
// transaction that reduces the owed balance. For a recurring bill (accountID is
// "recurring:<id>"), it posts the recurring's amount to its account/category and
// advances the recurring's NextDue so it's no longer shown as due.
func (a *App) RecordBillPayment(accountID, name string, amount money.Money) error {
	now := time.Now()
	if rid, ok := strings.CutPrefix(accountID, "recurring:"); ok {
		for _, r := range a.Recurring() {
			if r.ID != rid {
				continue
			}
			if r.AccountID == "" {
				return fmt.Errorf("appstate: recurring %q has no account to post to", r.Label)
			}
			t := domain.Transaction{
				ID: id.New(), AccountID: r.AccountID, CategoryID: r.CategoryID,
				Amount: r.Amount, Date: now, Payee: r.Label, Desc: r.Label,
			}
			if err := a.PutTransaction(t); err != nil {
				return err
			}
			return a.PutRecurring(r.Advance())
		}
		return fmt.Errorf("appstate: recurring bill not found")
	}
	t := domain.Transaction{
		ID: id.New(), AccountID: accountID, Amount: amount, Date: now,
		Payee: name, Desc: "Bill payment: " + name,
	}
	return a.PutTransaction(t)
}

// DeleteRecurring removes a recurring cash flow.
func (a *App) DeleteRecurring(id string) error {
	return a.del("recurring", id, a.store.DeleteRecurring)
}

// PutAllocProfile saves a capital-allocation weight profile (needs an ID and a name).
func (a *App) PutAllocProfile(p domain.AllocationProfile) error {
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
	return a.del("alloc profile", id, a.store.DeleteAllocProfile)
}

// PutFormula saves a custom formula (needs an ID, a name, and an expression).
func (a *App) PutFormula(f domain.Formula) error {
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
func (a *App) DeleteFormula(id string) error { return a.del("formula", id, a.store.DeleteFormula) }

// PutPlan saves a what-if plan (needs an ID, a name, and a positive horizon).
func (a *App) PutPlan(p domain.Plan) error {
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
func (a *App) DeletePlan(id string) error { return a.del("plan", id, a.store.DeletePlan) }

// PutCustomPage saves a user-authored page (needs an ID, a name, and a slug).
func (a *App) PutCustomPage(p domain.CustomPage) error {
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

// DeleteCustomPage removes a user-authored page.
func (a *App) DeleteCustomPage(id string) error {
	return a.del("customPage", id, a.store.DeleteCustomPage)
}

// PutArtifact saves a user artifact (needs an ID, a name, and a kind).
func (a *App) PutArtifact(art domain.Artifact) error {
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
	return a.del("artifact", id, a.store.DeleteArtifact)
}

// PutWorkflow saves a user automation (needs an ID and a name).
func (a *App) PutWorkflow(w workflow.Workflow) error {
	if w.ID == "" {
		return fmt.Errorf("appstate: workflow needs an id")
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("appstate: workflow needs a name")
	}
	if err := a.store.PutWorkflow(w); err != nil {
		return err
	}
	a.log.Info("workflow saved", "id", w.ID)
	return nil
}

// DeleteWorkflow removes a workflow.
func (a *App) DeleteWorkflow(id string) error {
	return a.del("workflow", id, a.store.DeleteWorkflow)
}

// engineVars builds the workflow/widget variable surface from the current dataset.
func (a *App) engineVars() map[string]float64 {
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return engineenv.Vars(engineenv.Data{
		Accounts: a.Accounts(), Transactions: a.Transactions(), Members: a.Members(),
		Budgets: a.Budgets(), Goals: a.Goals(), Tasks: a.Tasks(),
		Rates: currency.Rates{Base: base, Rates: a.Settings().FXRates}, Now: a.clock(),
	})
}

// txnContext builds the workflow context for a transaction-triggered run: the
// engine variable surface plus the triggering transaction's own fields, so a
// condition can reference txn_amount/txn_abs (major units) and txn_payee/txn_desc/
// txn_category/txn_account/txn_tags, and transaction-mutating actions know which
// transaction to change.
func (a *App) txnContext(t domain.Transaction) workflow.Context {
	ctx := workflow.Context{Vars: a.engineVars(), Strs: map[string]string{}, TxnID: t.ID}
	div := 1.0
	for i := 0; i < currency.Decimals(t.Amount.Currency); i++ {
		div *= 10
	}
	amt := float64(t.Amount.Amount) / div
	ctx.Vars["txn_amount"] = amt
	ctx.Vars["txn_abs"] = math.Abs(amt)
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
	effects, matched, err := workflow.Plan(w, ctx)
	if err != nil {
		return workflow.Run{}, err
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

// applyEffect performs one planned effect. Effects are deliberately write-safe and
// never create transactions, so a txn-added workflow can't trigger itself.
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
		_ = a.PutTask(domain.Task{
			ID: id.New(), Title: e.Title, Notes: e.Notes,
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceManual,
		})
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
// or without autopost, are skipped. Returns how many transactions were created.
func (a *App) PostDueRecurring(asOf time.Time) (int, error) {
	posted := 0
	for _, r := range a.Recurring() {
		if !r.Autopost || r.AccountID == "" {
			continue
		}
		changed := false
		for guard := 0; !r.NextDue.After(asOf) && guard < 600; guard++ {
			t := domain.Transaction{
				ID: id.New(), AccountID: r.AccountID, CategoryID: r.CategoryID,
				Date: r.NextDue, Amount: r.Amount, Desc: r.Label,
			}
			if err := a.store.PutTransaction(t); err != nil {
				return posted, err
			}
			posted++
			r = r.Advance()
			changed = true
		}
		if changed {
			if err := a.store.PutRecurring(r); err != nil {
				return posted, err
			}
		}
	}
	if posted > 0 {
		a.log.Info("posted due recurring", "count", posted)
	}
	return posted, nil
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

// isBudgetOver returns true when the given budget is currently in the StateOver
// state according to a full EvaluateRollup against the live dataset.
func (a *App) isBudgetOver(b domain.Budget) bool {
	now := a.clock()
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	cats := a.Categories()
	bs, be := budgeting.PeriodRange(b.Period, now, time.Monday)
	st, err := budgeting.EvaluateRollup(b, a.Transactions(), bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
	return err == nil && st.State == budgeting.StateOver
}

// applyFlagBudgetOver creates an open task for every budget that is currently
// over its limit, skipping any that already have an open task with the same
// title (deduplication).
func (a *App) applyFlagBudgetOver() {
	now := a.clock()
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	cats := a.Categories()
	txns := a.Transactions()
	existing := a.Tasks()
	for _, b := range a.Budgets() {
		bs, be := budgeting.PeriodRange(b.Period, now, time.Monday)
		st, err := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
		if err != nil || st.State != budgeting.StateOver {
			continue
		}
		title := "Budget over limit: " + b.Name
		dup := false
		for _, tk := range existing {
			if tk.Status == domain.StatusOpen && tk.Title == title {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		_ = a.PutTask(domain.Task{
			ID:       id.New(),
			Title:    title,
			Notes:    fmt.Sprintf("Budget %q is at %d%% of its limit.", b.Name, st.Percent),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Source:   domain.SourceManual,
		})
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
		if !r.NextDue.After(asOf) {
			a.RunTriggered(workflow.TriggerBillDue, nil)
			return
		}
	}
}

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
	// Detect whether this is a brand-new transaction (vs. an edit) so the
	// "transaction added" trigger fires only on real additions, from every add
	// path (quick-add, inline add, transfer, duplicate, import), not on edits.
	_, existed, _ := a.store.GetTransaction(t.ID)
	if err := a.store.PutTransaction(t); err != nil {
		return err
	}
	a.log.Info("transaction saved", "id", t.ID)
	if !existed && !a.triggersSuspended {
		a.RunTriggered(workflow.TriggerTxnAdded, &t)
	}
	return nil
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
func (a *App) DeleteTransaction(id string) error {
	return a.del("transaction", id, a.store.DeleteTransaction)
}

// RestoreTransactions upserts each transaction in txns, restoring them to the
// store regardless of whether they currently exist. Deleted transactions are
// re-created and mutated transactions are reverted to the supplied copies.
// This is the undo primitive for bulk operations on the Transactions ledger.
func (a *App) RestoreTransactions(txns []domain.Transaction) error {
	for _, t := range txns {
		if err := a.PutTransaction(t); err != nil {
			return fmt.Errorf("restore transaction %s: %w", t.ID, err)
		}
	}
	return nil
}

// DeleteTransactionWithTransferPair removes a transaction and, when it is one
// leg of a transfer, also removes the reciprocal leg so balances stay paired.
func (a *App) DeleteTransactionWithTransferPair(id string) error {
	all := a.Transactions()
	var target domain.Transaction
	found := false
	for _, t := range all {
		if t.ID == id {
			target, found = t, true
			break
		}
	}
	if err := a.DeleteTransaction(id); err != nil {
		return err
	}
	if !found || !target.IsTransfer() {
		return nil
	}
	for _, t := range all {
		if isReciprocalTransferLeg(target, t) {
			return a.DeleteTransaction(t.ID)
		}
	}
	return nil
}

func isReciprocalTransferLeg(target, candidate domain.Transaction) bool {
	return candidate.ID != target.ID &&
		candidate.IsTransfer() &&
		candidate.AccountID == target.TransferAccountID &&
		candidate.TransferAccountID == target.AccountID &&
		candidate.Amount.Amount == -target.Amount.Amount &&
		candidate.Date.Equal(target.Date)
}

func (a *App) PutBudget(b domain.Budget) error {
	if is := validate.ValidateBudget(b); !is.OK() {
		return is
	}
	if err := a.validateCustom("budget", b.Custom); err != nil {
		return err
	}
	wasOver := false
	for _, existing := range a.Budgets() {
		if existing.ID == b.ID {
			wasOver = a.isBudgetOver(existing)
			break
		}
	}
	if err := a.store.PutBudget(b); err != nil {
		return err
	}
	a.log.Info("budget saved", "id", b.ID)
	if !wasOver && !a.triggersSuspended && a.isBudgetOver(b) {
		a.RunTriggered(workflow.TriggerBudgetExceeded, nil)
	}
	return nil
}
func (a *App) DeleteBudget(id string) error { return a.del("budget", id, a.store.DeleteBudget) }

// CoverBudget moves amt of budgeted money from the source budget's limit to the
// destination's, covering an overspend without changing the household's total
// budgeted amount (the balanced, explainable budgeting.Transfer). Both adjusted
// budgets are persisted. The source must keep a positive limit — a budget with a
// non-positive limit fails validation — so a move that would drain the source is
// rejected.
func (a *App) CoverBudget(fromID, toID string, amt money.Money) error {
	budgets := a.Budgets()
	var from, to domain.Budget
	var haveFrom, haveTo bool
	for _, b := range budgets {
		switch b.ID {
		case fromID:
			from, haveFrom = b, true
		case toID:
			to, haveTo = b, true
		}
	}
	if !haveFrom {
		return fmt.Errorf("appstate: source budget %q not found", fromID)
	}
	if !haveTo {
		return fmt.Errorf("appstate: destination budget %q not found", toID)
	}
	res, err := budgeting.Transfer(from, to, amt, false)
	if err != nil {
		return err
	}
	if !res.From.Limit.IsPositive() {
		return fmt.Errorf("%w: %s would have nothing left", budgeting.ErrInsufficientSource, from.Name)
	}
	if err := a.PutBudget(res.From); err != nil {
		return err
	}
	if err := a.PutBudget(res.To); err != nil {
		return err
	}
	a.log.Info("budget cover", "from", fromID, "to", toID, "amount", amt.String())
	return nil
}

func (a *App) PutGoal(g domain.Goal) error {
	if is := validate.ValidateGoal(g); !is.OK() {
		return is
	}
	if err := a.validateCustom("goal", g.Custom); err != nil {
		return err
	}
	wasComplete := false
	for _, existing := range a.Goals() {
		if existing.ID == g.ID {
			wasComplete = existing.CurrentAmount.Amount >= existing.TargetAmount.Amount && existing.TargetAmount.Amount > 0
			break
		}
	}
	if err := a.store.PutGoal(g); err != nil {
		return err
	}
	a.log.Info("goal saved", "id", g.ID)
	isComplete := g.CurrentAmount.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0
	if !wasComplete && isComplete && !a.triggersSuspended {
		a.RunTriggered(workflow.TriggerGoalReached, nil)
	}
	return nil
}
func (a *App) DeleteGoal(id string) error { return a.del("goal", id, a.store.DeleteGoal) }

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
	if is := validate.ValidateTask(t); !is.OK() {
		return is
	}
	if err := a.store.PutTask(t); err != nil {
		return err
	}
	a.log.Info("task saved", "id", t.ID)
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
func (a *App) DeleteTask(id string) error { return a.del("task", id, a.store.DeleteTask) }

// CompleteTask marks the task identified by id as done and, if the task has a
// non-empty Recurrence, atomically saves a fresh open successor via
// taskrecur.NextOccurrence. nextID must be a freshly-generated ID (e.g.
// id.New()); now is the reference time used when the task has no Due date.
//
// Re-opening a done task (StatusDone → StatusOpen) must go through PutTask
// directly; only the open→done transition spawns a successor.
func (a *App) CompleteTask(taskID, nextID string, now time.Time) error {
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
	if err := a.PutTask(found); err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	if next, spawn := taskrecur.NextOccurrence(found, nextID, now); spawn {
		if err := a.PutTask(next); err != nil {
			return fmt.Errorf("spawn next occurrence: %w", err)
		}
		a.log.Info("spawned recurring task", "from", taskID, "next", next.ID, "due", next.Due)
	}
	return nil
}

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
