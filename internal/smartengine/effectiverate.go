// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/realizedrate"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-A13", a13EffectiveRate)
}

// interestWords are the category names that count as interest received.
//
// Matching on the household's own CATEGORY rather than on payee text is the
// whole safeguard: a category is something somebody chose deliberately, while a
// payee line is whatever the bank's processor wrote, and "INT ADJ REV" appears
// on plenty of things that are not interest. If a household files interest
// somewhere this does not match, the detector stays quiet — which is the correct
// failure for a feature whose output is an accusation about a bank.
var interestWords = []string{"interest", "dividend", "yield"}

// a13EffectiveRate — EC-6. Compares what an interest-bearing asset account
// ACTUALLY paid, from its own postings, against the rate recorded on it.
//
// A rate you were promised and a rate you received are different facts, and
// nothing in the app checked the second. A "high-yield" account quietly paying
// 0.4% costs a household more than most of the things it does flag.
func a13EffectiveRate(in Input) []smart.Insight {
	catIsInterest := map[string]bool{}
	for _, c := range in.Categories {
		name := strings.ToLower(c.Name)
		for _, w := range interestWords {
			if strings.Contains(name, w) {
				catIsInterest[c.ID] = true
				break
			}
		}
	}
	if len(catIsInterest) == 0 {
		return nil // nothing is filed as interest, so there is nothing to measure
	}

	byAccount := map[string][]realizedrate.Posting{}
	for _, t := range in.Transactions {
		if !catIsInterest[t.CategoryID] || t.IsTransfer() || !t.CountsInReports() {
			continue
		}
		if t.Amount.Amount <= 0 {
			continue // interest RECEIVED; a charge is a different story
		}
		byAccount[t.AccountID] = append(byAccount[t.AccountID],
			realizedrate.Posting{Date: t.Date, Minor: in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)})
	}

	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Class == domain.ClassLiability || a.ExpectedReturnAPR <= 0 {
			continue // no stated rate means no promise to check
		}
		postings := byAccount[a.ID]
		if len(postings) < realizedrate.MinPostings {
			continue
		}
		bal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		balMinor := in.toBaseMinor(bal.Amount, bal.Currency)
		res := realizedrate.Measure(postings, balMinor, in.Now)
		gap, under := realizedrate.Shortfall(res, a.ExpectedReturnAPR)
		if !under {
			continue
		}
		out = append(out, smart.Insight{
			Feature:  "SMART-A13",
			Page:     smart.PageAccounts,
			Key:      "SMART-A13:" + a.ID,
			Severity: smart.SeverityWarn,
		}.
			WithTitle("smart.a13.title", "%s isn't paying what it says", a.Name).
			WithDetail("smart.a13.detail",
				"%s is recorded at %s, but the %s of interest it actually posted over the last %s works out at %s a year — about %s short.",
				a.Name, fmtPct(a.ExpectedReturnAPR), in.hmoney(res.TotalMinor),
				plural(int64(res.Days), "day"), fmtPct(res.AnnualPct), fmtPct(gap)).
			// The gap in money terms, on the balance sitting there now. Declared as
			// annual so the "what to do next" ranking can weigh it (WF-SM3).
			WithAnnualAmount(in.baseMoney(pctOf(balMinor, gap))).
			WithPrepared(smart.Prepared{
				// Stated, because an unstated assumption is what makes a correct
				// number feel wrong: the rate is measured against TODAY's balance, so
				// an account that grew a lot over the period reads lower than it paid.
				Assumptions:    []string{"Measured against the balance in the account today."},
				AssumptionKeys: []string{"smart.a13.assumeBalance"},
			}).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/accounts", RelatedType: "account", RelatedID: a.ID}.
				WithLabel("smart.a13.action", "Open accounts")))
	}
	return out
}
