// SPDX-License-Identifier: MIT

// Package pages is the pure logic for user-authored custom pages: turning a page
// name into a stable URL slug, keeping slugs unique, ordering pages for the rail,
// filtering hidden ones, reordering by drag, and validating a page. It has no
// platform dependencies, so it unit-tests on native Go; the wasm UI persists the
// pages (in the dataset) and wires navigation, drag-reorder, and visibility.
package pages

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Slug converts a page name into a URL-safe segment: lowercased, with runs of
// non-alphanumeric characters collapsed to single hyphens and leading/trailing
// hyphens trimmed. A name with no usable characters yields "page" so the result
// is never empty (UniqueSlug then disambiguates).
func Slug(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "page"
	}
	return out
}

// UniqueSlug returns a slug derived from name that does not collide with any
// existing page's slug, except the page identified by exceptID (so renaming a
// page back to its own slug is allowed). Collisions are disambiguated by
// appending "-2", "-3", … The result is deterministic for a given input.
func UniqueSlug(name string, existing []domain.CustomPage, exceptID string) string {
	taken := make(map[string]bool, len(existing))
	for _, p := range existing {
		if p.ID == exceptID {
			continue
		}
		taken[p.Slug] = true
	}
	base := Slug(name)
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		cand := base + "-" + itoa(n)
		if !taken[cand] {
			return cand
		}
	}
}

// itoa renders a small positive int without importing strconv for one use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BySlug returns the page with the given slug and whether it was found.
func BySlug(ps []domain.CustomPage, slug string) (domain.CustomPage, bool) {
	for _, p := range ps {
		if p.Slug == slug {
			return p, true
		}
	}
	return domain.CustomPage{}, false
}

// ByID returns the page with the given id and whether it was found.
func ByID(ps []domain.CustomPage, id string) (domain.CustomPage, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return domain.CustomPage{}, false
}

// Ordered returns a copy of ps sorted for display: by Order ascending, ties
// broken by CreatedAt then ID so the result is stable and deterministic. The
// input is not modified.
func Ordered(ps []domain.CustomPage) []domain.CustomPage {
	out := append([]domain.CustomPage(nil), ps...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Visible returns the ordered pages with hidden ones removed — what the rail
// shows. The input is not modified.
func Visible(ps []domain.CustomPage) []domain.CustomPage {
	ordered := Ordered(ps)
	out := make([]domain.CustomPage, 0, len(ordered))
	for _, p := range ordered {
		if !p.Hidden {
			out = append(out, p)
		}
	}
	return out
}

// NextOrder returns an Order value that places a new page after all existing
// ones (max Order + 1, or 0 when there are none).
func NextOrder(ps []domain.CustomPage) int {
	max := -1
	for _, p := range ps {
		if p.Order > max {
			max = p.Order
		}
	}
	return max + 1
}

// Reorder moves the page identified by id to toIndex within the display order and
// returns the full set with Order renumbered 0..n-1 to match the new sequence, so
// the caller can persist every page's new position. toIndex is clamped to the
// valid range; an unknown id renumbers without moving anything. The input is not
// modified. Deterministic.
func Reorder(ps []domain.CustomPage, id string, toIndex int) []domain.CustomPage {
	ordered := Ordered(ps)
	from := -1
	for i, p := range ordered {
		if p.ID == id {
			from = i
			break
		}
	}
	if from >= 0 {
		if toIndex < 0 {
			toIndex = 0
		}
		if toIndex >= len(ordered) {
			toIndex = len(ordered) - 1
		}
		if toIndex != from {
			moved := ordered[from]
			ordered = append(ordered[:from], ordered[from+1:]...)
			ordered = append(ordered, domain.CustomPage{})
			copy(ordered[toIndex+1:], ordered[toIndex:])
			ordered[toIndex] = moved
		}
	}
	for i := range ordered {
		ordered[i].Order = i
	}
	return ordered
}

// Validate reports human-readable problems with a page, or nil if it's valid: a
// page needs a non-empty name and a non-empty slug. (Uniqueness is enforced at
// creation time via UniqueSlug, not here, since it needs the full set.)
func Validate(p domain.CustomPage) []string {
	var errs []string
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "A page needs a name.")
	}
	if strings.TrimSpace(p.Slug) == "" {
		errs = append(errs, "A page needs a web address (slug).")
	}
	return errs
}
