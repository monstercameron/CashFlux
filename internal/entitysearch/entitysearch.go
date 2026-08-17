// SPDX-License-Identifier: MIT

// Package entitysearch is one instant substring search across everything the
// household has recorded — accounts, transactions, budgets, goals and to-dos
// (LF-4).
//
// The command palette already ranks COMMANDS: "add a transaction", "export
// CSV". What it could not do is find a THING. Someone who remembers paying
// Greenfield Market but not which month, or has a goal called "Roof" somewhere
// among thirty, had to guess which page to open and then use that page's own
// filter. That is the app asking the user to know its information architecture
// before it will help them.
//
// Three properties shape the design:
//
//   - Substring, case-folded, no fuzzy matching. The palette's fuzzy matcher is
//     right for a fixed list of twenty commands; over ten thousand transactions
//     it produces confident nonsense, because with enough candidates something
//     always matches loosely. A person searching their own records knows what
//     they typed.
//   - Ranked by KIND first, not by score. An account named "Roof fund" and a
//     transaction described "roof repair" are not competing for one slot: the
//     account is a place, the transaction is an event, and burying five accounts
//     under two hundred matching transactions makes the search useless for the
//     thing it is best at.
//   - Every hit carries where to go AND what to do when it gets there. A result
//     that navigates to /transactions without applying a filter has moved the
//     reader to a haystack and called it an answer.
//
// Pure Go, no platform dependencies.
package entitysearch

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Kind names what a hit is.
type Kind string

const (
	// KindAccount is an account.
	KindAccount Kind = "account"
	// KindBudget is a budget.
	KindBudget Kind = "budget"
	// KindGoal is a goal.
	KindGoal Kind = "goal"
	// KindTask is a to-do.
	KindTask Kind = "task"
	// KindTransaction is a ledger row.
	KindTransaction Kind = "transaction"
)

// kindOrder ranks the kinds. Places and plans before events: there are a handful
// of accounts and goals and thousands of transactions, so ordering by score
// alone would bury every account under a wall of matching ledger rows.
var kindOrder = map[Kind]int{
	KindAccount: 0, KindBudget: 1, KindGoal: 2, KindTask: 3, KindTransaction: 4,
}

// MinQuery is the shortest query that is searched. Two characters over a full
// ledger matches almost everything, which is indistinguishable from matching
// nothing.
const MinQuery = 2

// DefaultLimit bounds the result list. A palette shows what fits on a screen;
// beyond that the answer is "narrow it", not another hundred rows.
const DefaultLimit = 20

// Hit is one result.
type Hit struct {
	Kind Kind
	// ID is the entity's id.
	ID string
	// Title is the primary line — the name or description.
	Title string
	// Subtitle is the secondary line: an account's type, a transaction's date and
	// amount. It is what disambiguates three results with the same title.
	Subtitle string
	// Route is where to go.
	Route string
	// Query, when set, is the text the destination should filter by, so a
	// transaction hit lands on the ledger already narrowed rather than dumping
	// the reader into the whole thing.
	Query string
	// pos is where the match fell in the searched text; an earlier match ranks
	// higher within a kind, because a name that STARTS with what you typed is
	// almost always the one you meant.
	pos int
}

// Input is everything searched. Each slice is optional; a caller that only wants
// accounts passes only accounts.
type Input struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
	// Limit caps the results; zero uses DefaultLimit.
	Limit int
	// FormatAmount renders a transaction's amount for its subtitle. Optional —
	// without it the subtitle carries the date alone, because this package must
	// not decide how money is displayed.
	FormatAmount func(domain.Transaction) string
}

// Search returns the matches for a query, ranked.
//
// A query shorter than MinQuery returns nothing rather than everything: two
// characters over a full ledger matches almost every row, which is the same
// experience as matching none of them, except slower.
func Search(query string, in Input) []Hit {
	q := strings.ToLower(strings.TrimSpace(query))
	if len([]rune(q)) < MinQuery {
		return nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	var hits []Hit

	for _, a := range in.Accounts {
		if a.Archived {
			// An archived account is not somewhere the reader can act. Including it
			// would answer "where is that account" with a place they cannot use.
			continue
		}
		if p, ok := match(q, a.Name); ok {
			hits = append(hits, Hit{Kind: KindAccount, ID: a.ID, Title: a.Name,
				Subtitle: string(a.Type), Route: "/accounts", pos: p})
		}
	}
	for _, b := range in.Budgets {
		if p, ok := match(q, b.Name); ok {
			hits = append(hits, Hit{Kind: KindBudget, ID: b.ID, Title: b.Name,
				Route: "/budgets", pos: p})
		}
	}
	for _, g := range in.Goals {
		if g.Archived {
			continue
		}
		if p, ok := match(q, g.Name); ok {
			hits = append(hits, Hit{Kind: KindGoal, ID: g.ID, Title: g.Name,
				Route: "/goals", pos: p})
		}
	}
	for _, t := range in.Tasks {
		if t.Status == domain.StatusDone {
			// A finished to-do is history. Surfacing it alongside open work makes
			// the reader check whether each result is still live.
			continue
		}
		if p, ok := match(q, t.Title); ok {
			hits = append(hits, Hit{Kind: KindTask, ID: t.ID, Title: t.Title,
				Route: "/todo", pos: p})
		}
	}
	for _, t := range in.Transactions {
		// Payee first, then description: someone searching a merchant is searching
		// the payee, and a description match on the same row should not produce a
		// second hit for one transaction.
		p, ok := match(q, t.Payee)
		if !ok {
			p, ok = match(q, t.Desc)
		}
		if !ok {
			continue
		}
		title := t.Payee
		if strings.TrimSpace(title) == "" {
			title = t.Desc
		}
		sub := t.Date.Format("Jan 2, 2006")
		if in.FormatAmount != nil {
			sub += " · " + in.FormatAmount(t)
		}
		hits = append(hits, Hit{Kind: KindTransaction, ID: t.ID, Title: title,
			Subtitle: sub, Route: "/transactions", Query: strings.TrimSpace(query), pos: p})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if kindOrder[hits[i].Kind] != kindOrder[hits[j].Kind] {
			return kindOrder[hits[i].Kind] < kindOrder[hits[j].Kind]
		}
		if hits[i].pos != hits[j].pos {
			return hits[i].pos < hits[j].pos
		}
		return hits[i].Title < hits[j].Title
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// match reports whether text contains the folded query, and where.
func match(q, text string) (int, bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return 0, false
	}
	i := strings.Index(t, q)
	if i < 0 {
		return 0, false
	}
	return i, true
}

// CountByKind tallies a result set, for a "3 accounts · 12 transactions" line.
func CountByKind(hits []Hit) map[Kind]int {
	out := map[Kind]int{}
	for _, h := range hits {
		out[h.Kind]++
	}
	return out
}
