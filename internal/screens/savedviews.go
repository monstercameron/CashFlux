// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/savedtxnview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// savedViewNativeID is the render-registry key every pinned saved-view dashboard
// tile shares; the specific view is carried in the spec's Settings (viewId), so
// one renderer serves every pinned view (a declarative widget whose source is the
// saved-view id).
const savedViewNativeID = "savedview"

// savedViewSettingsKey names the widget-setting that stores which saved view a
// pinned tile renders.
const savedViewSettingsKey = "viewId"

func init() {
	widgetrender.Register(savedViewNativeID, func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(savedViewPinnedTile, savedViewTileProps{
			App: c.App, Txns: c.Txns, Rates: c.Rates, Base: c.Base,
			ViewID: c.Spec.Settings[savedViewSettingsKey], SpecID: c.Spec.ID,
			Preview: c.Preview,
		})
	})
}

// baseAmountFunc returns an amount function that converts each transaction to the
// base currency, so a view's total sums correctly across currencies.
func baseAmountFunc(rates currency.Rates, base string) savedtxnview.AmountFunc {
	return func(t domain.Transaction) int64 {
		if c, err := rates.Convert(t.Amount, base); err == nil {
			return c.Amount
		}
		return t.Amount.Amount
	}
}

// txnSavedViewsMenuProps configures the toolbar "Views" affordance.
type txnSavedViewsMenuProps struct {
	App    *appstate.App
	Filter txnfilter.Criteria // current live ledger filter (for "Save current view…")
	Rates  currency.Rates
	Base   string
}

// TxnSavedViewsMenu renders the transactions toolbar's "Views" popover (TX3): a
// btn-tool that opens a list of saved views (each with its live matching count +
// total and a one-tap Apply) plus a "Save current view…" form. It is its own
// component so its popover hooks stay at stable positions and its variable-length
// list of rows is delegated to per-row components (never On* in a loop).
func TxnSavedViewsMenu(props txnSavedViewsMenuProps) ui.Node {
	openAtom := uistate.UseSavedViewsOpen()
	formAtom := uistate.UseSaveViewFormOpen()
	_ = uistate.UseDataRevision().Get()
	open := openAtom.Get()

	uiw.DismissPopover(open, "txn-saved-views", func() { openAtom.Set(false); formAtom.Set(false) })
	uiw.AnchorPopover(open, "txn-saved-views")

	toggle := ui.UseEvent(Prevent(func() { openAtom.Set(!openAtom.Get()) }))

	hidden := " hidden-menu"
	expanded := "false"
	if open {
		hidden = ""
		expanded = "true"
	}

	// The list body (and its per-view live totals) is only built when the popover is
	// open — lazily, not on every ledger render.
	var body ui.Node = Fragment()
	if open {
		body = savedViewsMenuBody(props)
	}

	return Div(css.Class("add-wrap"), Attr("id", "txn-saved-views"),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "txn-views-btn"),
			Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
			Title(uistate.T("savedViews.title")), OnClick(toggle),
			uiw.Icon(icon.List, css.Class("btn-tool-ico")), Span(uistate.T("savedViews.button"))),
		Div(ClassStr("add-menu saved-views-menu"+hidden), Attr("role", "menu"),
			Attr("aria-label", uistate.T("savedViews.title")), body),
	)
}

