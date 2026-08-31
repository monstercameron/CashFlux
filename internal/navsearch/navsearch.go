// SPDX-License-Identifier: MIT

// Package navsearch ranks the sidebar's destinations against what someone has
// typed into the rail's filter box.
//
// The rail holds around thirty destinations across four groups, most of them
// behind collapsed section headers, so finding one means either remembering which
// section it lives in or opening several. Typing is faster than remembering, but
// only if the ordering is right: a filter that returns matches in menu order makes
// you read the whole list again, which is the work the box was meant to remove.
//
// So results are RANKED, and the ranking encodes what a short query usually means.
// Someone typing "bud" wants Budgets, not "Annual budget grid" — a prefix is a
// stronger signal than a substring, and a match on the destination's own name is
// stronger than one on the section it sits in. Ties keep menu order, so the list
// never reshuffles for reasons the reader cannot see.
package navsearch

import "strings"

// Item is one rail destination, already localized. The package deliberately holds
// no i18n keys or icons: it ranks the words the user can actually see, which is
// also what makes it testable without a browser.
type Item struct {
	// Label is the destination's displayed name ("Net worth").
	Label string
	// Path is the route to navigate to. It is never matched against — a URL is
	// not what someone is searching for, and matching it would make "/p/" pull in
	// every custom page.
	Path string
	// Section is the localized group this destination sits under ("Plan &
	// forecast", "My pages"). Matched, but ranked below Label, so typing a section
	// name gathers its contents without burying a destination of the same name.
	Section string
}

// score rates one item against one already-normalized term. Higher is better;
// zero means no match. The gaps between tiers are wide enough that a strong match
// on one term cannot be outweighed by weak matches on others.
func score(it Item, term string) int {
	label := strings.ToLower(it.Label)
	switch {
	case label == term:
		return 100
	case strings.HasPrefix(label, term):
		return 75
	case wordPrefix(label, term):
		// "worth" matching "Net worth": the start of any word reads as deliberate,
		// where a match in the middle of one is usually incidental.
		return 60
	case strings.Contains(label, term):
		return 40
	}
	section := strings.ToLower(it.Section)
	switch {
	case section == term:
		return 20
	case strings.HasPrefix(section, term):
		return 15
	case strings.Contains(section, term):
		return 10
	}
	return 0
}

// wordPrefix reports whether term begins any word in s.
func wordPrefix(s, term string) bool {
	for i := 0; i+len(term) <= len(s); i++ {
		if i > 0 && !isBreak(s[i-1]) {
			continue
		}
		if s[i:i+len(term)] == term {
			return true
		}
	}
	return false
}

func isBreak(b byte) bool {
	return b == ' ' || b == '-' || b == '&' || b == '/' || b == '(' || b == '.'
}

// Filter returns the items matching every term in query, best first.
//
// Terms are ANDed rather than ORed. "net worth" should mean the one destination
// that is both, not every destination that is either — an OR here returns most of
// the rail for a two-word query, which is the opposite of narrowing. Order within
// a score is the order given, so the menu's own sequence survives as the tiebreak.
//
// An empty or whitespace-only query returns nil, not everything: the caller uses
// that to mean "not searching" and render the normal rail, and returning the full
// list would make an empty box look like a filter that had matched all of it.
func Filter(items []Item, query string) []Item {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return nil
	}
	type scored struct {
		item  Item
		total int
		order int
	}
	var hits []scored
	for i, it := range items {
		total := 0
		matchedAll := true
		for _, t := range terms {
			s := score(it, t)
			if s == 0 {
				matchedAll = false
				break
			}
			total += s
		}
		if matchedAll {
			hits = append(hits, scored{it, total, i})
		}
	}
	// Insertion sort: the list is tens of items, and a stable sort by (score desc,
	// original order) is the whole requirement.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0; j-- {
			if hits[j].total > hits[j-1].total {
				hits[j], hits[j-1] = hits[j-1], hits[j]
				continue
			}
			break
		}
	}
	out := make([]Item, len(hits))
	for i, h := range hits {
		out[i] = h.item
	}
	return out
}
