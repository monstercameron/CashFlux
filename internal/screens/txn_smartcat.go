// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// The three Smart+ categorization modes the review modal drives.
const (
	smartCatSuggest = "suggest" // SMART-T15: propose NEW categories to create
	smartCatAuto    = "auto"    // SMART-T16: assign uncategorized txns to existing cats
	smartCatRecat   = "recat"   // SMART-T17: flag mis-categorized txns and fix them
)

// smartCatScanCap bounds how many transactions each scan sends to the model, so
// one click can't ship a huge context (cost) or a long review list.
const smartCatScanCap = 40

// txnSmartCatAssign is one reviewable "assign this transaction to this category"
// row for the Auto / Recat modes.
type txnSmartCatAssign struct {
	TxnID   string
	Label   string // "payee — desc"
	Amount  string
	Current string // current category name (Recat only)
	CatID   string
	CatName string
}

// TxnSmartCatBody is the body of the Smart+ categorization review flip modal
// (mounted at the shell root by app.TxnSmartCatHost). It offers three AI scans —
// suggest new categories, auto-categorize the uncategorized, and review likely
// mis-categorizations — each producing a checklist the user confirms before
// anything is written. The scan is the consent step: nothing leaves the device
// until the user picks a mode and clicks Scan.
func TxnSmartCatBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseTxnSmartCatOpen()
	undoAtom := uistate.UseTxnUndo()
	pr := uistate.UsePrefs().Get()

	mode := ui.UseState(smartCatSuggest)
	loading := ui.UseState(false)
	scanned := ui.UseState(false)
	errText := ui.UseState("")
	catSugs := ui.UseState([]smartai.SuggestedCategory(nil))
	assigns := ui.UseState([]txnSmartCatAssign(nil))
	picked := ui.UseState(map[int]bool{})

	backendAI := pr.Normalize().BackendActive()
	hasProvider := app != nil && aiProviderConfigured(app, backendAI)
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	catIDByName := make(map[string]string, len(cats))
	existingLower := make(map[string]bool, len(cats))
	var catList strings.Builder
	for _, c := range cats {
		catName[c.ID] = c.Name
		catIDByName[c.Name] = c.ID
		existingLower[strings.ToLower(c.Name)] = true
		catList.WriteString(c.Name + "\n")
	}

	reset := func() {
		scanned.Set(false)
		errText.Set("")
		catSugs.Set(nil)
		assigns.Set(nil)
		picked.Set(map[int]bool{})
	}
	setMode := func(m string) { mode.Set(m); loading.Set(false); reset() }
	toggle := func(i int) {
		m := picked.Get()
		nm := make(map[int]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[i] = !nm[i]
		picked.Set(nm)
	}
	allPicked := func(n int) map[int]bool {
		m := make(map[int]bool, n)
		for i := 0; i < n; i++ {
			m[i] = true
		}
		return m
	}

	// runScan builds the mode's context at click time and places the AI call. Each
	// result parses against the REAL category list so the model can't invent one.
	runScan := func() {
		if app == nil {
			return
		}
		loading.Set(true)
		errText.Set("")
		scanned.Set(false)

		if mode.Get() == smartCatSuggest {
			var lines strings.Builder
			n := 0
			for _, t := range app.Transactions() {
				if t.IsTransfer() || t.CategoryID != "" {
					continue
				}
				lines.WriteString("- " + strings.TrimSpace(t.Payee+" — "+t.Desc) + " | " + fmtMoney(t.Amount) + "\n")
				if n++; n >= smartCatScanCap {
					break
				}
			}
			if n == 0 {
				catSugs.Set(nil)
				scanned.Set(true)
				loading.Set(false)
				return
			}
			runSmartAI(aiConn, smartai.SuggestCategories(lines.String(), catList.String()),
				func(text string) {
					s := smartai.ParseCategorySuggestions(text, existingLower)
					catSugs.Set(s)
					picked.Set(allPicked(len(s)))
					scanned.Set(true)
					loading.Set(false)
				},
				func(e string) { errText.Set(e); scanned.Set(true); loading.Set(false) })
			return
		}

		// Auto / Recat both produce numbered "N => Category" assignments; they differ
		// in which transactions they scan and whether the current category is shown.
		wantCategorized := mode.Get() == smartCatRecat
		var sample []domain.Transaction
		var lines strings.Builder
		for _, t := range app.Transactions() {
			if t.IsTransfer() {
				continue
			}
			has := t.CategoryID != ""
			if has != wantCategorized {
				continue
			}
			sample = append(sample, t)
			ref := strconv.Itoa(len(sample))
			line := ref + " | " + strings.TrimSpace(t.Payee+" — "+t.Desc) + " | " + fmtMoney(t.Amount)
			if wantCategorized {
				cur := catName[t.CategoryID]
				if cur == "" {
					cur = "(uncategorized)"
				}
				line += " | currently: " + cur
			}
			lines.WriteString(line + "\n")
			if len(sample) >= smartCatScanCap {
				break
			}
		}
		if len(sample) == 0 {
			assigns.Set(nil)
			scanned.Set(true)
			loading.Set(false)
			return
		}
		req := smartai.AutoCategorize(lines.String(), catList.String())
		if wantCategorized {
			req = smartai.Recategorize(lines.String(), catList.String())
		}
		runSmartAI(aiConn, req,
			func(text string) {
				parsed := smartai.ParseCategoryAssignments(text, len(sample), catIDByName)
				var rows []txnSmartCatAssign
				for _, a := range parsed {
					t := sample[a.Ref-1]
					if t.CategoryID == a.CategoryID {
						continue // no-op (recat suggested the same category)
					}
					rows = append(rows, txnSmartCatAssign{
						TxnID: t.ID, Label: strings.TrimSpace(t.Payee + " — " + t.Desc), Amount: fmtMoney(t.Amount),
						Current: catName[t.CategoryID], CatID: a.CategoryID, CatName: a.CategoryName,
					})
				}
				assigns.Set(rows)
				picked.Set(allPicked(len(rows)))
				scanned.Set(true)
				loading.Set(false)
			},
			func(e string) { errText.Set(e); scanned.Set(true); loading.Set(false) })
	}
	onScan := ui.UseEvent(Prevent(runScan))

	// applyResults writes the checked suggestions/assignments and closes the modal.
	applyResults := func() {
		sel := picked.Get()
		if mode.Get() == smartCatSuggest {
			created := 0
			for i, s := range catSugs.Get() {
				if !sel[i] {
					continue
				}
				c := domain.Category{ID: id.New(), Name: s.Name, Kind: domain.CategoryKind(s.Kind)}
				if err := app.PutCategory(c); err != nil {
					uistate.PostNotice(err.Error(), true)
					continue
				}
				created++
			}
			if created > 0 {
				uistate.PostNotice(uistate.T("smartcat.createdToast", plural(created, "category")), false)
			}
			uistate.BumpDataRevision()
			openAtom.Set(false)
			return
		}
		rows := assigns.Get()
		txByID := make(map[string]domain.Transaction, len(app.Transactions()))
		for _, t := range app.Transactions() {
			txByID[t.ID] = t
		}
		var prior []domain.Transaction
		applied := 0
		for i, r := range rows {
			if !sel[i] {
				continue
			}
			t, ok := txByID[r.TxnID]
			if !ok || t.CategoryID == r.CatID {
				continue
			}
			prior = append(prior, t)
			t.CategoryID = r.CatID
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(err.Error(), true)
				continue
			}
			applied++
		}
		if applied > 0 {
			undoAtom.Set(uistate.BulkSnapshot{Label: uistate.T("smartcat.appliedLabel", applied), Prior: prior})
			uistate.PostNotice(uistate.T("smartcat.appliedToast", plural(applied, "transaction")), false)
		}
		uistate.BumpDataRevision()
		openAtom.Set(false)
	}
	onApply := ui.UseEvent(Prevent(applyResults))

	// ---- render ----
	seg := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("smartcat.modeLabel"),
		Selected: mode.Get(),
		Options: []uiw.SegOption{
			{Value: smartCatSuggest, Label: uistate.T("smartcat.modeSuggest"), TestID: "smartcat-mode-suggest"},
			{Value: smartCatAuto, Label: uistate.T("smartcat.modeAuto"), TestID: "smartcat-mode-auto"},
			{Value: smartCatRecat, Label: uistate.T("smartcat.modeRecat"), TestID: "smartcat-mode-recat"},
		},
		OnSelect: setMode,
	})

	hint := uistate.T("smartcat.hintSuggest")
	switch mode.Get() {
	case smartCatAuto:
		hint = uistate.T("smartcat.hintAuto")
	case smartCatRecat:
		hint = uistate.T("smartcat.hintRecat")
	}

	var results ui.Node = Fragment()
	sel := picked.Get()
	nSelected := 0
	for _, v := range sel {
		if v {
			nSelected++
		}
	}
	switch {
	case !hasProvider:
		results = P(css.Class("notice"), uistate.T("smart.aiNeedsProvider"))
	case loading.Get():
		results = P(css.Class("muted"), Attr("data-testid", "smartcat-loading"), uistate.T("smartcat.scanning"))
	case errText.Get() != "":
		results = P(css.Class("err"), Attr("role", "alert"), errText.Get())
	case scanned.Get() && mode.Get() == smartCatSuggest && len(catSugs.Get()) == 0:
		results = P(css.Class("muted"), Attr("data-testid", "smartcat-empty"), uistate.T("smartcat.noneSuggest"))
	case scanned.Get() && mode.Get() != smartCatSuggest && len(assigns.Get()) == 0:
		results = P(css.Class("muted"), Attr("data-testid", "smartcat-empty"), uistate.T("smartcat.noneAssign"))
	case scanned.Get() && mode.Get() == smartCatSuggest:
		keyOf := func(s smartai.SuggestedCategory) any { return s.Name }
		idx := map[string]int{}
		for i, s := range catSugs.Get() {
			idx[s.Name] = i
		}
		results = Div(css.Class("rows"), Attr("data-testid", "smartcat-results"),
			MapKeyed(catSugs.Get(), keyOf, func(s smartai.SuggestedCategory) ui.Node {
				i := idx[s.Name]
				kindKey := "smartcat.kindExpense"
				if s.Kind == "income" {
					kindKey = "smartcat.kindIncome"
				}
				return ui.CreateElement(txnSmartCatRow, txnSmartCatRowProps{
					Label: s.Name, Sub: uistate.T(kindKey), Checked: sel[i],
					TestID: "smartcat-sug-" + strconv.Itoa(i), OnToggle: func() { toggle(i) },
				})
			}))
	case scanned.Get():
		keyOf := func(r txnSmartCatAssign) any { return r.TxnID }
		idx := map[string]int{}
		for i, r := range assigns.Get() {
			idx[r.TxnID] = i
		}
		results = Div(css.Class("rows"), Attr("data-testid", "smartcat-results"),
			MapKeyed(assigns.Get(), keyOf, func(r txnSmartCatAssign) ui.Node {
				i := idx[r.TxnID]
				sub := "→ " + r.CatName + " · " + r.Amount
				if mode.Get() == smartCatRecat && r.Current != "" {
					sub = r.Current + " → " + r.CatName + " · " + r.Amount
				}
				return ui.CreateElement(txnSmartCatRow, txnSmartCatRowProps{
					Label: r.Label, Sub: sub, Checked: sel[i],
					TestID: "smartcat-row-" + strconv.Itoa(i), OnToggle: func() { toggle(i) },
				})
			}))
	}

	scanLabel := uistate.T("smartcat.scan")
	if scanned.Get() {
		scanLabel = uistate.T("smartcat.rescan")
	}

	applyLabel := uistate.T("smartcat.createSelected")
	if mode.Get() != smartCatSuggest {
		applyLabel = uistate.T("smartcat.applySelected")
	}
	showApply := scanned.Get() && ((mode.Get() == smartCatSuggest && len(catSugs.Get()) > 0) || (mode.Get() != smartCatSuggest && len(assigns.Get()) > 0))

	// The disabled attribute is present-or-absent (an empty value still disables in
	// HTML), so append it only when the button should actually be disabled.
	scanArgs := []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "smartcat-scan"), OnClick(onScan)}
	if loading.Get() {
		scanArgs = append(scanArgs, Attr("disabled", "disabled"))
	}
	scanArgs = append(scanArgs, scanLabel)

	applyArgs := []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "smartcat-apply"), OnClick(onApply)}
	if nSelected == 0 {
		applyArgs = append(applyArgs, Attr("disabled", "disabled"))
	}
	applyArgs = append(applyArgs, applyLabel+" ("+strconv.Itoa(nSelected)+")")

	return Div(css.Class(tw.FlexCol, tw.Gap3),
		seg,
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), hint),
		If(hasProvider, Button(scanArgs...)),
		results,
		If(showApply, Button(applyArgs...)),
	)
}

// txnSmartCatRowProps drives one reviewable checklist row in the Smart+
// categorization modal.
type txnSmartCatRowProps struct {
	Label    string
	Sub      string
	Checked  bool
	TestID   string
	OnToggle func()
}

// txnSmartCatRow is one checklist row (its own component so its OnClick hook is
// never registered inside the results loop).
func txnSmartCatRow(props txnSmartCatRowProps) ui.Node {
	onToggle := ui.UseEvent(func() { props.OnToggle() })
	return Label(css.Class("row", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
		// Branded `.cf-check` (not a bare native checkbox): a native <input type=checkbox>
		// renders inconsistently across OS/browser/theme — on some laptops it's a tiny,
		// near-invisible dark-on-dark box — so the app styles its own. Match the rest of
		// the app's checklists (e.g. the cover-source picker).
		Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", props.TestID), CheckedIf(props.Checked), OnClick(onToggle)),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Label),
			Span(css.Class("row-meta"), props.Sub),
		),
	)
}
