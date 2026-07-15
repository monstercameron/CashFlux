// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// --- generic helpers ---

// mutationRev is a process-wide monotonic counter bumped on every successful
// write (putJSON) or delete (deleteRow). Because all entity mutations funnel
// through these two helpers, it is an O(1), always-correct cache key for
// render-time memoization (§1.6): any change to any entity advances it, so a
// memoized derived value (net worth, totals, budget health) recomputes exactly
// when — and only when — the underlying data actually changed. Atomic because
// native tests may touch a store from multiple goroutines.
var mutationRev atomic.Uint64

// MutationRev returns the current global mutation revision (see mutationRev).
func MutationRev() uint64 { return mutationRev.Load() }

// Rev returns the current mutation revision, for callers holding a *SQLiteStore.
func (s *SQLiteStore) Rev() uint64 { return mutationRev.Load() }

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
	mutationRev.Add(1)
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
	if n > 0 {
		mutationRev.Add(1)
	}
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

// --- Balance snapshots (valuation history) ---

// PutBalanceSnapshot persists a single balance snapshot, inserting or replacing by ID.
func (s *SQLiteStore) PutBalanceSnapshot(snap domain.BalanceSnapshot) error {
	return putJSON(s.db, "balance_snapshots", snap.ID, snap)
}

// ListBalanceSnapshots returns every balance snapshot for the given account, in
// insertion order (ascending by ID). Callers may re-sort by AsOf if needed.
func (s *SQLiteStore) ListBalanceSnapshots(accountID string) ([]domain.BalanceSnapshot, error) {
	return queryRows[domain.BalanceSnapshot](s.db,
		"SELECT data FROM balance_snapshots WHERE json_extract(data, '$.accountId') = ? ORDER BY id",
		accountID)
}

// --- Holdings ---

// PutHolding persists a single investment holding, inserting or replacing by ID.
func (s *SQLiteStore) PutHolding(h domain.Holding) error {
	return putJSON(s.db, "holdings", h.ID, h)
}

// GetHolding retrieves one holding by ID. Returns (zero, false, nil) when not found.
func (s *SQLiteStore) GetHolding(id string) (domain.Holding, bool, error) {
	return getJSON[domain.Holding](s.db, "holdings", id)
}

// DeleteHolding removes a holding by ID. Returns true if a row was deleted.
func (s *SQLiteStore) DeleteHolding(id string) (bool, error) {
	return deleteRow(s.db, "holdings", id)
}

// ListHoldings returns every persisted holding in insertion order.
func (s *SQLiteStore) ListHoldings() ([]domain.Holding, error) {
	return loadRows[domain.Holding](s.db, "holdings")
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
	rs, err := loadRows[rules.Rule](s.db, "rules")
	if err != nil {
		return nil, err
	}
	// Precedence is the user-controlled Order (lower runs first); ties fall back to
	// id for a stable order. First matching rule wins (internal/rules.FirstMatch).
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Order != rs[j].Order {
			return rs[i].Order < rs[j].Order
		}
		return rs[i].ID < rs[j].ID
	})
	return rs, nil
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

// --- Conversations (saved Insights chats) ---

