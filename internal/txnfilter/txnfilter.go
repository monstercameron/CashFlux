// SPDX-License-Identifier: MIT

// Package txnfilter holds the transaction list's filter/sort criteria and the
// pure logic that applies them. Keeping it platform-free means the filtering and
// sorting — a core behavior — are unit-tested on native Go, while the wasm screen
// and the localStorage atom just hold and persist a Criteria value.
package txnfilter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SortKeys are the columns the ledger can be ordered by, in display order.
var SortKeys = []string{"date", "amount", "payee", "category", "account"}

// Sort directions.
const (
	Asc  = "asc"
	Desc = "desc"
)

// Pagination defaults for the ledger. PageSizeAll is the sentinel page size that
// shows every row on one page; DefaultPageSize is the out-of-the-box window.
const (
	DefaultPageSize = 50
	PageSizeAll     = -1
)

// PageSizes are the offered page-size choices (plus "All" = PageSizeAll).
var PageSizes = []int{25, 50, 100}

// ValidSortKey reports whether k is a known sortable column.
func ValidSortKey(k string) bool {
	for _, s := range SortKeys {
		if s == k {
			return true
		}
	}
	return false
}

// DefaultDir is the natural direction for a sort key the first time it's used:
// date and amount lead with the largest/newest first (descending), text columns
// read A→Z (ascending).
func DefaultDir(key string) string {
	switch key {
	case "date", "amount":
		return Desc
	default:
		return Asc
	}
}

