// SPDX-License-Identifier: MIT

// Package txnfilter holds the transaction list's filter/sort criteria and the
// pure logic that applies them. Keeping it platform-free means the filtering and
// sorting — a core behavior — are unit-tested on native Go, while the wasm screen
// and the localStorage atom just hold and persist a Criteria value.
package txnfilter

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SortKeys are the columns the ledger can be ordered by, in display order.
var SortKeys = []string{"date", "amount", "payee", "category", "account", "source"}

// Sort directions.
const (
	Asc  = "asc"
	Desc = "desc"
)

// Pagination defaults for the ledger. PageSizeAll is the sentinel page size that
// shows every row on one page; DefaultPageSize is the out-of-the-box window.
const (
	DefaultPageSize = 25
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
	Text string `json:"text,omitempty"`
	// Account filters to transactions on this account (AccountID) OR linked to it as a
	// bill payment (BillAccountID), so an account that only receives linked payments
	// still shows them. Empty = no account filter.
	Account  string `json:"account,omitempty"`
	Category string `json:"category,omitempty"`
	// Categories is a comma-joined set of category IDs for a MULTI-category filter
	// (OR: a transaction matches if its category is any of them). It's a string (not
	// a slice) so Criteria stays comparable for ScopeChanged. When set it takes
	// precedence over Category. Drives the drill from a multi-category budget.
	Categories string `json:"categories,omitempty"`
	Member     string `json:"member,omitempty"`
	// BillAccount filters to transactions marked as bill payments toward this
	// liability account (Transaction.BillAccountID). Empty = no bill filter. Drives
	// the Debt page's "N payments" drill-through.
	BillAccount string `json:"billAccount,omitempty"`
	// Subscription filters to transactions marked as payments toward this subscription
	// name (Transaction.SubscriptionName). Empty = no subscription filter. Drives the
	// Subscriptions page's "N payments" drill-through.
	Subscription string `json:"subscription,omitempty"`
	// Source filters to transactions with this provenance (domain.TxnSource value:
	// "manual"/"imported"/"scanned"/"recurring"/"assistant"). Empty = no source filter.
	Source string `json:"source,omitempty"`
	// Tag filters to transactions carrying this exact tag (C49). Empty = no tag
	// filter. Matched case-insensitively against each of a transaction's Tags.
	Tag     string `json:"tag,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Sort    string `json:"sort,omitempty"`
	Dir     string `json:"dir,omitempty"`
	Cleared string `json:"cleared,omitempty"`
	// CustomKey/CustomVal filter by a transaction custom field's value (L18): a
	// row matches when its Custom[CustomKey] stringifies to CustomVal. Both empty
	// = no custom-field filter.
	CustomKey string `json:"customKey,omitempty"`
	CustomVal string `json:"customVal,omitempty"`
	// AmountMin/AmountMax filter by the transaction's ABSOLUTE amount in major
	// units (C53), e.g. "10" and "100" keep charges/income of $10–$100 regardless
	// of sign. Either bound may be empty (open-ended). Unparseable bounds are
	// ignored (treated as unset) so a half-typed number never hides everything.
	AmountMin string `json:"amountMin,omitempty"`
	AmountMax string `json:"amountMax,omitempty"`
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
	FieldText      FilterField = "text"
	FieldAccount   FilterField = "account"
	FieldCategory  FilterField = "category"
	FieldMember    FilterField = "member"
	FieldSource    FilterField = "source"
	FieldTag       FilterField = "tag"
	FieldFrom      FilterField = "from"
	FieldTo        FilterField = "to"
	FieldCleared   FilterField = "cleared"
	FieldAmountMin FilterField = "amountMin"
	FieldAmountMax FilterField = "amountMax"
	FieldCustom    FilterField = "custom"
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
	// A multi-category filter shows one removable chip per category (each resolves
	// to a name); the chip ✕ clears the whole category dimension via Without.
	for _, id := range splitCSV(c.Categories) {
		out = append(out, ActiveFilter{Field: FieldCategory, Value: id})
	}
	add(FieldMember, c.Member)
	add(FieldSource, c.Source)
	add(FieldTag, c.Tag)
	add(FieldFrom, c.From)
	add(FieldTo, c.To)
	add(FieldAmountMin, c.AmountMin)
	add(FieldAmountMax, c.AmountMax)
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
		c.Categories = ""
	case FieldMember:
		c.Member = ""
	case FieldSource:
		c.Source = ""
	case FieldTag:
		c.Tag = ""
	case FieldFrom:
		c.From = ""
	case FieldTo:
		c.To = ""
	case FieldAmountMin:
		c.AmountMin = ""
	case FieldAmountMax:
		c.AmountMax = ""
	case FieldCleared:
		c.Cleared = ""
	case FieldCustom:
		c.CustomKey, c.CustomVal = "", ""
	}
	return c
}

// splitCSV splits a comma-joined value into its non-empty, trimmed parts (nil for
// an empty string). Used for the multi-category filter's ID set.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// csvHas reports whether val is one of the comma-joined values in csv.
func csvHas(csv, val string) bool {
	for _, p := range splitCSV(csv) {
		if p == val {
			return true
		}
	}
	return false
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
	tagF := strings.ToLower(strings.TrimSpace(c.Tag))
	minMajor, hasMin := parseAmountBound(c.AmountMin)
	maxMajor, hasMax := parseAmountBound(c.AmountMax)
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
		// An account filter matches a transaction booked on the account (AccountID) OR
		// one linked to it as a bill payment (BillAccountID) — so filtering by an account
		// that only receives linked bill payments (e.g. an HOA obligation the money is
		// paid FROM another account) still surfaces those payments.
		case c.Account != "" && t.AccountID != c.Account && t.BillAccountID != c.Account:
		case c.BillAccount != "" && t.BillAccountID != c.BillAccount:
		case c.Subscription != "" && t.SubscriptionName != c.Subscription:
		case c.Categories != "" && !csvHas(c.Categories, t.CategoryID):
		case c.Categories == "" && c.Category != "" && t.CategoryID != c.Category:
		case c.Member != "" && t.MemberID != c.Member:
		case c.Source != "" && string(t.Source) != c.Source:
		case tagF != "" && !hasTag(t, tagF):
		case hasMin && AbsAmount(t) < currency.MinorFromMajor(minMajor, t.Amount.Currency):
		case hasMax && AbsAmount(t) > currency.MinorFromMajor(maxMajor, t.Amount.Currency):
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
	case "source":
		// Order by the display label so the column reads sensibly; untagged rows
		// ("—") sort after the named sources.
		return strings.Compare(strings.ToLower(a.Source.Label()), strings.ToLower(b.Source.Label()))
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
// transaction's payee, description, or any of its tags. Payee is a first-class
// field used by the rules engine and the activity screen, so a payee that differs
// from the description (e.g. a cleaned-up merchant name) must still be findable.
func matchText(t domain.Transaction, q string) bool {
	if strings.Contains(strings.ToLower(t.Payee), q) {
		return true
	}
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

// parseAmountBound parses an amount filter bound (major units, e.g. "12.50") to a
// float and whether it is usable. A blank or unparseable bound returns ok=false so
// it's treated as unset — a half-typed number never hides every row. A negative
// bound is clamped to 0 (the filter compares absolute amounts). (C53)
func parseAmountBound(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if v < 0 {
		v = -v
	}
	return v, true
}

// hasTag reports whether a transaction carries the given (already-lowercased) tag,
// matched exactly (case-insensitively) against each of its Tags. Exact match — not
// substring — so the tag filter is a precise facet, distinct from free-text search
// which already does substring matching across payee/desc/tags (C49).
func hasTag(t domain.Transaction, tag string) bool {
	for _, tg := range t.Tags {
		if strings.ToLower(tg) == tag {
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
