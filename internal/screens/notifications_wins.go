// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notificationview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// winsSeenKey stores the milestone keys the user has already been shown, so the
// fresh-win entrance plays once per newly-reached win rather than on every visit.
const winsSeenKey = "cashflux:wins-seen"

// noSpendScanDays bounds how far back the no-spend streak is counted.
const noSpendScanDays = 90

type notificationWinsSnapshot struct {
	Base  string
	Found []milestones.Milestone
}

var notificationWinsCache = newViewCache[notificationWinsSnapshot](1)

// winsSnapshot owns browser/store wiring; the full calculation is native Go.
func winsSnapshot(app *appstate.App, now time.Time, weekStart time.Weekday) notificationWinsSnapshot {
	key := revKey(app) + "|" + now.Format("2006-01-02 -0700") + "|" + strconv.Itoa(int(weekStart))
	return memoByRev(notificationWinsCache, key, func() notificationWinsSnapshot {
		if app == nil {
			return notificationWinsSnapshot{}
		}
		settings := app.Settings()
		base := settings.BaseCurrency
		if base == "" {
			base = "USD"
		}
		rates := currency.Rates{Base: base, Rates: settings.FXRates}
		accounts, txns := app.Accounts(), app.Transactions()
		netWorth := memoByRev(netWorthCache, revKey(app)+"|"+acctSig(accounts), func() ledger.NetWorthResult {
			value, _ := ledger.NetWorthExplained(accounts, txns, rates)
			return value
		})
		input := notificationview.PrepareWins(notificationview.WinsInput{
			Accounts: accounts, Transactions: txns, Tasks: app.Tasks(), Goals: app.Goals(), Budgets: app.Budgets(),
			BudgetLedger: budgetLedgerForApp(app), Rates: rates, Now: now, WeekStart: weekStart,
			NoSpendLimit: noSpendScanDays, NetWorth: &netWorth,
		})
		return notificationWinsSnapshot{Base: base, Found: milestones.Detect(input)}
	})
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
	// App is a stable pointer. Subscribe here so goal/ledger writes refresh the
	// tile even when the parent's next render has equal component props.
	_ = uistate.UseDataRevision().Get()
	weekStart := uistate.UsePrefs().Get().WeekStartWeekday()
	app := props.App
	if app == nil {
		app = appstate.Default
	}
	model := winsSnapshot(app, time.Now(), weekStart)
	if len(model.Found) == 0 {
		return Fragment()
	}

	return ui.CreateElement(notifWinsContent, notifWinsContentProps{Found: model.Found, Base: model.Base})
}

type notifWinsContentProps struct {
	Found []milestones.Milestone
	Base  string
}

func notifWinsContent(props notifWinsContentProps) ui.Node {
	found, base := props.Found, props.Base
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
