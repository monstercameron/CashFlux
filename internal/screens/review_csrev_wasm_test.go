// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func csrevArgs(t *testing.T) reviewSingleArgs {
	t.Helper()
	app := newReviewApp(t)
	txn := putCharge(t, app, "csrev-charge", "Acme Market", 12)
	row := reviewRow{Group: reviewqueue.Group{Key: "acme", Merchant: "Acme Market", Items: []domain.Transaction{txn}}, Tier: tierReady,
		HasSugg: true, Suggestion: catsuggest.Suggestion{CategoryID: "cat-food", Source: catsuggest.SourceRule}}
	return reviewSingleArgs{App: app, Index: reviewIndex{Rows: []reviewRow{row}, Total: 1}, Row: row, Txn: txn,
		CatFor: func(reviewRow) string { return "cat-food" }, GroupSize: 1, Targets: 1, Commit: reviewscope.CommitCharge,
		Confirm: func() {}, Skip: func() {}, Dismiss: func() {}, NewCat: func() {}, CancelAll: func() {},
		ToCategories: func(ui.Event) {}, ToRules: func(ui.Event) {}, PickCategory: func(ui.Event) {}, OnCommitScope: func(string) {},
	}
}

// Every extracted child has an explicit render contract. These check the same
// public DOM hooks as the pre-refactor surface, without a hook-baseline exemption.
func TestCSREVChildComponentsPreserveDOM(t *testing.T) {
	a := csrevArgs(t)
	bulk := reviewBulkProps{App: a.App, Index: a.Index, CatFor: a.CatFor, Selected: map[string]bool{"acme": true},
		SelectReady: func() {}, SelectLook: func() {}, Apply: func() {}, Clear: func() {}, MakeRules: func() {},
		ToggleGroup: func(string) {}, ToggleSelect: func(string) {}, SetManual: func(string, string) {},
	}
	for _, tc := range []struct {
		name, tag, testID string
		node              func() ui.Node
	}{
		{"empty", "button", "review-done", func() ui.Node {
			return ui.CreateElement(reviewEmptyView, reviewEmptyViewProps{Title: "Done", Sub: "No charges"})
		}},
		{"mode header", "button", "review-mode-single", func() ui.Node {
			return ui.CreateElement(reviewModeHeader, reviewModeHeaderProps{Mode: reviewModeBulk, OnSelect: func(string) {}})
		}},
		{"AI setup", "a", "review-ai-setup", func() ui.Node { return ui.CreateElement(reviewAISetup, reviewAISetupProps{OnClick: func() {}}) }},
		{"bulk body", "p", "review-total-line", func() ui.Node { return ui.CreateElement(reviewBulkBody, bulk) }},
		{"bulk tier", "button", "review-selectall-ready", func() ui.Node {
			return ui.CreateElement(reviewBulkTier, reviewBulkTierProps{Bulk: bulk, Tier: tierReady, Merchants: 1, Charges: 1})
		}},
		{"bulk footer", "button", "review-apply", func() ui.Node { return ui.CreateElement(reviewBulkFooter, bulk) }},
		{"single body", "div", "review-payee", func() ui.Node { return ui.CreateElement(reviewSingleBody, a) }},
		{"single card", "div", "review-payee", func() ui.Node { return ui.CreateElement(reviewSingleCard, a) }},
		{"single assignment", "select", "review-category-select", func() ui.Node { return ui.CreateElement(reviewSingleAssignment, a) }},
		{"single scope", "span", "review-scope-only", func() ui.Node { return ui.CreateElement(reviewCommitScope, a) }},
		{"single links", "a", "review-goto-rules", func() ui.Node { return ui.CreateElement(reviewSingleLinks, a) }},
		{"single actions", "button", "review-commit", func() ui.Node { return ui.CreateElement(reviewSingleActions, a) }},
		{"single footer", "span", "review-progress", func() ui.Node { return ui.CreateElement(reviewSingleFooter, a) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			f.Render(tc.node())
			if node := byTestID(f, tc.tag, tc.testID); node == nil {
				t.Fatalf("missing %s in %q", tc.testID, f.Text())
			}
		})
	}
}

