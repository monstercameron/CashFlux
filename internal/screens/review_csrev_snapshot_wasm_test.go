// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type csrevDOM struct {
	Tag      string
	Text     string            `json:",omitempty"`
	Attrs    map[string]string `json:",omitempty"`
	Children []csrevDOM        `json:",omitempty"`
}

// Framework-generated IDs encode component fiber positions, which intentionally
// change when a view becomes several components. Explicit product IDs and all
// visible text, tags, classes and accessibility attributes remain in the digest.
func csrevDOMSnapshot(node *render.QueryNode) csrevDOM {
	out := csrevDOM{Tag: node.Tag(), Attrs: map[string]string{}}
	for _, name := range []string{"class", "data-testid", "data-tier", "data-scope", "role", "aria-label", "aria-live", "aria-modal", "aria-hidden", "aria-checked", "aria-expanded", "tabindex", "disabled", "href", "type", "value", "selected", "title"} {
		if value := node.Attr(name); value != "" {
			out.Attrs[name] = value
		}
	}
	for _, name := range []string{"id", "aria-describedby", "aria-labelledby"} {
		if value := node.Attr(name); strings.HasPrefix(value, "review-") {
			out.Attrs[name] = value
		}
	}
	children := node.Children()
	if len(children) == 0 {
		out.Text = node.Text()
	}
	for _, child := range children {
		out.Children = append(out.Children, csrevDOMSnapshot(child))
	}
	return out
}

func csrevSnapshotArgs(t *testing.T) reviewSingleArgs {
	t.Helper()
	app := newReviewApp(t)
	txn := putCharge(t, app, "csrev-snapshot", "Acme Market", 12)
	row := reviewRow{Group: reviewqueue.Group{Key: "acme", Merchant: "Acme Market", Items: []domain.Transaction{txn}}, Tier: tierReady,
		HasSugg: true, Suggestion: catsuggest.Suggestion{CategoryID: "cat-food", Source: catsuggest.SourceRule}}
	return reviewSingleArgs{App: app, Index: reviewIndex{Rows: []reviewRow{row}, Total: 1}, Row: row, Txn: txn,
		CatFor: func(reviewRow) string { return "cat-food" }, GroupSize: 1, Targets: 1, Commit: reviewscope.CommitCharge,
		Confirm: func() {}, Skip: func() {}, Dismiss: func() {}, NewCat: func() {}, CancelAll: func() {},
		ToCategories: func(ui.Event) {}, ToRules: func(ui.Event) {}, PickCategory: func(ui.Event) {}, OnCommitScope: func(string) {},
	}
}

func csrevSnapshotSingle(a reviewSingleArgs) ui.Node {
	body, foot := reviewSingleView(a)
	return ui.Fragment(body, foot)
}

func csrevSnapshotBulk(a reviewSingleArgs) ui.Node {
	body, foot := reviewBulkView(a.App, a.Index, map[string]bool{"acme": true}, nil, nil,
		a.CatFor, func(string) {}, func(string) {}, func(string, string) {},
		func() {}, func() {}, func() {}, func() {}, func() {}, a.Notice, a.Prefs, 0,
		ui.Fragment(), ui.Fragment(), 0)
	return ui.Fragment(body, foot)
}

func TestCSREVSnapshotOriginalMarkup(t *testing.T) {
	for _, name := range []string{"single", "single-merchant", "single-split", "bulk", "bulk-empty"} {
		t.Run(name, func(t *testing.T) {
			a := csrevSnapshotArgs(t)
			if name == "single-merchant" {
				a.GroupSize, a.Targets, a.Commit = 2, 2, reviewscope.CommitMerchant
			}
			if name == "single-split" {
				a.Txn.Splits = []domain.CategorySplit{{CategoryID: "cat-food"}}
				a.Notice = "That did not save"
			}
			if name == "bulk-empty" {
				a.Index = reviewIndex{}
			}

			expected := csrevRenderSnapshot(t, a, true, strings.HasPrefix(name, "bulk"))
			actual := csrevRenderSnapshot(t, a, false, strings.HasPrefix(name, "bulk"))
			if !bytes.Equal(expected, actual) {
				t.Fatalf("rendered DOM changed\nbefore: %s\nafter: %s", expected, actual)
			}
		})
	}
}

func csrevSnapshotBeforeSingle(a reviewSingleArgs) ui.Node {
	body, foot := reviewSingleViewBeforeCSREV(a)
	return ui.Fragment(body, foot)
}

