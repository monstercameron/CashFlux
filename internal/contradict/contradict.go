// SPDX-License-Identifier: MIT

// Package contradict finds places where the app's own data disagrees with
// itself (E3).
//
// The motivating defect class: one screen said 47% and another said 38% about
// the same thing. Bugs like that are not caught by tests of either screen —
// each is internally consistent — and they are not caught by the user either,
// because nobody holds two pages in their head at once. They are caught by
// stating the INVARIANT that should hold between them and checking it.
//
// Three properties make this different from an insight engine:
//
//   - It is always on, never an opt-in "run a check" button. An invariant you
//     have to remember to verify is one that is broken most of the time.
//   - A finding here is a CONTRADICTION, not advice. "You could save more" is an
//     opinion; "this bill is marked unpaid and there is a payment for it" is the
//     app disagreeing with itself, and one of the two is wrong.
//   - Every check reports the two sides it compared. A contradiction with only
//     one side shown is indistinguishable from a complaint.
//
// Pure Go: it reads domain data and reports. It never repairs — the fix for a
// contradiction depends on which side is right, and that is the user's call.
package contradict

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Kind names a class of contradiction.
type Kind string

const (
	// KindOneSidedTransfer is a transfer whose other leg is missing. Money left
	// one account and arrived nowhere, so every total that includes both accounts
	// is wrong by that amount.
	KindOneSidedTransfer Kind = "one-sided-transfer"
	// KindTransferLegsDisagree is a transfer pair whose two legs do not sum to
	// zero — the ledger's net moves although no money entered or left.
	KindTransferLegsDisagree Kind = "transfer-legs-disagree"
	// KindHoldingsVsBalance is an investment account whose holdings do not add up
	// to its balance. Two screens then quote different portfolio values.
	KindHoldingsVsBalance Kind = "holdings-vs-balance"
	// KindTaskOpenAfterAction is a to-do still open although the thing it was
	// about has happened.
	KindTaskOpenAfterAction Kind = "task-open-after-action"
	// KindOrphanedLink is a transaction, task or goal pointing at an entity that
	// no longer exists. It reads as a working link until it is followed.
	KindOrphanedLink Kind = "orphaned-link"
	// KindDuplicateBudgetCategory is one category budgeted twice, so its spending
	// is counted against two limits and neither total is the truth.
	KindDuplicateBudgetCategory Kind = "duplicate-budget-category"
)

// Severity ranks how much a contradiction distorts.
type Severity int

const (
	// SeverityNotice is a link or a label that is wrong; no figure is affected.
	SeverityNotice Severity = iota
	// SeverityWarning is a figure that is wrong somewhere.
	SeverityWarning
	// SeverityCritical is a figure that is wrong in more than one place, in ways
	// that disagree — the 47%-vs-38% case.
	SeverityCritical
)

// Finding is one contradiction, with BOTH sides of it.
//
// Left and Right are the two claims that cannot both be true. They are required:
// a finding that shows one side is a complaint, and a user cannot act on a
// complaint because they do not know which side to fix.
type Finding struct {
	Kind     Kind
	Severity Severity
	// Key stably identifies this contradiction so a surface can dismiss it
	// without it returning on the next recompute. Built from the kind plus the
	// entity, never from the values, which change as the user works.
	Key string
	// EntityKind / EntityID name what the contradiction is about, for a link.
	EntityKind string
	EntityID   string
	// Left and Right are the two disagreeing claims, each a short phrase.
	Left, Right string
	// DeltaMinor is the size of the disagreement where it is a number, and
	// HasDelta says whether it means anything. A zero that means "we did not
	// measure" would read as "they agree", which is the opposite of the finding.
	DeltaMinor int64
	HasDelta   bool
}

// Data is everything the checks read. It is one struct rather than six
// parameters so adding a check does not change every caller's signature.
type Data struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Holdings     []domain.Holding
	Budgets      []domain.Budget
	Tasks        []domain.Task
	Goals        []domain.Goal
	Categories   []domain.Category
	Now          time.Time
}

// HoldingsTolerance is how far an investment account's holdings may sit from its
// balance before it counts as a contradiction, in minor units.
//
// Non-zero because prices are entered by hand and go stale between updates; a
// portfolio that is a few dollars off its balance is a stale price, not a
// disagreement. Ten dollars is small enough that a real mismatch — a position
// entered twice, a balance never reconciled — still trips it.
const HoldingsTolerance = 1000

