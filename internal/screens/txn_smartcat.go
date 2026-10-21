// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

const (
	smartCatSuggest = smartcatview.Suggest
	smartCatAuto    = smartcatview.Auto
	smartCatRecat   = smartcatview.Recat
	smartCatScanCap = smartcatview.ScanCap
)

// TxnSmartCatBody hosts consent, scan orchestration and review state.
func TxnSmartCatBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	open := uistate.UseTxnSmartCatOpen()
	pr := uistate.UsePrefs().Get()
	review := ui.UseState(smartcatview.Reset(smartCatSuggest))
	backend := uistate.BackendConnectionActive(pr)
	provider := app != nil && aiProviderConfigured(app, backend)
	conn := resolveAIConn(app, backend, pr.ServerURL, pr.ServerToken)
	cats := app.Categories()
	scan := func() {
		if app == nil {
			return
		}
		review.Set(review.Get().Start())
		mode := review.Get().Mode
		request := smartcatview.BuildScan(mode, app.Transactions(), cats)
		if len(request.Sample) == 0 {
			review.Set(review.Get().NoSample(mode))
			return
		}
		runSmartAI(conn, request.Request, func(text string) {
			if mode == smartCatSuggest {
				review.Set(review.Get().Finish(mode, smartcatview.ParseSuggestions(text, cats), nil))
			} else {
				review.Set(review.Get().Finish(mode, nil, smartcatview.ParseAssignments(text, request.Sample, cats)))
			}
		}, func(message string) { review.Set(review.Get().Fail(message)) })
	}
	apply := func() {
		s := review.Get()
		if s.Mode == smartCatSuggest {
			applySmartCatCategories(app, s, cats, func() { open.Set(false) })
			return
		}
		writes, ids := smartcatview.AssignmentWrites(s.Assignments, s.Picked, app.TransactionByID)
		if len(writes) == 0 {
			return
		}
		_ = CommitUndoable(app, uistate.T("smartcat.appliedLabel", len(writes)), ids, func() error { return app.PutTransactionsAtomic(writes) }, func(err error) {
			if err != nil {
				uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
				return
			}
			open.Set(false)
		})
	}
	return ui.CreateElement(txnSmartCatContent, txnSmartCatContentProps{
		State: review.Get(), Provider: provider,
		OnMode:   func(mode string) { review.Set(smartcatview.Reset(mode)) },
		OnToggle: func(i int) { review.Set(review.Get().Toggle(i)) },
		OnScan:   scan, OnApply: apply, OnConnect: func(e ui.Event) {
			if isPlainLeftClick(e) {
				open.Set(false)
			}
		},
	})
}

// applySmartCatCategories persists each selected category using the existing write path.
func applySmartCatCategories(app *appstate.App, s smartcatview.State, cats []domain.Category, close func()) {
	created := 0
	for _, c := range smartcatview.CategoryDrafts(s.Suggestions, s.Picked, cats) {
		c.ID = id.New()
		if err := app.PutCategory(c); err != nil {
			uistate.PostNotice(err.Error(), true)
			continue
		}
		created++
	}
	if created > 0 {
		uistate.PostNotice(uistate.T("smartcat.createdToast", textutil.Plural(created, "category")), false)
	}
	uistate.BumpDataRevision()
	close()
}

type txnSmartCatContentProps struct {
	State           smartcatview.State
	Provider        bool
	OnMode          func(string)
	OnToggle        func(int)
	OnScan, OnApply func()
	OnConnect       func(ui.Event)
}

