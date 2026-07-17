// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// dashboard_resume.go is the "Continue where you left off" card (#62): a quiet
// band between the hero and the bento that surfaces genuinely unfinished flows
// — a half-done reconciliation (the #48 saved draft), a review-inbox queue with
// items remaining, an import left mid-wizard, an unresolved budget
// over-assignment — each with a one-click jump back in. It renders nothing when
// there is nothing to resume, and a session dismiss keeps it polite.

// resumeRowKind selects a resume row's jump-back behavior.
type resumeRowKind string

const (
	resumeReview      resumeRowKind = "review"      // open the Review inbox
	resumeReconcile   resumeRowKind = "reconcile"   // deep-link the account row on /accounts
	resumeImport      resumeRowKind = "import"      // reopen the import review on /documents
	resumeImportStale resumeRowKind = "importStale" // rows lost to a reload: clear + open /documents
	resumeOverassign  resumeRowKind = "overassign"  // open the month-close flow on /budgets
)

// resumeRowProps configures one resume row.
type resumeRowProps struct {
	Kind   resumeRowKind
	Text   string // the unfinished-work sentence
	Action string // the button label ("Continue", "Resume", …)
	AcctID string // reconcile rows: the account to focus
	TestID string
}

// resumeRow is one unfinished-work line: the sentence plus its jump-back
// button. Own component so its hooks stay out of the card's row loop
// (CLAUDE.md hooks gotcha).
func resumeRow(p resumeRowProps) ui.Node {
	nav := router.UseNavigate()
	monthClose := uistate.UseMonthCloseOpen()
	kind, acctID := p.Kind, p.AcctID
	onGo := ui.UseEvent(func() {
		switch kind {
		case resumeReview:
			uistate.OpenReviewInbox()
		case resumeReconcile:
			uistate.SetDeepLinkFocus(`[data-testid="acct-row-` + acctID + `"]`)
			nav.Navigate(uistate.RoutePath("/accounts"))
		case resumeImport:
			nav.Navigate(uistate.RoutePath("/documents"))
		case resumeImportStale:
			// The rows are gone (reload); honesty means clearing the marker so
			// the card doesn't keep advertising work that no longer exists.
			uistate.ClearImportWIP()
			nav.Navigate(uistate.RoutePath("/documents"))
		case resumeOverassign:
			monthClose.Set(true)
			nav.Navigate(uistate.RoutePath("/budgets"))
		}
	})
	return Div(css.Class("resume-row"), Attr("data-testid", p.TestID),
		Span(css.Class("resume-row-text"), p.Text),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("aria-label", p.Action+" — "+p.Text),
			OnClick(onGo),
			p.Action,
		),
	)
}

// dashResumeCard assembles the resume rows from the live flow markers. All
// hooks sit above the early returns (stable positions).
func dashResumeCard() ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	dismissed := ui.UseState(false)
	onDismiss := ui.UseEvent(func() { dismissed.Set(true) })
	importRows := uistate.UseImportDraftRows().Get()
	pr := uistate.UsePrefs().Get().Normalize()
	vw := uistate.UsePeriod().Get()
	activeMember := uistate.UseActiveMember().Get()
	// The card reads several full-dataset signals (review queue, budget view);
	// defer it off the first paint like the other data-heavy dashboard bands.
	settled := useAfterSettle("resume")

	if app == nil || dismissed.Get() || !settled {
		return Fragment()
	}

	rows := make([]ui.Node, 0, 4)

	// Half-done reconciliations — the #48 saved drafts, one row per account.
	for _, a := range app.Accounts() {
		if _, _, ok := uistate.LoadReconcileDraft(a.ID); ok {
			rows = append(rows, ui.CreateElement(resumeRow, resumeRowProps{
				Kind: resumeReconcile, AcctID: a.ID,
				Text:   uistate.T("dashboard.resumeReconcile", a.Name),
				Action: uistate.T("dashboard.resumeResume"),
				TestID: "resume-reconcile-" + a.ID,
			}))
		}
	}

	// An import left mid-wizard: live rows resume the review; a marker without
	// rows means a reload dropped them — say so instead of promising a resume.
	if n := len(importRows); n > 0 {
		rows = append(rows, ui.CreateElement(resumeRow, resumeRowProps{
			Kind:   resumeImport,
			Text:   uistate.T("dashboard.resumeImport", n),
			Action: uistate.T("dashboard.resumeResume"),
			TestID: "resume-import",
		}))
	} else if uistate.ImportWIPCount() > 0 {
		rows = append(rows, ui.CreateElement(resumeRow, resumeRowProps{
			Kind:   resumeImportStale,
			Text:   uistate.T("dashboard.resumeImportStale"),
			Action: uistate.T("dashboard.resumeOpenDocs"),
			TestID: "resume-import-stale",
		}))
	}

	// Review inbox with items remaining.
	if n := len(reviewqueue.Queue(app.Transactions())); n > 0 {
		rows = append(rows, ui.CreateElement(resumeRow, resumeRowProps{
			Kind:   resumeReview,
			Text:   uistate.T("dashboard.resumeReview", n),
			Action: uistate.T("dashboard.resumeContinue"),
			TestID: "resume-review",
		}))
	}

	// Unresolved budget over-assignment — the same rule the month-close flow
	// applies (buildMonthCloseSummary), read off the memoized budget view.
	v := computeBudgetView(app, activeMember, vw, pr, false)
	overAssigned := int64(0)
	switch v.Method {
	case budgeting.MethodZeroBased:
		if ta := budgeting.ToAssign(v.BannerIncome+v.RolledOver, v.TotalLimit+v.SavingsAssigned); ta < 0 {
			overAssigned = -ta
		}
	case budgeting.MethodSimple:
		if d := v.BannerIncome - v.TotalLimit; d < 0 {
			overAssigned = -d
		}
	}
	if overAssigned > 0 {
		rows = append(rows, ui.CreateElement(resumeRow, resumeRowProps{
			Kind:   resumeOverassign,
			Text:   uistate.T("dashboard.resumeOverassign", fmtMoney(money.New(overAssigned, v.Base))),
			Action: uistate.T("dashboard.resumeResolve"),
			TestID: "resume-overassign",
		}))
	}

	if len(rows) == 0 {
		return Fragment()
	}
	// Keep the band glanceable: the four flow kinds above cap it naturally, but
	// many parallel reconcile drafts could still stack — show at most four rows.
	if len(rows) > 4 {
		rows = rows[:4]
	}

	return Div(css.Class("resume-card"),
		Attr("data-testid", "dash-resume-card"),
		Attr("role", "complementary"),
		Attr("aria-label", uistate.T("dashboard.resumeTitle")),
		Div(css.Class("resume-card-head"),
			Strong(uistate.T("dashboard.resumeTitle")),
			Button(css.Class("resume-dismiss"), Type("button"),
				Attr("aria-label", uistate.T("dashboard.resumeDismiss")),
				Attr("title", uistate.T("dashboard.resumeDismiss")),
				Attr("data-testid", "resume-dismiss"),
				OnClick(onDismiss),
				"×",
			),
		),
		Div(css.Class("resume-rows"), rows),
	)
}