// Criteria is the transaction list's filter and sort selection. It persists to
// localStorage (hence the JSON tags); Cleared is "", "yes", or "no". Sort is one
// of SortKeys; Dir is Asc or Desc.
type Criteria struct {
	Text     string `json:"text,omitempty"`
	Account  string `json:"account,omitempty"`
	Category string `json:"category,omitempty"`
	Member   string `json:"member,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Dir      string `json:"dir,omitempty"`
	Cleared  string `json:"cleared,omitempty"`
	// CustomKey/CustomVal filter by a transaction custom field's value (L18): a
	// row matches when its Custom[CustomKey] stringifies to CustomVal. Both empty
	// = no custom-field filter.
	CustomKey string `json:"customKey,omitempty"`
	CustomVal string `json:"customVal,omitempty"`
	// Pagination (persisted with the rest). Page is 1-based; PageSize 0 means the
	// default, PageSizeAll (negative) means "show all".
	Page     int `json:"page,omitempty"`
	PageSize int `json:"pageSize,omitempty"`
}

// Normalize fills defaults: sort defaults to date, the direction defaults to the
// key's natural direction (DefaultDir) when unset or invalid, the page is at
// least 1, and an unset page size becomes DefaultPageSize (a negative size is
// kept as the "all" sentinel).
func (c Criteria) Normalize() Criteria {
	if !ValidSortKey(c.Sort) {
		c.Sort = "date"
	}
	if c.Dir != Asc && c.Dir != Desc {
		c.Dir = DefaultDir(c.Sort)
	}
	if c.Page < 1 {
		c.Page = 1
	}
	if c.PageSize == 0 {
		c.PageSize = DefaultPageSize
	}
	return c
}

// ScopeChanged reports whether the filter/sort scope of two criteria differs —
// i.e. the result set or its order changed, ignoring pagination. The UI uses this
// to reset to page 1 when filters or sort change.
func ScopeChanged(prev, next Criteria) bool {
	a, b := prev.Normalize(), next.Normalize()
	a.Page, a.PageSize = 0, 0
	b.Page, b.PageSize = 0, 0
	return a != b
}

// ResetPageIfScopeChanged returns c with Page reset to 1 when its filter/sort
// scope differs from prev, so a new result set/order starts at the first page.
func (c Criteria) ResetPageIfScopeChanged(prev Criteria) Criteria {
	if ScopeChanged(prev, c) {
		c.Page = 1
	}
	return c
}

// FilterField identifies one removable filter dimension on Criteria. The values
// match the dimensions the compact filter toolbar exposes; Sort, Dir and
// pagination are deliberately not filter fields.
type FilterField string

// The filter dimensions, in toolbar display order.
const (
	FieldText     FilterField = "text"
	FieldAccount  FilterField = "account"
	FieldCategory FilterField = "category"
	FieldMember   FilterField = "member"
	FieldFrom     FilterField = "from"
	FieldTo       FilterField = "to"
	FieldCleared  FilterField = "cleared"
	FieldCustom   FilterField = "custom"
)

// ActiveFilter describes one engaged filter for the toolbar's count badge and
// removable chips. Value is the raw stored value (an entity ID, a date, the
// search text, or "yes"/"no" for Cleared); the view resolves IDs to display
// names. Field is what Without clears when the chip's ✕ is clicked.
type ActiveFilter struct {
	Field FilterField
	Value string
}

// ActiveFilters returns the filters currently narrowing the result set, in
// toolbar order. Whitespace-only text/date values count as inactive. Sort
// direction and pagination are never included.
func (c Criteria) ActiveFilters() []ActiveFilter {
	var out []ActiveFilter
	add := func(f FilterField, v string) {
		if strings.TrimSpace(v) != "" {
			out = append(out, ActiveFilter{Field: f, Value: v})
		}
	}
	add(FieldText, c.Text)
	add(FieldAccount, c.Account)
	add(FieldCategory, c.Category)
	add(FieldMember, c.Member)
	add(FieldFrom, c.From)
	add(FieldTo, c.To)
	if c.Cleared == "yes" || c.Cleared == "no" {
		out = append(out, ActiveFilter{Field: FieldCleared, Value: c.Cleared})
	}
	if c.CustomKey != "" && c.CustomVal != "" {
		out = append(out, ActiveFilter{Field: FieldCustom, Value: c.CustomVal})
	}
	return out
}

// ActiveCount is the number of engaged filters — the number shown on the
// "Filters" trigger badge.
func (c Criteria) ActiveCount() int { return len(c.ActiveFilters()) }

// Without returns c with the given filter field cleared, as when a chip's ✕ is
// clicked. Sort, direction and page size are preserved; the caller resets the
// page (the scope changed). An unknown field returns c unchanged.
func (c Criteria) Without(f FilterField) Criteria {
	switch f {
	case FieldText:
		c.Text = ""
	case FieldAccount:
		c.Account = ""
	case FieldCategory:
		c.Category = ""
	case FieldMember:
		c.Member = ""
	case FieldFrom:
		c.From = ""
	case FieldTo:
		c.To = ""
	case FieldCleared:
		c.Cleared = ""
	case FieldCustom:
		c.CustomKey, c.CustomVal = "", ""
	}
	return c
}

// Labels resolves entity IDs to display names for name-aware sorting (category,
// account). Missing entries fall back to the raw ID so sorting stays deterministic.
type Labels struct {
	Account  map[string]string
	Category map[string]string
}

func (l Labels) account(t domain.Transaction) string {
	if n := l.Account[t.AccountID]; n != "" {
		return n
	}
	return t.AccountID
}

func (l Labels) category(t domain.Transaction) string {
	if n := l.Category[t.CategoryID]; n != "" {
		return n
	}
	return t.CategoryID
}

// Apply returns the transactions matching c, sorted per c.Sort/c.Dir. Category
// and account sort by their raw IDs (no label context); use ApplyWithLabels to
// sort those by display name. The input slice is not mutated.
func Apply(txns []domain.Transaction, c Criteria) []domain.Transaction {
	return ApplyWithLabels(txns, c, Labels{})
}

// ApplyWithLabels is Apply with id→name maps so category/account sort by the
// names the user sees rather than opaque IDs.
func ApplyWithLabels(txns []domain.Transaction, c Criteria, labels Labels) []domain.Transaction {
	c = c.Normalize()

	ft := strings.ToLower(strings.TrimSpace(c.Text))
	var fromT, toT time.Time
	if s := strings.TrimSpace(c.From); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			fromT = d
		}
	}
	if s := strings.TrimSpace(c.To); s != "" {
		if d, err := dateutil.ParseDate(s); err == nil {
			toT = d
		}
	}

	out := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		switch {
		case c.Account != "" && t.AccountID != c.Account:
		case c.Category != "" && t.CategoryID != c.Category:
		case c.Member != "" && t.MemberID != c.Member:
		case !fromT.IsZero() && t.Date.Before(fromT):
		case !toT.IsZero() && t.Date.After(toT):
		case ft != "" && !matchText(t, ft):
		case c.Cleared == "yes" && !t.Cleared:
		case c.Cleared == "no" && t.Cleared:
		case c.CustomKey != "" && c.CustomVal != "" && customString(t.Custom[c.CustomKey]) != c.CustomVal:
		default:
			out = append(out, t)
		}
	}

	asc := c.Dir == Asc
	sort.SliceStable(out, func(i, j int) bool {
		if k := compare(out[i], out[j], c.Sort, labels); k != 0 {
			if asc {
				return k < 0
			}
			return k > 0
		}
		// Ties always break on ID ascending so the order is fully deterministic.
		return out[i].ID < out[j].ID
	})
	return out
}

// compare orders two transactions by the given key in ascending sense, returning
// -1, 0, or 1. Direction is applied by the caller.
func compare(a, b domain.Transaction, key string, l Labels) int {
	switch key {
	case "amount":
		switch x, y := AbsAmount(a), AbsAmount(b); {
		case x < y:
			return -1
		case x > y:
			return 1
		default:
			return 0
		}
	case "payee":
		return strings.Compare(strings.ToLower(a.Desc), strings.ToLower(b.Desc))
	case "category":
		return strings.Compare(strings.ToLower(l.category(a)), strings.ToLower(l.category(b)))
	case "account":
		return strings.Compare(strings.ToLower(l.account(a)), strings.ToLower(l.account(b)))
	default: // date
		switch {
		case a.Date.Before(b.Date):
			return -1
		case a.Date.After(b.Date):
			return 1
		default:
			return 0
		}
	}
}

// AbsAmount returns the absolute minor-unit amount of a transaction (for sorting
// by size regardless of income/expense sign).
func AbsAmount(t domain.Transaction) int64 {
	if a := t.Amount.Amount; a < 0 {
		return -a
	}
	return t.Amount.Amount
}

// matchText reports whether the (already-lowercased) query appears in a
// transaction's description or any of its tags.
func matchText(t domain.Transaction, q string) bool {
	if strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

// customString renders a transaction custom-field value to the string the filter
// compares against (matching the option strings the UI offers): strings pass
// through, bools become "true"/"false", everything else uses fmt.Sprint, and a
// missing value is "".
func customString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(x)
	}
}