func txnSmartCatContent(p txnSmartCatContentProps) ui.Node {
	view := smartcatview.PresentReview(p.State, p.Provider)
	return Div(css.Class(tw.FlexCol, tw.Gap3),
		uiw.Segmented(uiw.SegmentedProps{
			Label: uistate.T("smartcat.modeLabel"), Selected: p.State.Mode,
			Options: []uiw.SegOption{
				{Value: smartCatSuggest, Label: uistate.T("smartcat.modeSuggest"), TestID: "smartcat-mode-suggest"},
				{Value: smartCatAuto, Label: uistate.T("smartcat.modeAuto"), TestID: "smartcat-mode-auto"},
				{Value: smartCatRecat, Label: uistate.T("smartcat.modeRecat"), TestID: "smartcat-mode-recat"},
			}, OnSelect: p.OnMode,
		}),
		P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), uistate.T(view.HintKey)),
		If(p.Provider, ui.CreateElement(txnSmartCatButton, txnSmartCatButtonProps{
			TestID: "smartcat-scan", Label: uistate.T(view.ScanKey), Disabled: p.State.Loading, OnClick: p.OnScan,
		})),
		ui.CreateElement(txnSmartCatResults, txnSmartCatResultsProps{State: p.State, View: view, OnToggle: p.OnToggle, OnConnect: p.OnConnect}),
		If(view.ShowApply, ui.CreateElement(txnSmartCatButton, txnSmartCatButtonProps{
			TestID: "smartcat-apply", Label: uistate.T(view.ApplyKey) + view.ApplySuffix, Disabled: view.ApplyDisabled, OnClick: p.OnApply,
		})),
	)
}

type txnSmartCatButtonProps struct {
	TestID, Label string
	Disabled      bool
	OnClick       func()
}

func txnSmartCatButton(p txnSmartCatButtonProps) ui.Node {
	click := ui.UseEvent(Prevent(p.OnClick))
	args := []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", p.TestID), OnClick(click)}
	if p.Disabled {
		args = append(args, Attr("disabled", "disabled"))
	}
	return Button(append(args, p.Label)...)
}

type txnSmartCatResultsProps struct {
	State     smartcatview.State
	View      smartcatview.ReviewView
	OnToggle  func(int)
	OnConnect func(ui.Event)
}

func txnSmartCatResults(p txnSmartCatResultsProps) ui.Node {
	switch p.View.Result {
	case "provider":
		return Div(css.Class("smartcat-noprovider"), Attr("data-testid", "smartcat-no-provider"),
			P(css.Class("smartcat-noprovider-text"), uistate.T("smart.aiNeedsProvider")), ui.CreateElement(txnSmartCatConnect, txnSmartCatConnectProps{OnConnect: p.OnConnect}))
	case "loading":
		return P(css.Class("muted"), Attr("data-testid", "smartcat-loading"), uistate.T("smartcat.scanning"))
	case "error":
		return P(css.Class("err"), Attr("role", "alert"), p.State.Error)
	case "empty":
		return P(css.Class("muted"), Attr("data-testid", "smartcat-empty"), uistate.T(p.View.EmptyKey))
	case "rows":
		return Div(css.Class("rows"), Attr("data-testid", "smartcat-results"),
			MapKeyed(p.View.Rows, func(r smartcatview.ReviewRow) any { return r.Key }, func(r smartcatview.ReviewRow) ui.Node {
				sub := r.Sub
				if r.KindKey != "" {
					sub = uistate.T(r.KindKey)
				}
				return ui.CreateElement(txnSmartCatRow, txnSmartCatRowProps{
					Label: r.Label, Sub: sub, Checked: r.Checked, TestID: r.TestID, OnToggle: func() { p.OnToggle(r.Index) },
				})
			}))
	}
	return Fragment()
}

type txnSmartCatConnectProps struct{ OnConnect func(ui.Event) }

func txnSmartCatConnect(p txnSmartCatConnectProps) ui.Node {
	connect := ui.UseEvent(p.OnConnect)
	return smartCatConnectLink(connect)
}

// smartCatConnectLink is the route destination for adding an AI provider.
func smartCatConnectLink(onActivate any) ui.Node {
	click := OnClick(onActivate)
	args := []any{css.Class("btn btn-primary btn-sm"),
		Attr("data-testid", "smartcat-connect"),
		Attr("href", uistate.RoutePath("/settings/ai"))}
	if onActivate != nil {
		args = append(args, click)
	}
	args = append(args, uistate.T("review.scanConnect"))
	return A(args...)
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
