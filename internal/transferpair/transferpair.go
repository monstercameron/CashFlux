// SPDX-License-Identifier: MIT

// Package transferpair decides whether two rows are the two sides of one
// movement of money, and says how confident it is (C686, C692).
//
// # Why one more matcher
//
// The app had three, and none of them agreed:
//
//   - internal/contradict paired on the same CALENDAR DAY with no amount check at
//     all, and took the first row it found.
//   - internal/integrity paired on an EXACTLY OPPOSITE AMOUNT with no date check.
//   - internal/doublecount paired unflagged rows within three days.
//
// They run on different screens, so a transfer whose legs post a day apart is an
// error on one page and fine on another, and a transfer shaved by a wire fee is
// the reverse. Two screens contradicting each other about the same two rows is
// worse than either rule alone, and it is how a household ends up staring at
// dozens of "unmatched transfer" warnings it cannot act on.
//
// # What makes a pair VERIFIED
//
// The bar is deliberately high, because the cost of being wrong is asymmetric.
// Declining to pair leaves a row in a review queue, which is a small nuisance.
// Pairing wrongly links two unrelated movements, and from then on balances, cash
// flow and delete-both all treat them as one. So a pair is Verified only when
// there is exactly ONE candidate and a DESCRIPTOR agrees about which account the
// money went to.
//
// Amount and date alone cannot carry that. A household sweeping $750 into two
// different accounts on the same day produces two candidates that are identical
// under any amount-and-date rule. The descriptors tell them apart, because the
// account mask is the only thing in the row that names the far side.
package transferpair

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctref"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// Window is how far apart two legs may post and still be one movement.
//
// Four days. A transfer can leave on a Friday and land on a Monday, so anything
// shorter misses the most ordinary case there is. Beyond four, the coincidence
// rate climbs faster than the hit rate, and a queue that is usually wrong is one
// people stop reading.
const Window = 4 * 24 * time.Hour

// Confidence is how sure the matcher is, and it is the point of the package: the
// caller must act differently on each.
type Confidence int

const (
	// Unmatched means no candidate exists at all. A row flagged as a transfer with
	// no counterpart makes one account lie by exactly the moved amount.
	Unmatched Confidence = iota
	// Possible means a candidate exists but the evidence does not settle it —
	// several rows fit, or the only one that fits says nothing about where the
	// money went. These belong in a review queue, never in an automatic action.
	Possible
	// Verified means exactly one candidate and the descriptor agrees. Safe to act
	// on without asking.
	Verified
)

// String renders the confidence for logs and test failures.
func (c Confidence) String() string {
	switch c {
	case Verified:
		return "verified"
	case Possible:
		return "possible"
	default:
		return "unmatched"
	}
}

// Reason is one piece of evidence, so a review queue can say WHY rather than
// showing a verdict the reader has to take on trust.
type Reason string

const (
	// ReasonAmount: the amounts are exactly opposite, in the same currency.
	ReasonAmount Reason = "amount"
	// ReasonSameDay: both legs posted on the same calendar day.
	ReasonSameDay Reason = "same-day"
	// ReasonSettlementLag: the legs posted days apart, as a real transfer does.
	ReasonSettlementLag Reason = "settlement-lag"
	// ReasonDescriptorNames: a descriptor names the other account by its mask.
	ReasonDescriptorNames Reason = "descriptor-names-account"
	// ReasonAlreadyLinked: the rows already carry the same transfer group.
	ReasonAlreadyLinked Reason = "already-linked"
	// ReasonSeveralCandidates: more than one row fits, so none of them is settled.
	ReasonSeveralCandidates Reason = "several-candidates"
	// ReasonNoDescriptorEvidence: nothing in either row names the far account.
	ReasonNoDescriptorEvidence Reason = "no-descriptor-evidence"
	// ReasonDescriptorDisagrees: a descriptor names a DIFFERENT account than the
	// candidate sits on, which is evidence against the pair rather than for it.
	ReasonDescriptorDisagrees Reason = "descriptor-disagrees"
)

// Match is what the matcher concluded about one row.
type Match struct {
	// Leg is the row that was asked about.
	Leg domain.Transaction
	// Partner is the counterpart; zero when Confidence is Unmatched.
	Partner domain.Transaction
	// Others are the further candidates that prevented a verdict, so a review
	// queue can offer the real choice instead of hiding it.
	Others []domain.Transaction
	// Confidence is the verdict.
	Confidence Confidence
	// DaysApart is how far apart the two legs posted; 0 when Unmatched.
	DaysApart int
	// Reasons is the evidence behind the verdict.
	Reasons []Reason
}

