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

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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

// DefaultSortKey is the column the ledger falls back to: newest first.
const DefaultSortKey = "date"

// EffectiveSort returns the key and direction the ledger should ACTUALLY order by,
// given which sortable columns the user currently has on screen.
//
// Four of the six sortable columns can be hidden (Amount, Account, Category,
// Source), and hiding one used to leave its sort in force: the rows stayed ordered
// by a column that was no longer there, NO header showed a sort indicator, and the
// state survived a reload. A table whose order corresponds to nothing on screen is
// the same defect as a period label over the wrong month — the order is applied,
// and the page never says so.
//
// The stored preference is deliberately left alone. Falling back is a display
// decision, so re-showing the column restores the sort the user chose rather than
// silently discarding it the moment they freed up some width.
//
// hidden holds the sort keys whose columns are currently off; a key absent from it
// (or a nil map) is treated as visible, which is right for the columns that cannot
// be hidden at all.
func EffectiveSort(c Criteria, hidden map[string]bool) (key, dir string) {
	if c.Sort == "" || !hidden[c.Sort] {
		return c.Sort, c.Dir
	}
	return DefaultSortKey, DefaultDir(DefaultSortKey)
}

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
	// PeriodPreset records the user-facing period choice so rolling presets can
	// resolve against the current date after a reload. Empty is accepted for
	// legacy filters and migrated by ResolvePeriod; "custom" keeps From/To.
	PeriodPreset string `json:"periodPreset,omitempty"`
	Sort         string `json:"sort,omitempty"`
	Dir          string `json:"dir,omitempty"`
	Cleared      string `json:"cleared,omitempty"`
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
	// ScopeAny switches the category and tag dimensions from AND to OR: a
	// transaction matches when it is in any selected category OR carries any
	// selected tag, instead of having to satisfy both. Every other dimension
	// (dates, member, account…) keeps its AND.
	//
	// It exists for the budget drill-through, because that is how a budget counts
	// spend: a charge lands in a "#vacation" budget either by sitting in a tracked
	// category or by carrying the tag. Filtering AND-wise would open a ledger
	// missing exactly the charges the tag contributed, under a total that included
	// them (C585). No user-facing control sets it — it is a scope the app builds.
	ScopeAny bool `json:"scopeAny,omitempty"`
	// Pagination (persisted with the rest). Page is 1-based; PageSize 0 means the
	// default, PageSizeAll (negative) means "show all".
	Page     int `json:"page,omitempty"`
	PageSize int `json:"pageSize,omitempty"`
}

// Period preset values persisted with Criteria.
const (
	PeriodThisMonth       = "this-month"
	PeriodLastMonth       = "last-month"
	PeriodLastThreeMonths = "last-three-months"
	PeriodThisYear        = "this-year"
	PeriodAllDates        = "all-dates"
	PeriodCustom          = "custom"
)

// MonthPeriodPreset names an absolute calendar month as a stable period key.
func MonthPeriodPreset(year int, month time.Month) string {
	return fmt.Sprintf("month:%04d-%02d", year, month)
}

// ResolvePeriod resolves rolling presets to today's date bounds. Legacy filters
// without a preset are migrated when their stored range exactly matches a
// preset's current range; an exact calendar month gets a stable named-month key.
func (c Criteria) ResolvePeriod(now time.Time) Criteria {
	key := c.PeriodPreset
	if key == "" {
		key = InferPeriodPreset(c.From, c.To, now)
	}
	month, namedMonth := parseMonthPeriodPreset(key)
	switch {
	case key == PeriodThisMonth:
		c.From, c.To = monthBounds(now.Year(), now.Month())
	case key == PeriodLastMonth:
		last := now.AddDate(0, -1, 0)
		c.From, c.To = monthBounds(last.Year(), last.Month())
	case key == PeriodLastThreeMonths:
		from := now.AddDate(0, -2, 0)
		c.From, _ = monthBounds(from.Year(), from.Month())
		_, c.To = monthBounds(now.Year(), now.Month())
	case key == PeriodThisYear:
		c.From = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		c.To = time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case key == PeriodAllDates:
		c.From, c.To = "", ""
	case namedMonth:
		c.From, c.To = monthBounds(month.Year(), month.Month())
	case key == PeriodCustom:
	default:
		// Ignore a future or malformed key and migrate its bounds instead.
		c.PeriodPreset = ""
		return c.ResolvePeriod(now)
	}
	c.PeriodPreset = key
	return c
}

