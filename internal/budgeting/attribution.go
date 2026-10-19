// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberallocation"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MemberShare is one member's slice of a shared budget's spend over a period: the
// member who spent it and the amount, in the budget's limit currency. UnassignedMemberID
// carries spend that no member owns (a transaction/split with no MemberID) so the shares
// still sum to the budget's total.
type MemberShare struct {
	MemberID string
	Spent    money.Money
}

// UnassignedMemberID labels spend with no attributable member in an attribution result.
// It is not a real member id, so the UI renders it as "Unassigned" rather than looking it
// up in the member table.
const UnassignedMemberID = ""

// AttributeByMember breaks a budget's spend over [start, end) down by the member who spent
// it — the read-model behind the BG13 "you $180 · Priya $140" split bar in a shared budget's
// row expand. It mirrors spentCovered exactly (netting, split-line attribution, the same
// covers predicate) so the shares always sum to the budget's evaluated Spent, then buckets
// each contribution by its effective owner: a split line's LineOwner, or the transaction's
// MemberID for an unsplit charge. covers is the tracked category set (typically the budget's
// categories plus their descendants, from categorytree.DescendantsOfAll); an empty covers
// falls back to the budget's own tracked categories.
//
// Attribution is not scope-gated (it never applies ownsScope): a shared budget attributes
// every member's contribution, which is the whole point — attribution, not blame. Results
// are sorted by amount, largest first, then by member id for a stable order. Members who
// spent nothing are omitted.
func AttributeByMember(budget domain.Budget, all NettedLedger, start, end time.Time, rates currency.Rates, covers map[string]bool) ([]MemberShare, error) {
	limit := normalizedLimit(budget, rates)

	tracks := func(id string) bool { return budget.TracksCategory(id) || covers[id] }

	byMember := make(map[string]int64)
	accumulate := func(member string, amount int64) error {
		updated, err := money.Add(byMember[member], amount)
		if err != nil {
			return fmt.Errorf("sum attribution for member %q: %w", member, err)
		}
		byMember[member] = updated
		return nil
	}
	addTo := func(member string, amt money.Money) error {
		contribution, err := expenseContribution(amt)
		if err != nil {
			return err
		}
		conv, err := rates.Convert(contribution, limit.Currency)
		if err != nil {
			return err
		}
		return accumulate(member, conv.Amount)
	}
	addToOwners := func(owners []string, amt money.Money) error {
		contribution, err := expenseContribution(amt)
		if err != nil {
			return err
		}
		conv, err := rates.Convert(contribution, limit.Currency)
		if err != nil {
			return err
		}
		if len(owners) == 0 {
			return accumulate(UnassignedMemberID, conv.Amount)
		}
		for _, share := range memberallocation.SplitAmount(conv.Amount, owners) {
			if err := accumulate(share.MemberID, share.Amount); err != nil {
				return err
			}
		}
		return nil
	}

	for _, t := range all {
		if !t.CountsInReports() || !matchesScope(budget, t, start, end) {
			continue
		}
		// Mirror spentCovered's cross-category tag tracking: a tag-matched charge attributes
		// in full to its payer, once, ahead of category/split matching (no double count).
		if budget.TracksAnyTag(t.Tags) {
			if err := addToOwners(t.Owners(), t.Amount); err != nil {
				return nil, err
			}
			continue
		}
		if t.HasSplits() {
			byOwner := make(map[string][]money.Money)
			var fallback []money.Money
			for _, s := range t.Splits {
				if !tracks(s.CategoryID) {
					continue
				}
				if s.MemberID != "" {
					byOwner[s.MemberID] = append(byOwner[s.MemberID], s.Amount)
					continue
				}
				fallback = append(fallback, s.Amount)
			}
			for member, lines := range byOwner {
				net, err := signedLineTotal(lines)
				if err != nil {
					return nil, err
				}
				if err := addTo(member, net); err != nil {
					return nil, err
				}
			}
			if len(fallback) > 0 {
				net, err := signedLineTotal(fallback)
				if err != nil {
					return nil, err
				}
				if err := addToOwners(t.Owners(), net); err != nil {
					return nil, err
				}
			}
			continue
		}
		if !tracks(t.CategoryID) {
			continue
		}
		if err := addToOwners(t.Owners(), t.Amount); err != nil {
			return nil, err
		}
	}

	out := make([]MemberShare, 0, len(byMember))
	for member, minor := range byMember {
		if minor == 0 {
			continue
		}
		out = append(out, MemberShare{MemberID: member, Spent: money.New(minor, limit.Currency)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Spent.Amount != out[j].Spent.Amount {
			return out[i].Spent.Amount > out[j].Spent.Amount
		}
		return out[i].MemberID < out[j].MemberID
	})
	return out, nil
}
