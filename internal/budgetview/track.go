// SPDX-License-Identifier: MIT
package budgetview

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
)

// TrackScanMonths is the sheet's history window, in calendar months.
const TrackScanMonths = 12

// TrackRow combines a scanned candidate with a user's unsaved edits.
type TrackRow struct {
	Cand     untracked.Candidate
	Include  bool
	Amount   int64
	BudgetID string
	Raise    bool
}

// ParseTrackRows decodes valid edits; malformed segments are ignored.
func ParseTrackRows(s string) map[string]TrackRow {
	out := map[string]TrackRow{}
	for _, seg := range strings.Split(s, "|") {
		if seg == "" {
			continue
		}
		p := strings.Split(seg, "~")
		if len(p) != 5 {
			continue
		}
		amt, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			continue
		}
		out[p[0]] = TrackRow{
			Include: p[1] == "1", Amount: amt, BudgetID: p[3], Raise: p[4] == "1",
		}
	}
	return out
}

// EncodeTrackRow encodes one row in the existing atom format.
func EncodeTrackRow(catID string, r TrackRow) string {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	return catID + "~" + b(r.Include) + "~" + strconv.FormatInt(r.Amount, 10) + "~" + r.BudgetID + "~" + b(r.Raise)
}

// PutTrackRow replaces one category while preserving other encoded segments.
func PutTrackRow(encoded, catID string, r TrackRow) string {
	var keep []string
	for _, seg := range strings.Split(encoded, "|") {
		if seg == "" || strings.HasPrefix(seg, catID+"~") {
			continue
		}
		keep = append(keep, seg)
	}
	return strings.Join(append(keep, EncodeTrackRow(catID, r)), "|")
}

// ResolveTrackRows applies edits only to candidates still in the scan.
func ResolveTrackRows(cands []untracked.Candidate, edits map[string]TrackRow) []TrackRow {
	rows := make([]TrackRow, 0, len(cands))
	for _, c := range cands {
		r := TrackRow{Cand: c, Include: true, Amount: c.SuggestMinor, Raise: true}
		if e, ok := edits[c.CategoryID]; ok {
			r.Include, r.Amount, r.BudgetID, r.Raise = e.Include, e.Amount, e.BudgetID, e.Raise
		}
		rows = append(rows, r)
	}
	return rows
}

// VisibleDestinations preserves the current member's shared and individual scope.
func VisibleDestinations(budgets []domain.Budget, member string) []domain.Budget {
	var out []domain.Budget
	for _, b := range budgets {
		if member == "" || b.OwnerID == member || b.OwnerID == domain.GroupOwnerID {
			out = append(out, b)
		}
	}
	return out
}

// TrackPreview is the consequence shown before applying the sheet.
type TrackPreview struct {
	Effect    untracked.Effect
	RiskNames []string
}

// PreviewTrack calculates checked totals and the existing-destination risk note.
func PreviewTrack(rows []TrackRow, income, rolled, limit, savings int64, statuses []budgeting.Status, dests []domain.Budget, catNames map[string]string) (TrackPreview, error) {
	var choices []untracked.Choice
	for _, r := range rows {
		if r.Include {
			choices = append(choices, untracked.Choice{CategoryID: r.Cand.CategoryID, AmountMinor: r.Amount, BudgetID: r.BudgetID, Raise: r.Raise})
		}
	}
	pool, err := money.Add(income, rolled)
	if err != nil {
		return TrackPreview{}, fmt.Errorf("track preview: combine income and rollover: %w", err)
	}
	assigned, err := money.Add(limit, savings)
	if err != nil {
		return TrackPreview{}, fmt.Errorf("track preview: combine budgets and savings: %w", err)
	}
	effect, err := untracked.Impact(choices, pool, assigned)
	if err != nil {
		return TrackPreview{}, fmt.Errorf("track preview: calculate plan impact: %w", err)
	}
	spentOf, limitOf := map[string]int64{}, map[string]int64{}
	for _, st := range statuses {
		spentOf[st.Budget.ID], limitOf[st.Budget.ID] = st.Spent.Amount, st.Limit.Amount
	}
	risky, err := untracked.OverspendRisk(choices, func(id string) int64 { return spentOf[id] }, func(id string) int64 { return limitOf[id] })
	if err != nil {
		return TrackPreview{}, fmt.Errorf("track preview: calculate overspend risk: %w", err)
	}
	var names []string
	for _, id := range risky {
		for _, b := range dests {
			if b.ID == id {
				names = append(names, BudgetTitle(b.Name, catNames[b.CategoryID]))
			}
		}
	}
	return TrackPreview{Effect: effect, RiskNames: names}, nil
}

// BudgetTitle includes the category only when it adds information to the name.
func BudgetTitle(name, category string) string {
	switch {
	case name == "":
		return category
	case category != "" && !strings.EqualFold(category, name):
		return name + " \u00b7 " + category
	default:
		return name
	}
}