// ResolvePeriodChange resolves a filter being saved against the prior persisted
// value. If only the date bounds changed while the preset key stayed the same,
// treat the new bounds as the user's period selection and infer its stable key.
func (c Criteria) ResolvePeriodChange(previous Criteria, now time.Time) Criteria {
	if (c.From != previous.From || c.To != previous.To) && (c.PeriodPreset == previous.PeriodPreset || c.PeriodPreset == "") {
		c.PeriodPreset = InferPeriodPreset(c.From, c.To, now)
	}
	return c.ResolvePeriod(now)
}

// InferPeriodPreset returns the stable key for a date range, including rolling
// presets when the range matches today's resolution. Other full calendar months
// get a month:YYYY-MM key; explicit partial ranges remain custom.
func InferPeriodPreset(from, to string, now time.Time) string {
	if from == "" && to == "" {
		return PeriodAllDates
	}
	thisFrom, thisTo := monthBounds(now.Year(), now.Month())
	if from == thisFrom && to == thisTo {
		return PeriodThisMonth
	}
	last := now.AddDate(0, -1, 0)
	lastFrom, lastTo := monthBounds(last.Year(), last.Month())
	if from == lastFrom && to == lastTo {
		return PeriodLastMonth
	}
	threeAgo := now.AddDate(0, -2, 0)
	threeFrom, _ := monthBounds(threeAgo.Year(), threeAgo.Month())
	if _, threeTo := monthBounds(now.Year(), now.Month()); from == threeFrom && to == threeTo {
		return PeriodLastThreeMonths
	}
	yearFrom := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	yearTo := time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	if from == yearFrom && to == yearTo {
		return PeriodThisYear
	}
	if month, ok := parseFullMonthRange(from, to); ok {
		return MonthPeriodPreset(month.Year(), month.Month())
	}
	return PeriodCustom
}

func monthBounds(year int, month time.Month) (string, string) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func parseFullMonthRange(from, to string) (time.Time, bool) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil || start.Day() != 1 {
		return time.Time{}, false
	}
	_, end := monthBounds(start.Year(), start.Month())
	return start, to == end
}