// Has reports whether the match cites a given reason.
func (m Match) Has(r Reason) bool {
	for _, got := range m.Reasons {
		if got == r {
			return true
		}
	}
	return false
}

// For decides what the counterpart of one row is.
//
// It considers every row that could physically be the far side — a different row,
// on a different account, carrying the exactly opposite amount in the same
// currency, posted within Window — and then uses the descriptors to choose
// between them.
func For(leg domain.Transaction, txns []domain.Transaction, accounts []domain.Account) Match {
	m := Match{Leg: leg}

	// An existing group is the strongest evidence there is: somebody already said
	// so. Honour it before running a single heuristic.
	if leg.TransferGroupID != "" {
		for _, c := range txns {
			if c.ID != leg.ID && c.TransferGroupID == leg.TransferGroupID {
				m.Partner, m.Confidence = c, Verified
				m.DaysApart = daysBetween(leg.Date, c.Date)
				m.Reasons = []Reason{ReasonAlreadyLinked}
				return m
			}
		}
	}

	var candidates []domain.Transaction
	for _, c := range txns {
		if physicallyPairable(leg, c) {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		m.Confidence = Unmatched
		return m
	}

	legMask := maskOf(leg.AccountID, accounts)
	named, hasNamed := acctref.One(leg.Desc)

	// Descriptor evidence narrows before it decides. A row whose description names
	// an account is a row that told us where the money went, so candidates sitting
	// on any other account are ruled OUT by it rather than merely not helped.
	var agreeing []domain.Transaction
	if hasNamed {
		for _, c := range candidates {
			if m := maskOf(c.AccountID, accounts); m != "" && m == named.Digits {
				agreeing = append(agreeing, c)
			}
		}
	}
	// The far leg naming THIS account is equally good evidence, and it is what an
	// export that files both legs onto one account leaves behind.
	if len(agreeing) == 0 && legMask != "" {
		for _, c := range candidates {
			if acctref.Names(c.Desc, legMask) {
				agreeing = append(agreeing, c)
			}
		}
	}

	descriptorAgrees := len(agreeing) > 0
	pool := candidates
	if descriptorAgrees {
		pool = agreeing
	}

	sortByCloseness(leg, pool)
	m.Partner = pool[0]
	m.DaysApart = daysBetween(leg.Date, m.Partner.Date)
	if len(pool) > 1 {
		m.Others = pool[1:]
	}

	m.Reasons = append(m.Reasons, ReasonAmount)
	if m.DaysApart == 0 {
		m.Reasons = append(m.Reasons, ReasonSameDay)
	} else {
		m.Reasons = append(m.Reasons, ReasonSettlementLag)
	}

	switch {
	case len(pool) > 1:
		// Several rows fit equally well. Naming one would be a guess, and a guess
		// here links money between the wrong two accounts.
		m.Confidence = Possible
		m.Reasons = append(m.Reasons, ReasonSeveralCandidates)
	case descriptorAgrees:
		m.Confidence = Verified
		m.Reasons = append(m.Reasons, ReasonDescriptorNames)
	case hasNamed:
		// The row named an account and the only candidate is not on it. That is
		// evidence AGAINST this pairing, not an absence of evidence.
		m.Confidence = Possible
		m.Reasons = append(m.Reasons, ReasonDescriptorDisagrees)
	default:
		m.Confidence = Possible
		m.Reasons = append(m.Reasons, ReasonNoDescriptorEvidence)
	}
	return m
}

// physicallyPairable reports whether c could be the far side of leg at all,
// before any descriptor evidence is weighed.
func physicallyPairable(leg, c domain.Transaction) bool {
	if c.ID == leg.ID || c.AccountID == leg.AccountID {
		return false
	}
	if c.Amount.Currency != leg.Amount.Currency {
		// Two amounts sharing a number in different currencies are not the same
		// money, and converting to compare would invent a match out of a rate.
		return false
	}
	if leg.Amount.Amount == 0 || c.Amount.Amount != -leg.Amount.Amount {
		return false
	}
	// A row already linked to somebody else is spoken for.
	if c.IsTransfer() && c.TransferAccountID != leg.AccountID {
		return false
	}
	if c.TransferGroupID != "" && c.TransferGroupID != leg.TransferGroupID {
		return false
	}
	return absDuration(c.Date.Sub(leg.Date)) <= Window
}

// sortByCloseness puts the nearest-dated candidate first, breaking ties by id so
// the same ledger always produces the same answer.
func sortByCloseness(leg domain.Transaction, pool []domain.Transaction) {
	sort.SliceStable(pool, func(i, j int) bool {
		di := absDuration(pool[i].Date.Sub(leg.Date))
		dj := absDuration(pool[j].Date.Sub(leg.Date))
		if di != dj {
			return di < dj
		}
		return pool[i].ID < pool[j].ID
	})
}

func maskOf(accountID string, accounts []domain.Account) string {
	for _, a := range accounts {
		if a.ID == accountID {
			return a.MaskDigits()
		}
	}
	return ""
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func daysBetween(a, b time.Time) int {
	return int(absDuration(b.Sub(a)).Hours() / 24)
}

// Declared finds the counterpart a row EXPLICITLY points at: a row on the named
// account that points back at this one.
//
// It deliberately ignores the amount. A declared pair is somebody's statement
// that these two rows are one movement, and two declared legs carrying different
// amounts is a real and distinct fault — one leg was edited and the other was
// not — which is a different thing from a leg with no counterpart at all.
// Collapsing the two would tell a person "nothing arrived in the other account"
// when something did arrive, for the wrong amount.
//
// The nearest-dated candidate wins, ties broken by id, so the answer is stable.
func Declared(leg domain.Transaction, txns []domain.Transaction) (domain.Transaction, bool) {
	if !leg.IsTransfer() {
		return domain.Transaction{}, false
	}
	var pool []domain.Transaction
	for _, c := range txns {
		if c.ID == leg.ID || !c.IsTransfer() {
			continue
		}
		if c.AccountID != leg.TransferAccountID || c.TransferAccountID != leg.AccountID {
			continue
		}
		if leg.TransferGroupID != "" && c.TransferGroupID != "" && c.TransferGroupID != leg.TransferGroupID {
			continue
		}
		if absDuration(c.Date.Sub(leg.Date)) > Window {
			continue
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		return domain.Transaction{}, false
	}
	sortByCloseness(leg, pool)
	return pool[0], true
}

// NetStated sums a declared pair's legs in the convention each account STATES
// balances in, and reports whether they cancel.
//
// Summing the raw stored amounts is wrong, and wrong in a way that fires on
// healthy data. A payment into a card that stores its debt POSITIVE-owed is
// negative on both legs — money leaves checking, and the debt is reduced by
// subtraction — so a raw sum of -1000 looks like a thousand dollars evaporating
// when nothing is wrong at all. On a card that stores its debt NEGATIVE-owed the
// same two negative legs really are corruption. The stored numbers cannot tell
// those apart; the stated ones can, because stating is exactly the step that
// removes the per-account storage choice.
// It reports known=false when the answer cannot be trusted: a currency mismatch,
// or a liability whose storage convention had to be GUESSED because it carries
// no opening balance to infer from. A health warning built on a guess about which
// sign a debt is would fire on correct data, and a check that cries wolf is one
// people switch off.
func NetStated(a, b domain.Transaction, accounts []domain.Account) (netMinor int64, balanced, known bool) {
	if a.Amount.Currency != b.Amount.Currency {
		// Two currencies cannot be summed without a rate, and inventing one here
		// would manufacture agreement or disagreement out of an exchange rate.
		return 0, false, false
	}
	if !conventionKnown(a, accounts) || !conventionKnown(b, accounts) {
		return 0, false, false
	}
	net := statedAmount(a, accounts) + statedAmount(b, accounts)
	return net, net == 0, true
}

// conventionKnown reports whether this leg's account states balances in a
// convention that was READ rather than guessed — reconcile.ConventionOf owns
// that judgement, so it is asked rather than re-derived here.
func conventionKnown(t domain.Transaction, accounts []domain.Account) bool {
	for _, acc := range accounts {
		if acc.ID != t.AccountID {
			continue
		}
		_, known := reconcile.ConventionOf(acc, acc.OpeningBalance.Amount)
		return known
	}
	return false
}

// statedAmount reads one leg in its own account's stated convention.
func statedAmount(t domain.Transaction, accounts []domain.Account) int64 {
	for _, acc := range accounts {
		if acc.ID != t.AccountID {
			continue
		}
		conv, _ := reconcile.ConventionOf(acc, acc.OpeningBalance.Amount)
		return reconcile.Stated(t.Amount.Amount, conv)
	}
	// An account that is not in the list cannot be liability-signed, so the stored
	// figure is the best available reading of it.
	return t.Amount.Amount
}
