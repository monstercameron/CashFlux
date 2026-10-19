// SPDX-License-Identifier: MIT

package txntable

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

// NativeSpec describes a fixed native transaction tile.
func NativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}

// TableSpec describes the frame-backed ledger tile.
func TableSpec() domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "txn-table", Kind: domain.KindTable,
		Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions-full"}}}
}

// SurfaceSpecs turns the chosen ordered placements into widget specifications.
func SurfaceSpecs(ids []string) []domain.WidgetSpec {
	specs := make([]domain.WidgetSpec, 0, len(ids))
	for _, id := range ids {
		if id == "txn-table" {
			specs = append(specs, TableSpec())
		} else {
			specs = append(specs, NativeSpec(id))
		}
	}
	return specs
}

// PreviewArtifact resolves the requested artifact without exposing a pointer into the input slice.
func PreviewArtifact(id string, artifacts []domain.Artifact) *domain.Artifact {
	for _, artifact := range artifacts {
		if artifact.ID == id {
			return &artifact
		}
	}
	return nil
}

// DeletePrompt names every resolved transfer leg and any unfinished linked tasks.
func DeletePrompt(ids []string, rows []domain.Transaction, accounts []domain.Account, linked int, labels Labels) string {
	byID := Transactions(rows)
	names := map[string]string{}
	for _, account := range accounts {
		names[account.ID] = account.Name
	}
	prompts := make([]string, 0, len(ids))
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			continue
		}
		name := row.Payee
		if name == "" {
			name = row.Desc
		}
		prompts = append(prompts, labels.T("transactions.deleteConfirmDetail", name, dateutil.FormatDate(row.Date), labels.Money(row.Amount), names[row.AccountID]))
	}
	result := strings.Join(prompts, " ")
	if linked > 0 {
		result += " " + labels.T("transactions.deleteConfirmTasks", linked)
	}
	return result
}

// SuggestedCategory accepts only medium-or-better proposals compatible with the row's direction.
func SuggestedCategory(row domain.Transaction, suggestion catsuggest.Suggestion, found bool, categories []domain.Category, names map[string]string) string {
	if found && suggestion.Confidence >= catsuggest.ConfMedium && smartcatview.MatchesDirection(row, suggestion.CategoryID, categories) {
		return names[suggestion.CategoryID]
	}
	return ""
}

// TagFilterNotice reports an action only when the result count would otherwise look unchanged.
func TagFilterNotice(previous, next, effective txnfilter.Criteria, rows []domain.Transaction, labels txnfilter.Labels, shown int) bool {
	if txnfilter.ScopeChanged(previous, next) {
		result, _ := txnfilter.ApplyWithLabelsChecked(rows, effective, labels)
		if len(result) != shown {
			return false
		}
	}
	return true
}

// Fact is one existing secondary table fact, with separate CSS and test identities.
type Fact struct{ Class, TestID, Text string }

// CountFacts derives existing count, transfer, conversion and exclusion labels.
// The transaction count formatter is supplied by the toolbar's presentation edge.
func CountFacts(rows []domain.Transaction, base string, rates map[string]float64, cached []txnview.View, labels Labels, countLabel func(int) string) []Fact {
	facts := []Fact{{Class: "txn-count-fact", TestID: "txn-count-transaction-total", Text: countLabel(len(rows))}}
	var view txnview.View
	if len(cached) == 1 {
		view = cached[0]
	} else {
		view, _ = txnview.Summarize(rows, currency.Rates{Base: base, Rates: rates}, base)
	}
	if view.Transferred > 0 {
		facts = append(facts, Fact{Class: "txn-count-fact", TestID: "txn-count-moved", Text: labels.T("transactions.scopeMoved", labels.Money(money.New(view.Transferred, base)))})
	}
	if view.UnpairedRows > 0 {
		facts = append(facts, Fact{Class: "txn-count-fact txn-count-unpaired", Text: labels.T("transactions.scopeUnpaired", view.UnpairedRows)})
	}
	if view.Unconverted > 0 {
		facts = append(facts, Fact{Class: "txn-count-fact", TestID: "txn-count-unconverted", Text: labels.T("transactions.scopeUnconverted", view.Unconverted)})
	}
	if view.Excluded > 0 {
		facts = append(facts, Fact{Class: "txn-count-fact", TestID: "txn-count-excluded", Text: labels.T("transactions.scopeExcluded", view.Excluded)})
	}
	return facts
}

// OwnerLabel provides the existing unassigned, single-owner and shared labels.
func OwnerLabel(names []string, t Translator) string {
	switch len(names) {
	case 0:
		return t("who2.unassigned")
	case 1:
		return names[0]
	case 2:
		return t("who2.sharedBy", names[0]+" and "+names[1])
	}
	return t("who2.sharedBy", strings.Join(names[:len(names)-1], ", ")+", and "+names[len(names)-1])
}

// LegacyOwnerLabel supports old direct row callers that supply a single owner.
func LegacyOwnerLabel(member string, t Translator) (string, bool) {
	if strings.TrimSpace(member) != "" {
		return t("txu5.member", member), true
	}
	return t("r4c.unassigned"), false
}