func parseMonthPeriodPreset(key string) (time.Time, bool) {
	if !strings.HasPrefix(key, "month:") {
		return time.Time{}, false
	}
	month, err := time.Parse("2006-01", strings.TrimPrefix(key, "month:"))
	return month, err == nil
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
	// UI handlers normalize immediately after changing From/To. If a non-custom
	// preset key was carried over with those new bounds, re-key it before it can
	// be persisted or used by a later filter action.
	if c.PeriodPreset != "" && c.PeriodPreset != PeriodCustom {
		resolved := c.ResolvePeriod(time.Now())
		if resolved.From != c.From || resolved.To != c.To {
			c.PeriodPreset = InferPeriodPreset(c.From, c.To, time.Now())
		}
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

// ActiveFilter describes one engaged criterion. PageFilters returns these for
// the toolbar's count badge and removable chips; AllCriteria also includes date
// bounds. Value is the raw stored value and Field is what Without clears.
type ActiveFilter struct {
	Field FilterField
	Value string
}

// PageFilters returns user-selected filters currently narrowing the page, in
// toolbar order. Period bounds belong to the period control and are excluded.
// Whitespace-only values count as inactive; sort and pagination are never included.
func (c Criteria) PageFilters() []ActiveFilter {
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

// AllCriteria returns every active criterion, including the selected date bounds.
// Date bounds are useful when explaining a parsed query, even though the page's
// period control is intentionally excluded from its filter chips and badge.
func (c Criteria) AllCriteria() []ActiveFilter {
	out := make([]ActiveFilter, 0, len(c.PageFilters())+2)
	if strings.TrimSpace(c.From) != "" {
		out = append(out, ActiveFilter{Field: FieldFrom, Value: c.From})
	}
	if strings.TrimSpace(c.To) != "" {
		out = append(out, ActiveFilter{Field: FieldTo, Value: c.To})
	}
	return append(out, c.PageFilters()...)
}

// ActiveCount is the number of engaged filters — the number shown on the
// "Filters" trigger badge.
func (c Criteria) ActiveCount() int { return len(c.PageFilters()) }

// ClearPageFilters clears user filters while preserving the selected period and
// sort preference. Pagination returns to the first page.
func (c Criteria) ClearPageFilters() Criteria {
	return Criteria{
		From: c.From, To: c.To, PeriodPreset: c.PeriodPreset,
		Sort: c.Sort, Dir: c.Dir, PageSize: c.PageSize,
		Page: 1,
	}
}

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
		c.ScopeAny = false
	case FieldMember:
		c.Member = ""
		c.Members = ""
	case FieldSource:
		c.Source = ""
		c.Sources = ""
	case FieldTag:
		c.Tag = ""
		c.Tags = ""
		c.ScopeAny = false
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

// matchesCategory reports whether a transaction belongs to a category selection.
//
// It answers for the WHOLE transaction: its own category, or — when it carries a
// breakdown — any of its split lines' categories. That is the category-side
// reading of the split contract (domain/category_split.go), the same one
// budgeting.spentCovered and reports.categoryTotals use. Matching only the
// parent's CategoryID is what let a split receipt count toward a budget's bar
// while vanishing from the ledger that bar links to (C585).
//
// A transaction with splits keeps its own CategoryID as a fallback match, so a
// breakdown never makes a row LESS findable than it was before it was split.
func matchesCategory(t domain.Transaction, want func(categoryID string) bool) bool {
	if want(t.CategoryID) {
		return true
	}
	for _, s := range t.Splits {
		if want(s.CategoryID) {
			return true
		}
	}
	return false
}

// matchesScopeAny evaluates the OR'd category-or-tag dimension used by the
// budget drill-through: in one of the selected categories (whole or split), or
// carrying one of the selected tags. With neither dimension set it matches
// everything, so ScopeAny never narrows a filter that selected nothing.
func matchesScopeAny(t domain.Transaction, c Criteria, singleTag string) bool {
	catSel := c.Categories != "" || c.Category != ""
	tagSel := c.Tags != "" || singleTag != ""
	if !catSel && !tagSel {
		return true
	}
	if catSel {
		inCat := matchesCategory(t, func(id string) bool {
			if c.Categories != "" {
				return csvHas(c.Categories, id)
			}
			return id == c.Category
		})
		if inCat {
			return true
		}
	}
	if tagSel {
		if c.Tags != "" {
			return hasAnyTagCSV(t, c.Tags)
		}
		return hasTag(t, singleTag)
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
	c = c.dropScopeAnyOn(f)
	return c
}

// dropScopeAnyOn returns c with ScopeAny cleared when f is one of the two
// dimensions it re-wires.
//
// ScopeAny describes a scope the APP built — a budget's "in a tracked category
// OR carrying a tracked tag". The moment a user edits either of those dimensions
// by hand they are composing an ordinary filter, where every dimension ANDs, and
// leaving the OR in place would silently WIDEN the result set: picking a
// "Restaurants" category on top of a "#vacation" tag would show restaurants OR
// vacation charges, with nothing on screen distinguishing that from the "and"
// every other filter means.
// DrillToTag narrows c to exactly one tag, replacing any existing tag selection
// and leaving every other criterion (search text, period, member, …) alone.
//
// It is the row-level "Filter to #coffee" action (C651). Written here rather than
// inline in the ledger so the promise the chip makes — one tag, additive to the
// rest, and removable on its own through RemoveValue — is a tested property of
// the filter model instead of two lines in a click handler.
//
// An empty tag is a no-op: a chip with no tag is a bug upstream, and clearing the
// user's tag filter is not a reasonable guess at what they meant.
func (c Criteria) DrillToTag(tag string) Criteria {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return c
	}
	c.Tag, c.Tags = tag, ""
	// ScopeAny widens a category/tag filter to "any of these"; a drill-down to one
	// tag has nothing to widen, and leaving it set would silently change what the
	// single tag means.
	return c.dropScopeAnyOn(FieldTag)
}

func (c Criteria) dropScopeAnyOn(f FilterField) Criteria {
	if f == FieldCategory || f == FieldTag {
		c.ScopeAny = false
	}
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
	return c.dropScopeAnyOn(f)
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
	// BaseCurrency is the currency used by amount bounds and numeric searches.
	// An empty value defaults to USD for legacy callers.
	BaseCurrency string
	// ToBase converts transaction amounts to BaseCurrency. Cross-currency amount
	// filters without this hook report a conversion error and do not match the row.
	ToBase func(money.Money) (money.Money, error)
	// OnConversionError receives rows that could not be compared in BaseCurrency.
	OnConversionError func(domain.Transaction, error)
}

// ConversionIssue describes one transaction that could not be compared in the
// configured base currency.
type ConversionIssue struct {
	Transaction domain.Transaction
	Err         error
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
	filter := compileCriteria(c, labels)
	c = filter.criteria

	out := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if !filter.matches(t) {
			continue
		}
		out = append(out, t)
	}

	asc := c.Dir == Asc
	sort.SliceStable(out, func(i, j int) bool {
		if emptySortValue(out[i], c.Sort) != emptySortValue(out[j], c.Sort) {
			return !emptySortValue(out[i], c.Sort)
		}
		if k := compare(out[i], out[j], c.Sort, labels); k != 0 {
			if asc {
				return k < 0
			}
			return k > 0
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ApplyWithLabelsChecked applies the filter and returns every amount-conversion
// failure encountered while evaluating it. If labels also has OnConversionError,
// that callback is called for each issue as well.
func ApplyWithLabelsChecked(txns []domain.Transaction, c Criteria, labels Labels) ([]domain.Transaction, []ConversionIssue) {
	var issues []ConversionIssue
	callback := labels.OnConversionError
	labels.OnConversionError = func(txn domain.Transaction, err error) {
		issues = append(issues, ConversionIssue{Transaction: txn, Err: err})
		if callback != nil {
			callback(txn, err)
		}
	}
	return ApplyWithLabels(txns, c, labels), issues
}

type compiledCriteria struct {
	criteria           Criteria
	labels             Labels
	text, tag          string
	minMinor, maxMinor int64
	hasMin, hasMax     bool
	from, to           time.Time
}

func compileCriteria(c Criteria, labels Labels) compiledCriteria {
	c = c.Normalize()
	filter := compiledCriteria{
		criteria: c,
		labels:   labels,
		text:     strings.ToLower(strings.TrimSpace(c.Text)),
		tag:      strings.ToLower(strings.TrimSpace(c.Tag)),
	}
	filter.minMinor, filter.hasMin = parseAmountBound(c.AmountMin, labels.BaseCurrency)
	filter.maxMinor, filter.hasMax = parseAmountBound(c.AmountMax, labels.BaseCurrency)
	if d, err := dateutil.ParseDate(strings.TrimSpace(c.From)); err == nil {
		filter.from = d
	}
	if d, err := dateutil.ParseDate(strings.TrimSpace(c.To)); err == nil {
		filter.to = d
	}
	return filter
}

func (f compiledCriteria) matches(t domain.Transaction) bool {
	return matchesCriteria(t, f.criteria, f.labels, f.text, f.tag, f.minMinor, f.hasMin, f.maxMinor, f.hasMax, f.from, f.to)
}

func matchesCriteria(t domain.Transaction, c Criteria, labels Labels, ft, tagF string, minMinor int64, hasMin bool, maxMinor int64, hasMax bool, fromT, toT time.Time) bool {
	// Reject rows by ordinary scope before attempting currency conversion, so an FX
	// warning is only reported for a row the other active filters would show.
	if c.Accounts != "" && !csvHasAccount(c.Accounts, t) || c.Accounts == "" && c.Account != "" && t.AccountID != c.Account && t.BillAccountID != c.Account {
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
	if c.Uncategorized && (matchesCategory(t, func(id string) bool { return id != "" }) || t.IsTransfer()) {
		return false
	}
	if c.Members != "" && !matchesMemberSelection(c.Members, t) || c.Members == "" && c.Member != "" && !matchesMemberSelection(c.Member, t) {
		return false
	}
	if c.Sources != "" && !csvHas(c.Sources, string(t.Source)) || c.Sources == "" && c.Source != "" && string(t.Source) != c.Source {
		return false
	}
	if !c.ScopeAny && c.Tags != "" && !hasAnyTagCSV(t, c.Tags) || !c.ScopeAny && c.Tags == "" && tagF != "" && !hasTag(t, tagF) {
		return false
	}
	if c.ScopeAny && !matchesScopeAny(t, c, tagF) {
		return false
	}
	if !fromT.IsZero() && t.Date.Before(fromT) || !toT.IsZero() && calendarDayAfter(t.Date, toT) {
		return false
	}
	if c.Cleared == "yes" && !t.Cleared || c.Cleared == "no" && t.Cleared {
		return false
	}
	// Direction follows the domain's income/expense definitions, which exclude
	// transfer legs even when a leg's signed amount looks like income or spend.
	if c.Flow == "out" && !t.IsExpense() || c.Flow == "in" && !t.IsIncome() {
		return false
	}
	if c.CustomKey != "" && c.CustomVal != "" && customString(t.Custom[c.CustomKey]) != c.CustomVal {
		return false
	}
	if hasMin || hasMax {
		amount, err := amountInBase(t, labels)
		if err != nil {
			if labels.OnConversionError != nil {
				labels.OnConversionError(t, err)
			}
			return false
		}
		if hasMin && amount < minMinor || hasMax && amount > maxMinor {
			return false
		}
	}
	return ft == "" || matchTextWithLabels(t, ft, labels)
}

func calendarDayAfter(date, bound time.Time) bool {
	d := time.Date(date.UTC().Year(), date.UTC().Month(), date.UTC().Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(bound.UTC().Year(), bound.UTC().Month(), bound.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return d.After(b)
}

func amountInBase(t domain.Transaction, labels Labels) (int64, error) {
	amount, err := t.Amount.AbsChecked()
	if err != nil {
		return 0, fmt.Errorf("transaction %q amount magnitude: %w", t.ID, err)
	}
	base := effectiveBaseCurrency(labels.BaseCurrency)
	if base == amount.Currency {
		return amount.Amount, nil
	}
	if labels.ToBase == nil {
		return 0, fmt.Errorf("transaction %q: no conversion from %s to %s", t.ID, amount.Currency, base)
	}
	converted, err := labels.ToBase(amount)
	if err != nil {
		return 0, fmt.Errorf("transaction %q: convert %s to %s: %w", t.ID, amount.Currency, base, err)
	}
	if converted.Currency != base {
		return 0, fmt.Errorf("transaction %q: conversion returned %s, want %s", t.ID, converted.Currency, base)
	}
	return converted.Amount, nil
}

func emptySortValue(t domain.Transaction, key string) bool {
	switch key {
	case "category":
		if strings.TrimSpace(t.CategoryID) != "" {
			return false
		}
		for _, split := range t.Splits {
			if strings.TrimSpace(split.CategoryID) != "" {
				return false
			}
		}
		return true
	case "account":
		return strings.TrimSpace(t.AccountID) == ""
	default:
		return false
	}
}

// matchesMemberSelection matches an OR-set of member IDs. The reserved
// UnassignedMemberID selection represents rows whose actual MemberID is empty;
// other values continue to match member IDs exactly.
func matchesMemberSelection(selected string, txn domain.Transaction) bool {
	for _, id := range splitCSV(selected) {
		if id == UnassignedMemberID {
			if txn.IsUnassigned() {
				return true
			}
			continue
		}
		if txn.HasOwner(id) {
			return true
		}
	}
	return false
}

// compare orders two transactions by the given key in ascending sense, returning
// -1, 0, or 1. Direction is applied by the caller.
func compare(a, b domain.Transaction, key string, l Labels) int {
	switch key {
	case "amount":
		// SIGNED, not magnitude. The Amount column displays a signed figure —
		// ($620.00) in red beside $4,700.00 in green — so ordering it by absolute
		// value means the header says "Amount ↑" while the visible numbers do not
		// ascend. Every other column here sorts by what it displays; this one was
		// the exception, and the mismatch is invisible: a −$4.50 and a +$4.50 row
		// compared EQUAL and fell through to the id tiebreak.
		//
		// It also defeated the task the sort exists for. "Show me my biggest
		// expenses" is sort-amount-ascending, and under magnitude ordering a
		// $4,700 paycheck sat beside a $4,700 mortgage payment with no way to
		// separate them by sorting. Size-regardless-of-direction is still
		// reachable, and more precisely, by pairing this with the In/Out flow
		// filter.
		//
		// Known limitation, unchanged by this: minor units are compared raw, so a
		// multi-currency ledger orders by number-of-minor-units rather than by
		// converted value. Fixing that needs the FX table and the base currency,
		// which compare has no access to.
		switch x, y := a.Amount.Amount, b.Amount.Amount; {
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

// AbsAmount returns the absolute minor-unit amount of a transaction — its SIZE,
// with the income/expense direction discarded.
//
// It answers "how big is this?", which is the right question for the amount-range
// filter (whose fields are magnitudes with a zero floor), the "Large" quick
// filter, and the edit forms (which pair a magnitude field with a separate
// direction control). It is NOT the right question for the Amount column's sort:
// that column displays a signed figure, so it orders on the signed value.
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
	return matchTextWithLabels(t, q, Labels{Payee: resolve})
}

func matchTextWithLabels(t domain.Transaction, q string, labels Labels) bool {
	if amount, ok := parseSearchAmount(q, labels.BaseCurrency); ok {
		actual, err := amountInBase(t, labels)
		if err == nil && actual == amount {
			return true
		}
		if err != nil && labels.OnConversionError != nil {
			labels.OnConversionError(t, err)
		}
	}
	if strings.Contains(strings.ToLower(t.Payee), q) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Desc), q) {
		return true
	}
	if labels.Payee != nil {
		if strings.Contains(strings.ToLower(labels.Payee(t.Payee)), q) {
			return true
		}
	}
	tagQuery := strings.TrimPrefix(q, "#")
	for _, tag := range t.Tags {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), tagQuery) {
			return true
		}
	}
	return false
}

// parseSearchAmount recognizes an unambiguous exact amount query in major units.
// A leading currency symbol and grouping commas are accepted because they are
// common in the amount column's displayed value.
func parseSearchAmount(q, base string) (int64, bool) {
	s := strings.TrimSpace(q)
	base = effectiveBaseCurrency(base)
	s = strings.TrimPrefix(s, currency.Symbol(base))
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false
	}
	v, err := money.ParseMinor(s, currency.Decimals(base))
	if err != nil {
		return 0, false
	}
	if v < 0 {
		v = -v
	}
	return v, true
}

// parseAmountBound parses an amount filter bound (major units, e.g. "12.50") to
// base-currency minor units. A blank or unparseable bound returns ok=false so
// it's treated as unset — a half-typed number never hides every row. A negative
// bound is clamped to 0 (the filter compares absolute amounts). (C53)
func parseAmountBound(s, base string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	base = effectiveBaseCurrency(base)
	v, err := money.ParseMinor(s, currency.Decimals(base))
	if err != nil {
		return 0, false
	}
	if v < 0 {
		v, err = money.Abs(v)
		if err != nil {
			return 0, false
		}
	}
	return v, true
}

func effectiveBaseCurrency(base string) string {
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return "USD"
	}
	return base
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
