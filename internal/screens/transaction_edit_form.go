// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// merchantContextPanel renders the TX6 merchant "story" inside the edit modal:
// typical amount, this charge vs typical, visits this week/month, this month vs a
// typical month, and a tiny sparkline of recent charges. It is read-only and
// quiet, and returns an empty fragment for transfers or one-off merchants (fewer
// than merchantstats.MinCharges charges), so it never nags. All amounts are
// converted to the household base currency via the FX table so a multi-currency
// history reads as one figure.
func merchantContextPanel(app *appstate.App, txn domain.Transaction) ui.Node {
	if txn.IsTransfer() {
		return Fragment()
	}
	merchant := strings.TrimSpace(app.PayeeResolver().Resolve(firstNonEmpty(txn.Payee, txn.Desc)))
	if merchant == "" {
		return Fragment()
	}
	// Shared with the row trend chip (merchant_trend.go): compute once, render the
	// same story. Returns ok=false for a one-off merchant with too little history.
	stats, base, ok := computeMerchantStats(app, merchant)
	if !ok {
		return Fragment()
	}
	thisMag, hasThis := toBaseMag(app, txn.Amount, base)
	return Div(css.Class("card", tw.P3, tw.FlexCol, tw.Gap15), Attr("data-testid", "merchant-context-panel"),
		merchantStoryNodes(stats, merchant, base, thisMag, hasThis))
}