// savedViewsMenuBody builds the open popover's contents: the save-current control
// (or naming form), then the saved-view rows or an empty state.
func savedViewsMenuBody(props txnSavedViewsMenuProps) ui.Node {
	app := props.App
	formAtom := uistate.UseSaveViewFormOpen()
	views := app.SavedTxnViews()
	amount := baseAmountFunc(props.Rates, props.Base)

	// "Save current view…": disabled with an explanation when no filters are active.
	hasFilters := props.Filter.ActiveCount() > 0
	var saveControl ui.Node
	if formAtom.Get() && hasFilters {
		saveControl = ui.CreateElement(saveViewForm, saveViewFormProps{App: app, Filter: props.Filter})
	} else {
		onOpenForm := ui.UseEvent(Prevent(func() { formAtom.Set(true) }))
		if hasFilters {
			saveControl = Button(css.Class("add-item saved-views-save"), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "saved-views-save"), OnClick(onOpenForm),
				uiw.Icon(icon.Plus, css.Class("btn-tool-ico")), Text(uistate.T("savedViews.saveCurrent")))
		} else {
			saveControl = Div(css.Class("add-item saved-views-save is-disabled"),
				Attr("data-testid", "saved-views-save-disabled"), Attr("aria-disabled", "true"),
				Title(uistate.T("savedViews.saveDisabled")),
				uiw.Icon(icon.Plus, css.Class("btn-tool-ico")), Text(uistate.T("savedViews.saveCurrent")))
		}
	}

	var list ui.Node
	if len(views) == 0 {
		list = P(css.Class("empty saved-views-empty"), uistate.T("savedViews.empty"))
	} else {
		rows := make([]any, 0, len(views)+1)
		rows = append(rows, css.Class("saved-views-list"))
		for _, v := range views {
			count, total := v.Summary(app.Transactions(), amount)
			rows = append(rows, ui.CreateElement(savedViewRow, savedViewRowProps{
				App: app, View: v, Base: props.Base, Count: count, Total: total,
			}))
		}
		list = Div(rows...)
	}

	return Fragment(
		Div(css.Class("saved-views-head"), H3(css.Class("t-caption"), uistate.T("savedViews.title"))),
		saveControl,
		list,
	)
}

// saveViewFormProps configures the inline "name this view" form.
type saveViewFormProps struct {
	App    *appstate.App
	Filter txnfilter.Criteria
}

// saveViewForm is the inline naming form for saving the current filter as a view.
// Own component: owns its name-draft state and its save hook.
func saveViewForm(props saveViewFormProps) ui.Node {
	name := ui.UseState("")
	errMsg := ui.UseState("")
	formAtom := uistate.UseSaveViewFormOpen()
	openAtom := uistate.UseSavedViewsOpen()

	onName := ui.UseEvent(func(v string) { name.Set(v); errMsg.Set("") })
	onCancel := ui.UseEvent(Prevent(func() { formAtom.Set(false); name.Set(""); errMsg.Set("") }))
	onSave := ui.UseEvent(Prevent(func() {
		if strings.TrimSpace(name.Get()) == "" {
			errMsg.Set(uistate.T("savedViews.nameRequired"))
			return
		}
		if _, err := props.App.SaveTxnView(name.Get(), props.Filter, 0); err != nil {
			if err == savedtxnview.ErrNameTaken {
				errMsg.Set(uistate.T("savedViews.nameTaken"))
			} else {
				errMsg.Set(err.Error())
			}
			return
		}
		formAtom.Set(false)
		name.Set("")
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		openAtom.Set(true)
	}))

	var errNode ui.Node = Fragment()
	if errMsg.Get() != "" {
		errNode = P(css.Class("field-error"), Attr("role", "alert"), Text(errMsg.Get()))
	}

	return Div(css.Class("saved-views-form"),
		Label(css.Class("t-caption"), Attr("for", "saved-view-name"), uistate.T("savedViews.nameLabel")),
		Input(css.Class("field"), Attr("id", "saved-view-name"), Type("text"),
			Attr("data-testid", "saved-view-name"), Placeholder(uistate.T("savedViews.namePlaceholder")),
			Value(name.Get()), OnInput(onName)),
		errNode,
		Div(css.Class("saved-views-form-actions"),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "saved-view-cancel"),
				OnClick(onCancel), uistate.T("savedViews.cancel")),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "saved-view-save"),
				OnClick(onSave), uistate.T("savedViews.save")),
		),
	)
}

// savedViewRowProps carries one saved view plus its freshly-computed live totals.
type savedViewRowProps struct {
	App   *appstate.App
	View  savedtxnview.SavedTxnView
	Base  string
	Count int
	Total int64
}

