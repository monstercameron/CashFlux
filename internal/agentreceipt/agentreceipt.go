// SPDX-License-Identifier: MIT

// Package agentreceipt aggregates what an agent did over a conversation into a
// short, plain-English cumulative receipt — "this chat: 3 transactions
// categorized, 1 category created" (AG20). It is pure: it turns a list of applied
// operation Kinds (from changeset.Receipt.Kinds, accumulated across the chat)
// into counted, human-readable phrases, and formats an optional cost line from a
// token total and dollar estimate computed at the UI edge. No syscall/js.
//
// AG20's point is trust: households let an agent touch shared money only when
// every change it made is visible and countable. This package is that count.
package agentreceipt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
)

// phrase describes how one op Kind reads in the receipt: a past-tense action and
// its singular/plural noun, e.g. add_transaction → "1 transaction recorded" /
// "3 transactions recorded".
type phrase struct {
	singular string
	plural   string
}

// kindPhrases maps each agent op Kind to its receipt wording. Kinds mirror the
// assistant tool / changeset dispatch names. An unknown Kind falls back to a
// generic "change" noun so a new op still counts rather than vanishing.
var kindPhrases = map[string]phrase{
	"add_transaction":              {"transaction recorded", "transactions recorded"},
	"categorize_transactions":      {"transaction categorized", "transactions categorized"},
	"categorize_transaction":       {"transaction categorized", "transactions categorized"},
	"create_category":              {"category created", "categories created"},
	"add_task":                     {"task added", "tasks added"},
	"complete_task":                {"task completed", "tasks completed"},
	"add_goal_contribution":        {"goal contribution added", "goal contributions added"},
	"add_account":                  {"account created", "accounts created"},
	"add_transfer":                 {"transfer recorded", "transfers recorded"},
	"update_account_balance":       {"balance updated", "balances updated"},
	"delete_transaction":           {"transaction deleted", "transactions deleted"},
	"merge_duplicate_transactions": {"duplicate merged", "duplicates merged"},
	"create_rule":                  {"rule created", "rules created"},
	"create_budget":                {"budget created", "budgets created"},
	"create_goal":                  {"goal created", "goals created"},
	"create_workflow":              {"workflow created", "workflows created"},
}

// nounFor returns the receipt phrase for a count of a given Kind, using the
// singular for n == 1 and a generic fallback for unknown kinds.
func nounFor(kind string, n int) string {
	p, ok := kindPhrases[kind]
	if !ok {
		if n == 1 {
			return "change applied"
		}
		return "changes applied"
	}
	if n == 1 {
		return p.singular
	}
	return p.plural
}

// Tally is the accumulated agent activity for one conversation: how many times
// each op Kind ran, plus the running token and cost totals.
type Tally struct {
	// Counts maps op Kind → number of successful applications this chat.
	Counts map[string]int
	// Tokens is the cumulative token usage across the conversation's turns.
	Tokens int
	// CostUSD is the cumulative estimated dollar cost; HasCost is false when no
	// cost estimate is available (unknown model / no usage).
	CostUSD float64
	HasCost bool
}

// NewTally returns an empty tally ready to Add to.
func NewTally() *Tally { return &Tally{Counts: map[string]int{}} }

// AddKinds increments the tally by one applied op per Kind (the output of
// changeset.Receipt.Kinds). Call it once per applied changeset.
func (t *Tally) AddKinds(kinds []string) {
	if t.Counts == nil {
		t.Counts = map[string]int{}
	}
	for _, k := range kinds {
		t.Counts[k]++
	}
}

// AddCost accumulates a turn's token usage and (when known) dollar cost.
func (t *Tally) AddCost(tokens int, costUSD float64, hasCost bool) {
	t.Tokens += tokens
	if hasCost {
		t.CostUSD += costUSD
		t.HasCost = true
	}
}

// TotalActions reports the number of successful agent operations tallied.
func (t *Tally) TotalActions() int {
	n := 0
	for _, c := range t.Counts {
		n += c
	}
	return n
}

// ActionPhrases returns the counted action phrases in a stable, human order —
// most-frequent first, ties broken alphabetically by phrase — e.g.
// ["3 transactions categorized", "1 category created"].
func (t *Tally) ActionPhrases() []string {
	type row struct {
		kind  string
		count int
		text  string
	}
	rows := make([]row, 0, len(t.Counts))
	for k, c := range t.Counts {
		if c <= 0 {
			continue
		}
		rows = append(rows, row{k, c, fmt.Sprintf("%d %s", c, nounFor(k, c))})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].text < rows[j].text
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.text
	}
	return out
}

// CostPhrase returns the plain-English cost line ("~$0.04, 1,240 tokens"), or ""
// when nothing has been spent/used yet. Honesty rules (UX-09): the dollar figure
// runs through the shared ai.FormatCostUSD so a sub-cent spend never collapses to
// a false "$0.00" (it reads "<$0.001" or "$0.004"), and tokens spent on a model
// with no known pricing say "cost unavailable" rather than implying the turn was
// free. Every assistant cost display formats through the one path.
func (t *Tally) CostPhrase() string {
	if t.Tokens <= 0 && !t.HasCost {
		return ""
	}
	parts := make([]string, 0, 2)
	if t.HasCost {
		c := ai.FormatCostUSD(t.CostUSD)
		// "<$0.001" already reads as an upper bound; leave the "~" off it.
		if strings.HasPrefix(c, "<") {
			parts = append(parts, c)
		} else {
			parts = append(parts, "~"+c)
		}
	}
	if t.Tokens > 0 {
		parts = append(parts, groupThousands(t.Tokens)+" tokens")
	}
	if !t.HasCost {
		parts = append(parts, "cost unavailable")
	}
	return strings.Join(parts, ", ")
}

// Summary renders the full one-line cumulative receipt, e.g.
// "This chat: 3 transactions categorized, 1 category created · ~$0.04, 1,240
// tokens". Returns "" when the agent has made no changes and spent nothing.
func (t *Tally) Summary() string {
	actions := t.ActionPhrases()
	cost := t.CostPhrase()
	if len(actions) == 0 && cost == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("This chat: ")
	if len(actions) > 0 {
		b.WriteString(strings.Join(actions, ", "))
	} else {
		b.WriteString("no changes")
	}
	if cost != "" {
		b.WriteString(" · ")
		b.WriteString(cost)
	}
	return b.String()
}

// groupThousands renders a non-negative int with comma thousands separators.
func groupThousands(n int) string {
	if n < 0 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
