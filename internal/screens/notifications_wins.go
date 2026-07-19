// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// winsSeenKey stores the milestone keys the user has already been shown, so the
// fresh-win entrance plays once per newly-reached win rather than on every visit.
const winsSeenKey = "cashflux:wins-seen"

// noSpendScanDays bounds how far back the no-spend streak is counted.
const noSpendScanDays = 90

// noSpendStreak counts consecutive COMPLETED days (ending yesterday) with no
// expense. Today is excluded so a not-yet-spent morning doesn't inflate the streak.
func noSpendStreak(txns []domain.Transaction, now time.Time) int {
	spendDays := map[string]bool{}
	for _, t := range txns {
		if t.IsExpense() {
			spendDays[t.Date.Format("2006-01-02")] = true
		}
	}
	streak := 0
	for i := 1; i <= noSpendScanDays; i++ {
		if spendDays[now.AddDate(0, 0, -i).Format("2006-01-02")] {
			break
		}
		streak++
	}
	return streak
}

// keptBudgetsLastMonth counts budgets that finished the last completed month at or
// under their limit with real spend against them (a budget you never touched isn't
// a win), and returns that month's period key. Errors on a single budget skip it.
func keptBudgetsLastMonth(app *appstate.App, rates currency.Rates, now time.Time) (int, string) {
	lastMonth := now.AddDate(0, 0, -now.Day()) // any day in the previous month
	start, end := budgeting.PeriodRange(domain.PeriodMonthly, lastMonth, uistate.LoadPrefs().WeekStartWeekday())
	kept := 0
	for _, b := range app.Budgets() {
		if b.Period != domain.PeriodMonthly {
			continue // only monthly budgets have a clean "last month" to grade
		}
		spent, err := budgeting.Spent(b, app.Transactions(), start, end, rates)
		if err != nil {
			continue
		}
		limit := b.Limit
		if limit.Currency == "" {
			limit = money.New(limit.Amount, rates.Base)
		}
		if spent.Amount > 0 && spent.Amount <= limit.Amount {
			kept++
		}
	}
	return kept, lastMonth.Format("2006-01")
}

// winsInput gathers the milestone primitives from the live dataset.
func winsInput(app *appstate.App) milestones.Input {
	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	var reached []string
	tasks := app.Tasks()
	for _, g := range app.Goals() {
		if goals.Reached(g, tasks, now) {
			reached = append(reached, g.Name)
		}
	}

	var nw int64
	if res, err := ledger.NetWorthExplained(app.Accounts(), app.Transactions(), rates); err == nil {
		nw = res.Net.Amount
	}

	kept, periodKey := keptBudgetsLastMonth(app, rates, now)

	return milestones.Input{
		ReachedGoals:  reached,
		NetWorthMinor: nw,
		NoSpendDays:   noSpendStreak(app.Transactions(), now),
		KeptBudgets:   kept,
		KeptPeriodKey: periodKey,
		Now:           now,
	}
}

// milestoneCopy renders the localized title + message for a milestone. Net-worth
// amounts are formatted in the base currency.
func milestoneCopy(m milestones.Milestone, base string) (title, msg string) {
	switch m.Kind {
	case milestones.KindGoalReached:
		return uistate.T("wins.goalTitle", m.Name), uistate.T("wins.goalMsg", m.Name)
	case milestones.KindNetWorth:
		amt := fmtMoney(money.New(m.Value, base))
		return uistate.T("wins.netWorthTitle", amt), uistate.T("wins.netWorthMsg", amt)
	case milestones.KindNoSpendStreak:
		return uistate.T("wins.noSpendTitle", m.Value), uistate.T("wins.noSpendMsg", m.Value)
	case milestones.KindKeptBudgets:
		return uistate.T("wins.keptTitle", m.Value), uistate.T("wins.keptMsg", m.Value)
	}
	return "", ""
}

// milestoneIcon picks the glyph for a milestone kind.
func milestoneIcon(k milestones.Kind) icon.Name {
	switch k {
	case milestones.KindNetWorth:
		return icon.TrendingUp
	case milestones.KindNoSpendStreak:
		return icon.Sparkles
	default:
		return icon.CheckCircle
	}
}

