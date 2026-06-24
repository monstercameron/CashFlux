// SPDX-License-Identifier: MIT

// Package cmdmatch ranks command-palette commands against a fuzzy query. A power
// user thinks in verbs ("add", "export"), so each command carries optional
// keywords/aliases matched alongside its noun title — typing "add" can surface
// "New transaction". Matching is case-insensitive subsequence search; title
// matches rank above keyword-only matches, and ties keep input order.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package cmdmatch

import (
	"sort"
	"strings"
)

// Command is a searchable palette command: a stable Title plus optional Keywords
// (verbs / synonyms / aliases) that also match the query.
type Command struct {
	ID       string
	Title    string
	Keywords []string
}

// keywordPenalty makes a title match always outrank a keyword-only match of the
// same raw score, so the noun a user sees still wins when it matches directly.
const keywordPenalty = 1000

// Match returns the commands matching query, ranked best-first. An empty/blank
// query returns every command in its original order. Otherwise a command matches
// when the query is a subsequence (case-insensitive) of its title or any keyword;
// the command's score is the best (lowest) across them, and results sort ascending
// by score with input order breaking ties.
func Match(query string, cmds []Command) []Command {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Command, len(cmds))
		copy(out, cmds)
		return out
	}

	type scored struct {
		cmd   Command
		score int
		order int
	}
	var hits []scored
	for i, c := range cmds {
		best := -1
		if s, ok := subseqScore(q, strings.ToLower(c.Title)); ok {
			best = s
		}
		for _, kw := range c.Keywords {
			if s, ok := subseqScore(q, strings.ToLower(kw)); ok {
				s += keywordPenalty
				if best < 0 || s < best {
					best = s
				}
			}
		}
		if best >= 0 {
			hits = append(hits, scored{cmd: c, score: best, order: i})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score < hits[b].score
		}
		return hits[a].order < hits[b].order
	})
	out := make([]Command, len(hits))
	for i, h := range hits {
		out[i] = h.cmd
	}
	return out
}

// subseqScore reports whether query is an (ASCII, lowercased) subsequence of s and,
// if so, a score where lower is better: the index of the first matched character
// plus the total gaps between matched characters — so early, contiguous matches
// (e.g. a prefix) rank ahead of scattered ones.
func subseqScore(query, s string) (int, bool) {
	qi, first, prev, gaps := 0, -1, -1, 0
	for i := 0; i < len(s) && qi < len(query); i++ {
		if s[i] == query[qi] {
			if first < 0 {
				first = i
			}
			if prev >= 0 {
				gaps += i - prev - 1
			}
			prev = i
			qi++
		}
	}
	if qi < len(query) {
		return 0, false
	}
	return first + gaps, true
}
