// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// reportActionsProps carries the period label the follow-up task titles use.
type reportActionsProps struct{ PeriodLabel string }

// reportActionsMenu is the /reports "Turn into action" menu — the report
// culminates in next steps instead of stopping at charts: create a follow-up
// task for this period, open the transaction review inbox, or jump to budgets
// or goals to act on what the report showed.
func reportActionsMenu(props reportActionsProps) ui.Node {
	nav := router.UseNavigate()
	reviewAtom := uistate.UseReviewInbox()
	addFollowUp := func() {
		app := appstate.Default
		if app == nil {
			return
		}
		title := uistate.T("reports.actionTaskTitle", props.PeriodLabel)
		task := domain.Task{
			ID: id.New(), Title: title, Status: domain.StatusOpen,
			Priority: domain.PriorityMedium, Source: domain.SourceManual,
			Due: time.Now().AddDate(0, 0, 7),
		}
		if err := app.PutTask(task); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("todo.suggestAdded", title))
	}
	return uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText:   uistate.T("reports.actionsMenu"),
		TriggerLabel:  uistate.T("reports.actionsMenuTitle"),
		TriggerTestID: "reports-actions-btn",
		TriggerClass:  "strip-toggle",
		Items: []uiw.OverflowMenuItem{
			{Label: uistate.T("reports.actionAddTask"), Icon: icon.Plus, TestID: "reports-action-task", OnSelect: addFollowUp},
			{Label: uistate.T("reports.actionReview"), Icon: icon.Receipt, TestID: "reports-action-review", OnSelect: func() {
				reviewAtom.Set(true)
			}},
			{Label: uistate.T("reports.actionBudgets"), Icon: icon.Budgets, TestID: "reports-action-budgets", OnSelect: func() {
				nav.Navigate(uistate.RoutePath("/budgets"))
			}},
			{Label: uistate.T("reports.actionGoal"), Icon: icon.Goals, TestID: "reports-action-goal", OnSelect: func() {
				nav.Navigate(uistate.RoutePath("/goals"))
				uistate.SetAddTarget("goal")
			}},
		},
	})
}
