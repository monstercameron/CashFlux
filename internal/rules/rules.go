// Package rules is the auto-categorization engine: user-defined rules that match
// a transaction's payee/description and assign a category (and tags). Matching is
// pure and deterministic — first matching rule wins — and unit-tested on native
// Go. The UI manages the rules and applies them on entry/import.
package rules

import "strings"

// Rule matches transaction text and assigns a category and/or tags.
type Rule struct {
	ID            string
	Match         string // case-insensitive substring matched against payee + description
	SetCategoryID string
	SetTags       []string
}

// matches reports whether pattern (trimmed, case-insensitive) is a substring of
// text. An empty pattern never matches.
func matches(text, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

// FirstMatch returns the first rule whose Match is found in text, or nil.
func FirstMatch(rs []Rule, text string) *Rule {
	for i := range rs {
		if matches(text, rs[i].Match) {
			return &rs[i]
		}
	}
	return nil
}

// Category returns the category id assigned by the first rule matching the
// payee/description, or "" if none match.
func Category(rs []Rule, payee, desc string) string {
	if r := FirstMatch(rs, payee+" "+desc); r != nil {
		return r.SetCategoryID
	}
	return ""
}

// Tags returns the tags assigned by the first rule matching the payee/
// description, or nil if none match.
func Tags(rs []Rule, payee, desc string) []string {
	if r := FirstMatch(rs, payee+" "+desc); r != nil {
		return r.SetTags
	}
	return nil
}
