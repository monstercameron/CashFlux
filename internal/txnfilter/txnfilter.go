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
	Tag string `json:"tag,omitempty"`
	// Accounts / Members / Sources / Tags are the MULTI-value counterparts of the
	// single Account/Member/Source/Tag fields — a comma-joined set of values matched
	// OR-within the dimension (a transaction passes if it matches ANY selected value),
	// AND across dimensions. They're comma-joined strings (not slices) so Criteria
	// stays comparable for ScopeChanged. When a multi field is non-empty it takes
	// precedence over its single counterpart. (Categories already does this for
	// category.) Accounts also matches BillAccountID, like the single Account.
	Accounts string `json:"accounts,omitempty"`
	Members  string `json:"members,omitempty"`
	Sources  string `json:"sources,omitempty"`
	Tags     string `json:"tags,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Dir      string `json:"dir,omitempty"`
	Cleared  string `json:"cleared,omitempty"`
	// Flow filters by money direction: "out" keeps expenses (a negative amount),
	// "in" keeps income (a positive amount). Empty = both. Drives the natural-
	// language search's "spent" / "received" clause (TX2).
	Flow string `json:"flow,omitempty"`
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
	// Uncategorized, when true, keeps only non-transfer transactions with no
	// category — the "needs categorizing" quick-filter preset (TXC-3).
	Uncategorized bool `json:"uncategorized,omitempty"`
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
	FieldFlow      FilterField = "flow"
	FieldAmountMin FilterField = "amountMin"
	// FieldUncategorized is the "no category yet" quick filter (TXC-3).
	FieldUncategorized FilterField = "uncategorized"
	FieldAmountMax     FilterField = "amountMax"
	FieldCustom        FilterField = "custom"
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
	// Each categorical dimension shows one removable chip per selected value (each
	// resolves to a display name in the view); the chip ✕ removes just that value via
	// RemoveValue.
	for _, f := range categoricalFields {
		for _, v := range c.SelectedValues(f) {
			out = append(out, ActiveFilter{Field: f, Value: v})
		}
	}
	add(FieldFrom, c.From)
	add(FieldTo, c.To)
	add(FieldAmountMin, c.AmountMin)
	add(FieldAmountMax, c.AmountMax)
	if c.Cleared == "yes" || c.Cleared == "no" {
		out = append(out, ActiveFilter{Field: FieldCleared, Value: c.Cleared})
	}
	if c.Flow == "in" || c.Flow == "out" {
		out = append(out, ActiveFilter{Field: FieldFlow, Value: c.Flow})
	}
	if c.Uncategorized {
		out = append(out, ActiveFilter{Field: FieldUncategorized, Value: "1"})
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
		c.Accounts = ""
	case FieldCategory:
		c.Category = ""
		c.Categories = ""
	case FieldMember:
		c.Member = ""
		c.Members = ""
	case FieldSource:
		c.Source = ""
		c.Sources = ""
	case FieldTag:
		c.Tag = ""
		c.Tags = ""
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
	case FieldFlow:
		c.Flow = ""
	case FieldUncategorized:
		c.Uncategorized = false
	case FieldCustom:
		c.CustomKey, c.CustomVal = "", ""
	}
	return c
}

// SingleAccount reports the account the filter is scoped to when it targets
// EXACTLY ONE account, and ok=false otherwise. It is the gate for register mode
// (TX12): the running-balance column only makes sense against a single account's
// chronological history. A single-value multi set (Accounts with one id) counts,
// as does the single Account field; any other account facet (multiple accounts,
// or none) is not a single-account scope. Other filter dimensions (date, text,
// category, …) are irrelevant here — they narrow which of that account's rows are
// visible, which register mode handles by folding over the full history.
func (c Criteria) SingleAccount() (id string, ok bool) {
	multi := splitCSV(c.Accounts)
	switch {
	case len(multi) == 1:
		return multi[0], true
	case len(multi) > 1:
		return "", false
	case c.Account != "":
		return c.Account, true
	default:
		return "", false
	}
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

// csvHasAccount reports whether any account id in csv matches the transaction's booked
// account or its bill-linked account (mirroring the single Account filter's dual match).
func csvHasAccount(csv string, t domain.Transaction) bool {
	for _, id := range splitCSV(csv) {
		if t.AccountID == id || t.BillAccountID == id {
			return true
		}
	}
	return false
}

// hasAnyTagCSV reports whether the transaction carries any of the comma-joined tags
// (case-insensitive) — the multi-value counterpart of hasTag.
func hasAnyTagCSV(t domain.Transaction, csv string) bool {
	for _, tag := range splitCSV(csv) {
		if hasTag(t, strings.ToLower(strings.TrimSpace(tag))) {
			return true
		}
	}
	return false
}

// toggleCSV adds value to a comma-joined set if absent, or removes it if present.
func toggleCSV(csv, value string) string {
	if csvHas(csv, value) {
		return removeFromCSV(csv, value)
	}
	return strings.Join(append(splitCSV(csv), value), ",")
}

// removeFromCSV removes value from a comma-joined set (no-op if absent).
func removeFromCSV(csv, value string) string {
	parts := splitCSV(csv)
	out := parts[:0]
	for _, p := range parts {
		if p != value {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

// mergeSingleIntoMulti folds a set single-value field into the multi set, so a filter
// arrived at via a single-value drill-through is preserved once the user starts
// multi-selecting the same dimension.
func mergeSingleIntoMulti(csv, single string) string {
	if single != "" && !csvHas(csv, single) {
		return strings.Join(append(splitCSV(csv), single), ",")
	}
	return csv
}

// categoricalFields are the filter dimensions that support multi-value selection.
var categoricalFields = []FilterField{FieldAccount, FieldCategory, FieldMember, FieldSource, FieldTag}

// multiPtr returns pointers to the multi (comma-joined) and single fields for a
// categorical dimension, or (nil, nil) for a non-categorical field.
func (c *Criteria) multiPtr(f FilterField) (multi, single *string) {
	switch f {
	case FieldAccount:
		return &c.Accounts, &c.Account
	case FieldCategory:
		return &c.Categories, &c.Category
	case FieldMember:
		return &c.Members, &c.Member
	case FieldSource:
		return &c.Sources, &c.Source
	case FieldTag:
		return &c.Tags, &c.Tag
	}
	return nil, nil
}

// ToggleValue adds or removes value in a categorical dimension's multi set — the action
// of clicking a filter option. Any set single counterpart is folded in first (then
// cleared) so the multi set becomes the single source of truth. Non-categorical fields
// are returned unchanged.
func (c Criteria) ToggleValue(f FilterField, value string) Criteria {
	multi, single := c.multiPtr(f)
	if multi == nil {
		return c
	}
	*multi = toggleCSV(mergeSingleIntoMulti(*multi, *single), value)
	*single = ""
	return c
}

// SelectedValues returns the currently-selected values for a categorical dimension (the
// multi set, plus the single value if set and not already present), for highlighting the
// chosen options and rendering per-value chips.
func (c Criteria) SelectedValues(f FilterField) []string {
	multi, single := c.multiPtr(f)
	if multi == nil {
		return nil
	}
	out := splitCSV(*multi)
	if *single != "" && !csvHas(*multi, *single) {
		out = append(out, *single)
	}
	return out
}

// RemoveValue removes one value from a categorical dimension (a per-value chip ✕),
// clearing the single counterpart if it matches. Non-categorical fields clear entirely.
func (c Criteria) RemoveValue(f FilterField, value string) Criteria {
	multi, single := c.multiPtr(f)
	if multi == nil {
		return c.Without(f)
	}
	*multi = removeFromCSV(*multi, value)
	if *single == value {
		*single = ""
	}
	return c
}

// Labels resolves entity IDs to display names for name-aware sorting (category,
// account). Missing entries fall back to the raw ID so sorting stays deterministic.
type Labels struct {
	Account  map[string]string
	Category map[string]string
	// Payee, when set, resolves a raw payee to its cleaned display name (the TX1/SM-1
	// merchant-cleanup alias). Text search then also matches the cleaned name, so a
	// search for the clean merchant name finds every charge even though the raw payee
	// on the transaction is unchanged. Nil = match raw fields only (the pure default).
	Payee func(raw string) string
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
		case c.Accounts != "" && !csvHasAccount(c.Accounts, t):
		case c.Accounts == "" && c.Account != "" && t.AccountID != c.Account && t.BillAccountID != c.Account:
		case c.BillAccount != "" && t.BillAccountID != c.BillAccount:
		case c.Subscription != "" && t.SubscriptionName != c.Subscription:
		// TODO(splits): matches only the whole-transaction category — a split line's
		// category won't surface its transaction here, so a budget's drill-through can
		// disagree with its splits-aware bar. See the split contract in
		// domain/category_split.go; fix = also match any Splits[i].CategoryID.
		case c.Categories != "" && !csvHas(c.Categories, t.CategoryID):
		case c.Categories == "" && c.Category != "" && t.CategoryID != c.Category:
		case c.Uncategorized && (t.CategoryID != "" || t.IsTransfer()):
		case c.Members != "" && !csvHas(c.Members, t.MemberID):
		case c.Members == "" && c.Member != "" && t.MemberID != c.Member:
		case c.Sources != "" && !csvHas(c.Sources, string(t.Source)):
		case c.Sources == "" && c.Source != "" && string(t.Source) != c.Source:
		case c.Tags != "" && !hasAnyTagCSV(t, c.Tags):
		case c.Tags == "" && tagF != "" && !hasTag(t, tagF):
		case hasMin && AbsAmount(t) < currency.MinorFromMajor(minMajor, t.Amount.Currency):
		case hasMax && AbsAmount(t) > currency.MinorFromMajor(maxMajor, t.Amount.Currency):
		case !fromT.IsZero() && t.Date.Before(fromT):
		case !toT.IsZero() && t.Date.After(toT):
		case ft != "" && !matchText(t, ft, labels.Payee):
		case c.Cleared == "yes" && !t.Cleared:
		case c.Cleared == "no" && t.Cleared:
		// Flow keeps only expenses ("out", a negative amount) or only income ("in",
		// a positive amount); a zero-amount row matches neither direction.
		case c.Flow == "out" && t.Amount.Amount >= 0:
		case c.Flow == "in" && t.Amount.Amount <= 0:
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
// transaction's payee, description, cleaned display name, or any of its tags. Payee
// is a first-class field used by the rules engine and the activity screen, so a payee
// that differs from the description (e.g. a cleaned-up merchant name) must still be
// findable. resolve (optional) maps the raw payee to its cleaned alias so a search for
// the clean merchant name matches every charge even though the raw payee is unchanged.
func matchText(t domain.Transaction, q string, resolve func(string) string) bool {
	if strings.Contains(strings.ToLower(t.Payee), q) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	if resolve != nil {
		if strings.Contains(strings.ToLower(resolve(t.Payee)), q) {
			return true
		}
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