// loadWinsSeen returns the set of milestone keys already celebrated.
func loadWinsSeen() map[string]bool {
	seen := map[string]bool{}
	for _, k := range strings.Split(uistate.KVGet(winsSeenKey), "\n") {
		if k = strings.TrimSpace(k); k != "" {
			seen[k] = true
		}
	}
	return seen
}

// notifWinsWidget is the celebratory counterweight to the warning feed: a warm
// "Recent wins" card at the top of the Notification Center listing the good news —
// goals funded, net-worth rungs crossed, budgets kept, no-spend streaks. When a win
// is newly reached (not seen before) the card plays its one-shot wins-rise
// entrance — restrained by design (v1.2.3 motion spec: no confetti or playful
// physics for financial milestones) and gated off under prefers-reduced-motion.
// Renders nothing when there's nothing to celebrate.
func notifWinsWidget(props notifProps) ui.Node {
	app := props.App
	if app == nil {
		app = appstate.Default
	}
	if app == nil {
		return Fragment()
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	found := milestones.Detect(winsInput(app))
	if len(found) == 0 {
		return Fragment()
	}

	seen := loadWinsSeen()
	hasFresh := false
	for _, m := range found {
		if !seen[m.Key] {
			hasFresh = true
			break
		}
	}
	// Latch the celebration for this mount: recomputing from storage after the
	// mark-seen effect would otherwise cut the fresh-win entrance mid-view.
	celebrate := ui.UseState(hasFresh)

	// Remember every currently-present win so the entrance doesn't replay next visit.
	keys := make([]string, 0, len(found))
	for _, m := range found {
		keys = append(keys, m.Key)
	}
	joined := strings.Join(keys, "\n")
	ui.UseEffect(func() func() {
		merged := loadWinsSeen()
		for _, k := range keys {
			merged[k] = true
		}
		out := make([]string, 0, len(merged))
		for k := range merged {
			out = append(out, k)
		}
		uistate.KVSet(winsSeenKey, strings.Join(out, "\n"))
		return nil
	}, joined)

	rows := make([]ui.Node, 0, len(found))
	for i, m := range found {
		title, msg := milestoneCopy(m, base)
		fresh := !seen[m.Key]
		rows = append(rows, winsRow(i, m, title, msg, fresh))
	}

	// v1.2.3 motion spec: no confetti for financial actions. A fresh win gets the
	// single restrained wins-rise entrance on the card — that is the celebration.
	cardCls := "wins-card"
	if celebrate.Get() {
		cardCls += " wins-card-fresh"
	}

	return Div(ClassStr(cardCls), Attr("data-testid", "wins-card"),
		Div(css.Class("wins-head"),
			uiw.Icon(icon.Sparkles, css.Class("wins-head-icon")),
			Div(css.Class("wins-head-text"),
				Div(css.Class("wins-title"), uistate.T("wins.title")),
				Div(css.Class("wins-sub"), uistate.T("wins.subtitle")),
			),
		),
		Div(css.Class("wins-list"), rows),
	)
}

// winsRow renders one milestone line. A freshly-reached win carries a small "New"
// badge so the eye finds it among any it's already seen. idx keeps the testid unique
// even when two wins share a kind (e.g. two goals reached in the same window).
func winsRow(idx int, m milestones.Milestone, title, msg string, fresh bool) ui.Node {
	var badge ui.Node = Fragment()
	if fresh {
		badge = Span(css.Class("wins-new"), uistate.T("wins.newBadge"))
	}
	return Div(css.Class("wins-row"), Attr("data-testid", "wins-row-"+string(m.Kind)+"-"+strconv.Itoa(idx)),
		Span(css.Class("wins-row-icon"), uiw.Icon(milestoneIcon(m.Kind), css.Class("wins-icon-svg"))),
		Div(css.Class("wins-row-text"),
			Div(css.Class("wins-row-title"), title, badge),
			Div(css.Class("wins-row-msg"), msg),
		),
	)
}
