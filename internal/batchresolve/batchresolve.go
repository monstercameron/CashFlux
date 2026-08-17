// SPDX-License-Identifier: MIT

// Package batchresolve compresses a review backlog into a handful of proposed
// rules (E4).
//
// A 186-item review queue is not 186 decisions. It is usually six or seven
// decisions repeated: "everything from this merchant is groceries", made
// twenty-eight times. Reviewing it one row at a time is the app asking the user
// to be the pattern-matcher, which is the one job software is unambiguously
// better at.
//
// So the engine reads the queue, finds the repeated shapes, and proposes rules:
// "186 items resolve under 6 rules — 172 the app is confident about, 14 that
// need you." The 14 are the point as much as the 172. A batch tool that claims
// to handle everything is one nobody trusts twice.
//
// Three rules shape it:
//
//   - A proposal must be EVIDENCED by the rows it came from, and those rows have
//     to agree. A merchant filed to three different categories does not produce a
//     rule; it produces a question.
//   - Confidence is about the evidence, not the count. Twenty rows that all agree
//     is high confidence; twenty rows that mostly agree is not, no matter how
//     many there are.
//   - Coverage is reported honestly, including what is left. "186 items resolve"
//     when 14 do not is the kind of claim that survives exactly one use.
//
// Pure Go: it proposes, it never applies.
package batchresolve

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/rules"
)

// MinRows is how many rows a merchant needs before a rule is proposed for it.
//
// Three, because two is a coincidence often enough to matter: a rule built from
// two rows is a rule the user will be correcting later, and the cost of a wrong
// rule is every future transaction it silently mis-files.
const MinRows = 3

// AgreementForHigh is the share of rows that must agree on one category for a
// proposal to be high-confidence.
//
// Not 1.0: a merchant with nineteen groceries and one "gifts" is still a
// groceries rule, and demanding unanimity would push the most useful proposals
// into the needs-you pile for a single outlier. Not 0.6 either — at that point
// the rule is wrong for two rows in five, which is not a rule, it is a guess.
const AgreementForHigh = 0.85

// Tier says how much a proposal should be trusted.
type Tier int

const (
	// TierNeedsYou is a proposal whose rows do not agree well enough to apply
	// without being read.
	TierNeedsYou Tier = iota
	// TierConfident is a proposal whose rows agree.
	TierConfident
)

// Row is one item in the review backlog.
type Row struct {
	// TxnID is the transaction awaiting review.
	TxnID string
	// Payee is what the row groups by. Callers should pass the normalized
	// merchant key, so "KROGER #418" and "Kroger #522" land together.
	Payee string
	// CategoryID is the category this row would be filed to — the app's current
	// suggestion, or the category a similar row already carries. Empty means
	// nothing has been suggested, and the row cannot support a proposal.
	CategoryID string
}

// Proposal is one rule the engine would create, and the rows behind it.
type Proposal struct {
	// Match is the rule's match string — the merchant.
	Match string
	// CategoryID is what the rule would file matching transactions to.
	CategoryID string
	// TxnIDs are the rows this proposal resolves, in input order. They are the
	// evidence: a proposal that cannot show its rows is asking to be trusted.
	TxnIDs []string
	// Agreement is the share of the merchant's rows that pointed at CategoryID.
	Agreement float64
	// Tier is whether the rows agree well enough to apply unread.
	Tier Tier
	// Disagreed is how many of the merchant's rows pointed somewhere else. It is
	// reported rather than hidden: those rows are exactly the ones a user should
	// look at before accepting.
	Disagreed int
}

// Count is how many rows this proposal resolves.
func (p Proposal) Count() int { return len(p.TxnIDs) }

// Rule renders the proposal as a rules-engine rule, with the caller's id.
func (p Proposal) Rule(id string) rules.Rule {
	return rules.Rule{ID: id, Match: p.Match, SetCategoryID: p.CategoryID}
}

// Plan is the whole compression: the proposals plus what they do and do not
// cover.
type Plan struct {
	// Proposals are ordered by how many rows they resolve, most first — the
	// biggest win is the one worth reading first.
	Proposals []Proposal
	// TotalRows is the size of the backlog considered.
	TotalRows int
	// Unresolved is how many rows no proposal covers: merchants seen too few
	// times, or rows with no suggested category at all.
	Unresolved int
}

// Confident and NeedsYou split the proposals by tier.
func (p Plan) Confident() []Proposal { return p.byTier(TierConfident) }

// NeedsYou returns the proposals whose rows do not agree well enough.
func (p Plan) NeedsYou() []Proposal { return p.byTier(TierNeedsYou) }

