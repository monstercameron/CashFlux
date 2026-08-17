package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// QuickFill keys — stable identifiers the UI maps to localized chip labels (BG4).
//
// C667: the keys name PERIODS, not months, and the prior-limit suggestion is no
// longer called "last period" alongside three spend figures. The chips used to
// average whole calendar months regardless of the budget's cadence, so a weekly
// or quarterly budget was offered monthly history under a period label, and
// "Last period" showed the previous LIMIT sitting in a row of actual spending
// with nothing to distinguish a plan from a fact.
const (
	QuickFillLastPeriod  = "last-period-spend" // the previous whole period's spend
	QuickFillAvg3        = "avg-3"             // mean spend over the last 3 whole periods
	QuickFillAvg6        = "avg-6"             // mean spend over the last 6 whole periods
	QuickFillPriorLimit  = "prior-limit"       // the previous period's effective budget (a plan, not spend)
	QuickFillUnderfunded = "underfunded"       // amount still needed to fund the target (BG1)
)

// QuickFillKind says what a suggestion's figure IS, so the UI can label a prior
// LIMIT differently from actual SPEND. Offering both in one undifferentiated row
// is how a budget could be rewritten to last period's plan by a user who read the
// chip as last period's spending (C667).
type QuickFillKind string

// The kinds of figure a quick-fill chip can carry.
const (
	QuickFillSpend  QuickFillKind = "spend"  // money actually spent over whole past periods
	QuickFillLimit  QuickFillKind = "limit"  // a budget that was set, not money that moved
	QuickFillTarget QuickFillKind = "target" // what the budget's funding target still needs
)

// QuickFill is one computed one-tap fill suggestion for a budget's amount (BG4).
// The UI renders it as a chip showing a localized label plus fmtMoney(Amount);
// Key identifies which suggestion it is so the label copy lives in i18n, not here.
type QuickFill struct {
	// Key is the stable suggestion identifier (one of the QuickFill* constants).
	Key string
	// Kind says whether Amount is spending, a prior limit, or a target shortfall.
	Kind QuickFillKind
	// Amount is the computed fill value in the budget's limit currency.
	Amount money.Money
	// Periods is how many whole periods Amount averages (1 for a single period,
	// 0 when the figure is not an average of spending).
	Periods int
}

// QuickFillInput carries everything QuickFills needs beyond the transaction
// history: the reference "now", the household week-start and pay-cycle anchor
// (for period math), the FX rates, the rollup category set, and the target
// underfunding from BG1 (Needed().Needed). Pass HasUnderfunded=false to omit
// the underfunded chip (e.g. the budget has no target).
type QuickFillInput struct {
	Now       time.Time
	WeekStart time.Weekday
	// PayAnchor snaps biweekly periods to the household's real payday, exactly as
	// the budget cards do. Zero falls back to the internal fortnight grid.
	PayAnchor time.Time
	Rates     currency.Rates
	// Covers is the budget's rollup category set — its tracked categories AND
	// their descendants, per categorytree.DescendantsOfAll. This is the set the
	// card's spending bar counts against (EvaluateRollup), so the suggestions
	// have to count it too; without it a parent-category budget whose spend all
	// sits in sub-categories was offered ~$0 of history beside a card reading
	// four figures (C667).
	Covers         map[string]bool
	Underfunded    money.Money
	HasUnderfunded bool
}

// QuickFillWindow describes the history the spend suggestions were computed from,
// so the UI can say it rather than leave the user to guess: the budget's cadence,
// how many whole periods were read, and the half-open [From, To) range they span.
// A zero Periods means the spend figures could not be computed at all.
type QuickFillWindow struct {
	Period  domain.Period
	Periods int
	From    time.Time
	To      time.Time
}

// quickFillPeriods is how many whole past periods the spend suggestions read.
const quickFillPeriods = 6