// savedViewRow renders one saved view: name, live count + total, Apply, and a
// small action cluster (pin to dashboard, set/edit an amount alert, delete). It is
// its own component so each row owns its interactive handlers and its alert-form
// toggle state (no On* props inside a variable-length loop).
func savedViewRow(props savedViewRowProps) ui.Node {
	v := props.View
	alertOpen := ui.UseState(false)

	openAtom := uistate.UseSavedViewsOpen()
	dismissAtom := uistate.UseSavedViewThresholdDismissals()
	// Resolve every shared atom here in render — UseLayoutItems / UseTxFilter are
	// hooks and must never be called inside an event handler; the handlers below
	// only read/set the captured atoms.
	layoutAtom := uistate.UseLayoutItems()
	txFilterAtom := uistate.UseTxFilter()

	apply := ui.UseEvent(Prevent(func() {
		next := v.Criteria.ResetPageIfScopeChanged(txFilterAtom.Get())
		txFilterAtom.Set(next)
		uistate.PersistTxFilter(next)
		uistate.BumpDataRevision()
		openAtom.Set(false)
	}))
	pin := ui.UseEvent(Prevent(func() {
		pinSavedView(props.App, v, layoutAtom)
		uistate.PostNotice(uistate.T("savedViews.pinned", v.Name), false)
		openAtom.Set(false)
	}))
	del := ui.UseEvent(Prevent(func() {
		_ = props.App.DeleteTxnView(v.ID)
		uistate.PostNotice(uistate.T("savedViews.deleted", v.Name), false)
		uistate.BumpDataRevision()
	}))
	toggleAlert := ui.UseEvent(Prevent(func() { alertOpen.Set(!alertOpen.Get()) }))

	summary := uistate.T("savedViews.rowSummary",
		fmtMoney(money.New(props.Total, props.Base)), savedViewMatches(props.Count))

	alertLabel := uistate.T("savedViews.setAlert")
	if v.Threshold > 0 {
		alertLabel = uistate.T("savedViews.editAlert")
	}

	// Threshold notice: shown when the live total has crossed the view's threshold
	// and this exact (view + threshold) notice hasn't been dismissed.
	var notice ui.Node = Fragment()
	if v.CrossedThreshold(props.Total) && !dismissAtom.Get()[v.DismissalKey()] {
		key := v.DismissalKey()
		dismiss := ui.UseEvent(Prevent(func() { uistate.DismissSavedViewThreshold(dismissAtom, key) }))
		notice = Div(css.Class("saved-view-alert"), Attr("role", "status"), Attr("data-testid", "saved-view-threshold"),
			uiw.Icon(icon.Bell, css.Class("btn-tool-ico")),
			Span(uistate.T("savedViews.thresholdNotice", v.Name, fmtMoney(money.New(props.Total, props.Base)))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "saved-view-threshold-dismiss"),
				OnClick(dismiss), uistate.T("savedViews.thresholdDismiss")),
		)
	}

	var alertForm ui.Node = Fragment()
	if alertOpen.Get() {
		alertForm = ui.CreateElement(savedViewAlertForm, savedViewAlertFormProps{App: props.App, View: v, OnClose: func() { alertOpen.Set(false) }})
	}

	return Div(css.Class("saved-view-row"), Attr("data-testid", "saved-view-row"),
		Div(css.Class("saved-view-main"),
			Button(css.Class("saved-view-apply"), Type("button"), Attr("data-testid", "saved-view-apply"),
				Attr("aria-label", uistate.T("savedViews.applyAria", v.Name)), OnClick(apply),
				Span(css.Class("saved-view-name"), Text(v.Name)),
				Span(css.Class("saved-view-summary t-caption"), Text(summary))),
			Div(css.Class("saved-view-actions"),
				Button(css.Class("btn btn-icon"), Type("button"), Attr("data-testid", "saved-view-pin"),
					Attr("aria-label", uistate.T("savedViews.pin")), Title(uistate.T("savedViews.pin")),
					OnClick(pin), uiw.Icon(icon.Dashboard, css.Class(tw.W4, tw.H4))),
				Button(css.Class("btn btn-icon"), Type("button"), Attr("data-testid", "saved-view-alert-toggle"),
					Attr("aria-label", alertLabel), Title(alertLabel),
					OnClick(toggleAlert), uiw.Icon(icon.Bell, css.Class(tw.W4, tw.H4))),
				Button(css.Class("btn btn-icon"), Type("button"), Attr("data-testid", "saved-view-delete"),
					Attr("aria-label", uistate.T("savedViews.delete")), Title(uistate.T("savedViews.delete")),
					OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
			),
		),
		notice,
		alertForm,
	)
}

