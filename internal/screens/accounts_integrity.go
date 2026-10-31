// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// accounts_integrity.go is the UI for the earmark-integrity check (XC7): a quiet
// warn-tone line on an over-earmarked account's row. Checked currency and
// availability facts come from goals.BuildAccountReservations; this only renders.

type accountEarmarkWarnProps struct {
	Account domain.Account
	Balance money.Money
}

// accountEarmarkWarning renders a warn line when goals have earmarked more against
// this account than it actually holds — meaning goal money has silently been spent
// ("Holds $1,400 but $2,000 is earmarked — $600 of goal money has been spent") —
// with a "Transfer to savings" action (opens the account transfer editor prefilled)
// and a "Review goals" nav. It is Fragment() when the account is healthy. Its own
// component so its hooks keep a stable render position (called from AccountRow).
func accountEarmarkWarning(props accountEarmarkWarnProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	position := goalReservations(app).ByAccount[props.Account.ID]
	if len(position.GoalRefs) == 0 || position.Known && !position.Overbooked {
		return Fragment()
	}
	// The interactive child mounts only for a warning; all of its hooks are
	// unconditional across known-breach and unavailable transitions.
	return ui.CreateElement(accountEarmarkWarningBody, position)
}

func accountEarmarkWarningBody(position goalsvc.AccountReservation) ui.Node {
	a := position.Account
	acctEditAtom := uistate.UseAccountEdit()
	nav := router.UseNavigate()
	onTransfer := ui.UseEvent(Prevent(func() { acctEditAtom.Set(uistate.AccountEdit{ID: a.ID, Mode: uistate.AcctEditModeTransfer}) }))
	onReviewGoals := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/goals")) }))
	return Div(css.Class("row-meta"), Attr("data-testid", "acct-earmark-warn-"+a.ID), Role("status"),
		ui.CreateElement(reservationNotice, reservationNoticeProps{Position: position}),
		Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "acct-earmark-transfer-"+a.ID), OnClick(onTransfer), uistate.T("integrity.transfer")),
		Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "acct-earmark-review-"+a.ID), OnClick(onReviewGoals), uistate.T("integrity.reviewGoals")))
}
