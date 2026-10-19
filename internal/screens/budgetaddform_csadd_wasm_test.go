// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgetform"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func csaddView(t *testing.T) budgetAddViewProps {
	t.Helper()
	app := guardTestApp(t)
	d := budgetform.Initial("Transportation", "100", "c-transport", "", "USD")
	noop := func() {}
	text := func(string) {}
	pair := func(string, string) {}
	return budgetAddMessages(budgetAddViewProps{Draft: d, Base: "USD", Categories: app.Categories(), Members: app.Members(),
		Preview: budgetform.Inspect(d, app.Categories(), nil, app.Members(), "", ""),
		Actions: budgetAddActions{Name: text, Limit: text, NewCategory: text, Direction: text, Owner: text, Period: text, Method: text, Tags: text, VarName: text, Category: text, Copy: text,
			Create: noop, Rollover: noop, Advanced: noop, OpenTemplate: noop, CloseTemplate: noop, Cancel: noop, Submit: noop, Apply: noop, Suggest: noop, Also: text, Exclude: text, Custom: pair, Bucket: pair}})
}

func TestCSADDShortcuts(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	opened, copied := 0, ""
	p.Actions.OpenTemplate = func() { opened++ }
	p.Actions.Copy = func(v string) { copied = v }
	for _, withCopy := range []bool{false, true, false, true} {
		p.Budgets = nil
		if withCopy {
			p.Budgets = []domain.Budget{{ID: "copy", Name: "Copy me"}}
		}
		p = budgetAddMessages(p)
		f.Render(ui.CreateElement(budgetAddShortcuts, p))
		byTestID(f, "button", "budgets-template-503020").Click()
		f.Stabilize()
		picker := byTestID(f, "select", "budget-copy-existing")
		if withCopy {
			if picker == nil {
				t.Fatal("missing copy")
			}
			picker.Dispatch("onchange", render.Event{Value: "copy"})
			f.Stabilize()
		} else if picker != nil {
			t.Fatal("unexpected copy")
		}
	}
	if opened != 4 || copied != "copy" {
		t.Fatal(opened, copied)
	}
	p.Template.Err = errors.New("private")
	f.Render(ui.CreateElement(budgetAddShortcuts, p))
	if byTestID(f, "p", "budget-template-income-incomplete") == nil || strings.Contains(f.Text(), "private") {
		t.Fatal(f.Text())
	}
}
func TestCSADDIdentity(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	name, direction := "", ""
	p.Actions.Name = func(v string) { name = v }
	p.Actions.Direction = func(v string) { direction = v }
	for _, saving := range []bool{false, true, false} {
		p.Preview.Saving = saving
		f.Render(ui.CreateElement(budgetAddIdentity, p))
		f.InputByID("budget-add", "New name")
		byTestID(f, "select", "budget-direction").Dispatch("onchange", render.Event{Value: "save"})
		f.Stabilize()
		if name != "New name" || direction != "save" {
			t.Fatal(name, direction)
		}
		if (byTestID(f, "p", "budget-direction-hint") != nil) != saving {
			t.Fatal("direction hint", saving)
		}
		if f.ByID("budget-add").Attr("aria-required") != "true" {
			t.Fatal("name required semantics")
		}
	}
}
func TestCSADDLimit(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	p.Actions.Limit = func(v string) { got = v }
	for _, msg := range []string{"", "Bad amount", ""} {
		p.LimitError = msg
		f.Render(ui.CreateElement(budgetAddLimit, p))
		input := byTestID(f, "input", "budget-add-limit")
		input.Input("42.00")
		f.Stabilize()
		if got != "42.00" || input.Attr("aria-required") != "true" {
			t.Fatal(got)
		}
		if (input.Attr("aria-invalid") == "true") != (msg != "") {
			t.Fatal(input.Attr("aria-invalid"), msg)
		}
	}
}
func TestCSADDCategory(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	toggled := 0
	typed := ""
	p.Actions.Create = func() { toggled++ }
	p.Actions.NewCategory = func(v string) { typed = v }
	for _, create := range []bool{false, true, false, true} {
		p.Draft.CreateCategory = create
		p.Preview = budgetform.Inspect(p.Draft, p.Categories, nil, nil, "", "")
		p = budgetAddMessages(p)
		f.Render(ui.CreateElement(budgetAddCategory, p))
		byTestID(f, "input", "budget-create-cat").Dispatch("onchange", render.Event{})
		f.Stabilize()
		if create {
			byTestID(f, "input", "budget-new-cat-name").Input("Pets")
			f.Stabilize()
			if typed != "Pets" {
				t.Fatal(typed)
			}
		} else {
			picker := byTestID(f, "button", "budget-existing-cat")
			if picker == nil || picker.Text() != "Transportation" {
				t.Fatal(f.Text())
			}
		}
	}
	if toggled != 4 {
		t.Fatal(toggled)
	}
	p.Categories = nil
	p.Preview = budgetform.Inspect(p.Draft, nil, nil, nil, "", "")
	p = budgetAddMessages(p)
	f.Render(ui.CreateElement(budgetAddCategory, p))
	if !isDisabled(byTestID(f, "input", "budget-create-cat")) || byTestID(f, "span", "budget-create-cat-forced") == nil {
		t.Fatal(f.Text())
	}
}

