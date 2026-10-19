// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// CollapseThreshold limits a closed dimension to eight options plus selections.
const CollapseThreshold = 8

// UnassignedOwner is the explicit choice that clears transaction ownership.
const UnassignedOwner = "__bulk_unassigned__"

// Option is a platform-independent filter choice.
type Option struct{ Value, Label string }

// CategoryTotal carries a category amount and its display share.
type CategoryTotal struct {
	ID, Name string
	Minor    int64
	Share    float64
}

// Presentation supplies localization at the rendering boundary.
type Presentation struct {
	Text  func(string, ...any) string
	Money func(money.Money) string
	Date  func(time.Time) string
}

// FormatDate contains the toolbar's pure formatFilterDate decision.
func FormatDate(style prefs.DateStyle, value string) string {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	layout := DateLayout(style)
	return parsed.Format(layout)
}

// ParseDate contains the toolbar's pure parseFilterDate decision.
func ParseDate(style prefs.DateStyle, value string) (string, bool) {
	for _, layout := range []string{DateLayout(style), "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Format("2006-01-02"), true
		}
	}
	return "", false
}

// DateLayout contains the toolbar's pure filterDateLayout decision.
func DateLayout(style prefs.DateStyle) string {
	switch style {
	case prefs.DateUS:
		return "01/02/2006"
	case prefs.DateEU:
		return "02/01/2006"
	case prefs.DateLong:
		return "Jan 2, 2006"
	default:
		return "2006-01-02"
	}
}

// ApplyParsedDateBounds contains the toolbar's pure applyParsedDateBounds decision.
func ApplyParsedDateBounds(current *txnfilter.Criteria, parsed txnfilter.Criteria) {
	if parsed.From != "" || parsed.To != "" {
		current.From, current.To = parsed.From, parsed.To
		current.PeriodPreset = txnfilter.PeriodCustom
	}
}

// Contains contains the toolbar's pure containsStr decision.
func Contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// ChipDate contains the toolbar's pure formatTxnChipDateWith decision.
func ChipDate(iso string, format func(time.Time) string) string {
	date, err := dateutil.ParseDate(iso)
	if err != nil {
		return iso
	}
	return format(date)
}

// ExcludedCount contains the toolbar's pure excludedTxnCount decision.
func ExcludedCount(txns []domain.Transaction) int {
	n := 0
	for _, txn := range txns {
		if txn.ExcludeFromReports {
			n++
		}
	}
	return n
}

// VisibleOptions contains the toolbar's pure visibleFilterGroupOptions decision.
func VisibleOptions(options []Option, selected []string, collapsed bool) []Option {
	if !collapsed {
		visible := make([]Option, 0, len(options))
		for _, option := range options {
			if option.Value != "" {
				visible = append(visible, option)
			}
		}
		return visible
	}

	visible := make([]Option, 0, CollapseThreshold+len(selected))
	for _, option := range options {
		if option.Value == "" {
			continue
		}
		if len(visible) < CollapseThreshold || Contains(selected, option.Value) {
			visible = append(visible, option)
		}
	}
	return visible
}

// AddTag contains the toolbar's pure addTxnTag decision.
func AddTag(tags []string, tag string) []string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return tags
	}
	for _, existing := range tags {
		if strings.EqualFold(strings.TrimSpace(existing), tag) {
			return tags
		}
	}
	return append(append([]string(nil), tags...), tag)
}

// HasTag contains the toolbar's pure txnHasTag decision.
func HasTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if strings.EqualFold(strings.TrimSpace(existing), strings.TrimSpace(tag)) {
			return true
		}
	}
	return false
}

// UserColumnVisible contains the toolbar's pure txnUserColumnVisible decision.
func UserColumnVisible(enabled bool, memberScope string, memberCount int) bool {
	return enabled && memberScope == "" && memberCount > 1
}

// CountNumber contains the toolbar's pure txnCountNumber decision.
func CountNumber(n int) string {
	digits := strconv.Itoa(n)
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var out strings.Builder
	out.Grow(len(digits) + len(digits)/3)
	out.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		out.WriteByte(',')
		out.WriteString(digits[i : i+3])
	}
	return out.String()
}

// MemberInitials contains the toolbar's pure txnMemberInitials decision.
func MemberInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "—"
	}
	first := []rune(parts[0])
	if len(first) == 0 {
		return "—"
	}
	initials := strings.ToUpper(string(first[0]))
	if len(parts) > 1 {
		last := []rune(parts[len(parts)-1])
		if len(last) > 0 {
			initials += strings.ToUpper(string(last[0]))
		}
	}
	return initials
}

// ToggleOwner contains the toolbar's pure toggleTxnBulkOwner decision.
func ToggleOwner(selected []string, value string) []string {
	if value == UnassignedOwner {
		if len(selected) == 1 && selected[0] == value {
			return nil
		}
		return []string{value}
	}
	result := make([]string, 0, len(selected)+1)
	found := false
	for _, owner := range selected {
		if owner == UnassignedOwner {
			continue
		}
		if owner == value {
			found = true
			continue
		}
		result = append(result, owner)
	}
	if !found {
		result = append(result, value)
	}
	return result
}

// AppliedOwners contains the toolbar's pure txnBulkAppliedOwners decision.
func AppliedOwners(selected []string) []string {
	for _, owner := range selected {
		if owner == UnassignedOwner {
			return nil
		}
	}
	return append([]string(nil), selected...)
}

