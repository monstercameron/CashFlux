// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The leak notice (C672): rows that look like transfers and were never linked are
// counting as income and spending RIGHT NOW, and this says so where the distortion
// actually shows.
//
// The ticket is explicit that these rows must not be hidden: a household can
// legitimately call a category "Transfer" for something that really is spending,
// and quietly dropping every row that wears the name would conceal real money —
// silently, which is worse than the leak. The structural relationship
// (TransferAccountID) stays the only thing that decides what counts.
//
// So this changes no total. It states the size of the error beside the figures it
// affects, and offers the one action that resolves it. A report that is wrong by a
// known amount and says so is honest — but only if the amount is right, which is
// why the figures are split per currency and clipped to the same window the
// surrounding figures were computed over. A notice that quoted the household's
// whole history beside one quarter's numbers, or summed yen into dollars, would be
// worse than the silence it replaced.

// transferLeakProps configures the notice.
type transferLeakProps struct {
	// Txns is the population the surrounding figures were computed from, so the
	// notice describes THOSE figures rather than the household.
	Txns []domain.Transaction
	// From / To clip the notice to the same window the surrounding figures cover.
	// Half-open [From, To); zero on either side means unbounded there.
	From time.Time
	To   time.Time
	// Where names the surface, so the sentence can say what is affected.
	Where string
}

// transferLeakNotice renders the leak statement, or nothing when the population
// is clean — which is the common case, so this is a state and not a fixture.
func transferLeakNotice(props transferLeakProps) ui.Node {
	// Hooks first, unconditionally, before any early return. A conditional hook is
	// the framework's documented way to get a handler that silently does nothing,
	// and this component is meant to be mounted on more than one surface.
	reviewAtom := uistate.UseTransferReviewOpen()
	openReview := ui.UseEvent(Prevent(func() { reviewAtom.Set(true) }))
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	suspects := txnclassify.SuspectsIn(props.Txns, func(id string) string {
		return budgetCategoryName(app, id)
	}, props.From, props.To)
	if len(suspects) == 0 {
		return Fragment()
	}
	total := txnclassify.Leaked(suspects)

	// One line per currency. Leaked deliberately does not FX-convert, so adding
	// its figures up and stamping the household's currency on the result would
	// print a number belonging to no currency at all.
	//
	// Income and spending stay separate within each: money you did not earn and
	// money you did not spend are two different wrongnesses, and one netted figure
	// would understate both.
	var lines []ui.Node
	for _, cl := range txnclassify.LeakedByCurrency(suspects) {
		if cl.Totals.IncomeMinor > 0 {
			lines = append(lines, Span(css.Class("txnleak-part"),
				Attr("data-testid", "txnleak-income-"+cl.Currency),
				uistate.T("txnleak.income", fmtMoney(money.New(cl.Totals.IncomeMinor, cl.Currency)))))
		}
		if cl.Totals.SpendingMinor > 0 {
			lines = append(lines, Span(css.Class("txnleak-part"),
				Attr("data-testid", "txnleak-spending-"+cl.Currency),
				uistate.T("txnleak.spending", fmtMoney(money.New(cl.Totals.SpendingMinor, cl.Currency)))))
		}
	}

	return Div(css.Class("txnleak"), Attr("data-testid", "txnleak-notice"), Attr("role", "status"),
		Div(css.Class("txnleak-main"),
			Span(css.Class("txnleak-title"), Attr("data-testid", "txnleak-title"),
				uistate.T("txnleak.title", plural(total.Count, "row"), props.Where)),
			Div(css.Class("txnleak-parts", tw.TextDim), lines),
		),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "txnleak-review"),
			Title(uistate.T("txnleak.reviewTitle")), OnClick(openReview), uistate.T("txnleak.review")),
	)
}