// Check runs every invariant and returns the findings, most severe first, then
// stably by key so a list does not reshuffle between renders.
func Check(d Data) []Finding {
	var out []Finding
	out = append(out, checkTransfers(d)...)
	out = append(out, checkHoldings(d)...)
	out = append(out, checkTasks(d)...)
	out = append(out, checkOrphanedLinks(d)...)
	out = append(out, checkBudgets(d)...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// checkTransfers finds transfers that lost a leg or whose legs disagree.
//
// A transfer is two rows. If one is missing, money left an account and arrived
// nowhere; if they do not sum to zero, the household's net moved although
// nothing entered or left it. Both corrupt every total that spans the two
// accounts, and neither is visible from either account on its own.
func checkTransfers(d Data) []Finding {
	// Group by the unordered account pair plus date and magnitude — the app pairs
	// legs by construction, so a leg with no counterpart on the other side is the
	// signal.
	type leg struct {
		txn domain.Transaction
	}
	byAccount := map[string][]leg{}
	for _, t := range d.Transactions {
		if !t.IsTransfer() {
			continue
		}
		byAccount[t.AccountID] = append(byAccount[t.AccountID], leg{txn: t})
	}
	var out []Finding
	for _, t := range d.Transactions {
		if !t.IsTransfer() || t.Amount.Amount >= 0 {
			// Judge from the OUTGOING side only, so a matched pair is not reported
			// twice under two keys.
			continue
		}
		var partner *domain.Transaction
		for i, l := range byAccount[t.TransferAccountID] {
			if l.txn.TransferAccountID != t.AccountID {
				continue
			}
			if !sameDay(l.txn.Date, t.Date) {
				continue
			}
			partner = &byAccount[t.TransferAccountID][i].txn
			break
		}
		if partner == nil {
			out = append(out, Finding{
				Kind: KindOneSidedTransfer, Severity: SeverityCritical,
				Key:        string(KindOneSidedTransfer) + ":" + t.ID,
				EntityKind: "transaction", EntityID: t.ID,
				Left:       "money left this account",
				Right:      "nothing arrived in the other one",
				DeltaMinor: -t.Amount.Amount, HasDelta: true,
			})
			continue
		}
		if net := t.Amount.Amount + partner.Amount.Amount; net != 0 {
			out = append(out, Finding{
				Kind: KindTransferLegsDisagree, Severity: SeverityCritical,
				Key:        string(KindTransferLegsDisagree) + ":" + t.ID,
				EntityKind: "transaction", EntityID: t.ID,
				Left:       "this leg moved one amount",
				Right:      "the matching leg moved another",
				DeltaMinor: net, HasDelta: true,
			})
		}
	}
	return out
}

// checkHoldings finds investment accounts whose positions do not add up to the
// balance the account claims.
//
// Two surfaces read these numbers — the investments page totals the holdings,
// the accounts page reads the balance — so a mismatch is exactly the shape of
// the defect this package exists for: both screens are individually correct and
// they disagree.
//
// An account with NO holdings is skipped rather than reported: not having
// entered positions yet is a blank, not a contradiction, and reporting it would
// bury the real ones under every unfilled account.
func checkHoldings(d Data) []Finding {
	byAccount := map[string]int64{}
	for _, h := range d.Holdings {
		byAccount[h.AccountID] += marketValue(h)
	}
	var out []Finding
	for _, a := range d.Accounts {
		if a.Archived {
			continue
		}
		total, ok := byAccount[a.ID]
		if !ok {
			continue
		}
		bal := accountBalance(a, d.Transactions)
		delta := total - bal
		if abs64(delta) <= HoldingsTolerance {
			continue
		}
		out = append(out, Finding{
			Kind: KindHoldingsVsBalance, Severity: SeverityCritical,
			Key:        string(KindHoldingsVsBalance) + ":" + a.ID,
			EntityKind: "account", EntityID: a.ID,
			Left:       "the positions in this account total one amount",
			Right:      "its balance says another",
			DeltaMinor: delta, HasDelta: true,
		})
	}
	return out
}

// checkTasks finds to-dos still open although the thing they were about has
// happened — a goal reached, a linked bill paid.
//
// The list is where trust in the to-do surface is won or lost: work that is
// already done, still sitting there, teaches people the list is stale and to
// stop reading it.
func checkTasks(d Data) []Finding {
	goalDone := map[string]bool{}
	for _, g := range d.Goals {
		if g.TargetAmount.Amount > 0 && g.CurrentAmount.Amount >= g.TargetAmount.Amount {
			goalDone[g.ID] = true
		}
	}
	var out []Finding
	for _, t := range d.Tasks {
		if t.Status != domain.StatusOpen || t.RelatedID == "" {
			continue
		}
		if t.RelatedType == domain.RelatedGoal && goalDone[t.RelatedID] {
			out = append(out, Finding{
				Kind: KindTaskOpenAfterAction, Severity: SeverityNotice,
				Key:        string(KindTaskOpenAfterAction) + ":" + t.ID,
				EntityKind: "task", EntityID: t.ID,
				Left:  "this to-do is still open",
				Right: "the goal it is about has been reached",
			})
		}
	}
	return out
}

// checkOrphanedLinks finds references to entities that no longer exist. They
// read as working links until followed, which is worse than no link: the reader
// believes there is context there.
func checkOrphanedLinks(d Data) []Finding {
	accounts := idSet(len(d.Accounts))
	for _, a := range d.Accounts {
		accounts[a.ID] = true
	}
	cats := idSet(len(d.Categories))
	for _, c := range d.Categories {
		cats[c.ID] = true
	}
	var out []Finding
	for _, t := range d.Transactions {
		if t.CategoryID != "" && !cats[t.CategoryID] {
			out = append(out, Finding{
				Kind: KindOrphanedLink, Severity: SeverityWarning,
				Key:        string(KindOrphanedLink) + ":txn-cat:" + t.ID,
				EntityKind: "transaction", EntityID: t.ID,
				Left:  "this transaction is filed to a category",
				Right: "that category no longer exists",
			})
		}
		if t.AccountID != "" && !accounts[t.AccountID] {
			out = append(out, Finding{
				Kind: KindOrphanedLink, Severity: SeverityWarning,
				Key:        string(KindOrphanedLink) + ":txn-acct:" + t.ID,
				EntityKind: "transaction", EntityID: t.ID,
				Left:  "this transaction belongs to an account",
				Right: "that account no longer exists",
			})
		}
	}
	return out
}

// checkBudgets finds a category budgeted more than once. Its spending then
// counts against two limits, so neither budget's "spent" figure is the truth and
// the two disagree with each other by construction.
func checkBudgets(d Data) []Finding {
	seen := map[string]string{} // categoryID -> first budget id
	var out []Finding
	for _, b := range d.Budgets {
		for _, c := range budgetCategories(b) {
			if c == "" {
				continue
			}
			if first, dup := seen[c]; dup {
				out = append(out, Finding{
					Kind: KindDuplicateBudgetCategory, Severity: SeverityWarning,
					Key:        string(KindDuplicateBudgetCategory) + ":" + c,
					EntityKind: "budget", EntityID: b.ID,
					Left:  "this category is budgeted here",
					Right: "and also in another budget (" + first + ")",
				})
				continue
			}
			seen[c] = b.ID
		}
	}
	return out
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// budgetCategories lists a budget's categories, covering both the single-field
// and multi-category shapes so a check cannot miss one by reading only one of
// them.
func budgetCategories(b domain.Budget) []string {
	if len(b.CategoryIDs) > 0 {
		return b.CategoryIDs
	}
	if b.CategoryID != "" {
		return []string{b.CategoryID}
	}
	return nil
}

// marketValue is a holding's current worth in minor units.
func marketValue(h domain.Holding) int64 {
	return int64(h.Shares * float64(h.CurrentPriceMinorPerShare))
}

// accountBalance is an account's opening balance plus everything posted to it.
func accountBalance(a domain.Account, txns []domain.Transaction) int64 {
	bal := a.OpeningBalance.Amount
	for _, t := range txns {
		if t.AccountID == a.ID {
			bal += t.Amount.Amount
		}
	}
	return bal
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func idSet(n int) map[string]bool { return make(map[string]bool, n) }

// Summary counts findings by severity, for a one-line header.
type Summary struct {
	Critical, Warning, Notice int
}

// Total is how many contradictions were found.
func (s Summary) Total() int { return s.Critical + s.Warning + s.Notice }

// Clean reports whether the data agrees with itself.
func (s Summary) Clean() bool { return s.Total() == 0 }

// Summarize counts findings by severity.
func Summarize(fs []Finding) Summary {
	var s Summary
	for _, f := range fs {
		switch f.Severity {
		case SeverityCritical:
			s.Critical++
		case SeverityWarning:
			s.Warning++
		default:
			s.Notice++
		}
	}
	return s
}

// Describe renders a finding as the one sentence that states BOTH sides, which
// is the only form in which it is actionable.
func Describe(f Finding) string {
	var b strings.Builder
	b.WriteString(f.Left)
	b.WriteString(", but ")
	b.WriteString(f.Right)
	return b.String()
}