func csrevSnapshotBeforeBulk(a reviewSingleArgs) ui.Node {
	body, foot := reviewBulkViewBeforeCSREV(a.App, a.Index, map[string]bool{"acme": true}, nil, nil,
		a.CatFor, func(string) {}, func(string) {}, func(string, string) {},
		func() {}, func() {}, func() {}, func() {}, func() {}, a.Notice, a.Prefs, 0,
		ui.Fragment(), ui.Fragment(), 0)
	return ui.Fragment(body, foot)
}

func csrevRenderSnapshot(t *testing.T, a reviewSingleArgs, before, bulk bool) []byte {
	t.Helper()
	f := render.New(t)
	defer f.Cleanup()
	component := csrevSnapshotSingle
	if bulk {
		component = csrevSnapshotBulk
	}
	if before {
		if bulk {
			component = csrevSnapshotBeforeBulk
		} else {
			component = csrevSnapshotBeforeSingle
		}
	}
	f.Render(ui.CreateElement(component, a))
	raw, err := json.Marshal(csrevDOMSnapshot(f.Container()))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// These immutable reference renderers were copied from the ticket's starting
// review_surface.go. Keep their product markup frozen: this test compares the
// full DOM before and after componentization, including classes, attributes and
// copy. They are test fixtures, not a second production implementation.
func reviewSingleViewBeforeCSREV(a reviewSingleArgs) (ui.Node, ui.Node) {
	app, idx, row, cur, catFor := a.App, a.Index, a.Row, a.Txn, a.CatFor
	notice, pr := a.Notice, a.Prefs
	if cur.ID == "" {
		return P(css.Class("rvs-note"), uistate.T("review.emptySub")), Fragment()
	}
	kind := domain.KindExpense
	if cur.Amount.Amount > 0 {
		kind = domain.KindIncome
	}
	reason := reviewqueue.ReasonFor(cur)
	canCategorize := reviewqueue.CanAssignCategory(cur)
	selArgs := []any{css.Class("field"), Attr("data-testid", "review-category-select"),
		Attr("aria-label", uistate.T("review.categoryLabel")), OnChange(a.PickCategory)}
	if !canCategorize {
		selArgs = append(selArgs, Attr("disabled", "disabled"))
	}
	selArgs = append(selArgs, categorySelectNodes(app.Categories(), kind, catFor(row))...)
	scopeLine := []any{css.Class("rvs-cscope"), Attr("data-testid", "review-commit-scope")}
	if a.GroupSize > 1 {
		scopeLine = append(scopeLine, uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.commitScopeLabel"),
			Selected: string(a.Commit),
			Options: []uiw.SegOption{
				{Value: string(reviewscope.CommitCharge), Label: uistate.T("review.scopeThisCharge"),
					TestID: "review-scope-charge"},
				{Value: string(reviewscope.CommitMerchant),
					Label:  uistate.T("review.scopeAllMerchant", a.GroupSize, strings.TrimSpace(row.Group.Merchant)),
					TestID: "review-scope-merchant"},
			},
			OnSelect: a.OnCommitScope,
		}))
	} else {
		scopeLine = append(scopeLine,
			Span(css.Class("rvs-cscope-label"), uistate.T("review.commitScopeLabel")),
			Span(css.Class("rvs-cscope-fixed"), Attr("data-testid", "review-scope-only"),
				uistate.T("review.scopeThisCharge")))
	}
	scopePicker := Div(scopeLine...)
	leaveGlyph := Span(css.Class("rvs-links-out"), Attr("aria-hidden", "true"),
		uistate.T("review.leavesReviewGlyph"))
	crossLinks := Div(css.Class("rvs-links"), Attr("data-testid", "review-cross-links"),
		Span(css.Class("rvs-links-lead"), uistate.T("review.linksLead")),
		reviewDestinationLink("review-goto-categories", "/categories", uistate.T("review.gotoCategories"),
			uistate.T("review.leavesReviewAria", uistate.T("review.gotoCategories")), a.ToCategories, leaveGlyph),
		reviewDestinationLink("review-goto-rules", "/rules", uistate.T("review.gotoRules"),
			uistate.T("review.leavesReviewAria", uistate.T("review.gotoRules")), a.ToRules, leaveGlyph),
	)
	body := Div(css.Class("rvs-single"),
		Div(
			Div(css.Class("rvw-card"),
				Div(css.Class("rvw-card-top"),
					Span(css.Class("rvw-reason is-uncat"), Attr("data-testid", "review-reason"),
						uistate.T("review.reasonLead", reviewReasonLabel(reason))),
					Span(css.Class("rvw-date"), pr.FormatDate(cur.Date)),
				),
				Div(css.Class("rvw-payee"), Attr("data-testid", "review-payee"), row.Group.Merchant),
				Div(css.Class("rvw-rawpayee"), rawPayeeOf(cur)),
				Div(css.Class("rvw-meta"),
					reviewMoneyNode("rvw-amount", cur.Amount),
					If(reviewAcctName(app, cur.AccountID) != "",
						Span(css.Class("rvw-acct"), reviewAcctName(app, cur.AccountID))),
				),
			),
			Div(css.Class("rvs-assign"),
				Div(css.Class("rvw-assign-label"), uistate.T("review.categoryLabel")),
				Div(css.Class("rvs-assign-row"),
					Select(selArgs...),
					Button(css.Class("btn btn-sm rvs-newcat"), Type("button"),
						Attr("data-testid", "review-new-category"),
						Attr("aria-label", uistate.T("review.newCategoryAria")),
						Attr("disabled", disabledIf(!canCategorize)),
						OnClick(a.NewCat), uistate.T("review.newCategory")),
				),
				If(row.HasSugg, Div(css.Class("rvs-why"), Attr("data-testid", "review-suggest-why"),
					reviewWhy(row.Suggestion))),
				If(catFor(row) != "", P(css.Class("rvs-pending"), Attr("data-testid", "review-pending-note"),
					uistate.T("review.pendingNote"))),
				If(!canCategorize,
					P(css.Class("rvs-warn"), Attr("data-testid", "review-split-warn"),
						uistate.T("review.splitNeeded"))),
				If(notice != "", P(css.Class("rvs-notice"), Attr("role", "alert"),
					Attr("data-testid", "review-commit-err"), notice)),
			),
		),
		scopePicker,
		crossLinks,
		ui.CreateElement(reviewContextBand, reviewContextBandProps{App: app, Index: idx, Row: row, Txn: cur, Prefs: pr}),
	)
	confirmLabel := uistate.T("review.categorizeNext")
	snoozeLabel, dismissLabel := uistate.T("review.snooze"), uistate.T("review.dismiss")
	if a.Targets > 1 {
		confirmLabel = uistate.T("review.categorizeAllN", a.Targets)
		snoozeLabel = uistate.T("review.snoozeN", a.Targets)
		dismissLabel = uistate.T("review.dismissN", a.Targets)
	}
	var commitControl ui.Node
	if canCategorize {
		commitControl = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-commit"), OnClick(a.Confirm), confirmLabel)
	} else {
		commitControl = Span(css.Class("rvs-warn"), Attr("data-testid", "review-split-action"), uistate.T("review.splitNeeded"))
	}
	acts := Span(css.Class("rvs-foot-acts"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-dismiss"),
			OnClick(a.Dismiss), dismissLabel),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-skip"),
			OnClick(a.Skip), snoozeLabel),
		commitControl,
	)
	if canCategorize && a.Confirming {
		acts = ui.CreateElement(reviewConfirmAll, reviewConfirmAllProps{
			Targets:  a.Targets,
			Merchant: strings.TrimSpace(row.Group.Merchant),
			Category: reviewCatName(app, catFor(row)),
			Facts:    reviewGroupFacts(row, pr),
			OnCancel: a.CancelAll, OnGo: a.Confirm,
		})
	}
	foot := Fragment(
		Span(css.Class("rvs-foot-sub"), Attr("data-testid", "review-progress"),
			uistate.T("review.leftCount", idx.Total)),
		Span(css.Class("rvs-hidden"), Attr("data-testid", "review-total"), strconv.Itoa(idx.Total)),
		acts,
	)
	return body, foot
}

