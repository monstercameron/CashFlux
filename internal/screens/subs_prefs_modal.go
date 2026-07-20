// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// subsPrefsButtonProps configures the detection-preferences trigger button.
type subsPrefsButtonProps struct {
	Label string // includes the active-filter badge
}

// subsPrefsButton is the tiny isolated subscriber that opens the detection-
// preferences modal (only it + the host re-render on open/close).
func subsPrefsButton(props subsPrefsButtonProps) ui.Node {
	open := uistate.UseSubsPrefsOpen()
	click := ui.UseEvent(Prevent(func() { open.Set(true) }))
	return Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "subs-detect-prefs-toggle"),
		Title(uistate.T("subs.detectPrefsTitle")), OnClick(click), props.Label)
}

// SubsDetectPrefsFormProps configures the detection-preferences flip-modal body.
type SubsDetectPrefsFormProps struct {
	OnDone func() // called to close the modal
}

// SubsDetectPrefsForm is the subscription-detection preferences flip-modal body:
// the sensitivity (minimum repeats) select plus the account-type and category
// ignore filters. Every control saves immediately (and bumps the data revision so
// the subscriptions list behind the modal recomputes live); Done just closes. Its
// own component so its hooks sit at stable positions.
func SubsDetectPrefsForm(props SubsDetectPrefsFormProps) ui.Node {
	app := appstate.Default
	// The form's own controls bump the data revision, but it never subscribed to
	// one — so the surface behind the modal updated while the modal itself showed
	// what it had rendered on open. That was invisible while every control was a
	// toggle showing its own state, and stopped being invisible the moment the
	// modal grew a list that a control REMOVES a row from: un-hiding a rejection
	// put it back in the review queue and left it sitting in the hidden list.
	_ = uistate.UseDataRevision().Get()

	onMinOccur := ui.UseEvent(func(e ui.Event) {
		n, _ := strconv.Atoi(e.GetValue())
		uistate.SaveSubsDetectPrefs(uistate.LoadSubsDetectPrefs().WithMinOccurrences(n))
		uistate.BumpDataRevision()
	})
	done := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	if app == nil {
		return Fragment()
	}
	detectPrefs := uistate.LoadSubsDetectPrefs()

	// Account types that actually appear in transactions — the filter list stays
	// relevant to the dataset.
	acctTypeByID := map[string]string{}
	for _, ac := range app.Accounts() {
		acctTypeByID[ac.ID] = string(ac.Type)
	}
	acctTypesInUse := map[string]bool{}
	for _, t := range app.Transactions() {
		if typ, ok := acctTypeByID[t.AccountID]; ok && typ != "" {
			acctTypesInUse[typ] = true
		}
	}
	orderedAcctTypes := make([]string, 0, len(domain.AllAccountTypes))
	for _, typ := range domain.AllAccountTypes {
		if acctTypesInUse[string(typ)] {
			orderedAcctTypes = append(orderedAcctTypes, string(typ))
		}
	}

	// Top-level expense categories only (detection runs on expenses).
	expenseCats := make([]domain.Category, 0)
	for _, c := range app.Categories() {
		if c.Kind == domain.KindExpense && c.ParentID == "" {
			expenseCats = append(expenseCats, c)
		}
	}
	var catFilterSection ui.Node
	if len(expenseCats) == 0 {
		catFilterSection = P(css.Class("row-meta"), uistate.T("subs.detectCategoriesNone"))
	} else {
		catFilterSection = Div(css.Class(tw.Fold(tw.Mt2)),
			Span(css.Class("row-meta "+tw.Fold(tw.FontMedium, tw.Block, tw.Mb1)), uistate.T("subs.detectCategoriesLabel")),
			Div(css.Class(tw.Fold(tw.Flex, tw.FlexWrap, tw.Gap2)),
				MapKeyed(expenseCats,
					func(c domain.Category) any { return "cat|" + c.ID },
					func(c domain.Category) ui.Node {
						return ui.CreateElement(SubsDetectCatRow, subsDetectCatRowProps{
							CatID:   c.ID,
							Label:   c.Name,
							Ignored: detectPrefs.HasIgnoredCategory(c.ID),
						})
					},
				),
			),
		)
	}

	return Div(css.Class("subs-prefs-modal"), Attr("data-testid", "subs-detect-prefs"),
		P(css.Class("row-meta"), uistate.T("subs.detectPrefsDesc")),
		Div(css.Class(tw.Fold(tw.Mt2, tw.Mb1)),
			Span(css.Class("row-meta "+tw.Fold(tw.FontMedium, tw.Block, tw.Mb1)), uistate.T("subs.detectSensitivityLabel")),
			Select(css.Class("field"), Attr("data-testid", "subs-detect-min-occur"),
				Attr("aria-label", uistate.T("subs.detectSensitivityLabel")), OnChange(onMinOccur),
				Option(Value("2"), SelectedIf(detectPrefs.MinOccurrencesOrDefault() == 2), uistate.T("subs.detectSens2")),
				Option(Value("3"), SelectedIf(detectPrefs.MinOccurrencesOrDefault() == 3), uistate.T("subs.detectSens3")),
				Option(Value("4"), SelectedIf(detectPrefs.MinOccurrencesOrDefault() == 4), uistate.T("subs.detectSens4")),
			),
		),
		If(len(orderedAcctTypes) > 0, Div(css.Class(tw.Fold(tw.Mt2, tw.Mb1)),
			Span(css.Class("row-meta "+tw.Fold(tw.FontMedium, tw.Block, tw.Mb1)), uistate.T("subs.detectAccountTypesLabel")),
			Div(css.Class(tw.Fold(tw.Flex, tw.FlexWrap, tw.Gap2)),
				MapKeyed(orderedAcctTypes,
					func(typ string) any { return "accttype|" + typ },
					func(typ string) ui.Node {
						return ui.CreateElement(SubsDetectAcctTypeRow, subsDetectAcctTypeRowProps{
							AcctType: typ,
							Label:    uistate.T("acctType." + typ),
							Ignored:  detectPrefs.HasIgnoredAccountType(typ),
						})
					},
				),
			),
		)),
		catFilterSection,
		rhyWeakSignalsSection(app),
		rhyHiddenSection(),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "subs-prefs-done"), OnClick(done), uistate.T("recurring.done")),
		),
	)
}