func TestCSREVSingleChildrenOwnLiveCallbacks(t *testing.T) {
	a := csrevArgs(t)
	var calls []string
	a.Confirm = func() { calls = append(calls, "confirm") }
	a.Skip = func() { calls = append(calls, "skip") }
	a.Dismiss = func() { calls = append(calls, "dismiss") }
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(reviewSingleActions, a))
	for _, id := range []string{"review-commit", "review-skip", "review-dismiss"} {
		byTestID(f, "button", id).Click()
	}
	if strings.Join(calls, ",") != "confirm,skip,dismiss" {
		t.Fatalf("action callbacks = %v", calls)
	}
	a.Notice = "That did not save"
	a.NewCat = func() { calls = append(calls, "new") }
	a.PickCategory = func(e ui.Event) { calls = append(calls, e.GetValue()) }
	f.Render(ui.CreateElement(reviewSingleAssignment, a))
	byTestID(f, "select", "review-category-select").Change("cat-food")
	byTestID(f, "button", "review-new-category").Click()
	if strings.Join(calls, ",") != "confirm,skip,dismiss,cat-food,new" {
		t.Fatalf("assignment callbacks = %v", calls)
	}
	if errorNode := byTestID(f, "p", "review-commit-err"); errorNode == nil || errorNode.Attr("role") != "alert" || errorNode.Text() != a.Notice {
		t.Fatal("save failure lost its original alert")
	}
	a.ToCategories = func(ui.Event) { calls = append(calls, "categories") }
	a.ToRules = func(ui.Event) { calls = append(calls, "rules") }
	f.Render(ui.CreateElement(reviewSingleLinks, a))
	byTestID(f, "a", "review-goto-categories").Click()
	byTestID(f, "a", "review-goto-rules").Click()
	if !strings.HasSuffix(strings.Join(calls, ","), "categories,rules") {
		t.Fatalf("destination callbacks = %v", calls)
	}
}

func TestCSREVScopeAndFooterChangeWithoutConditionalHooks(t *testing.T) {
	a := csrevArgs(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(reviewCommitScope, a))
	if byTestID(f, "span", "review-scope-only") == nil {
		t.Fatal("single scope answer missing")
	}
	a.GroupSize = 2
	var picked string
	a.OnCommitScope = func(value string) { picked = value }
	f.Render(ui.CreateElement(reviewCommitScope, a))
	byTestID(f, "button", "review-scope-merchant").Click()
	if picked != string(reviewscope.CommitMerchant) {
		t.Fatalf("scope callback = %q", picked)
	}
	a.Targets = 2
	f.Render(ui.CreateElement(reviewSingleFooter, a))
	if button := byTestID(f, "button", "review-commit"); button == nil || button.Text() != uistate.T("review.categorizeAllN", 2) {
		t.Fatal("merchant action count changed")
	}
	a.Txn.Splits = []domain.CategorySplit{{CategoryID: "cat-food", Amount: a.Txn.Amount}}
	a.Txn.Splits[0].Amount.Amount++
	f.Render(ui.CreateElement(reviewSingleFooter, a))
	if byTestID(f, "span", "review-split-action") == nil || byTestID(f, "button", "review-commit") != nil {
		t.Fatal("unfinished split can be committed")
	}
}

func TestCSREVModeHeaderAISetupAndBulkFooterCallbacks(t *testing.T) {
	a := csrevArgs(t)
	f := render.New(t)
	defer f.Cleanup()
	var selected string
	f.Render(ui.CreateElement(reviewModeHeader, reviewModeHeaderProps{Mode: reviewModeBulk, OnSelect: func(value string) { selected = value }}))
	byTestID(f, "button", "review-mode-single").Click()
	if selected != reviewModeSingle {
		t.Fatal("mode callback missing")
	}
	setup := false
	f.Render(ui.CreateElement(reviewAISetup, reviewAISetupProps{OnClick: func() { setup = true }}))
	byTestID(f, "a", "review-ai-setup").Click()
	if !setup {
		t.Fatal("setup callback missing")
	}
	var actions []string
	p := reviewBulkProps{Index: a.Index, MakeRules: func() { actions = append(actions, "rule") }, Clear: func() { actions = append(actions, "clear") }, Apply: func() { actions = append(actions, "apply") }}
	f.Render(ui.CreateElement(reviewBulkFooter, p))
	if !isDisabled(byTestID(f, "button", "review-apply")) {
		t.Fatal("empty selection enabled")
	}
	p.Selected = map[string]bool{"acme": true}
	f.Render(ui.CreateElement(reviewBulkFooter, p))
	for _, id := range []string{"review-makerule", "review-clear", "review-apply"} {
		byTestID(f, "button", id).Click()
	}
	if strings.Join(actions, ",") != "rule,clear,apply" {
		t.Fatalf("bulk callbacks = %v", actions)
	}
}

func TestCSREVSurfaceSwitchesModesAndRebuildsOnRevision(t *testing.T) {
	a := csrevArgs(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(reviewSurfaceReady, struct{}{}))
	defer uistate.ResetReviewSession()
	byTestID(f, "button", "review-mode-single").Click()
	if byTestID(f, "select", "review-category-select") == nil {
		t.Fatal("single mode did not mount")
	}
	byTestID(f, "button", "review-mode-bulk").Click()
	if byTestID(f, "button", "review-apply") == nil {
		t.Fatal("bulk mode did not mount")
	}
	putCharge(t, a.App, "csrev-second", "Second Merchant", 13)
	uistate.BumpDataRevision()
	f.Render(ui.CreateElement(reviewSurfaceReady, struct{}{}))
	if line := byTestID(f, "p", "review-total-line"); line == nil || !strings.Contains(line.Text(), "2 charges") {
		t.Fatalf("new revision did not rebuild index: %q", f.Text())
	}
}