// savedViewAlertFormProps configures the per-row amount-threshold form.
type savedViewAlertFormProps struct {
	App     *appstate.App
	View    savedtxnview.SavedTxnView
	OnClose func()
}

// savedViewAlertForm sets or clears a view's amount threshold (major units). A
// blank value clears the alert. Own component: owns its draft + save hook.
func savedViewAlertForm(props savedViewAlertFormProps) ui.Node {
	initial := ""
	if props.View.Threshold > 0 {
		initial = strconv.FormatFloat(float64(props.View.Threshold)/100, 'f', -1, 64)
	}
	val := ui.UseState(initial)
	errMsg := ui.UseState("")

	onVal := ui.UseEvent(func(v string) { val.Set(v); errMsg.Set("") })
	onSave := ui.UseEvent(Prevent(func() {
		var minor int64
		if s := strings.TrimSpace(val.Get()); s != "" {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil || f < 0 {
				errMsg.Set(uistate.T("savedViews.thresholdPlaceholder"))
				return
			}
			minor = int64(f*100 + 0.5)
		}
		nv := props.View
		nv.Threshold = minor
		if err := props.App.UpdateTxnView(nv); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		props.OnClose()
	}))
	onCancel := ui.UseEvent(Prevent(func() { props.OnClose() }))

	var errNode ui.Node = Fragment()
	if errMsg.Get() != "" {
		errNode = P(css.Class("field-error"), Attr("role", "alert"), Text(errMsg.Get()))
	}

	return Div(css.Class("saved-view-alert-form"),
		Label(css.Class("t-caption"), uistate.T("savedViews.thresholdLabel")),
		Input(css.Class("field"), Type("text"), Attr("data-testid", "saved-view-threshold-input"),
			Placeholder(uistate.T("savedViews.thresholdPlaceholder")), Value(val.Get()), OnInput(onVal)),
		errNode,
		Div(css.Class("saved-views-form-actions"),
			Button(css.Class("btn btn-sm"), Type("button"), OnClick(onCancel), uistate.T("savedViews.cancel")),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "saved-view-threshold-save"),
				OnClick(onSave), uistate.T("savedViews.save")),
		),
	)
}

// pinSavedView turns a saved view into a persisted dashboard widget: a KindNative
// placement whose Settings carry the view id (its source), added to the dashboard
// layout and rendered by the shared savedViewNativeID renderer. Mirrors the
// Studio "publish to dashboard" flow (userSpecPrefix id + PutPlacements + layout
// append).
// pinSavedView takes the bento layout atom (resolved by the caller during render —
// UseLayoutItems is a hook and must never be called inside an event handler) and
// only reads/sets it here, so pinning stays a safe guarded update from a callback.
func pinSavedView(app *appstate.App, v savedtxnview.SavedTxnView, layoutAtom state.Atom[[]dashlayout.Item]) {
	wid := userSpecPrefix + id.New()
	spec := domain.WidgetSpec{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            wid,
		Kind:          domain.KindNative,
		Title:         v.Name,
		NativeID:      savedViewNativeID,
		Settings:      map[string]string{savedViewSettingsKey: v.ID},
	}
	if spec.Validate() != nil {
		return
	}
	item := dashlayout.Item{ID: wid, ColSpan: 2, RowSpan: 1}
	pl := domain.Placement{SchemaVersion: domain.WidgetSpecVersion, ID: wid, Surface: "dashboard", Spec: spec, Layout: item}
	if err := app.PutPlacements([]domain.Placement{pl}); err != nil {
		return
	}
	next := append([]dashlayout.Item(nil), layoutAtom.Get()...)
	next = append(next, item)
	layoutAtom.Set(next)
	uistate.PersistItems(next)
	uistate.RequestPersist()
}