// rhyWeakSignalsSection lists the repeating patterns discovery judged too weak
// to propose — the Silent tier. It is display-only (no hooks), so it is safe
// inside the preferences form's render.
//
// The review strip's header names this count and links here. A number the user
// is told about and cannot reach is worse than no number at all: it was the
// reason the strip could claim "57 found" while offering five. Each entry
// carries the same evidence sentence the strip would have shown, so "too weak"
// is a judgment the user can check rather than take on trust.
func rhyWeakSignalsSection(app *appstate.App) ui.Node {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	_, weak := rhySplitCandidates(app, time.Now(), base)
	if len(weak) == 0 {
		return Fragment()
	}
	items := []any{css.Class("rhy-weak-list"), Attr("data-testid", "rhy-weak-signals-list")}
	for _, c := range weak {
		items = append(items, Li(
			Span(css.Class("rhy-weak-name"), c.Payee),
			Span(css.Class("rhy-weak-ev"), rhyEvidenceSentence(c.Evidence, base)),
		))
	}
	return Div(css.Class(tw.Fold(tw.Mt3)),
		Span(css.Class("row-meta "+tw.Fold(tw.FontMedium, tw.Block, tw.Mb1)),
			uistate.T("subs.weakSignalsLabel", len(weak))),
		P(css.Class("row-meta"), uistate.T("subs.weakSignalsDesc")),
		Ul(items...),
	)
}

// rhyHiddenSection lists the candidates the user rejected with "Not recurring",
// each with the way back.
//
// Rejecting wrote a suppressed signature and nothing surfaced it again — one
// misclick permanently hid a real commitment, with no route back and no way to
// even learn that something was missing. A destructive action with no undo has no
// business being a single unconfirmed click, and the cheapest honest fix is to
// keep the list where the other detection judgments already live. It stays quiet:
// a small section at the tail, rendered only when there is something in it.
func rhyHiddenSection() ui.Node {
	hidden := uistate.LoadRecurPins().SuppressedList()
	if len(hidden) == 0 {
		return Fragment()
	}
	items := []any{css.Class("rhy-weak-list rhy-hidden-list"), Attr("data-testid", "rhy-hidden-list")}
	for _, h := range hidden {
		items = append(items, ui.CreateElement(rhyHiddenRow, rhyHiddenRowProps{Entry: h}))
	}
	return Div(css.Class(tw.Fold(tw.Mt3)),
		Span(css.Class("row-meta "+tw.Fold(tw.FontMedium, tw.Block, tw.Mb1)),
			uistate.T("rhythm.hiddenLabel", len(hidden))),
		P(css.Class("row-meta"), uistate.T("rhythm.hiddenDesc")),
		Ul(items...),
	)
}

// rhyHiddenRowProps drives one rejected-candidate row.
type rhyHiddenRowProps struct{ Entry uistate.SuppressedEntry }

// rhyHiddenRow renders one rejected candidate with its un-hide verb. Its own
// component so the hook sits at a stable position inside the list.
func rhyHiddenRow(props rhyHiddenRowProps) ui.Node {
	sig := props.Entry.Signature
	restore := ui.UseEvent(Prevent(func() {
		uistate.UnsuppressSignature(sig)
		uistate.BumpDataRevision()
	}))
	return Li(
		Span(css.Class("rhy-weak-name"), props.Entry.Name),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "rhy-unhide-"+rhySlug(sig)),
			Title(uistate.T("rhythm.unsuppressTitle")), OnClick(restore),
			uistate.T("rhythm.unsuppress")),
	)
}
