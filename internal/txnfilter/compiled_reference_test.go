// SPDX-License-Identifier: MIT

package txnfilter

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// legacyCompiled freezes the matcher before CSV selections and numeric search
// were prepared. It is a test/benchmark oracle, never a production fallback.
type legacyCompiled struct {
	c              Criteria
	labels         Labels
	text, tag      string
	min, max       int64
	hasMin, hasMax bool
	from, to       time.Time
}

func legacyCompile(c Criteria, labels Labels) legacyCompiled {
	f := legacyCompiled{c: c.Normalize(), labels: labels, text: strings.ToLower(strings.TrimSpace(c.Text)), tag: strings.ToLower(strings.TrimSpace(c.Tag))}
	f.min, f.hasMin = parseAmountBound(c.AmountMin, labels.BaseCurrency)
	f.max, f.hasMax = parseAmountBound(c.AmountMax, labels.BaseCurrency)
	f.from, _ = dateutil.ParseDate(strings.TrimSpace(c.From))
	f.to, _ = dateutil.ParseDate(strings.TrimSpace(c.To))
	return f
}

func legacyCSVAccount(csv string, t domain.Transaction) bool {
	for _, id := range splitCSV(csv) {
		if t.AccountID == id || t.BillAccountID == id {
			return true
		}
	}
	return false
}

func legacyCSVTags(csv string, t domain.Transaction) bool {
	for _, tag := range splitCSV(csv) {
		if hasTag(t, strings.ToLower(strings.TrimSpace(tag))) {
			return true
		}
	}
	return false
}

func legacyMembers(csv string, t domain.Transaction) bool {
	for _, id := range splitCSV(csv) {
		if id == UnassignedMemberID {
			if t.IsUnassigned() {
				return true
			}
			continue
		}
		if t.HasOwner(id) {
			return true
		}
	}
	return false
}

func (f legacyCompiled) scopeAny(t domain.Transaction) bool {
	c := f.c
	categorySelected, tagSelected := c.Categories != "" || c.Category != "", c.Tags != "" || f.tag != ""
	if !categorySelected && !tagSelected {
		return true
	}
	if categorySelected && matchesCategory(t, func(id string) bool {
		if c.Categories != "" {
			return csvHas(c.Categories, id)
		}
		return id == c.Category
	}) {
		return true
	}
	if tagSelected {
		if c.Tags != "" {
			return legacyCSVTags(c.Tags, t)
		}
		return hasTag(t, f.tag)
	}
	return false
}

func (f legacyCompiled) matches(t domain.Transaction) bool {
	c, labels := f.c, f.labels
	if c.Accounts != "" && !legacyCSVAccount(c.Accounts, t) || c.Accounts == "" && c.Account != "" && t.AccountID != c.Account && t.BillAccountID != c.Account {
		return false
	}
	if c.BillAccount != "" && t.BillAccountID != c.BillAccount || c.Subscription != "" && t.SubscriptionName != c.Subscription {
		return false
	}
	if !c.ScopeAny && c.Categories != "" && !matchesCategory(t, func(id string) bool { return csvHas(c.Categories, id) }) {
		return false
	}
	if !c.ScopeAny && c.Categories == "" && c.Category != "" && !matchesCategory(t, func(id string) bool { return id == c.Category }) {
		return false
	}
	if c.Uncategorized && (!hasUncategorizedCategory(t) || t.IsTransfer()) {
		return false
	}
	if c.Members != "" && !legacyMembers(c.Members, t) || c.Members == "" && c.Member != "" && !legacyMembers(c.Member, t) {
		return false
	}
	if c.Sources != "" && !csvHas(c.Sources, string(t.Source)) || c.Sources == "" && c.Source != "" && string(t.Source) != c.Source {
		return false
	}
	if !c.ScopeAny && c.Tags != "" && !legacyCSVTags(c.Tags, t) || !c.ScopeAny && c.Tags == "" && f.tag != "" && !hasTag(t, f.tag) {
		return false
	}
	if c.ScopeAny && !f.scopeAny(t) {
		return false
	}
	if !f.from.IsZero() && t.Date.Before(f.from) || !f.to.IsZero() && calendarDayAfter(t.Date, f.to) {
		return false
	}
	if c.Cleared == "yes" && !t.Cleared || c.Cleared == "no" && t.Cleared {
		return false
	}
	if c.Flow == "out" && !t.IsExpense() || c.Flow == "in" && !t.IsIncome() {
		return false
	}
	if c.CustomKey != "" && c.CustomVal != "" && customString(t.Custom[c.CustomKey]) != c.CustomVal {
		return false
	}
	if f.hasMin || f.hasMax {
		amount, err := amountInBase(t, labels)
		if err != nil {
			if labels.OnConversionError != nil {
				labels.OnConversionError(t, err)
			}
			return false
		}
		if f.hasMin && amount < f.min || f.hasMax && amount > f.max {
			return false
		}
	}
	return f.text == "" || f.matchesText(t)
}