func TestCSADDExistingCategoryPreview(t *testing.T) {
	p := csaddView(t)
	p.Draft.CreateCategory = true
	p.Draft.NewCategoryName = "transportation"
	p.Preview = budgetform.Inspect(p.Draft, p.Categories, nil, p.Members, "", "")
	p = budgetAddMessages(p)

	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetAddCategory, p))

	fate := byTestID(f, "span", "budget-cat-fate")
	if fate == nil || !strings.Contains(strings.ToLower(fate.Text()), "existing") || strings.Contains(strings.ToLower(fate.Text()), "create") {
		t.Fatalf("existing category preview = %q", f.Text())
	}
}

func TestCSADDSuggestion(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	used := 0
	p.Actions.Suggest = func() { used++ }
	for _, amount := range []int64{0, 12345, 0, 200} {
		p.Suggestion.Amount = money.New(amount, "USD")
		p.SuggestScope = ""
		if amount > 0 {
			p.SuggestScope = "Transportation and children"
		}
		f.Render(ui.CreateElement(budgetAddSuggestion, p))
		button := byTestID(f, "button", "budget-use-suggest")
		if amount > 0 {
			if button == nil || byTestID(f, "span", "budget-suggest-scope") == nil {
				t.Fatal(f.Text())
			}
			button.Click()
			f.Stabilize()
		} else if button != nil {
			t.Fatal("unexpected suggestion")
		}
	}
	if used != 2 {
		t.Fatal(used)
	}
}
func TestCSADDSuggestionNote(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	p.Suggestion.Err = errors.New("private")
	f.Render(ui.CreateElement(budgetAddSuggestionNote, p))
	if !strings.Contains(f.Text(), uistate.T("budgets.incomplete")) || strings.Contains(f.Text(), "private") {
		t.Fatal(f.Text())
	}
	p.Preview.WantNew = true
	p.Preview.TypedCategory = "Pets"
	f.Render(ui.CreateElement(budgetAddSuggestionNote, p))
	if byTestID(f, "span", "budget-suggest-none") == nil || strings.Contains(f.Text(), uistate.T("budgets.incomplete")) {
		t.Fatal(f.Text())
	}
	p.Preview.WantNew = false
	p.Suggestion.Err = nil
	f.Render(ui.CreateElement(budgetAddSuggestionNote, p))
	if f.Text() != "" {
		t.Fatal(f.Text())
	}
}
func TestCSADDAdvanced(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	toggles := 0
	p.Actions.Advanced = func() { toggles++ }
	for _, open := range []bool{false, true, false, true} {
		p.Draft.Advanced = open
		p.Preview.AdvancedKey = "budgets.advancedShow"
		if open {
			p.Preview.AdvancedKey = "budgets.advancedHide"
		}
		f.Render(ui.CreateElement(budgetAddAdvanced, p))
		button := byTestID(f, "button", "budget-add-advanced")
		if button.Attr("aria-expanded") != ariaBool(open) {
			t.Fatal(button.Attr("aria-expanded"))
		}
		button.Click()
		f.Stabilize()
		if (byTestID(f, "div", "budget-add-group-tracking") != nil) != open {
			t.Fatal("advanced visibility")
		}
	}
	if toggles != 4 {
		t.Fatal(toggles)
	}
}
func TestCSADDTracking(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	p.Actions.Tags = func(v string) { got = v }
	for _, count := range []int{0, 2, 0} {
		p.Preview.ExpenseCount = count
		f.Render(ui.CreateElement(budgetAddTracking, p))
		byTestID(f, "input", "budget-add-tags").Input("#vacation,food")
		f.Stabilize()
		if got != "#vacation,food" || byTestID(f, "div", "budget-add-group-tracking") == nil {
			t.Fatal(f.Text())
		}
	}
}
func TestCSADDBehaviour(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	rolled := 0
	p.Actions.Rollover = func() { rolled++ }
	for _, members := range [][]domain.Member{nil, {{ID: "m", Name: "Maya"}}, nil} {
		p.Members = members
		p.Preview = budgetform.Inspect(p.Draft, p.Categories, nil, members, "", "")
		p = budgetAddMessages(p)
		f.Render(ui.CreateElement(budgetAddBehaviour, p))
		inputs := f.AllByTag("input")
		if len(inputs) != 1 {
			t.Fatal(len(inputs))
		}
		inputs[0].Dispatch("onchange", render.Event{})
		f.Stabilize()
		if (byTestID(f, "span", "budget-owner-hint") != nil) != (len(members) > 0) {
			t.Fatal(f.Text())
		}
	}
	if rolled != 3 || byTestID(f, "span", "budget-add-method-hint") == nil {
		t.Fatal(rolled, f.Text())
	}
}
func TestCSADDFormula(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	p.Actions.VarName = func(v string) { got = v }
	f.Render(ui.CreateElement(budgetAddFormula, p))
	f.InputByID("budget-add-varname", "my_handle")
	f.Stabilize()
	if got != "my_handle" || byTestID(f, "div", "budget-add-group-formula") == nil {
		t.Fatal(got, f.Text())
	}
}
func TestCSADDFooter(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	cancelled := 0
	p.Actions.Cancel = func() { cancelled++ }
	for _, blocked := range []bool{true, false, true} {
		p.Preview.Blocked = blocked
		p.Blocker = ""
		if blocked {
			p.Blocker = "Missing fields"
		}
		f.Render(ui.CreateElement(budgetAddFooter, p))
		if isDisabled(byTestID(f, "button", "budget-add-submit")) != blocked {
			t.Fatal(blocked)
		}
		if (byTestID(f, "span", "budget-add-blockers") != nil) != blocked {
			t.Fatal(f.Text())
		}
		f.AllByTag("button")[0].Click()
		f.Stabilize()
	}
	if cancelled != 3 {
		t.Fatal(cancelled)
	}
}
func TestCSADDTemplateReviewAndRow(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	back, applied, toggled, bucket := 0, 0, "", ""
	p.Actions.CloseTemplate = func() { back++ }
	p.Actions.Apply = func() { applied++ }
	p.Actions.Exclude = func(id string) { toggled = id }
	p.Actions.Bucket = func(id, v string) { bucket = id + ":" + v }
	for _, count := range []int{0, 1, 2, 0, 1} {
		p.Template.Proposals = nil
		for i := 0; i < count; i++ {
			id := string(rune('a' + i))
			p.Template.Proposals = append(p.Template.Proposals, budgeting.BudgetProposal{Category: domain.Category{ID: id, Name: id}, LimitMinor: 100, Bucket: budgeting.BucketNeeds})
		}
		f.Render(ui.CreateElement(budgetAddTemplateReview, p))
		if len(f.AllByTag("input")) != count || (byTestID(f, "p", "budget-tmpl-total") != nil) != (count > 0) {
			t.Fatalf("count=%d text=%q", count, f.Text())
		}
		byTestID(f, "button", "budget-tmpl-back").Click()
		byTestID(f, "form", "budget-tmpl-review").Dispatch("onsubmit", render.Event{})
		if count > 0 {
			byTestID(f, "input", "tmpl-pick-a").Dispatch("onchange", render.Event{})
			byTestID(f, "select", "tmpl-bucket-a").Dispatch("onchange", render.Event{Value: "2"})
		}
		f.Stabilize()
	}
	if back != 5 || applied != 5 || toggled != "a" || bucket != "a:2" {
		t.Fatal(back, applied, toggled, bucket)
	}
	p.Draft.Excluded = map[string]bool{"a": true}
	f.Render(ui.CreateElement(budgetAddTemplateReview, p))
	if byTestID(f, "p", "budget-tmpl-total") != nil {
		t.Fatal(f.Text())
	}
}
func TestCSADDContent(t *testing.T) {
	p := csaddView(t)
	f := render.New(t)
	defer f.Cleanup()
	submitted := 0
	p.Actions.Submit = func() { submitted++ }
	f.Render(ui.CreateElement(budgetAddContent, p))
	form := byTestID(f, "form", "budget-add-form")
	if form == nil {
		t.Fatal("missing form")
	}
	form.Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if submitted != 1 {
		t.Fatal(submitted)
	}
	for _, id := range []string{"budget-add-name-field", "budget-add-limit-field", "budget-add-category-field", "budget-add-tmpl"} {
		if byTestID(f, "div", id) == nil {
			t.Fatal(id, f.Text())
		}
	}
}