func (s *SQLiteStore) PutConversation(c domain.Conversation) error {
	return putJSON(s.db, "conversations", c.ID, c)
}
func (s *SQLiteStore) GetConversation(id string) (domain.Conversation, bool, error) {
	return getJSON[domain.Conversation](s.db, "conversations", id)
}
func (s *SQLiteStore) DeleteConversation(id string) (bool, error) {
	return deleteRow(s.db, "conversations", id)
}
func (s *SQLiteStore) ListConversations() ([]domain.Conversation, error) {
	return loadRows[domain.Conversation](s.db, "conversations")
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

// --- Allocation profiles (saved capital-allocation weight mixes) ---

func (s *SQLiteStore) PutAllocProfile(p domain.AllocationProfile) error {
	return putJSON(s.db, "allocprofiles", p.ID, p)
}
func (s *SQLiteStore) GetAllocProfile(id string) (domain.AllocationProfile, bool, error) {
	return getJSON[domain.AllocationProfile](s.db, "allocprofiles", id)
}
func (s *SQLiteStore) DeleteAllocProfile(id string) (bool, error) {
	return deleteRow(s.db, "allocprofiles", id)
}
func (s *SQLiteStore) ListAllocProfiles() ([]domain.AllocationProfile, error) {
	return loadRows[domain.AllocationProfile](s.db, "allocprofiles")
}

// --- Saved formulas (custom calculations) ---

func (s *SQLiteStore) PutFormula(f domain.Formula) error {
	return putJSON(s.db, "formulas", f.ID, f)
}
func (s *SQLiteStore) GetFormula(id string) (domain.Formula, bool, error) {
	return getJSON[domain.Formula](s.db, "formulas", id)
}
func (s *SQLiteStore) DeleteFormula(id string) (bool, error) {
	return deleteRow(s.db, "formulas", id)
}
func (s *SQLiteStore) ListFormulas() ([]domain.Formula, error) {
	return loadRows[domain.Formula](s.db, "formulas")
}

// --- Plans (saved what-if scenarios) ---

func (s *SQLiteStore) PutPlan(p domain.Plan) error {
	return putJSON(s.db, "plans", p.ID, p)
}
func (s *SQLiteStore) GetPlan(id string) (domain.Plan, bool, error) {
	return getJSON[domain.Plan](s.db, "plans", id)
}
func (s *SQLiteStore) DeletePlan(id string) (bool, error) {
	return deleteRow(s.db, "plans", id)
}
func (s *SQLiteStore) ListPlans() ([]domain.Plan, error) {
	return loadRows[domain.Plan](s.db, "plans")
}

// --- Custom pages (user-authored pages of custom widgets) ---

func (s *SQLiteStore) PutCustomPage(p domain.CustomPage) error {
	return putJSON(s.db, "custompages", p.ID, p)
}
func (s *SQLiteStore) GetCustomPage(id string) (domain.CustomPage, bool, error) {
	return getJSON[domain.CustomPage](s.db, "custompages", id)
}
func (s *SQLiteStore) DeleteCustomPage(id string) (bool, error) {
	return deleteRow(s.db, "custompages", id)
}
func (s *SQLiteStore) ListCustomPages() ([]domain.CustomPage, error) {
	return loadRows[domain.CustomPage](s.db, "custompages")
}

// --- Widget placements (unified widget instances on a surface) ---

// placementKey is the row key for a placement: "<surface>/<id>", so the same
// widget id can be placed on different surfaces without colliding.
func placementKey(p domain.Placement) string { return p.Surface + "/" + p.ID }

// PutPlacement upserts one placement.
func (s *SQLiteStore) PutPlacement(p domain.Placement) error {
	return putJSON(s.db, "placements", placementKey(p), p)
}

// DeletePlacement removes one placement.
func (s *SQLiteStore) DeletePlacement(p domain.Placement) (bool, error) {
	return deleteRow(s.db, "placements", placementKey(p))
}

// ListPlacements returns every persisted placement across all surfaces.
func (s *SQLiteStore) ListPlacements() ([]domain.Placement, error) {
	return loadRows[domain.Placement](s.db, "placements")
}

// PlacementsForSurface returns the persisted placements for one surface.
func (s *SQLiteStore) PlacementsForSurface(surface string) ([]domain.Placement, error) {
	all, err := s.ListPlacements()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(all))
	for _, p := range all {
		if p.Surface == surface {
			out = append(out, p)
		}
	}
	return out, nil
}

// --- Molecules (compound engine-variable formula definitions) ---

// PutMolecule upserts a compound-variable definition (keyed by Name).
func (s *SQLiteStore) PutMolecule(m domain.Molecule) error {
	return putJSON(s.db, "molecules", m.Name, m)
}

// DeleteMolecule removes a molecule definition.
func (s *SQLiteStore) DeleteMolecule(name string) (bool, error) {
	return deleteRow(s.db, "molecules", name)
}

// ListMolecules returns every persisted molecule definition.
func (s *SQLiteStore) ListMolecules() ([]domain.Molecule, error) {
	return loadRows[domain.Molecule](s.db, "molecules")
}

// --- Artifacts (user-stored images and datasets) ---