func (f legacyCompiled) matchesText(t domain.Transaction) bool {
	q, labels := f.text, f.labels
	if amount, ok := parseSearchAmount(q, labels.BaseCurrency); ok {
		actual, err := amountInBase(t, labels)
		if err == nil && actual == amount {
			return true
		}
		if err != nil && labels.OnConversionError != nil {
			labels.OnConversionError(t, err)
		}
	}
	if strings.Contains(strings.ToLower(t.Payee), q) || strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	if labels.Payee != nil && strings.Contains(strings.ToLower(labels.Payee(t.Payee)), q) {
		return true
	}
	query := strings.TrimPrefix(q, "#")
	for _, tag := range t.Tags {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), query) {
			return true
		}
	}
	return false
}

func legacyApply(rows []domain.Transaction, c Criteria, labels Labels, checked bool) ([]domain.Transaction, []ConversionIssue) {
	var issues []ConversionIssue
	if checked {
		callback := labels.OnConversionError
		labels.OnConversionError = func(t domain.Transaction, err error) {
			issues = append(issues, ConversionIssue{Transaction: t, Err: err})
			if callback != nil {
				callback(t, err)
			}
		}
	}
	f := legacyCompile(c, labels)
	out := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		if f.matches(row) {
			out = append(out, row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if emptySortValue(out[i], f.c.Sort) != emptySortValue(out[j], f.c.Sort) {
			return !emptySortValue(out[i], f.c.Sort)
		}
		if cmp := compare(out[i], out[j], f.c.Sort, labels); cmp != 0 {
			if f.c.Dir == Asc {
				return cmp < 0
			}
			return cmp > 0
		}
		return out[i].ID < out[j].ID
	})
	return out, issues
}

func legacyPresetCounts(rows []domain.Transaction, c Criteria, s PresetScope, labels Labels) PresetCounts {
	uncategorized := legacyCompile(c.Without(FieldUncategorized), labels)
	review := legacyCompile(c.Without(FieldTag), labels)
	large := legacyCompile(c.Without(FieldAmountMin), labels)
	month := legacyCompile(c.Without(FieldFrom).Without(FieldTo), labels)
	var out PresetCounts
	for _, row := range rows {
		if uncategorized.matches(row) && !row.IsTransfer() && hasUncategorizedCategory(row) {
			out.Uncategorized++
		}
		if s.ReviewTag != "" && review.matches(row) {
			for _, tag := range row.Tags {
				if tagEqual(tag, s.ReviewTag) {
					out.NeedsReview++
					break
				}
			}
		}
		if (s.LargeMinor > 0 || s.LargeFor != nil) && large.matches(row) {
			amount, err := amountInBase(row, labels)
			if err != nil {
				if labels.OnConversionError != nil {
					labels.OnConversionError(row, err)
				}
			} else if amount >= presetLargeThreshold(row, s) {
				out.Large++
			}
		}
		if !s.MonthStart.IsZero() && !s.MonthEnd.IsZero() && month.matches(row) && !row.Date.Before(s.MonthStart) && row.Date.Before(s.MonthEnd) {
			out.ThisMonth++
		}
	}
	return out
}
