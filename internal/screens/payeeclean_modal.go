// SPDX-License-Identifier: MIT

//go:build js && wasm

// Payee-cleanup flip modal (SM-1), opened from a transaction row's kebab. It surfaces
// the merchant-name mapping that also lives on /rules, but scoped to one transaction:
// a deterministic clean-name suggestion (SMART, payeeclean), an optional "Suggest with
// AI" for the cryptic cases (SMART+, smartai.MerchantCleanup), and a scope choice —
// clean just THIS transaction, or map ALL charges with the same raw name (which writes
// a payee alias so past + future imports normalize too, exactly like the /rules editor).
package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeeclean"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// PayeeCleanFormID is the id shared by the modal's body <form> and the FlipPanel
// footer's Save (type=submit form=…) button, so the pinned footer submits the form
// while the body scrolls beneath it.
const PayeeCleanFormID = "payeeclean-form"

// rawPayeeOf returns the raw string the resolver keys on for a transaction (its payee,
// or description when the payee is blank) — the same key an alias must match.
func rawPayeeOf(t domain.Transaction) string {
	return strings.TrimSpace(firstNonEmpty(t.Payee, t.Desc))
}

// PayeeCleanBody is the body of the payee-cleanup flip modal (mounted at the shell root
// by app.PayeeCleanHost). It renders as a <form> (PayeeCleanFormID) whose Save is the
// FlipPanel's pinned footer button, so the footer stays fixed while the body scrolls.
func PayeeCleanBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UsePayeeClean()
	txnID := openAtom.Get()
	pr := uistate.UsePrefs().Get()

	// State declared unconditionally (stable hook order) before any early return.
	name := ui.UseState("")
	scope := ui.UseState("all")
	loading := ui.UseState(false)
	errText := ui.UseState("")
	seeded := ui.UseState("") // the txn id the name field was seeded for

	if app == nil || txnID == "" {
		return Fragment()
	}
	var txn domain.Transaction
	found := false
	for _, t := range app.Transactions() {
		if t.ID == txnID {
			txn = t
			found = true
			break
		}
	}
	if !found {
		return Fragment()
	}
	rawPayee := strings.TrimSpace(txn.Payee)
	hasPayee := rawPayee != "" // only a real payee can be mapped across charges (an alias)
	messy := rawPayeeOf(txn)   // the payee, or the description when there's no payee

	// Any existing cleanup for this merchant: its current clean name (so reopening the
	// modal shows the CLEAN name, not the raw string again) and its rename lineage.
	var alias domain.PayeeAlias
	if hasPayee {
		for _, al := range app.PayeeAliases() {
			if strings.EqualFold(strings.TrimSpace(al.RawPayee), rawPayee) {
				alias = al
				break
			}
		}
	}
	hasAlias := strings.TrimSpace(alias.ID) != ""

	// Seed the editable name the first time this txn's modal opens (re-seed if reopened
	// for a different transaction). Prefer the CURRENT clean name when this merchant was
	// already cleaned, so reopening confirms the name in effect rather than re-suggesting
	// from the raw processor string; otherwise offer the deterministic suggestion.
	if seeded.Get() != txnID {
		if hasAlias {
			name.Set(alias.Display)
		} else {
			name.Set(payeeclean.Suggest(messy))
		}
		if hasPayee {
			scope.Set("all")
		} else {
			scope.Set("this")
		}
		errText.Set("")
		seeded.Set(txnID)
	}

	// Rename lineage for the history strip: the original raw string, each prior clean
	// name (with the date it was superseded), then the name now in effect.
	var histRows []payeeRenameRow
	if hasAlias {
		histRows = append(histRows, payeeRenameRow{Name: rawPayee, Meta: uistate.T("payeeClean.historyOriginal"), Mod: "is-raw"})
		for _, h := range alias.History {
			histRows = append(histRows, payeeRenameRow{Name: h.Display, Meta: pr.FormatDate(h.At)})
		}
		histRows = append(histRows, payeeRenameRow{Name: alias.Display, Meta: uistate.T("payeeClean.historyCurrent"), Mod: "is-current"})
	}

	// How many transactions share this raw PAYEE (drives the "All N" label) — only a
	// payee is mappable across charges via an alias.
	sameCount := 0
	if hasPayee {
		for _, t := range app.Transactions() {
			if strings.EqualFold(strings.TrimSpace(t.Payee), rawPayee) {
				sameCount++
			}
		}
	}

	// AI availability (SMART+ SMART-T5): a configured provider + the feature enabled.
	backendAI := pr.Normalize().BackendActive()
	hasProvider := aiProviderConfigured(app, backendAI)
	aiEnabled := hasProvider && uistate.LoadSmartSettings().IsEnabled("SMART-T5")
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onScope := func(v string) { scope.Set(v) }

	suggestAI := ui.UseEvent(Prevent(func() {
		if loading.Get() {
			return
		}
		loading.Set(true)
		errText.Set("")
		runSmartAI(aiConn, smartai.MerchantCleanup(messy),
			func(text string) {
				clean := strings.Trim(strings.TrimSpace(text), `"'.`)
				if clean != "" {
					name.Set(clean)
				}
				loading.Set(false)
			},
			func(e string) { errText.Set(e); loading.Set(false) })
	}))

	save := ui.UseEvent(Prevent(func() {
		disp := strings.TrimSpace(name.Get())
		if disp == "" {
			errText.Set(uistate.T("payeeClean.needName"))
			return
		}
		if scope.Get() == "all" && hasPayee {
			// Map every charge with this payee (past + future) via a payee alias — the
			// same mechanism the /rules manager writes.
			if err := app.PutPayeeAlias(domain.PayeeAlias{RawPayee: rawPayee, Display: disp}); err != nil {
				errText.Set(err.Error())
				return
			}
		} else {
			// Just this transaction: rename what the ledger shows for it (the row's
			// description is its display name, so set that).
			t := txn
			t.Desc = disp
			if err := app.PutTransaction(t); err != nil {
				errText.Set(err.Error())
				return
			}
		}
		uistate.BumpDataRevision()
		uistate.ClosePayeeClean()
	}))

	// AI suggest button (only when a provider is configured + T5 enabled).
	var aiBtn ui.Node = Fragment()
	if aiEnabled {
		label := uistate.T("payeeClean.suggestAI")
		if loading.Get() {
			label = uistate.T("payeeClean.suggesting")
		}
		aiBtn = Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "payeeclean-ai"), Attr("aria-disabled", ariaBool(loading.Get())), OnClick(suggestAI),
			smartGlyph(false, tw.Fold(tw.W35, tw.H35)), Span(label))
	}

	// Scope options: renaming just this transaction is always available; mapping ALL
	// charges (a payee alias) only when there's a payee to map.
	scopeOpts := []uiw.SegOption{}
	if hasPayee {
		scopeOpts = append(scopeOpts, uiw.SegOption{Value: "all", Label: uistate.T("payeeClean.scopeAll", sameCount)})
	}
	scopeOpts = append(scopeOpts, uiw.SegOption{Value: "this", Label: uistate.T("payeeClean.scopeThis")})

	return Form(css.Class("pclean", tw.FlexCol, tw.Gap3),
		Attr("id", PayeeCleanFormID), Attr("data-testid", "payeeclean-modal"), OnSubmit(save),
		// The raw descriptor, read-only, so the user sees what's being mapped.
		Div(css.Class("pclean-raw"),
			Div(css.Class(tw.Text12, tw.TextDim), uistate.T("payeeClean.rawLabel")),
			Div(css.Class("pclean-raw-val"), messy),
		),
		// The editable clean name + the optional AI suggest.
		uiw.FormField(uistate.T("payeeClean.nameLabel"),
			Div(css.Class("pclean-name-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "payeeclean-name"),
					Attr("aria-label", uistate.T("payeeClean.nameLabel")), Value(name.Get()), OnInput(onName)),
				aiBtn,
			),
		),
		// Scope: this transaction, or all charges with this raw name.
		Div(css.Class("pclean-scope"),
			Div(css.Class(tw.Text12, tw.TextDim, tw.Mb1), uistate.T("payeeClean.scopeLabel")),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    uistate.T("payeeClean.scopeLabel"),
				Options:  scopeOpts,
				Selected: scope.Get(),
				OnSelect: onScope,
			}),
			Div(css.Class(tw.Text12, tw.TextDim, tw.Mt1),
				IfElse(scope.Get() == "all",
					Span(uistate.T("payeeClean.scopeAllHint")),
					Span(uistate.T("payeeClean.scopeThisHint")))),
		),
		// Rename history: the merchant's clean-name lineage, so reopening shows what it
		// used to be called, not just the current name.
		If(len(histRows) > 0, payeeCleanHistoryNode(histRows)),
		If(errText.Get() != "", P(css.Class("err"), Attr("role", "alert"), errText.Get())),
		// No inline footer: the FlipPanel's pinned Cancel/Save footer submits this form
		// (see PayeeCleanHost), so the buttons stay fixed while the body scrolls.
	)
}

// payeeRenameRow is one line of the cleanup modal's rename-history strip: a name the
// merchant has been shown as, plus a short meta tag (Original / a date / Now).
type payeeRenameRow struct {
	Name string
	Meta string
	Mod  string // extra state class: "is-raw" | "is-current" | ""
}

// payeeCleanHistoryNode renders the rename lineage as a compact vertical list: the raw
// original at the top, each prior clean name in the middle, and the name in effect at
// the bottom — so the modal keeps a visible trail of what a merchant used to be called.
func payeeCleanHistoryNode(rows []payeeRenameRow) ui.Node {
	items := make([]any, 0, len(rows)+1)
	items = append(items, css.Class("pclean-history-list"))
	for _, r := range rows {
		cls := "pclean-history-item"
		if r.Mod != "" {
			cls += " " + r.Mod
		}
		items = append(items, Div(css.Class(cls),
			Span(css.Class("pclean-history-name"), r.Name),
			Span(css.Class("pclean-history-meta"), r.Meta)))
	}
	return Div(css.Class("pclean-history"),
		Div(css.Class("pclean-history-label", tw.Text12, tw.TextDim), uistate.T("payeeClean.historyLabel")),
		Div(items...))
}