// savedViewTileProps configures a pinned saved-view dashboard tile.
type savedViewTileProps struct {
	App    *appstate.App
	Txns   []domain.Transaction
	Rates  currency.Rates
	Base   string
	ViewID string
	SpecID string
	// Preview marks a non-interactive render (Studio live preview): the tile drops
	// its drag/resize/gear affordances, matching every other dashboard widget.
	Preview bool
}

// savedViewPinnedTile renders a pinned saved view on the dashboard: the view's
// name, its live total, and match count, as one click-through card that applies
// the view's criteria and navigates to /transactions. A view that has since been
// deleted renders a gentle "unavailable" state rather than crashing.
func savedViewPinnedTile(props savedViewTileProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()

	v, ok := props.App.SavedTxnView(props.ViewID)
	if !ok {
		return uiw.Widget(uiw.WidgetProps{
			ID: props.SpecID, Title: uistate.T("savedViews.title"),
			Draggable: !props.Preview, Resizable: !props.Preview, Preview: props.Preview,
			Body: P(css.Class("empty"), uistate.T("savedViews.empty")),
		})
	}

	count, total := v.Summary(props.Txns, baseAmountFunc(props.Rates, props.Base))
	// Resolve the filter atom in render — UseTxFilter is a hook; the handler only
	// reads/sets the captured atom.
	txFilterAtom := uistate.UseTxFilter()
	go2 := ui.UseEvent(Prevent(func() {
		next := v.Criteria.ResetPageIfScopeChanged(txFilterAtom.Get())
		txFilterAtom.Set(next)
		uistate.PersistTxFilter(next)
		uistate.BumpDataRevision()
		nav.Navigate(uistate.RoutePath("/transactions"))
	}))

	// The tile is one click-through card: an eyebrow that names it as a saved view
	// (with the filter icon for identity), the live total as the hero figure, the
	// match count, and an "Open" affordance that reads as tappable.
	return uiw.Widget(uiw.WidgetProps{
		ID: props.SpecID, Title: v.Name,
		Draggable: !props.Preview, Resizable: !props.Preview, Preview: props.Preview,
		BodyClass: tw.Fold(tw.Flex, tw.FlexCol, tw.MinH0),
		Body: Button(css.Class("saved-view-tile"), Type("button"), Attr("data-testid", "saved-view-tile"),
			Attr("aria-label", uistate.T("savedViews.applyAria", v.Name)), OnClick(go2),
			Div(css.Class("saved-view-tile-eyebrow"),
				uiw.Icon(icon.List, css.Class("saved-view-tile-ico")),
				Span(Text(uistate.T("savedViews.eyebrow")))),
			Div(css.Class("saved-view-tile-body"),
				Div(css.Class("saved-view-tile-figs"),
					Span(css.Class("saved-view-tile-total"), Text(fmtMoney(money.New(total, props.Base)))),
					Span(css.Class("saved-view-tile-sub"), Text(savedViewMatches(count)))),
				Span(css.Class("saved-view-tile-go"),
					Span(Text(uistate.T("savedViews.open"))),
					uiw.Icon(icon.ChevronRight, css.Class("saved-view-tile-go-ico")))),
		),
	})
}

// savedViewMatches returns the correctly pluralized match-count phrase for a saved
// view ("1 match" / "N matches") — the generic plural() helper mangles "match".
func savedViewMatches(n int) string {
	if n == 1 {
		return uistate.T("savedViews.matchesOne")
	}
	return uistate.T("savedViews.matchesMany", n)
}