// QuickFills computes the one-tap fill figures for a budget (BG4): the previous
// whole period's spend, the trailing 3- and 6-period averages, the previous
// period's effective budget, and — when in.HasUnderfunded — the amount still
// needed to fund the budget's target (from BG1's Needed).
//
// C667: every figure is computed over the budget's OWN cadence and the SAME
// category population its card reports against (in.Covers plus the budget's own
// tracked categories and tags, via SpentRollup), so a suggestion can never come
// from a different data set than the spent total the user is looking at.
// Suggestions whose value cannot be computed (an FX gap in a period) are omitted,
// and the returned window says what was actually read.
func QuickFills(budget domain.Budget, all []domain.Transaction, in QuickFillInput) ([]QuickFill, QuickFillWindow) {
	cur := normalizedLimit(budget, in.Rates).Currency
	var out []QuickFill
	win := QuickFillWindow{Period: budget.Period}

	// The period immediately before the one containing Now, and the run of whole
	// periods behind it — the same anchored grid the cards evaluate on.
	curStart, _ := PeriodRangeAnchored(budget.Period, in.Now, in.WeekStart, in.PayAnchor)
	prevStart, prevEnd := PeriodRangeAnchored(budget.Period, curStart.AddDate(0, 0, -1), in.WeekStart, in.PayAnchor)

	if spends, starts, ok := periodSpends(budget, all, in, quickFillPeriods); ok {
		out = append(out,
			QuickFill{Key: QuickFillLastPeriod, Kind: QuickFillSpend, Amount: spends[0], Periods: 1},
			QuickFill{Key: QuickFillAvg3, Kind: QuickFillSpend, Amount: averagePeriods(spends[:3], cur), Periods: 3},
			QuickFill{Key: QuickFillAvg6, Kind: QuickFillSpend, Amount: averagePeriods(spends[:6], cur), Periods: 6},
		)
		win.Periods = quickFillPeriods
		win.From, win.To = starts[len(starts)-1], prevEnd
	}

	// The previous period's effective budget: the base limit plus any one-time
	// boost that applied to it. A plan, not spending — Kind says so, and the UI
	// labels it apart from the three figures above.
	lastLimit := normalizedLimit(budget, in.Rates).Amount + budget.PeriodBoost(prevStart)
	out = append(out, QuickFill{Key: QuickFillPriorLimit, Kind: QuickFillLimit, Amount: money.New(lastLimit, cur)})

	if in.HasUnderfunded {
		out = append(out, QuickFill{Key: QuickFillUnderfunded, Kind: QuickFillTarget, Amount: money.New(in.Underfunded.Amount, cur)})
	}
	return out, win
}

// periodSpends returns the budget's spend over each of the last n WHOLE periods
// of the budget's own cadence (index 0 = the period immediately before the one
// containing in.Now), in the budget's limit currency, alongside each period's
// start date. ok is false if any period fails to evaluate.
func periodSpends(budget domain.Budget, all []domain.Transaction, in QuickFillInput, n int) ([]money.Money, []time.Time, bool) {
	spends := make([]money.Money, n)
	starts := make([]time.Time, n)
	curStart, _ := PeriodRangeAnchored(budget.Period, in.Now, in.WeekStart, in.PayAnchor)
	ref := curStart.AddDate(0, 0, -1)
	for i := range n {
		start, end := PeriodRangeAnchored(budget.Period, ref, in.WeekStart, in.PayAnchor)
		spent, err := SpentRollup(budget, all, start, end, in.Rates, in.Covers)
		if err != nil {
			return nil, nil, false
		}
		spends[i], starts[i] = spent, start
		ref = start.AddDate(0, 0, -1)
	}
	return spends, starts, true
}

// averagePeriods returns the mean of the given per-period spends (integer
// division, floored), in currency cur. An empty slice yields zero.
func averagePeriods(periods []money.Money, cur string) money.Money {
	if len(periods) == 0 {
		return money.Zero(cur)
	}
	var sum int64
	for _, m := range periods {
		sum += m.Amount
	}
	return money.New(sum/int64(len(periods)), cur)
}