func (s *SQLiteStore) PutArtifact(a domain.Artifact) error {
	return putJSON(s.db, "artifacts", a.ID, a)
}
func (s *SQLiteStore) GetArtifact(id string) (domain.Artifact, bool, error) {
	return getJSON[domain.Artifact](s.db, "artifacts", id)
}
func (s *SQLiteStore) DeleteArtifact(id string) (bool, error) {
	return deleteRow(s.db, "artifacts", id)
}
func (s *SQLiteStore) ListArtifacts() ([]domain.Artifact, error) {
	return loadRows[domain.Artifact](s.db, "artifacts")
}

// --- Workflows + run history ---

func (s *SQLiteStore) PutWorkflow(w workflow.Workflow) error {
	return putJSON(s.db, "workflows", w.ID, w)
}
func (s *SQLiteStore) GetWorkflow(id string) (workflow.Workflow, bool, error) {
	return getJSON[workflow.Workflow](s.db, "workflows", id)
}
func (s *SQLiteStore) DeleteWorkflow(id string) (bool, error) {
	return deleteRow(s.db, "workflows", id)
}
func (s *SQLiteStore) ListWorkflows() ([]workflow.Workflow, error) {
	return loadRows[workflow.Workflow](s.db, "workflows")
}

func (s *SQLiteStore) PutWorkflowRun(r workflow.Run) error {
	return putJSON(s.db, "workflowruns", r.ID, r)
}
func (s *SQLiteStore) DeleteWorkflowRun(id string) (bool, error) {
	return deleteRow(s.db, "workflowruns", id)
}
func (s *SQLiteStore) ListWorkflowRuns() ([]workflow.Run, error) {
	return loadRows[workflow.Run](s.db, "workflowruns")
}

// --- Earmarks (allocation commitments without cash movement) ---

func (s *SQLiteStore) PutEarmark(e domain.Earmark) error {
	return putJSON(s.db, "earmarks", e.ID, e)
}
func (s *SQLiteStore) GetEarmark(id string) (domain.Earmark, bool, error) {
	return getJSON[domain.Earmark](s.db, "earmarks", id)
}
func (s *SQLiteStore) DeleteEarmark(id string) (bool, error) {
	return deleteRow(s.db, "earmarks", id)
}
func (s *SQLiteStore) ListEarmarks() ([]domain.Earmark, error) {
	return loadRows[domain.Earmark](s.db, "earmarks")
}

// --- Transaction links (order groups, refund pairs — XC0b) ---

// PutTxnLink inserts or updates a transaction-link row by id.
func (s *SQLiteStore) PutTxnLink(l domain.TxnLink) error {
	return putJSON(s.db, "txnlinks", l.ID, l)
}

// GetTxnLink returns the transaction link with the given id.
func (s *SQLiteStore) GetTxnLink(id string) (domain.TxnLink, bool, error) {
	return getJSON[domain.TxnLink](s.db, "txnlinks", id)
}

// DeleteTxnLink removes a transaction-link row by id (releasing its members;
// the transactions themselves are never touched).
func (s *SQLiteStore) DeleteTxnLink(id string) (bool, error) {
	return deleteRow(s.db, "txnlinks", id)
}

// ListTxnLinks returns all transaction-link rows.
func (s *SQLiteStore) ListTxnLinks() ([]domain.TxnLink, error) {
	return loadRows[domain.TxnLink](s.db, "txnlinks")
}

// --- Payee aliases (merchant-name cleanup — TX1) ---

// PutPayeeAlias inserts or updates a payee-alias row by id.
func (s *SQLiteStore) PutPayeeAlias(p domain.PayeeAlias) error {
	return putJSON(s.db, "payeealiases", p.ID, p)
}

// GetPayeeAlias returns the payee alias with the given id.
func (s *SQLiteStore) GetPayeeAlias(id string) (domain.PayeeAlias, bool, error) {
	return getJSON[domain.PayeeAlias](s.db, "payeealiases", id)
}

// DeletePayeeAlias removes a payee-alias row by id.
func (s *SQLiteStore) DeletePayeeAlias(id string) (bool, error) {
	return deleteRow(s.db, "payeealiases", id)
}