// sparklineSVG draws a tiny polyline of the last-N charge magnitudes with no
// chart library — a bare SVG scaled to the min/max of the series. A series of one
// point (or all-equal values) draws a flat baseline.
func sparklineSVG(series []int64) ui.Node {
	if len(series) < 2 {
		return Fragment()
	}
	const w, h = 120, 24
	min, max := series[0], series[0]
	for _, v := range series {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	pts := make([]string, len(series))
	for i, v := range series {
		x := float64(i) * float64(w) / float64(len(series)-1)
		var y = float64(h) / 2
		if span > 0 {
			// Higher spend → higher on screen (smaller y).
			y = float64(h) - float64(v-min)*float64(h-2)/float64(span) - 1
		}
		pts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	return Svg(
		Attr("viewBox", fmt.Sprintf("0 0 %d %d", w, h)),
		Attr("width", strconv.Itoa(w)), Attr("height", strconv.Itoa(h)),
		Attr("role", "img"), Attr("aria-label", uistate.T("merchantPanel.sparklineAlt")),
		Attr("preserveAspectRatio", "none"),
		Polyline(Attr("points", strings.Join(pts, " ")),
			Attr("fill", "none"), Attr("stroke", "var(--accent)"), Attr("stroke-width", "1.5")),
	)
}

// receiptThumbnails renders the transaction's attached receipts as small
// clickable thumbnails (TX5). Each thumbnail is its own component so its click
// handler is loop-hook-safe. Bytes are resolved from the live artifact set.
func receiptThumbnails(app *appstate.App, txn domain.Transaction) ui.Node {
	byID := map[string]domain.Artifact{}
	for _, a := range app.Artifacts() {
		byID[a.ID] = a
	}
	var thumbs []ui.Node
	for _, ref := range txn.Attachments {
		art, ok := byID[ref.ArtifactID]
		var dataURL string
		if ok && len(art.Bytes) > 0 {
			dataURL = artifacts.DataURL(art.MIME, art.Bytes)
		}
		thumbs = append(thumbs, ui.CreateElement(receiptThumb, receiptThumbProps{Ref: ref, DataURL: dataURL}))
	}
	return Div(css.Class(tw.Flex, tw.Gap2), Attr("data-testid", "txn-receipt-thumbs"), thumbs)
}

// receiptThumbProps configures one receipt thumbnail.
type receiptThumbProps struct {
	Ref     domain.AttachmentRef
	DataURL string // empty when the artifact bytes aren't available
}

// receiptThumb is a single receipt thumbnail: an image button that opens the
// shared receipt preview overlay (the same full-view the table's paperclip uses)
// on click. Owning its own OnClick hook keeps it safe inside the attachments loop.
func receiptThumb(props receiptThumbProps) ui.Node {
	preview := uistate.UseTxnPreview()
	ref := props.Ref
	open := ui.UseEvent(Prevent(func() { preview.Set(ref) }))
	label := uistate.T("merchantPanel.openReceipt", firstNonEmpty(ref.Name, ref.ArtifactID))
	inner := Span(css.Class("muted", tw.P1), "📎")
	if props.DataURL != "" {
		inner = Img(Attr("src", props.DataURL), Attr("alt", label),
			css.Class(tw.W10, tw.H10), Attr("style", "object-fit:cover;border-radius:4px;"))
	}
	return Button(css.Class("btn btn-icon"), Type("button"), Attr("data-testid", "txn-receipt-thumb"),
		Attr("aria-label", label), Title(label), OnClick(open), inner)
}

// TransactionEditFormProps configures the TransactionEditForm component.
type TransactionEditFormProps struct {
	// TxnID is the id of the transaction being edited; looked up live from appstate.
	TxnID string
	// OnDone is called after a successful save or delete (and on Cancel) so the
	// caller (TxnEditHost) can close the modal. On a validation error the form
	// stays open and OnDone is not called.
	OnDone func()
}

// TransactionEditForm is the standalone edit-a-transaction form used by the
// TxnEditHost modal (the widgetized transactions page drills into it on row
// click). It looks the transaction up by id, seeds its fields, validates on
// save, and persists via appstate. Mirrors the inline edit controls in
// transactions_row.go and the add-form submit/error pattern.
func TransactionEditForm(props TransactionEditFormProps) ui.Node {
	return ui.CreateElement(transactionEditForm, props)
}

func transactionEditForm(props TransactionEditFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	// Look the transaction up live so an edit always works against current data.
	var txn domain.Transaction
	found := false
	for _, t := range app.Transactions() {
		if t.ID == props.TxnID {
			txn, found = t, true
			break
		}
	}

	categories := app.Categories()
	members := app.Members()

	// Seed display values from the transaction (amount shown in major units,
	// absolute, with the sign preserved separately on save).
	amountMajor := ""
	dateISO := ""
	if found {
		amountMajor = money.FormatMinor(txnfilter.AbsAmount(txn), currency.Decimals(txn.Amount.Currency))
		dateISO = dateutil.FormatDate(txn.Date)
	}

	descS := ui.UseState(txn.Desc)
	payeeS := ui.UseState(txn.Payee)
	amountS := ui.UseState(amountMajor)
	catS := ui.UseState(txn.CategoryID)
	dateS := ui.UseState(dateISO)
	memberS := ui.UseState(txn.MemberID)
	tagsS := ui.UseState(strings.Join(txn.Tags, ", "))
	clearedS := ui.UseState(txn.Cleared)
	noteS := ui.UseState(txn.Note)                  // TXC-2: free-text memo
	excludeS := ui.UseState(txn.ExcludeFromReports) // TXC-1: keep in balance, drop from budgets/reports
	errMsg := ui.UseState("")
	// TX7: after a category change is saved, hold the similar-transaction candidates
	// so the inline "N more look like this" offer can render. Empty means no offer.
	simState := ui.UseState[[]similartxns.Candidate](nil)
	// simTarget is the just-saved transaction the offer would recategorize others to
	// match (its new category is the one applied on "Recategorize them").
	simTarget := ui.UseState(domain.Transaction{})
	// C58: the split-into-categories editor, revealed by a toggle inside this modal
	// so the breakdown is visible and editable without leaving the edit form.
	splitOpen := ui.UseState(false)
	toggleSplit := ui.UseEvent(func() { splitOpen.Set(!splitOpen.Get()) })
	// saveSplits persists a breakdown set in the embedded SplitEditor. It writes the
	// stored transaction directly (the editor validated Σ=amount), so it is
	// independent of this form's unsaved field drafts.
	saveSplits := func(updated domain.Transaction) {
		if err := app.PutTransaction(updated); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		if updated.HasSplits() {
			uistate.PostNotice(uistate.T("splitEditor.saved"), false)
		} else {
			uistate.PostNotice(uistate.T("splitEditor.cleared"), false)
		}
	}

	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onPayee := ui.UseEvent(func(v string) { payeeS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onTags := ui.UseEvent(func(v string) { tagsS.Set(v) })
	onCleared := ui.UseEvent(func(e ui.Event) { clearedS.Set(e.IsChecked()) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })
	onExclude := ui.UseEvent(func(e ui.Event) { excludeS.Set(e.IsChecked()) })

	// Quick-add a category right from the picker — faster than leaving to /categories.
	// A toggle reveals a name field; adding creates the category (matching the
	// transaction's income/expense flow) and immediately selects it on this form.
	addingCat := ui.UseState(false)
	newCatName := ui.UseState("")
	onNewCatName := ui.UseEvent(func(v string) { newCatName.Set(v) })
	toggleAddCat := ui.UseEvent(func() {
		addingCat.Set(!addingCat.Get())
		newCatName.Set("")
		errMsg.Set("")
	})
	// doAddCat creates the typed category and selects it. A plain closure so both the
	// Add button and the Enter key (which must NOT submit the outer edit form) run it.
	doAddCat := func() {
		n := strings.TrimSpace(newCatName.Get())
		if n == "" {
			errMsg.Set(uistate.T("categories.nameRequired"))
			return
		}
		kind := domain.KindExpense
		if txn.IsIncome() {
			kind = domain.KindIncome
		}
		c := domain.Category{ID: id.New(), Name: n, Kind: kind}
		if err := app.PutCategory(c); err != nil {
			errMsg.Set(err.Error())
			return
		}
		catS.Set(c.ID) // select the just-created category on this form
		newCatName.Set("")
		addingCat.Set(false)
		errMsg.Set("")
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("categories.addedToast", n), false)
	}
	addCat := ui.UseEvent(Prevent(func() { doAddCat() }))
	onNewCatKey := ui.UseEvent(func(e ui.KeyboardEvent) {
		switch e.GetKey() {
		case "Enter":
			e.PreventDefault() // add the category instead of submitting the edit form
			doAddCat()
		case "Escape":
			e.PreventDefault()
			addingCat.Set(false)
			newCatName.Set("")
		}
	})

	save := ui.UseEvent(Prevent(func() {
		t := txn
		amt, err := money.ParseMinor(strings.TrimSpace(amountS.Get()), currency.Decimals(t.Amount.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("transactions.positiveAmount"))
			return
		}
		if t.Amount.IsNegative() {
			amt = -amt // preserve the original income/expense sign
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateS.Get()))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		t.Desc = strings.TrimSpace(descS.Get())
		t.Payee = strings.TrimSpace(payeeS.Get())
		t.Amount = money.New(amt, t.Amount.Currency)
		t.CategoryID = catS.Get()
		t.Date = date
		if memberS.Get() != "" {
			t.MemberID = memberS.Get()
		}
		t.Tags = textutil.CommaFields(tagsS.Get())
		t.Cleared = clearedS.Get()
		t.Note = strings.TrimSpace(noteS.Get())
		t.ExcludeFromReports = excludeS.Get()
		// C58: an amount edit must not silently desync an existing category breakdown —
		// budgets attribute per split line, so a mismatched total would misreport. Block
		// and point at the split section below instead of quietly dropping the split.
		if t.HasSplits() && !t.SplitsReconcile() {
			errMsg.Set(uistate.T("transactions.splitAmountMismatch"))
			return
		}
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// C33-style learn tally: record the payee→category correction on an explicit set.
		if t.CategoryID != "" {
			learn := t.Payee
			if learn == "" {
				learn = t.Desc
			}
			uistate.IncrementLearnTally(learn, t.CategoryID)
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("toast.txnUpdated"), false)

		// TX1 learning flow: when the user renamed the payee, quietly offer to always
		// show the original (raw) name as the new one — creating a view-layer alias so
		// every transaction with that raw name reads cleanly (the raw stays on the txn).
		origPayee := strings.TrimSpace(txn.Payee)
		newPayee := strings.TrimSpace(payeeS.Get())
		if origPayee != "" && newPayee != "" && !strings.EqualFold(origPayee, newPayee) &&
			!app.PayeeResolver().HasLearned(origPayee) {
			uistate.ConfirmModal(uistate.T("payeealias.learnPrompt", origPayee, newPayee), false, func(ok bool) {
				if !ok {
					return
				}
				if err := app.PutPayeeAlias(domain.PayeeAlias{RawPayee: origPayee, Display: newPayee}); err != nil {
					uistate.PostNotice(err.Error(), true)
					return
				}
				uistate.BumpDataRevision()
				uistate.PostNotice(uistate.T("payeealias.learned", origPayee, newPayee), false)
			})
		}

		// TX7: when the category changed to a real category, proactively find similar
		// transactions (alias/payee match via TX1, else the rules matcher) that carry a
		// different or no category, and offer to recategorize them too. The offer keeps
		// the modal open (inline preview + Apply); already-categorized rows are listed
		// but never overwritten without the click.
		if t.CategoryID != "" && t.CategoryID != txn.CategoryID {
			cands := similartxns.Find(t, app.Transactions(), t.CategoryID, app.PayeeResolver(), app.Rules())
			if len(cands) > 0 {
				simTarget.Set(t)
				simState.Set(cands)
				return // hold the modal open to show the offer
			}
		}

		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	del := ui.UseEvent(Prevent(func() {
		id := props.TxnID
		uistate.ConfirmModal(uistate.T("transactions.deleteConfirm", txn.Desc), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteTransaction(id); err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			if props.OnDone != nil {
				props.OnDone()
			}
		})
	}))

	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	// TX7 offer actions. applySimilar recategorizes every listed candidate to the
	// saved transaction's new category (an explicit click — already-categorized rows
	// are only changed here, never silently). alwaysRule routes to the prefilled-rule
	// flow (C32). dismissSimilar closes the offer and the modal.
	applySimilar := ui.UseEvent(func() {
		target := simTarget.Get()
		n := 0
		for _, c := range simState.Get() {
			t := c.Txn
			t.CategoryID = target.CategoryID
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(err.Error(), true)
				continue
			}
			n++
		}
		simState.Set(nil)
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("similartxns.applied", n), false)
		if props.OnDone != nil {
			props.OnDone()
		}
	})
	alwaysRule := ui.UseEvent(func() {
		target := simTarget.Get()
		phrase := strings.TrimSpace(target.Payee)
		if phrase == "" {
			phrase = strings.TrimSpace(target.Desc)
		}
		uistate.SetRuleDraft(phrase, target.CategoryID)
		simState.Set(nil)
		if props.OnDone != nil {
			props.OnDone()
		}
		router.Navigate(uistate.RoutePath("/rules"))
	})
	dismissSimilar := ui.UseEvent(func() {
		simState.Set(nil)
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	// Attach a receipt image (L29): upload it as an Artifact, link it to the live
	// transaction, and bump the shared data revision so both this modal and the
	// table (whose paperclip opens the preview) reflect it. Operates on the stored
	// transaction, so it persists independently of unsaved field edits in the form.
	attach := ui.UseEvent(Prevent(func() {
		pickFile("image/*", func(name, mime string, data []byte) {
			art := domain.Artifact{ID: id.New(), Name: name, Kind: "image", MIME: mime, Bytes: data, Size: len(data), CreatedAt: time.Now()}
			if err := app.PutArtifact(art); err != nil {
				errMsg.Set(err.Error())
				return
			}
			cur := txn
			cur.Attachments = append(cur.Attachments, domain.AttachmentRef{ArtifactID: art.ID, Name: name, Kind: "image", MIME: mime})
			if err := app.PutTransaction(cur); err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("transactions.attachReceiptTitle"), false)
		})
	}))

	if !found {
		return Div(css.Class("form-grid"),
			P(css.Class("empty"), uistate.T("txnwidget.notFound")),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
		)
	}

	// TX7: the apply-to-similar offer replaces the form body once a category change
	// has saved and similar transactions were found. It shows a short preview (up to
	// 5) + Apply / Always-do-this / dismiss. Rendered here so the modal stays a
	// single surface (the footer Save/Cancel are inert while it shows).
	if cands := simState.Get(); len(cands) > 0 {
		resolver := app.PayeeResolver()
		catNames := make(map[string]string, len(categories))
		for _, c := range categories {
			catNames[c.ID] = c.Name
		}
		const previewMax = 5
		var rows []ui.Node
		for i, c := range cands {
			if i >= previewMax {
				break
			}
			name := resolver.Resolve(firstNonEmpty(c.Txn.Payee, c.Txn.Desc))
			rows = append(rows, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class(tw.Flex1), name),
				Span(css.Class("muted"), c.Txn.Date.Format("Jan 2")),
				Span(css.Class("muted"), fmtMoney(c.Txn.Amount)),
				If(c.AlreadyCategorized, Span(css.Class("muted"), uistate.T("similartxns.hasCategory"))),
			))
		}
		if len(cands) > previewMax {
			rows = append(rows, P(css.Class("muted"), uistate.T("similartxns.moreCount", len(cands)-previewMax)))
		}
		offer := uistate.T("similartxns.offer", len(cands))
		if len(cands) == 1 {
			offer = uistate.T("similartxns.offerOne")
		}
		return Div(css.Class("form-grid"), Attr("data-testid", "txn-recat-offer"), Attr("role", "status"),
			P(css.Class("t-body"), offer),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), rows),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txn-recat-apply"), OnClick(applySimilar), uistate.T("similartxns.apply")),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-recat-rule"), OnClick(alwaysRule), uistate.T("similartxns.alwaysDo")),
				Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "txn-recat-dismiss"), OnClick(dismissSimilar), uistate.T("similartxns.dismiss")),
			),
		)
	}

	catOpts := uiw.OptionsFrom(
		categories,
		func(c domain.Category) string { return c.ID },
		func(c domain.Category) string { return c.Name },
		catS.Get(),
	)
	catOpts = append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}}, catOpts...)

	memberOpts := uiw.OptionsFrom(
		members,
		func(m domain.Member) string { return m.ID },
		func(m domain.Member) string { return m.Name },
		memberS.Get(),
	)

	// The current breakdown, listed read-only so a split is visible the moment the
	// modal opens; the full editor is one tap away on the toggle. Category id → name
	// resolves against the live category list (unknown/empty ids read as
	// uncategorized). Static text only, so a plain loop is loop-hook-safe.
	catNameByID := make(map[string]string, len(categories))
	for _, c := range categories {
		catNameByID[c.ID] = c.Name
	}
	// XC10: resolve a line's owner name so the read-only breakdown shows who a
	// line is attributed to when it carries an owner different from the payer.
	memberNameByID := make(map[string]string, len(members))
	for _, m := range members {
		memberNameByID[m.ID] = m.Name
	}
	var splitLines []ui.Node
	for _, s := range txn.Splits {
		n := catNameByID[s.CategoryID]
		if n == "" {
			n = uistate.T("transactions.uncategorized")
		}
		// Only show the owner tag on lines that carry their own owner (an empty
		// MemberID means "same as transaction" — nothing to disambiguate).
		ownerTag := ""
		if s.MemberID != "" {
			name := memberNameByID[s.MemberID]
			if name == "" {
				name = s.MemberID
			}
			ownerTag = uistate.T("splitEditor.ownerFor", name)
		}
		splitLines = append(splitLines, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class("badge badge-split"), "⑂"),
			Span(n),
			If(ownerTag != "", Span(css.Class("muted"), ownerTag)),
			Span(css.Class("muted"), fmtMoney(s.Amount)),
		))
	}

	return Form(css.Class("form-grid txn-edit"), Attr("id", "txn-edit-form"), Attr("data-testid", "txn-edit-form"), OnSubmit(save),
		labeledField(uistate.T("transactions.descPlaceholder"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), Value(descS.Get()), OnInput(onDesc))),
		labeledField(uistate.T("transactions.payeeLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.payeeLabel")), Placeholder(uistate.T("transactions.payeeLabel")), Value(payeeS.Get()), OnInput(onPayee))),
		labeledField(uistate.T("transactions.amountPlaceholder"),
			Input(css.Class("field"), Type("number"), Step("0.01"), Placeholder(uistate.T("transactions.amountPlaceholder")), Value(amountS.Get()), OnInput(onAmount))),
		uiw.FormField(uistate.T("transactions.categoryLabel"),
			Div(css.Class("txn-cat-picker"),
				Div(css.Class("txn-cat-row"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   catOpts,
						Selected:  catS.Get(),
						AriaLabel: uistate.T("transactions.categoryLabel"),
						OnChange:  func(v string) { catS.Set(v) },
					}),
					Button(css.Class("btn btn-tool txn-cat-new", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
						Attr("data-testid", "txn-edit-newcat-toggle"), Attr("aria-expanded", ariaBool(addingCat.Get())),
						Title(uistate.T("transactions.newCategory")), OnClick(toggleAddCat),
						uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
						Span(uistate.T("transactions.newCategory")))),
				// Inline create: name field + Add; creates the category (typed to the
				// transaction's flow) and selects it without leaving the modal.
				If(addingCat.Get(),
					Div(css.Class("txn-cat-add"),
						Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-edit-newcat-name"),
							Attr("aria-label", uistate.T("transactions.newCategoryName")),
							Placeholder(uistate.T("transactions.newCategoryName")),
							Value(newCatName.Get()), OnInput(onNewCatName), OnKeyDown(onNewCatKey)),
						Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txn-edit-newcat-add"),
							OnClick(addCat), uistate.T("transactions.newCategoryAdd")),
						Button(css.Class("btn btn-ghost"), Type("button"),
							OnClick(toggleAddCat), uistate.T("transactions.newCategoryCancel")))))),
		labeledField(uistate.T("transactions.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
		labeledField(uistate.T("transactions.tagsLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.tagsLabel")), Placeholder(uistate.T("transactions.tagsPlaceholder")), Value(tagsS.Get()), OnInput(onTags))),
		If(len(members) > 1, uiw.FormField(uistate.T("transactions.whoLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   memberOpts,
				Selected:  memberS.Get(),
				AriaLabel: uistate.T("transactions.whoLabel"),
				OnChange:  func(v string) { memberS.Set(v) },
			}))),
		Label(css.Class("txn-check"),
			Input(Type("checkbox"), Attr("aria-label", uistate.T("txnwidget.clearedLabel")), CheckedIf(clearedS.Get()), OnChange(onCleared)),
			Span(uistate.T("txnwidget.clearedLabel"))),
		// TXC-2: a free-text memo for this transaction (distinct from the description).
		uiw.FormField(uistate.T("transactions.noteLabel"),
			Textarea(css.Class("field"), Attr("rows", "2"), Attr("data-testid", "txn-edit-note"),
				Attr("aria-label", uistate.T("transactions.noteLabel")),
				Attr("placeholder", uistate.T("transactions.notePlaceholder")), OnInput(onNote), noteS.Get())),
		// TXC-1: exclude from budgets & reports (still counts toward account balances).
		// The hint sits on its own line BELOW the checkbox row so it never runs into
		// the label; a hairline separates this "reporting" control from the "Cleared
		// (reconciled)" checkbox above so the two aren't mistaken for each other.
		Div(css.Class("txn-exclude-field", tw.FlexCol, tw.Gap1),
			Label(css.Class("txn-check"),
				Input(Type("checkbox"), Attr("data-testid", "txn-edit-exclude"), Attr("aria-label", uistate.T("transactions.excludeLabel")),
					CheckedIf(excludeS.Get()), OnChange(onExclude)),
				Span(uistate.T("transactions.excludeLabel"))),
			Span(css.Class("muted", tw.Text12), uistate.T("transactions.excludeHint"))),
		// TX6: the merchant context panel — the merchant's story (typical amount,
		// this charge vs typical, visits this week/month, this month vs a typical
		// month, a tiny sparkline). Read-only, omitted for transfers and one-off
		// merchants (< 3 charges).
		merchantContextPanel(app, txn),
		// Receipts: attach a new image; existing receipts render as thumbnails that
		// open a full view on click (TX5), so the modal shows the receipt itself, not
		// just a count.
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-edit-attach"), OnClick(attach), uistate.T("transactions.attachReceiptTitle")),
				If(len(txn.Attachments) > 0, Span(css.Class("muted"), receiptCountLabel(len(txn.Attachments)))),
			),
			If(len(txn.Attachments) > 0, receiptThumbnails(app, txn)),
		),
		// C58: split-into-categories, right in the modal — the current breakdown is
		// always visible; the toggle reveals the full editor (kept as type=button
		// children so nothing here submits the form).
		Div(Attr("data-testid", "txn-edit-splits"),
			If(len(splitLines) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), splitLines)),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-edit-split-toggle"),
				Attr("aria-expanded", ariaBool(splitOpen.Get())), OnClick(toggleSplit),
				uistate.T(splitToggleKey(splitOpen.Get(), txn.HasSplits()))),
			If(splitOpen.Get(), ui.CreateElement(SplitEditor, splitEditorProps{
				Txn: txn, Categories: categories, Members: members, OnSave: saveSplits,
			})),
		),
		errText("txn-edit-err", errMsg.Get()),
		// Delete stays in the body (left-aligned, apart from the panel footer's
		// Cancel/Save): the FlipPanel's pinned .set-foot owns Cancel + Save now (via
		// FormID → native submit), fixing the old floating mid-panel button row.
		Div(css.Class(tw.Flex, tw.Mt2, tw.Mb2),
			Button(css.Class("btn btn-del"), Type("button"), Attr("data-testid", "txn-edit-delete"), OnClick(del), uistate.T("action.delete")),
		),
	)
}