func (p Plan) byTier(t Tier) []Proposal {
	out := make([]Proposal, 0, len(p.Proposals))
	for _, pr := range p.Proposals {
		if pr.Tier == t {
			out = append(out, pr)
		}
	}
	return out
}

// ResolvedRows is how many backlog rows the proposals cover.
func (p Plan) ResolvedRows() int {
	var n int
	for _, pr := range p.Proposals {
		n += pr.Count()
	}
	return n
}

// ConfidentRows is how many rows the confident proposals cover — the number in
// "172 of 186".
func (p Plan) ConfidentRows() int {
	var n int
	for _, pr := range p.Confident() {
		n += pr.Count()
	}
	return n
}

// ReductionPct is how much smaller the backlog becomes if every proposal is
// accepted, as a whole percent. Zero when there is nothing to compress, so a
// caller never divides by an empty backlog.
func (p Plan) ReductionPct() int {
	if p.TotalRows <= 0 {
		return 0
	}
	return int(float64(p.ResolvedRows()) / float64(p.TotalRows) * 100)
}

// Build reads a review backlog and proposes the rules that would clear it.
//
// Rows with no suggested category cannot support a proposal — there is nothing
// to propose — and are counted as unresolved rather than dropped, because a plan
// that quietly ignores a third of the queue reports a reduction it did not
// achieve.
func Build(rows []Row) Plan {
	plan := Plan{TotalRows: len(rows)}
	if len(rows) == 0 {
		return plan
	}
	type group struct {
		order   []string // category ids, first-seen order, for stable ties
		byCat   map[string][]string
		total   int
		display string
	}
	groups := map[string]*group{}
	var groupOrder []string
	for _, r := range rows {
		key := merchantKey(r.Payee)
		if key == "" || r.CategoryID == "" {
			plan.Unresolved++
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &group{byCat: map[string][]string{}, display: strings.TrimSpace(r.Payee)}
			groups[key] = g
			groupOrder = append(groupOrder, key)
		}
		if _, seen := g.byCat[r.CategoryID]; !seen {
			g.order = append(g.order, r.CategoryID)
		}
		g.byCat[r.CategoryID] = append(g.byCat[r.CategoryID], r.TxnID)
		g.total++
	}

	for _, key := range groupOrder {
		g := groups[key]
		if g.total < MinRows {
			// Too few to generalize from. Two rows agreeing is a coincidence often
			// enough that a rule built on it is one the user corrects later — and a
			// wrong rule silently mis-files every future transaction it matches.
			plan.Unresolved += g.total
			continue
		}
		// The winning category is the most common; ties break on first-seen so the
		// same backlog always produces the same plan.
		best, bestN := "", 0
		for _, cat := range g.order {
			if n := len(g.byCat[cat]); n > bestN {
				best, bestN = cat, n
			}
		}
		agreement := float64(bestN) / float64(g.total)
		tier := TierNeedsYou
		if agreement >= AgreementForHigh {
			tier = TierConfident
		}
		plan.Proposals = append(plan.Proposals, Proposal{
			Match:      g.display,
			CategoryID: best,
			TxnIDs:     g.byCat[best],
			Agreement:  agreement,
			Tier:       tier,
			Disagreed:  g.total - bestN,
		})
		// The rows that pointed elsewhere are NOT resolved by this proposal. They
		// stay in the queue, and saying so is what stops the reduction figure from
		// being a lie.
		plan.Unresolved += g.total - bestN
	}

	sort.SliceStable(plan.Proposals, func(i, j int) bool {
		if plan.Proposals[i].Count() != plan.Proposals[j].Count() {
			return plan.Proposals[i].Count() > plan.Proposals[j].Count()
		}
		return plan.Proposals[i].Match < plan.Proposals[j].Match
	})
	return plan
}

// merchantKey folds a payee for grouping: lower-cased, trimmed, and with a
// trailing store number removed so "KROGER #418" and "Kroger #522" are one
// merchant.
//
// Only a TRAILING number token is stripped. Removing digits anywhere would merge
// "76 Gas" with "Gas", and merging two real merchants is a worse error than
// failing to merge two spellings of one: the first produces a rule that mis-files
// transactions forever, the second just leaves a row in the queue.
func merchantKey(payee string) string {
	s := strings.ToLower(strings.TrimSpace(payee))
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	for len(fields) > 1 {
		last := fields[len(fields)-1]
		if !isStoreNumber(last) {
			break
		}
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

// isStoreNumber reports whether a token is a store-number suffix: all digits, or
// a "#"-prefixed run of digits.
func isStoreNumber(tok string) bool {
	t := strings.TrimPrefix(tok, "#")
	if t == "" {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