// ListPayeeAliases returns all payee-alias rows.
func (s *SQLiteStore) ListPayeeAliases() ([]domain.PayeeAlias, error) {
	return loadRows[domain.PayeeAlias](s.db, "payeealiases")
}

// --- Subscription cancellations ---

func (s *SQLiteStore) PutSubscriptionCancellation(c domain.SubscriptionCancellation) error {
	return putJSON(s.db, "subcancellations", c.ID, c)
}
func (s *SQLiteStore) GetSubscriptionCancellation(id string) (domain.SubscriptionCancellation, bool, error) {
	return getJSON[domain.SubscriptionCancellation](s.db, "subcancellations", id)
}
func (s *SQLiteStore) DeleteSubscriptionCancellation(id string) (bool, error) {
	return deleteRow(s.db, "subcancellations", id)
}
func (s *SQLiteStore) ListSubscriptionCancellations() ([]domain.SubscriptionCancellation, error) {
	return loadRows[domain.SubscriptionCancellation](s.db, "subcancellations")
}

// --- Subscription ignores ---

// PutSubscriptionIgnore upserts a subscription-ignore record by its ID.
func (s *SQLiteStore) PutSubscriptionIgnore(ig domain.SubscriptionIgnore) error {
	return putJSON(s.db, "subignores", ig.ID, ig)
}

// GetSubscriptionIgnore retrieves a subscription-ignore record by its ID.
func (s *SQLiteStore) GetSubscriptionIgnore(id string) (domain.SubscriptionIgnore, bool, error) {
	return getJSON[domain.SubscriptionIgnore](s.db, "subignores", id)
}

// DeleteSubscriptionIgnore removes a subscription-ignore record by its ID.
func (s *SQLiteStore) DeleteSubscriptionIgnore(id string) (bool, error) {
	return deleteRow(s.db, "subignores", id)
}

// ListSubscriptionIgnores returns every persisted subscription-ignore record.
func (s *SQLiteStore) ListSubscriptionIgnores() ([]domain.SubscriptionIgnore, error) {
	return loadRows[domain.SubscriptionIgnore](s.db, "subignores")
}

// --- Shared expenses + settlements (the roommate "settle up" ledger) ---

func (s *SQLiteStore) PutSharedExpense(e domain.SharedExpense) error {
	return putJSON(s.db, "sharedexpenses", e.ID, e)
}
func (s *SQLiteStore) GetSharedExpense(id string) (domain.SharedExpense, bool, error) {
	return getJSON[domain.SharedExpense](s.db, "sharedexpenses", id)
}
func (s *SQLiteStore) DeleteSharedExpense(id string) (bool, error) {
	return deleteRow(s.db, "sharedexpenses", id)
}
func (s *SQLiteStore) ListSharedExpenses() ([]domain.SharedExpense, error) {
	return loadRows[domain.SharedExpense](s.db, "sharedexpenses")
}

func (s *SQLiteStore) PutSettlement(st domain.Settlement) error {
	return putJSON(s.db, "settlements", st.ID, st)
}
func (s *SQLiteStore) GetSettlement(id string) (domain.Settlement, bool, error) {
	return getJSON[domain.Settlement](s.db, "settlements", id)
}
func (s *SQLiteStore) DeleteSettlement(id string) (bool, error) {
	return deleteRow(s.db, "settlements", id)
}
func (s *SQLiteStore) ListSettlements() ([]domain.Settlement, error) {
	return loadRows[domain.Settlement](s.db, "settlements")
}

// --- Audit Log ---

// PutAuditEntry inserts or replaces one audit-log entry. The entry must have a
// non-empty ID. Callers are responsible for ensuring the summary has been passed
// through auditlog.Redact before storage.
func (s *SQLiteStore) PutAuditEntry(e auditlog.Entry) error {
	return putJSON(s.db, "audit_log", e.ID, e)
}

// ListAuditEntries returns at most limit entries in reverse-chronological order
// (newest first). If limit ≤ 0 all stored entries are returned.
func (s *SQLiteStore) ListAuditEntries(limit int) ([]auditlog.Entry, error) {
	entries, err := loadRows[auditlog.Entry](s.db, "audit_log")
	if err != nil {
		return nil, err
	}
	// Reverse for newest-first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	return entries, nil
}