func TestCSADDFormSubmitsAndResetsThroughChildren(t *testing.T) {
	app := guardTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	done := 0
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{OnDone: func() { done++ }, Seed: uistate.BudgetAddSeed{Name: "Transportation", CategoryID: "c-transport", LimitMajor: "125.50"}}))
	byTestID(f, "form", "budget-add-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	budgets := app.Budgets()
	if len(budgets) != 1 || budgets[0].Limit.Amount != 12550 || done != 1 {
		t.Fatal(budgets, done)
	}
	if got := f.ByID("budget-add").Attr("data-cf-value"); got != "" {
		t.Fatal(got)
	}
	if byTestID(f, "span", "budget-add-blockers") == nil {
		t.Fatal("reset form has no blocker")
	}
}

func TestCSADDStorageFailureUsesCatalogAndKeepsFormOpen(t *testing.T) {
	app := guardTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	done := 0
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{OnDone: func() { done++ }, Seed: uistate.BudgetAddSeed{Name: "Transportation", CategoryID: "c-transport", LimitMajor: "125.50"}}))
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	byTestID(f, "form", "budget-add-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	message := f.ByID("budget-err")
	if message == nil || message.Text() != uistate.T("budgets.writeFailed") || done != 0 {
		t.Fatal(message, done, f.Text())
	}
}

