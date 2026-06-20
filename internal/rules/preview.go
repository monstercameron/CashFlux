package rules

// MatchCount returns how many of texts the rule would match — the "matches N
// existing transactions" preview a user wants before applying a rule to existing
// data (which is otherwise blind). Each text is typically a transaction's
// payee + " " + description, matched case-insensitively as a substring.
func (r Rule) MatchCount(texts []string) int {
	n := 0
	for _, t := range texts {
		if matches(t, r.Match) {
			n++
		}
	}
	return n
}

// Covered returns how many of texts are matched by at least one rule (first-match-
// wins), so the UI can show "N of M transactions auto-file by your rules" — a
// coverage signal that surfaces the gap of uncategorized-but-coverable entries.
func Covered(rs []Rule, texts []string) int {
	n := 0
	for _, t := range texts {
		if FirstMatch(rs, t) != nil {
			n++
		}
	}
	return n
}

// Uncovered returns how many of texts no rule matches — the rules a user might
// still want to add.
func Uncovered(rs []Rule, texts []string) int {
	return len(texts) - Covered(rs, texts)
}
