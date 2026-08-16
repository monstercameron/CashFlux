// SPDX-License-Identifier: MIT

// Package spotlight is the map from "how do I add an income source?" to a control
// on a screen (PS8).
//
// The assistant could already answer that question in words, and words are the
// weakest possible answer to it: the person is looking at the app right now, and a
// paragraph describing a button is strictly worse than the button. What was missing
// was a vocabulary — a way for the model to name a real control without being told
// the DOM.
//
// So this is a curated list of the places somebody actually gets stuck, each
// pinned to the route it lives on and the test id it is marked with. Curated
// rather than generated on purpose: a model given the whole DOM will confidently
// point at the wrong thing, while a model given forty named destinations either
// finds one or admits it cannot. The second failure is recoverable and the first
// is not.
//
// Pure: no syscall/js. Table-tested on native Go.
package spotlight

import "strings"

// Target is one place the assistant can point at.
type Target struct {
	// ID is the stable name the model uses ("add-account").
	ID string
	// Route is the screen it lives on.
	Route string
	// TestID is the data-testid of the control to ring. Empty means the route
	// itself is the destination — some answers are "that's a whole screen", and
	// ringing an arbitrary control on it would be worse than ringing nothing.
	TestID string
	// What is a plain-English description of what the control does, used both to
	// match a request and to narrate the step.
	What string
	// Aliases are other ways people ask for it.
	Aliases []string
}

// targets is the curated set. Each entry is somewhere a real question lands.
var targets = []Target{
	{ID: "add-account", Route: "/accounts", TestID: "accounts-add", What: "add an account",
		Aliases: []string{"new account", "add a bank", "add savings", "open an account"}},
	{ID: "add-transaction", Route: "/transactions", TestID: "txn-add", What: "add a transaction",
		Aliases: []string{"record a purchase", "log spending", "enter an expense", "add income"}},
	{ID: "add-budget", Route: "/budgets", TestID: "budgets-add", What: "set up a budget",
		Aliases: []string{"new budget", "create a budget", "budget for a category"}},
	{ID: "add-goal", Route: "/goals", TestID: "goals-add", What: "start a savings goal",
		Aliases: []string{"new goal", "save for something", "set a target"}},
	{ID: "add-recurring", Route: "/recurring", TestID: "recurring-add", What: "add a recurring bill or income",
		Aliases: []string{"add a bill", "add income source", "add a subscription", "set up a payday", "recurring payment"}},
	{ID: "import-csv", Route: "/documents", TestID: "documents-csv", What: "import a bank CSV",
		Aliases: []string{"import statement", "upload transactions", "bring in my bank data"}},
	{ID: "ai-key", Route: "/settings", TestID: "settings-ai-key", What: "add your AI key",
		Aliases: []string{"set up the assistant", "openai key", "api key", "turn on ai"}},
	{ID: "categories", Route: "/categories", What: "manage your categories",
		Aliases: []string{"add a category", "rename a category", "edit categories"}},
	{ID: "rules", Route: "/rules", What: "set up an auto-categorisation rule",
		Aliases: []string{"auto categorize", "categorisation rule", "always put this in"}},
	{ID: "members", Route: "/members", What: "add someone to the household",
		Aliases: []string{"add a person", "add my partner", "household member"}},
	{ID: "backup", Route: "/settings", TestID: "settings-export", What: "back up your data",
		Aliases: []string{"export", "save a copy", "download my data"}},
	{ID: "appearance", Route: "/appearance", What: "change the look of the app",
		Aliases: []string{"dark mode", "theme", "change colours", "text size"}},
	{ID: "spend-meter", Route: "/smart", TestID: "ai-spend-meter", What: "see what AI has cost you",
		Aliases: []string{"ai spend", "what is ai costing", "ai bill"}},
	{ID: "activity", Route: "/activity", What: "see every change that's been made",
		Aliases: []string{"history", "audit", "what changed", "undo something"}},
}

// All returns the curated targets.
func All() []Target { return append([]Target(nil), targets...) }

// Get returns one target by id.
func Get(id string) (Target, bool) {
	for _, t := range targets {
		if strings.EqualFold(t.ID, strings.TrimSpace(id)) {
			return t, true
		}
	}
	return Target{}, false
}

// Find matches a free-text request to a target.
//
// Matching is on WORDS, not on substrings: a phrase matches when every one of its
// significant words appears somewhere in the request. Substring matching looked
// fine on the phrases it was written against and failed on real sentences —
// "how do I add a new income source?" does not contain the literal string "add
// income source", and a person asking that would have been told the app has no
// such thing. The most specific matching phrase (the most words) wins, so "add a
// subscription" beats a target that merely shares the word "add".
//
// It returns false rather than a best guess when nothing matches: pointing
// confidently at the wrong control is worse than saying "I'm not sure where that
// is", because the person then follows the wrong instruction all the way to its
// end.
func Find(request string) (Target, bool) {
	words := significantWords(strings.ToLower(request))
	if len(words) == 0 {
		return Target{}, false
	}
	inRequest := make(map[string]bool, len(words))
	for _, w := range words {
		inRequest[w] = true
	}
	best, bestScore := Target{}, 0
	consider := func(t Target, phrase string) {
		parts := significantWords(strings.ToLower(strings.ReplaceAll(phrase, "-", " ")))
		if len(parts) == 0 || len(parts) <= bestScore {
			return
		}
		for _, p := range parts {
			if !inRequest[p] {
				return
			}
		}
		best, bestScore = t, len(parts)
	}
	for _, t := range targets {
		consider(t, t.ID)
		consider(t, t.What)
		for _, a := range t.Aliases {
			consider(t, a)
		}
	}
	return best, bestScore > 0
}

// filler are the words that carry no meaning for matching. Dropping them is what
// lets "add a new income source" match "add income source"; keeping them would
// make every phrase depend on the exact articles somebody happened to type.
var filler = map[string]bool{
	"a": true, "an": true, "the": true, "my": true, "our": true, "i": true,
	"do": true, "how": true, "to": true, "in": true, "on": true, "for": true,
	"can": true, "where": true, "s": true, "is": true, "it": true, "want": true,
	"new": true, "of": true, "me": true, "you": true, "please": true,
}

// significantWords splits text into lowercase alphanumeric words, dropping filler.
func significantWords(text string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		w := cur.String()
		cur.Reset()
		if w != "" && !filler[w] {
			out = append(out, w)
		}
	}
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// Names returns every id, for the tool description so the model knows exactly what
// it may ask for rather than inventing a name and failing.
func Names() []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.ID)
	}
	return out
}
