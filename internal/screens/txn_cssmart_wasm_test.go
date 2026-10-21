// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCSSMARTReviewContentChildren(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	var mode string
	scans, applies, toggled := 0, 0, -1
	review := smartcatview.Reset(smartcatview.Suggest).Finish(smartcatview.Suggest, []smartai.SuggestedCategory{
		{Name: "Travel", Kind: "expense"}, {Name: "Salary", Kind: "income"},
	}, nil)
	p := txnSmartCatContentProps{
		State: review, Provider: true, OnMode: func(value string) { mode = value },
		OnScan: func() { scans++ }, OnApply: func() { applies++ }, OnToggle: func(i int) { toggled = i },
	}
	f.Render(ui.CreateElement(txnSmartCatContent, p))
	byTestID(f, "button", "smartcat-mode-auto").Click()
	if mode != smartcatview.Auto {
		t.Fatalf("mode = %q", mode)
	}
	byTestID(f, "button", "smartcat-scan").Click()
	byTestID(f, "button", "smartcat-apply").Click()
	byTestID(f, "input", "smartcat-sug-1").Click()
	if scans != 1 || applies != 1 || toggled != 1 {
		t.Fatalf("callbacks scan=%d apply=%d row=%d", scans, applies, toggled)
	}
	if got := byTestID(f, "button", "smartcat-apply").Text(); got != uistate.T("smartcat.createSelected")+" (2)" {
		t.Fatal(got)
	}
	// Shrinking and growing results must retain each row's independent handler.
	p.State.Suggestions = p.State.Suggestions[:1]
	f.Render(ui.CreateElement(txnSmartCatContent, p))
	byTestID(f, "input", "smartcat-sug-0").Click()
	if toggled != 0 {
		t.Fatal("row handler retained old index")
	}
	p.State.Picked = nil
	p.State.Loading = true
	f.Render(ui.CreateElement(txnSmartCatContent, p))
	if byTestID(f, "button", "smartcat-scan").Attr("disabled") != "disabled" || byTestID(f, "button", "smartcat-apply").Attr("disabled") != "disabled" {
		t.Fatal("disabled attributes missing")
	}
}

func TestCSSMARTReviewResultsChildren(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	review := smartcatview.Reset(smartcatview.Auto)
	renderResults := func(provider bool) {
		f.Render(ui.CreateElement(txnSmartCatResults, txnSmartCatResultsProps{
			State: review, View: smartcatview.PresentReview(review, provider), OnToggle: func(int) {}, OnConnect: func(ui.Event) {},
		}))
	}
	renderResults(false)
	if byTestID(f, "div", "smartcat-no-provider") == nil {
		t.Fatal("provider notice")
	}
	review = review.Start()
	renderResults(true)
	if byTestID(f, "p", "smartcat-loading") == nil {
		t.Fatal("loading")
	}
	review = review.Fail("Scan failed")
	renderResults(true)
	if !strings.Contains(f.Text(), "Scan failed") {
		t.Fatal(f.Text())
	}
	review = smartcatview.Reset(smartcatview.Auto).NoSample(smartcatview.Auto)
	renderResults(true)
	if byTestID(f, "p", "smartcat-empty").Text() != uistate.T("smartcat.noneAssign") {
		t.Fatal(f.Text())
	}
	review = smartcatview.Reset(smartcatview.Suggest).NoSample(smartcatview.Suggest)
	renderResults(true)
	if byTestID(f, "p", "smartcat-empty").Text() != uistate.T("smartcat.noneSuggest") {
		t.Fatal(f.Text())
	}
	review = smartcatview.Reset(smartcatview.Recat).Finish(smartcatview.Recat, nil, []smartcatview.Assignment{
		{TxnID: "one", Label: "Store", Current: "Old", CatName: "Food", Amount: "$2"},
	})
	renderResults(true)
	if byTestID(f, "input", "smartcat-row-0") == nil || !strings.Contains(f.Text(), "Old → Food · $2") {
		t.Fatal(f.Text())
	}
	review = smartcatview.Reset(smartcatview.Suggest)
	renderResults(true)
	if f.Text() != "" {
		t.Fatal("unscanned results should be empty")
	}
}

func TestCSSMARTConnectChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	called := 0
	f.Render(ui.CreateElement(txnSmartCatConnect, txnSmartCatConnectProps{OnConnect: func(ui.Event) { called++ }}))
	link := byTestID(f, "a", "smartcat-connect")
	if link == nil || link.Attr("href") != "/settings/ai" {
		t.Fatal("settings destination")
	}
	link.Click()
	if called != 1 {
		t.Fatal(called)
	}
}

func TestCSSMARTEvidenceChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(catSuggestEvidence, catSuggestEvidenceProps{}))
	if byTestID(f, "p", "catsuggest-none") == nil {
		t.Fatal("empty evidence")
	}
	f.Render(ui.CreateElement(catSuggestEvidence, catSuggestEvidenceProps{Available: true, Name: "Food", Why: "From your history"}))
	if byTestID(f, "div", "catsuggest-suggestion") == nil || !strings.Contains(f.Text(), "From your history") || !strings.Contains(f.Text(), "Food") {
		t.Fatal(f.Text())
	}
}

func TestCSSMARTCategoryAIChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	called := 0
	p := catSuggestAIActionProps{OnAsk: func() { called++ }}
	f.Render(ui.CreateElement(catSuggestAIAction, p))
	if byTestID(f, "button", "catsuggest-ai") != nil {
		t.Fatal("AI opt-in")
	}
	p.Enabled = true
	f.Render(ui.CreateElement(catSuggestAIAction, p))
	button := byTestID(f, "button", "catsuggest-ai")
	if button.Text() != uistate.T("catSuggest.suggestAI") {
		t.Fatal(button.Text())
	}
	button.Click()
	if called != 1 {
		t.Fatal(called)
	}
	p.Loading = true
	f.Render(ui.CreateElement(catSuggestAIAction, p))
	button = byTestID(f, "button", "catsuggest-ai")
	if button.Attr("aria-disabled") != "true" || button.Text() != uistate.T("catSuggest.asking") {
		t.Fatal(button.Text())
	}
}

func TestCSSMARTBatchChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	changed := 0
	p := catSuggestBatchProps{OnChange: func() { changed++ }}
	f.Render(ui.CreateElement(catSuggestBatch, p))
	if byTestID(f, "input", "catsuggest-batch") != nil {
		t.Fatal("empty batch")
	}
	p.Others = 3
	p.Checked = true
	f.Render(ui.CreateElement(catSuggestBatch, p))
	checkbox := byTestID(f, "input", "catsuggest-batch")
	if checkbox == nil || !strings.Contains(f.Text(), uistate.T("catSuggest.batchLabel", 3)) {
		t.Fatal(f.Text())
	}
	checkbox.Dispatch("onchange", render.Event{})
	if changed != 1 {
		t.Fatal(changed)
	}
}

func TestCSSMARTCategoryFormChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	saved, created := 0, 0
	picked := ""
	p := catSuggestFormProps{
		Txn:        domain.Transaction{ID: "one", Payee: "Store", Amount: money.New(-100, "USD")},
		Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}},
		OnSave:     func() { saved++ }, OnNewCategory: func() { created++ }, OnPick: func(id string) { picked = id },
		OnAsk: func() {}, OnBatch: func() {}, OnPickerPick: func(string) {}, OnPickerClose: func() {},
	}
	f.Render(ui.CreateElement(catSuggestForm, p))
	form := f.ByID(CatSuggestFormID)
	if form == nil || form.Attr("data-testid") != "catsuggest-modal" || !strings.Contains(form.Text(), "Store") {
		t.Fatal("form contract")
	}
	form.Dispatch("onsubmit", render.Event{})
	byTestID(f, "select", "catsuggest-pick").Change("food")
	byTestID(f, "button", "catsuggest-new-category").Click()
	if saved != 1 || created != 1 || picked != "food" {
		t.Fatalf("save %d new %d pick %q", saved, created, picked)
	}
}

func TestCSSMARTSentenceInputChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	input := ""
	applied := 0
	f.Render(ui.CreateElement(txnDraftSentenceInput, txnDraftSentenceInputProps{
		Sentence: "initial", OnInput: func(value string) { input = value }, OnApply: func() { applied++ },
	}))
	field := f.ByID("sm15-input")
	if field == nil || field.Attr("aria-label") != uistate.T("sm15.label") {
		t.Fatal("sentence label")
	}
	field.Input("spent 40 at Store")
	field.Dispatch("onkeydown", render.Event{Key: "Escape"})
	if applied != 0 {
		t.Fatal("non-Enter applied")
	}
	field.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if input != "spent 40 at Store" || applied != 1 {
		t.Fatal(input, applied)
	}
}

func TestCSSMARTDraftAIChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	called := 0
	p := txnDraftAIActionProps{LabelKey: "sm15.aiAction", OnAsk: func() { called++ }}
	f.Render(ui.CreateElement(txnDraftAIAction, p))
	if byTestID(f, "button", "sm15-ai-btn") != nil {
		t.Fatal("invisible AI button")
	}
	p.Visible = true
	f.Render(ui.CreateElement(txnDraftAIAction, p))
	byTestID(f, "button", "sm15-ai-btn").Click()
	if called != 1 {
		t.Fatal(called)
	}
	p.Loading = true
	p.LabelKey = "sm15.aiPending"
	f.Render(ui.CreateElement(txnDraftAIAction, p))
	button := byTestID(f, "button", "sm15-ai-btn")
	if button.Attr("aria-disabled") != "true" || button.Text() != uistate.T("sm15.aiPending") {
		t.Fatal(button.Text())
	}
}

func TestCSSMARTDraftOfferChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	called := 0
	settings := smart.Settings{}
	p := txnDraftOfferProps{Settings: settings, OnApply: func() { called++ }}
	f.Render(ui.CreateElement(txnDraftOffer, p))
	if byTestID(f, "div", "smart-rowhint-sm15") != nil {
		t.Fatal("empty offer")
	}
	p.View = smartcatview.PresentDraft("spent 40", TxnDraftFill{AmountMajor: "40", Payee: "Store"}, TxnDraftFill{}, false, false, settings)
	f.Render(ui.CreateElement(txnDraftOffer, p))
	if !strings.Contains(f.Text(), "Store") || !strings.Contains(f.Text(), uistate.T("sm15.readLocal")) {
		t.Fatal(f.Text())
	}
	byTestID(f, "button", "smart-rowhint-act-sm15").Click()
	if called != 1 {
		t.Fatal(called)
	}
}

func TestCSSMARTEmptyScanShellNeverCallsProvider(t *testing.T) {
	app := pickerTestApp(t)
	settings := app.Settings()
	settings.OpenAIKey = "unused-empty-scan-key"
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if len(app.Transactions()) != 0 {
		t.Fatal("empty scan fixture contains transactions")
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TxnSmartCatBody, struct{}{}))
	for _, mode := range []string{"suggest", "auto", "recat", "suggest"} {
		byTestID(f, "button", "smartcat-mode-"+mode).Click()
		f.Stabilize()
		byTestID(f, "button", "smartcat-scan").Click()
		f.Stabilize()
		empty := byTestID(f, "p", "smartcat-empty")
		if empty == nil || byTestID(f, "button", "smartcat-apply") != nil {
			t.Fatal("empty scan offered writes")
		}
		key := "smartcat.noneAssign"
		if mode == "suggest" {
			key = "smartcat.noneSuggest"
		}
		if empty.Text() != uistate.T(key) || byTestID(f, "button", "smartcat-scan").Text() != uistate.T("smartcat.rescan") {
			t.Fatal(f.Text())
		}
	}
}
