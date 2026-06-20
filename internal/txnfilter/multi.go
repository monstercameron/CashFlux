package txnfilter

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// FieldTags is the multi-select tags dimension (the single-value Criteria has no
// tags filter — tags are only reachable via free-text search there). Declared here
// so the multi-select model can carry it without touching the existing field set.
const FieldTags FilterField = "tags"

// MultiCriteria is the multi-value filter model behind C83: each dimension holds a
// set of selected values, matched with the standard mental model — OR within a
// dimension (any selected account/category/member/tag), AND across dimensions. An
// empty dimension is unconstrained (matches everything). It is additive: the
// single-value Criteria is unchanged; the screen migrates onto this later.
type MultiCriteria struct {
	Accounts   []string
	Categories []string
	Members    []string
	Tags       []string
}

// Matches reports whether a transaction passes every engaged dimension. Within a
// dimension the transaction need match only one selected value (OR); across
// dimensions it must satisfy all engaged ones (AND). Account/Category/Member match
// the transaction's id; Tags matches when the transaction carries any selected tag.
func (m MultiCriteria) Matches(t domain.Transaction) bool {
	if !inSetOrEmpty(m.Accounts, t.AccountID) {
		return false
	}
	if !inSetOrEmpty(m.Categories, t.CategoryID) {
		return false
	}
	if !inSetOrEmpty(m.Members, t.MemberID) {
		return false
	}
	if len(m.Tags) > 0 && !anyShared(m.Tags, t.Tags) {
		return false
	}
	return true
}

// Filter returns the transactions that Match, preserving input order (sorting and
// pagination remain the single-value Criteria's job).
func (m MultiCriteria) Filter(txns []domain.Transaction) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if m.Matches(t) {
			out = append(out, t)
		}
	}
	return out
}

// IsEmpty reports whether no dimension is engaged (the filter selects everything).
func (m MultiCriteria) IsEmpty() bool {
	return len(m.Accounts) == 0 && len(m.Categories) == 0 && len(m.Members) == 0 && len(m.Tags) == 0
}

// Normalize returns a copy with every dimension de-duplicated and sorted, so two
// selections built in a different order compare equal and chips render stably.
func (m MultiCriteria) Normalize() MultiCriteria {
	return MultiCriteria{
		Accounts:   dedupSorted(m.Accounts),
		Categories: dedupSorted(m.Categories),
		Members:    dedupSorted(m.Members),
		Tags:       dedupSorted(m.Tags),
	}
}

// Equal reports whether two filters select the same set (order-insensitive). Slices
// aren't comparable with ==, so this is the explicit replacement the scope-change
// check needs.
func (m MultiCriteria) Equal(other MultiCriteria) bool {
	a, b := m.Normalize(), other.Normalize()
	return equalStrings(a.Accounts, b.Accounts) &&
		equalStrings(a.Categories, b.Categories) &&
		equalStrings(a.Members, b.Members) &&
		equalStrings(a.Tags, b.Tags)
}

// Values returns the selected values for one dimension (nil for an unknown field).
func (m MultiCriteria) Values(field FilterField) []string {
	switch field {
	case FieldAccount:
		return m.Accounts
	case FieldCategory:
		return m.Categories
	case FieldMember:
		return m.Members
	case FieldTags:
		return m.Tags
	default:
		return nil
	}
}

// Toggle adds value to a dimension if absent, or removes it if present — the
// natural action of clicking a checkbox. Unknown fields are ignored.
func (m MultiCriteria) Toggle(field FilterField, value string) MultiCriteria {
	if contains(m.set(field), value) {
		return m.Without(field, value)
	}
	return m.Add(field, value)
}

// Add returns a copy with value added to a dimension (no-op if already present or
// the field is unknown).
func (m MultiCriteria) Add(field FilterField, value string) MultiCriteria {
	cur := m.set(field)
	if contains(cur, value) {
		return m
	}
	return m.withSet(field, append(append([]string{}, cur...), value))
}

// Without returns a copy with one value removed from a dimension — the per-value
// chip ✕, as opposed to clearing the whole dimension.
func (m MultiCriteria) Without(field FilterField, value string) MultiCriteria {
	cur := m.set(field)
	next := make([]string, 0, len(cur))
	for _, v := range cur {
		if v != value {
			next = append(next, v)
		}
	}
	return m.withSet(field, next)
}

// ActiveValues lists one entry per selected value, in dimension then value order,
// for the per-value removable chips. Each chip's ✕ maps to Without(Field, Value).
func (m MultiCriteria) ActiveValues() []ActiveFilter {
	var out []ActiveFilter
	for _, field := range []FilterField{FieldAccount, FieldCategory, FieldMember, FieldTags} {
		for _, v := range dedupSorted(m.set(field)) {
			out = append(out, ActiveFilter{Field: field, Value: v})
		}
	}
	return out
}

// set returns the raw slice for a dimension.
func (m MultiCriteria) set(field FilterField) []string {
	switch field {
	case FieldAccount:
		return m.Accounts
	case FieldCategory:
		return m.Categories
	case FieldMember:
		return m.Members
	case FieldTags:
		return m.Tags
	default:
		return nil
	}
}

// withSet returns a copy with one dimension replaced.
func (m MultiCriteria) withSet(field FilterField, vals []string) MultiCriteria {
	switch field {
	case FieldAccount:
		m.Accounts = vals
	case FieldCategory:
		m.Categories = vals
	case FieldMember:
		m.Members = vals
	case FieldTags:
		m.Tags = vals
	}
	return m
}

// inSetOrEmpty reports whether set is empty (unconstrained) or contains val.
func inSetOrEmpty(set []string, val string) bool {
	return len(set) == 0 || contains(set, val)
}

// anyShared reports whether the two slices share at least one value.
func anyShared(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := seen[v]; ok {
			return true
		}
	}
	return false
}

func contains(set []string, val string) bool {
	for _, v := range set {
		if v == val {
			return true
		}
	}
	return false
}

// dedupSorted returns a sorted copy with duplicates removed (nil-safe).
func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
