// SPDX-License-Identifier: MIT

package syncengine

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func unionByID[T any](base, overlay []T, idOf func(T) string) []T {
	if len(base) == 0 {
		return overlay
	}
	if len(overlay) == 0 {
		return base
	}
	index := make(map[string]int, len(base)+len(overlay))
	out := make([]T, 0, len(base)+len(overlay))
	for _, item := range base {
		k := idOf(item)
		if k != "" {
			index[k] = len(out)
		}
		out = append(out, item)
	}
	for _, item := range overlay {
		k := idOf(item)
		if k != "" {
			if pos, exists := index[k]; exists {
				out[pos] = item
				continue
			}
			index[k] = len(out)
		}
		out = append(out, item)
	}
	return out
}

func mergeMap(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func mergeTypedDatasetsFull(base, overlay store.Dataset) store.Dataset {
	out := overlay
	if base.SchemaVersion > out.SchemaVersion {
		out.SchemaVersion = base.SchemaVersion
	}
	out.Members = unionByID(base.Members, overlay.Members, func(v domain.Member) string { return v.ID })
	out.Accounts = unionByID(base.Accounts, overlay.Accounts, func(v domain.Account) string { return v.ID })
	out.Categories = unionByID(base.Categories, overlay.Categories, func(v domain.Category) string { return v.ID })
	out.Transactions = unionByID(base.Transactions, overlay.Transactions, func(v domain.Transaction) string { return v.ID })
	out.Budgets = unionByID(base.Budgets, overlay.Budgets, func(v domain.Budget) string { return v.ID })
	out.Goals = unionByID(base.Goals, overlay.Goals, func(v domain.Goal) string { return v.ID })
	out.Holdings = unionByID(base.Holdings, overlay.Holdings, func(v domain.Holding) string { return v.ID })
	out.RealizedSales = unionByID(base.RealizedSales, overlay.RealizedSales, func(v domain.RealizedSale) string { return v.ID })
	out.BalanceSnapshots = unionByID(base.BalanceSnapshots, overlay.BalanceSnapshots, func(v domain.BalanceSnapshot) string { return v.ID })
	out.Tasks = unionByID(base.Tasks, overlay.Tasks, func(v domain.Task) string { return v.ID })
	out.CustomFields = unionByID(base.CustomFields, overlay.CustomFields, func(v customfields.Def) string { return v.ID })
	out.Rules = unionByID(base.Rules, overlay.Rules, func(v rules.Rule) string { return v.ID })
	out.Documents = unionByID(base.Documents, overlay.Documents, func(v domain.Document) string { return v.ID })
	out.SavedInsights = unionByID(base.SavedInsights, overlay.SavedInsights, func(v domain.SavedInsight) string { return v.ID })
	out.Conversations = unionByID(base.Conversations, overlay.Conversations, func(v domain.Conversation) string { return v.ID })
	out.Recurring = unionByID(base.Recurring, overlay.Recurring, func(v domain.Recurring) string { return v.ID })
	out.AllocProfiles = unionByID(base.AllocProfiles, overlay.AllocProfiles, func(v domain.AllocationProfile) string { return v.ID })
	out.Formulas = unionByID(base.Formulas, overlay.Formulas, func(v domain.Formula) string { return v.ID })
	out.Plans = unionByID(base.Plans, overlay.Plans, func(v domain.Plan) string { return v.ID })
	out.CustomPages = unionByID(base.CustomPages, overlay.CustomPages, func(v domain.CustomPage) string { return v.ID })
	out.Placements = unionByID(base.Placements, overlay.Placements, func(v domain.Placement) string { return v.ID })
	out.Molecules = unionByID(base.Molecules, overlay.Molecules, func(v domain.Molecule) string { return v.Name })
	out.Artifacts = unionByID(base.Artifacts, overlay.Artifacts, func(v domain.Artifact) string { return v.ID })
	out.Workflows = unionByID(base.Workflows, overlay.Workflows, func(v workflow.Workflow) string { return v.ID })
	out.WorkflowRuns = unionByID(base.WorkflowRuns, overlay.WorkflowRuns, func(v workflow.Run) string { return v.ID })
	out.SharedExpenses = unionByID(base.SharedExpenses, overlay.SharedExpenses, func(v domain.SharedExpense) string { return v.ID })
	out.Settlements = unionByID(base.Settlements, overlay.Settlements, func(v domain.Settlement) string { return v.ID })
	out.Earmarks = unionByID(base.Earmarks, overlay.Earmarks, func(v domain.Earmark) string { return v.ID })
	out.TxnLinks = unionByID(base.TxnLinks, overlay.TxnLinks, func(v domain.TxnLink) string { return v.ID })
	out.Events = unionByID(base.Events, overlay.Events, func(v domain.Event) string { return v.ID })
	out.AccountGroups = unionByID(base.AccountGroups, overlay.AccountGroups, func(v domain.AccountGroup) string { return v.ID })
	out.SweepRules = unionByID(base.SweepRules, overlay.SweepRules, func(v domain.SweepRule) string { return v.ID })
	out.Institutions = unionByID(base.Institutions, overlay.Institutions, func(v domain.Institution) string { return v.ID })
	out.PayeeAliases = unionByID(base.PayeeAliases, overlay.PayeeAliases, func(v domain.PayeeAlias) string { return v.ID })
	out.SubscriptionIgnores = unionByID(base.SubscriptionIgnores, overlay.SubscriptionIgnores, func(v domain.SubscriptionIgnore) string { return v.ID })
	out.SubscriptionCancellations = unionByID(base.SubscriptionCancellations, overlay.SubscriptionCancellations, func(v domain.SubscriptionCancellation) string { return v.ID })
	out.DebtPaymentLinks = unionByID(base.DebtPaymentLinks, overlay.DebtPaymentLinks, func(v domain.DebtPaymentLink) string { return v.ID })
	out.DebtPayeeRules = unionByID(base.DebtPayeeRules, overlay.DebtPayeeRules, func(v domain.DebtPayeeRule) string { return v.ID })
	out.AuditEntries = unionByID(base.AuditEntries, overlay.AuditEntries, func(v auditlog.Entry) string { return v.ID })
	out.TransferOperations = unionByID(base.TransferOperations, overlay.TransferOperations, func(v store.TransferOperationReceipt) string { return v.ID })
	out.KV = mergeMap(base.KV, overlay.KV)
	out.SettingsKV = mergeMap(base.SettingsKV, overlay.SettingsKV)
	if len(base.Settings.AISpend) > 0 || len(overlay.Settings.AISpend) > 0 {
		out.Settings.AISpend = unionByID(base.Settings.AISpend, overlay.Settings.AISpend, func(v aispend.Bucket) string { return v.Month.Format("2006-01") + "/" + v.Feature })
	}
	return out
}

// DatasetToCheckpointSnapshot projects a decrypted store.Dataset JSON onto a
// deterministic conflictcheckpoint.Snapshot and entity counts so the conflict
// UI can display exact human-readable diffs between local and server copies.
func DatasetToCheckpointSnapshot(rawJSON []byte) (conflictcheckpoint.Snapshot, map[string]int, error) {
	if len(rawJSON) == 0 {
		return conflictcheckpoint.Snapshot{Available: false}, nil, nil
	}
	ds, err := store.Import(rawJSON)
	if err != nil {
		return conflictcheckpoint.Snapshot{Available: false}, nil, fmt.Errorf("syncengine: decode dataset for checkpoint: %w", err)
	}
	counts := map[string]int{
		"accounts":     len(ds.Accounts),
		"categories":   len(ds.Categories),
		"transactions": len(ds.Transactions),
		"budgets":      len(ds.Budgets),
		"goals":        len(ds.Goals),
		"tasks":        len(ds.Tasks),
		"recurring":    len(ds.Recurring),
		"artifacts":    len(ds.Artifacts),
	}
	var records []conflictcheckpoint.Record
	for _, a := range ds.Accounts {
		records = append(records, conflictcheckpoint.Record{
			Entity: "account",
			ID:     a.ID,
			Label:  strings.TrimSpace(a.Name),
			Values: map[string]string{
				"name":     a.Name,
				"class":    string(a.Class),
				"type":     string(a.Type),
				"currency": a.Currency,
				"archived": fmt.Sprintf("%t", a.Archived),
			},
		})
	}
	for _, c := range ds.Categories {
		records = append(records, conflictcheckpoint.Record{
			Entity: "category",
			ID:     c.ID,
			Label:  strings.TrimSpace(c.Name),
			Values: map[string]string{
				"name":   c.Name,
				"parent": c.ParentID,
				"kind":   string(c.Kind),
			},
		})
	}
	for _, tx := range ds.Transactions {
		label := strings.TrimSpace(tx.Payee)
		if label == "" {
			label = strings.TrimSpace(tx.Desc)
		}
		if label == "" {
			label = tx.ID
		}
		records = append(records, conflictcheckpoint.Record{
			Entity: "transaction",
			ID:     tx.ID,
			Label:  fmt.Sprintf("%s (%s, %d %s)", label, tx.Date.Format("2006-01-02"), tx.Amount.Amount, tx.Amount.Currency),
			Values: map[string]string{
				"date":     tx.Date.Format("2006-01-02"),
				"amount":   fmt.Sprintf("%d", tx.Amount.Amount),
				"currency": tx.Amount.Currency,
				"payee":    tx.Payee,
				"desc":     tx.Desc,
				"account":  tx.AccountID,
				"category": tx.CategoryID,
				"cleared":  fmt.Sprintf("%t", tx.Cleared),
			},
		})
	}
	for _, b := range ds.Budgets {
		records = append(records, conflictcheckpoint.Record{
			Entity: "budget",
			ID:     b.ID,
			Label:  fmt.Sprintf("%s (%d %s)", b.Name, b.Limit.Amount, b.Limit.Currency),
			Values: map[string]string{
				"name":     b.Name,
				"category": b.CategoryID,
				"limit":    fmt.Sprintf("%d", b.Limit.Amount),
				"currency": b.Limit.Currency,
				"period":   string(b.Period),
			},
		})
	}
	for _, g := range ds.Goals {
		records = append(records, conflictcheckpoint.Record{
			Entity: "goal",
			ID:     g.ID,
			Label:  strings.TrimSpace(g.Name),
			Values: map[string]string{
				"name":    g.Name,
				"target":  fmt.Sprintf("%d", g.TargetAmount.Amount),
				"current": fmt.Sprintf("%d", g.CurrentAmount.Amount),
			},
		})
	}
	for _, tk := range ds.Tasks {
		records = append(records, conflictcheckpoint.Record{
			Entity: "task",
			ID:     tk.ID,
			Label:  strings.TrimSpace(tk.Title),
			Values: map[string]string{
				"title":  tk.Title,
				"status": string(tk.Status),
			},
		})
	}
	return conflictcheckpoint.Snapshot{Available: true, Records: records}, counts, nil
}