func reviewBulkViewBeforeCSREV(
	app *appstate.App, idx reviewIndex,
	sel map[string]bool, openG map[string]bool, manual map[string]string,
	catFor func(reviewRow) string,
	toggleGroup, toggleSelect func(string), setManual func(string, string),
	selReady, selLook, applySel, clearSel, makeRules any, notice string, pr prefs.Prefs, focused int,
	scanStrip, setupLink ui.Node, rev int,
) (ui.Node, ui.Node) {
	decisions := len(idx.Rows)
	selectReadyClick := OnClick(selReady)
	selectLookClick := OnClick(selLook)
	var tiers []ui.Node
	for _, t := range []reviewTier{tierReady, tierLook, tierNone} {
		m, c := fmtTierCount(idx.Rows, t)
		if m == 0 {
			continue
		}
		rows := make([]reviewRow, 0, m)
		focusKey := ""
		if focused >= 0 && focused < len(idx.Rows) {
			focusKey = idx.Rows[focused].Group.Key
		}
		for _, r := range idx.Rows {
			if r.Tier == t {
				rows = append(rows, r)
			}
		}
		var act ui.Node = Fragment()
		switch t {
		case tierReady:
			act = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-selectall-ready"), selectReadyClick,
				uistate.T("review.selectAll", m))
		case tierLook:
			act = Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "review-selectall-look"), selectLookClick,
				uistate.T("review.selectAll", m))
		}
		tiers = append(tiers, Div(css.Class("rvs-tier "+tierMod(t)), Attr("data-tier", tierMod(t)),
			Attr("role", "group"), Attr("aria-label", tierLabel(t)),
			Div(css.Class("rvs-tier-head"),
				Span(css.Class("rvs-tier-mark")),
				Span(css.Class("rvs-tier-name"), tierLabel(t)),
				Span(css.Class("rvs-tier-count"), uistate.T("review.tierCountPlain", plural(m, "merchant"), plural(c, "charge"))),
				Span(css.Class("rvs-tier-act"), act),
			),
			Div(css.Class("rvs-groups"),
				MapKeyed(rows, func(r reviewRow) any { return r.Group.Key + "#" + strconv.Itoa(rev) }, func(r reviewRow) ui.Node {
					return ui.CreateElement(reviewGroupRow, reviewGroupRowProps{
						Row: r, App: app, Selected: sel[r.Group.Key], Open: openG[r.Group.Key], Focused: r.Group.Key == focusKey,
						CategoryID: catFor(r), Manual: manual[r.Group.Key] != "",
						Prefs: pr, OnToggleOpen: toggleGroup, OnToggleSelect: toggleSelect,
						OnCategory: setManual,
					})
				}),
			),
		))
	}
	body := Div(
		reviewIntro(idx.Total, decisions),
		P(css.Class("rvs-note"), uistate.T("review.collapseNote")),
		scanStrip,
		Div(css.Class("rvs-legend"),
			Span(Span(css.Class("rvs-dot is-ready")), uistate.T("review.legendHigh")),
			Span(Span(css.Class("rvs-dot is-look")), uistate.T("review.legendMid")),
			Span(Span(css.Class("rvs-dot is-none")), uistate.T("review.legendNone")),
		),
		If(notice != "", P(css.Class("rvs-notice"), Attr("data-testid", "review-notice"),
			Attr("role", "status"), Attr("aria-live", "polite"), notice)),
		Fragment(nodesToAny(tiers)...),
	)
	selMerchants, selCharges := 0, 0
	for _, r := range idx.Rows {
		if sel[r.Group.Key] {
			selMerchants++
			selCharges += len(r.Group.Items)
		}
	}
	label := uistate.T("review.nothingPicked")
	sub := uistate.T("review.pickHint")
	confirm := uistate.T("review.confirm")
	if selMerchants > 0 {
		label = uistate.T("review.selectionCountPlain", plural(selMerchants, "merchant"), plural(selCharges, "charge"))
		sub = uistate.T("review.undoAvailable")
		confirm = uistate.T("review.confirmN", selCharges)
	}
	var disabled any
	if selMerchants == 0 {
		disabled = Attr("disabled", "disabled")
	}
	foot := Fragment(
		Span(css.Class("rvs-foot-n"), Attr("data-testid", "review-selection"),
			Attr("role", "status"), Attr("aria-live", "polite"), label),
		Span(css.Class("rvs-foot-sub"), sub),
		setupLink,
		Span(css.Class("rvs-foot-acts"),
			If(selMerchants > 0,
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-makerule"),
					OnClick(makeRules), uistate.T("review.makeRule"))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-clear"),
				disabled,
				OnClick(clearSel), uistate.T("review.clear")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-apply"), disabled,
				OnClick(applySel), confirm),
		),
	)
	return body, foot
}