// The mock does not synchronize reflected boolean attributes when they are
// removed and re-added. Verify the button contract on fresh fixtures; the row
// test above independently exercises cardinality and selection on one mount.
func TestCSADDTemplateReviewDisabledState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rows     int
		excluded bool
		disabled bool
	}{
		{"empty", 0, false, true}, {"selected", 1, false, false}, {"excluded", 1, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := csaddView(t)
			f := render.New(t)
			defer f.Cleanup()
			if tc.rows > 0 {
				p.Template.Proposals = []budgeting.BudgetProposal{{Category: domain.Category{ID: "a", Name: "A"}, LimitMinor: 100}}
			}
			p.Draft.Excluded = map[string]bool{"a": tc.excluded}
			f.Render(ui.CreateElement(budgetAddTemplateReview, p))
			if got := isDisabled(byTestID(f, "button", "budget-tmpl-apply")); got != tc.disabled {
				t.Fatalf("disabled=%v want=%v", got, tc.disabled)
			}
		})
	}
}

func TestCSADDFormPreservesDraftAcrossTemplateReview(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{}))
	f.InputByID("budget-add", "Trip")
	f.Stabilize()
	byTestID(f, "button", "budgets-template-503020").Click()
	f.Stabilize()
	if byTestID(f, "form", "budget-tmpl-review") == nil {
		t.Fatal("template did not open")
	}
	byTestID(f, "button", "budget-tmpl-back").Click()
	f.Stabilize()
	if byTestID(f, "form", "budget-add-form") == nil || f.ByID("budget-add").Attr("data-cf-value") != "Trip" {
		t.Fatal("draft was lost", f.Text())
	}
}
