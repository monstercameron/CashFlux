// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/nlfilter"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

// Indexes holds lookup labels and sorted unique tags for an immutable snapshot.
type Indexes struct {
	Accounts, Categories, Members map[string]string
	Tags                          []string
}

// BuildIndexes creates independent lookup maps and normalized tag choices.
func BuildIndexes(accounts []domain.Account, categories []domain.Category, members []domain.Member, rows []domain.Transaction) Indexes {
	result := Indexes{Accounts: make(map[string]string, len(accounts)), Categories: make(map[string]string, len(categories)), Members: make(map[string]string, len(members))}
	for _, a := range accounts {
		result.Accounts[a.ID] = a.Name
	}
	for _, c := range categories {
		result.Categories[c.ID] = c.Name
	}
	for _, m := range members {
		result.Members[m.ID] = m.Name
	}
	tags := make(map[string]bool)
	for _, row := range rows {
		for _, tag := range row.Tags {
			if tag = strings.TrimSpace(tag); tag != "" {
				tags[tag] = true
			}
		}
	}
	for tag := range tags {
		result.Tags = append(result.Tags, tag)
	}
	sort.Strings(result.Tags)
	return result
}

// NaturalLanguageContext preserves the caller's wall-clock date and constructs
// the parser vocabulary. Converting now to UTC here changes "today" near midnight.
func NaturalLanguageContext(categories []domain.Category, rows []domain.Transaction, tags []string, resolve func(string) string, now time.Time, weekStart time.Weekday) nlfilter.Context {
	result := nlfilter.Context{Now: now, WeekStart: weekStart, Tags: tags, ResolvePayee: resolve}
	for _, c := range categories {
		result.Categories = append(result.Categories, nlfilter.NameID{Name: c.Name, ID: c.ID})
	}
	seen := make(map[string]bool)
	for _, row := range rows {
		name := row.Payee
		if resolve != nil {
			name = resolve(name)
		}
		if name != "" && !seen[name] {
			seen[name] = true
			result.Payees = append(result.Payees, name)
		}
	}
	return result
}

// ApplyNaturalLanguage overwrites only dimensions explicitly parsed from text.
func ApplyNaturalLanguage(current, parsed txnfilter.Criteria) txnfilter.Criteria {
	current.Text = parsed.Text
	if parsed.AmountMin != "" {
		current.AmountMin = parsed.AmountMin
	}
	if parsed.AmountMax != "" {
		current.AmountMax = parsed.AmountMax
	}
	ApplyParsedDateBounds(&current, parsed)
	if parsed.Cleared != "" {
		current.Cleared = parsed.Cleared
	}
	if parsed.Flow != "" {
		current.Flow = parsed.Flow
	}
	if parsed.Categories != "" {
		current.Categories = parsed.Categories
	}
	if parsed.Tags != "" {
		current.Tags = parsed.Tags
	}
	return current
}

// ClearBudgetScope removes only the filters contributed by a budget drill.
func ClearBudgetScope(f txnfilter.Criteria) txnfilter.Criteria {
	f.Category, f.Categories, f.Tag, f.Tags = "", "", "", ""
	f.ScopeAny = false
	f.From, f.To, f.PeriodPreset = "", "", txnfilter.PeriodAllDates
	return f
}

// LensName names one member or delegates the localized multiple-member label.
func LensName(ids []string, on bool, names map[string]string, multiple func(int) string) string {
	if !on || len(ids) == 0 {
		return ""
	}
	if len(ids) > 1 {
		return multiple(len(ids))
	}
	return names[ids[0]]
}

// SearchOptions filters real options by label and counts selected choices before
// search, so narrowing text does not change the dimension's selection badge.
func SearchOptions(options []Option, selected []string, query string) ([]Option, int) {
	needle := strings.ToLower(strings.TrimSpace(query))
	result := make([]Option, 0, len(options))
	count := 0
	for _, o := range options {
		if o.Value == "" {
			continue
		}
		if Contains(selected, o.Value) {
			count++
		}
		if needle == "" || strings.Contains(strings.ToLower(o.Label), needle) {
			result = append(result, o)
		}
	}
	return result, count
}

// Categories labels and stably orders category amounts without mutating the view.
func Categories(values []txnview.CategoryTotal, names map[string]string, uncategorized string) []CategoryTotal {
	result := make([]CategoryTotal, 0, len(values))
	for _, value := range values {
		name := names[value.ID]
		if name == "" {
			name = uncategorized
		}
		result = append(result, CategoryTotal{ID: value.ID, Name: name, Minor: value.Minor})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Minor == result[j].Minor {
			return result[i].Name < result[j].Name
		}
		return result[i].Minor > result[j].Minor
	})
	return result
}

// FilterChipLabel chooses the localized description of one active dimension.
func (p Presentation) FilterChipLabel(af txnfilter.ActiveFilter, names Indexes, base, customLabel string) string {
	switch af.Field {
	case txnfilter.FieldText:
		return p.Text("transactions.chipFiltering", af.Value)
	case txnfilter.FieldAccount:
		return p.Text("transactions.chipAccount", names.Accounts[af.Value])
	case txnfilter.FieldCategory:
		return p.Text("transactions.chipCategory", names.Categories[af.Value])
	case txnfilter.FieldMember:
		return p.Text("transactions.chipMember", names.Members[af.Value])
	case txnfilter.FieldSource:
		return p.Text("transactions.chipSource", domain.TxnSource(af.Value).Label())
	case txnfilter.FieldTag:
		return p.Text("transactions.chipTag", af.Value)
	case txnfilter.FieldAmountMin:
		return p.AmountChipLabel("min", af.Value, base)
	case txnfilter.FieldAmountMax:
		return p.AmountChipLabel("max", af.Value, base)
	case txnfilter.FieldFrom:
		return p.Text("transactions.chipFrom", ChipDate(af.Value, p.Date))
	case txnfilter.FieldTo:
		return p.Text("transactions.chipTo", ChipDate(af.Value, p.Date))
	case txnfilter.FieldCleared:
		if af.Value == "yes" {
			return p.Text("transactions.cleared")
		}
		return p.Text("transactions.notCleared")
	case txnfilter.FieldFlow:
		if af.Value == "in" {
			return p.Text("transactions.chipFlowIn")
		}
		return p.Text("transactions.chipFlowOut")
	case txnfilter.FieldUncategorized:
		return p.Text("transactions.chipUncategorized")
	case txnfilter.FieldCustom:
		return p.Text("transactions.chipCustom", customLabel, af.Value)
	}
	return af.Value
}