// SameOwners contains the toolbar's pure sameOwnerList decision.
func SameOwners(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// WithCurrency contains the toolbar's pure txnCountWithCurrency decision.
func WithCurrency(value money.Money, base string) money.Money {
	if value.Currency == "" {
		value.Currency = base
	}
	return value
}

// CategoryShare contains the toolbar's pure txnCategoryShare decision.
func CategoryShare(amount, spent int64) float64 {
	if spent <= 0 || amount <= 0 {
		return 0
	}
	return float64(amount) * 100 / float64(spent)
}

// ShowUpcoming contains the toolbar's pure txnCountShouldShowUpcoming decision.
func ShowUpcoming(filter txnfilter.Criteria) bool {
	return strings.TrimSpace(filter.Text) == ""
}

// ShowsNet contains the toolbar's pure txnCountShowsNet decision.
func ShowsNet(spent, income money.Money) bool {
	return spent.Amount > 0 && income.Amount > 0
}

// AnnouncementScope contains the toolbar's pure txnAnnouncementScope decision.
func AnnouncementScope(filter txnfilter.Criteria) txnfilter.Criteria {
	filter.Sort, filter.Dir, filter.Page, filter.PageSize = "", "", 0, 0
	return filter
}

// LiveMode contains the toolbar's pure liveMode decision.
func LiveMode(announce bool) string {
	if announce {
		return "polite"
	}
	return "off"
}

// AmountChipLabel formats the toolbar decision with caller-supplied presentation.
func (p Presentation) AmountChipLabel(bound, value, base string) string {
	operator := ""
	switch bound {
	case "min":
		operator = "≥"
	case "max":
		operator = "≤"
	default:
		return value
	}
	minor, err := money.ParseMinor(value, currency.Decimals(base))
	if err != nil {
		return p.Text("transactions.chipAmountBound", operator, value)
	}
	return p.Text("transactions.chipAmountBound", operator, p.Money(money.New(minor, base)))
}

// RangeError formats the toolbar decision with caller-supplied presentation.
func (p Presentation) RangeError(filter txnfilter.Criteria, base string) string {
	if filter.AmountMin != "" && filter.AmountMax != "" {
		min, minErr := money.ParseMinor(filter.AmountMin, currency.Decimals(base))
		max, maxErr := money.ParseMinor(filter.AmountMax, currency.Decimals(base))
		if minErr == nil && maxErr == nil && min > max {
			return p.Text("transactions.filterRangeAmountInvalid")
		}
	}
	if filter.From != "" && filter.To != "" {
		from, fromErr := dateutil.ParseDate(filter.From)
		to, toErr := dateutil.ParseDate(filter.To)
		if fromErr == nil && toErr == nil && from.After(to) {
			return p.Text("transactions.filterRangeDateInvalid")
		}
	}
	return ""
}

// PageRange formats the toolbar decision with caller-supplied presentation.
func (p Presentation) PageRange(page, pageSize, total int) string {
	if total <= 0 {
		return ""
	}
	if pageSize == 0 {
		pageSize = txnfilter.DefaultPageSize
	}
	if pageSize <= 0 || pageSize >= total {
		return p.Text("txu5.range", CountNumber(1), CountNumber(total), CountNumber(total))
	}
	if page < 1 {
		page = 1
	}
	lastPage := (total-1)/pageSize + 1
	if page > lastPage {
		page = lastPage
	}
	start := (page-1)*pageSize + 1
	end := total
	if pageSize-1 < total-start {
		end = start + pageSize - 1
	}
	return p.Text("txu5.range", CountNumber(start), CountNumber(end), CountNumber(total))
}

// CountPeriod formats the toolbar decision with caller-supplied presentation.
func (p Presentation) CountPeriod(filter txnfilter.Criteria) string {
	if filter.From == "" && filter.To == "" {
		return p.Text("r2a.allDates")
	}
	from, fromErr := time.Parse("2006-01-02", filter.From)
	to, toErr := time.Parse("2006-01-02", filter.To)
	if filter.From == "" && toErr == nil {
		return p.Text("txnTQ6.throughDate", p.Date(to))
	}
	if filter.To == "" && fromErr == nil {
		return p.Text("txnTQ6.fromDate", p.Date(from))
	}
	if fromErr == nil && toErr == nil && from.Year() == to.Year() && from.Month() == to.Month() && from.Day() == 1 && to.AddDate(0, 0, 1).Day() == 1 {
		return p.Text("r2a.month", p.Text("budgets.month."+strings.ToLower(from.Month().String())), from.Year())
	}
	if fromErr == nil && toErr == nil {
		return p.Text("txnperiod.customRange", p.Date(from), p.Date(to))
	}
	return p.Text("r2a.customRange")
}

// CategoryTopFive formats the toolbar decision with caller-supplied presentation.
func (p Presentation) CategoryTopFive(categories []CategoryTotal, selected string) []CategoryTotal {
	if len(categories) <= 5 {
		return categories
	}
	result := append([]CategoryTotal(nil), categories[:5]...)
	selectedFound := false
	for _, category := range result {
		selectedFound = selectedFound || selected != "" && category.ID == selected
	}
	other := CategoryTotal{ID: "__other__", Name: p.Text("v5sum.other")}
	for _, category := range categories[5:] {
		if selected != "" && category.ID == selected && !selectedFound {
			result = append(result, category)
			selectedFound = true
			continue
		}
		var err error
		other.Minor, err = money.Add(other.Minor, category.Minor)
		if err != nil {
			return categories
		}
	}
	if other.Minor == 0 {
		return result
	}
	return append(result, other)
}