// PlanTrackUntracked constructs all writes before the caller's atomic save.
// newID supplies identities without hiding randomness in this pure package.
func PlanTrackUntracked(cands []untracked.Candidate, rows map[string]TrackRow, budgets []domain.Budget, base string, rates currency.Rates, activeMemberID string, newID func() string) ([]domain.Budget, []domain.Budget, error) {
	byID := make(map[string]domain.Budget, len(budgets))
	for _, b := range budgets {
		byID[b.ID] = b
	}
	updates := map[string]domain.Budget{}
	var creates []domain.Budget
	for _, c := range cands {
		r := TrackRow{Cand: c, Include: true, Amount: c.SuggestMinor, Raise: true}
		if e, ok := rows[c.CategoryID]; ok {
			r.Include, r.Amount, r.BudgetID, r.Raise = e.Include, e.Amount, e.BudgetID, e.Raise
		}
		if !r.Include || r.Amount <= 0 {
			continue
		}
		if r.BudgetID == "" {
			scope, owner := domain.ScopeShared, domain.GroupOwnerID
			if activeMemberID != "" {
				scope, owner = domain.ScopeIndividual, activeMemberID
			}
			creates = append(creates, domain.Budget{ID: newID(), Name: c.Name, CategoryID: c.CategoryID,
				Limit: money.New(r.Amount, base), Period: c.Period, Scope: scope, OwnerID: owner})
			continue
		}
		b, ok := updates[r.BudgetID]
		if !ok {
			b, ok = byID[r.BudgetID]
		}
		if !ok {
			return nil, nil, fmt.Errorf("track untracked: destination budget %q no longer exists", r.BudgetID)
		}
		tracked := b.TrackedCategoryIDs()
		found := false
		for _, categoryID := range tracked {
			if categoryID == c.CategoryID {
				found = true
				break
			}
		}
		if !found {
			b.CategoryIDs = append(append([]string(nil), tracked...), c.CategoryID)
		}
		if r.Raise {
			budgetCurrency := b.Limit.Currency
			if budgetCurrency == "" {
				budgetCurrency = base
			}
			amount, err := rates.Convert(money.New(r.Amount, base), budgetCurrency)
			if err != nil {
				return nil, nil, fmt.Errorf("track untracked: convert raise for %q: %w", b.Name, err)
			}
			amount, err = budgeting.PeriodEquivalent(amount, c.Period, b.Period)
			if err != nil {
				return nil, nil, fmt.Errorf("track untracked: scale raise for %q: %w", b.Name, err)
			}
			current := b.Limit
			if current.Currency == "" {
				current = money.New(current.Amount, budgetCurrency)
			}
			b.Limit, err = current.Add(amount)
			if err != nil {
				return nil, nil, fmt.Errorf("track untracked: add raise to %q: %w", b.Name, err)
			}
		}
		updates[b.ID] = b
	}
	result := make([]domain.Budget, 0, len(updates))
	for _, b := range budgets {
		if updated, ok := updates[b.ID]; ok {
			result = append(result, updated)
		}
	}
	return creates, result, nil
}

// TrackAmountEdit accepts nonnegative, representable input and otherwise keeps
// the previous amount, including when parsing an incomplete input string.
func TrackAmountEdit(r TrackRow, raw, base string) TrackRow {
	if amount, err := money.ParseMinor(raw, currency.Decimals(base)); err == nil && amount >= 0 {
		r.Amount = amount
	}
	return r
}

// CandidateDate formats the optional last-seen date on a tracked row.
func CandidateDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return dateutil.FormatDate(t)
}

// TrackScanRange preserves the historical calendar-month lookback from the end.
func TrackScanRange(end time.Time) time.Time { return dateutil.AddMonths(end, -TrackScanMonths) }

// ParsePayAnchor decodes the optional ISO date, treating malformed input as unset.
func ParsePayAnchor(raw string) time.Time { t, _ := dateutil.ParseDate(raw); return t }

// ScanUntracked projects the existing historical-median hint into the scanner.
func ScanUntracked(txns []domain.Transaction, ledger budgeting.NettedLedger, cats []domain.Category, budgets []domain.Budget, start, end time.Time, rates currency.Rates, base string, week time.Weekday, payAnchor time.Time) ([]untracked.Candidate, error) {
	hint := func(catID string) (int64, domain.Period, bool) {
		covers := categorytree.DescendantsOfAll(cats, []string{catID})
		b := domain.Budget{ID: "suggestion:" + catID, CategoryID: catID, Period: domain.PeriodMonthly, Limit: money.Zero(rates.Base)}
		rec, err := budgeting.RecommendBudgetLimitWithContext(b, covers, ledger.Transactions(), start, 3, money.Zero(rates.Base), rates, week, payAnchor)
		if err == nil && rec.HasEvidence && rec.Median > 0 {
			return rec.Median, domain.PeriodMonthly, true
		}
		return 0, "", false
	}
	return untracked.Scan(txns, cats, budgets, TrackScanRange(end), end, rates, base, TrackScanMonths, hint)
}

// WithIncluded returns a row with the checkbox's explicit checked value.
func (r TrackRow) WithIncluded(included bool) TrackRow { r.Include = included; return r }

// WithDestination changes only the selected budget identity.
func (r TrackRow) WithDestination(id string) TrackRow { r.BudgetID = id; return r }

// ToggleRaise changes only the existing-budget raise choice.
func (r TrackRow) ToggleRaise() TrackRow { r.Raise = !r.Raise; return r }
