// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-A14", a14UtilCrossing)
}

// utilLookbackDays is how far back "before" is.
//
// Thirty days, which is roughly a statement cycle: the comparison a household
// can check against the paper that arrives, rather than against a moment the app
// picked for itself.
const utilLookbackDays = 30

// a14UtilCrossing — EC-8. Reports a credit card whose utilization has crossed
// from one band into another over the last statement cycle.
//
// A BAND IS A STATE AND A CROSSING IS AN EVENT, and only the event is news. The
// credit page already shows every card's current band; repeating it as an alert
// would be telling somebody something they can already see, every time they look.
//
// The bands are credithealth's own — deliberately not a fresh 30/50/90 set — so
// the app cannot answer "is my utilization bad?" two different ways depending on
// which screen asked.
func a14UtilCrossing(in Input) []smart.Insight {
	cut := in.Now.AddDate(0, 0, -utilLookbackDays)
	var past []domain.Transaction
	for _, t := range in.Transactions {
		if !t.Date.After(cut) {
			past = append(past, t)
		}
	}

	var out []smart.Insight
	for _, a := range in.Accounts {
		if a.Archived || a.Type != domain.TypeCreditCard || a.CreditLimit.Amount <= 0 {
			continue // no limit recorded means no utilization to cross
		}
		nowBal, err := ledger.Balance(a, in.Transactions)
		if err != nil {
			continue
		}
		thenBal, err := ledger.Balance(a, past)
		if err != nil {
			continue
		}
		limit := abs64(a.CreditLimit.Amount)
		nowPct := utilPct(nowBal.Amount, limit)
		thenPct := utilPct(thenBal.Amount, limit)
		crossing, moved := credithealth.Cross(thenPct, nowPct)
		if !moved {
			continue
		}

		sev := smart.SeverityNudge
		titleKey, titleFmt := "smart.a14.titleUp", "%s has moved up into %s use"
		if crossing.Improved() {
			// A watch that only ever reports bad news is one people learn to dread
			// and then stop reading, so a card coming back down is worth the same
			// line.
			sev = smart.SeverityInfo
			titleKey, titleFmt = "smart.a14.titleDown", "%s has come back down to %s use"
		} else if crossing.To == credithealth.BandUtilWorst {
			sev = smart.SeverityWarn
		}

		ins := smart.Insight{
			Feature:  "SMART-A14",
			Page:     smart.PageAccounts,
			Key:      "SMART-A14:" + a.ID + ":" + string(crossing.To),
			Severity: sev,
		}.
			WithTitle(titleKey, titleFmt, a.Name, bandWord(crossing.To)).
			WithDetail("smart.a14.detail",
				"%s was at %d%% of its limit a month ago and is at %d%% now.",
				a.Name, thenPct, nowPct).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/accounts",
				RelatedType: "account", RelatedID: a.ID}.
				WithLabel("smart.a14.action", "Open accounts"))

		// Only a worsening carries a figure, and the figure is what it would take
		// to get back — not the balance, which the card already shows. An
		// improvement needs no action and offering one would invent work.
		if crossing.Worsened() {
			if back := payToReach(nowBal.Amount, limit, bandCeiling(crossing.From)); back > 0 {
				ins = ins.
					WithDetail("smart.a14.detailPay",
						"%s was at %d%% of its limit a month ago and is at %d%% now. Paying %s would put it back.",
						a.Name, thenPct, nowPct, in.hmoney(back)).
					WithOneTimeAmount(money.New(back, in.Base))
			}
		}
		out = append(out, ins)
	}
	return out
}

// utilPct is a card's balance as a percentage of its limit, as a magnitude.
func utilPct(balanceMinor, limitMinor int64) int {
	if limitMinor <= 0 {
		return -1
	}
	return int(abs64(balanceMinor) * 100 / limitMinor)
}

// bandCeiling is the highest utilization still inside a band, so "paying this
// would put it back" lands just inside rather than just outside.
func bandCeiling(b credithealth.UtilBand) int {
	switch b {
	case credithealth.BandUtilBest:
		return 10
	case credithealth.BandUtilGood:
		return 30
	case credithealth.BandUtilFair:
		return 50
	case credithealth.BandUtilPoor:
		return 80
	}
	return 100
}

// payToReach is what would have to come off the balance to land at a target
// utilization. Zero when the card is already there.
func payToReach(balanceMinor, limitMinor int64, targetPct int) int64 {
	target := limitMinor * int64(targetPct) / 100
	owed := abs64(balanceMinor)
	if owed <= target {
		return 0
	}
	return owed - target
}

// bandWord is the plain-English name of a utilization band.
//
// The band values are internal labels ("Fair", "Worst"); a sentence saying "has
// moved up into Worst use" reads like a database dump, and "high" is what a
// person would say.
func bandWord(b credithealth.UtilBand) string {
	switch b {
	case credithealth.BandUtilBest:
		return "very low"
	case credithealth.BandUtilGood:
		return "low"
	case credithealth.BandUtilFair:
		return "moderate"
	case credithealth.BandUtilPoor:
		return "high"
	case credithealth.BandUtilWorst:
		return "very high"
	}
	return "unknown"
}
