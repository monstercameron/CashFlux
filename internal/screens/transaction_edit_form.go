// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/amountmath"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnmove"
	"github.com/monstercameron/CashFlux/internal/txnprov"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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

// sparklineSVG draws a tiny polyline of a series with no chart library — a bare
// SVG scaled to its min/max. A series of one point (or all-equal values) draws a
// flat baseline. alt is the accessible name — every call site says what ITS
// sparkline shows (QA CF-30: the report's net-worth shape used to announce
// "Recent charges at this merchant").
func sparklineSVG(series []int64, alt string) ui.Node {
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
		Attr("role", "img"), Attr("aria-label", alt),
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
	// C520: the DIRECTION of an existing transaction used to be immutable. The
	// form parsed the amount, rejected anything <= 0, then re-applied the original
	// sign — so a charge imported as income could never be corrected to a spend
	// without deleting and re-entering it. Correcting a mistake is the whole point
	// of an edit form, so direction is now a control rather than a constant.
	//
	// Transfers are excluded: their sign is structural (which leg of the move this
	// is), not a classification the user should flip here.
	dirOut := txnDirOut
	if txn.Amount.IsPositive() {
		dirOut = txnDirIn
	}
	directionS := ui.UseState(dirOut)

	catS := ui.UseState(txn.CategoryID)
	dateS := ui.UseState(dateISO)
	memberS := ui.UseState(txn.MemberID)
	tagsS := ui.UseState(strings.Join(txn.Tags, ", "))
	clearedS := ui.UseState(txn.Cleared)
	noteS := ui.UseState(txn.Note)                  // TXC-2: free-text memo
	excludeS := ui.UseState(txn.ExcludeFromReports) // TXC-1: keep in balance, drop from budgets/reports
	// The movement classifier: which of your OWN accounts this row moved to or
	// from, and (for a card or loan) whether it counts as the payment. An import
	// files every row as plain income or spending because that is all a bank line
	// says; until this control existed a row could only be BORN a transfer, never
	// become one, so a statement full of savings sweeps and card payments read as
	// thousands of dollars of earning and spending that never happened.
	transferToS := ui.UseState(txn.TransferAccountID)
	debtPayS := ui.UseState(txn.BillAccountID != "")
	// Which account this row is POSTED to — distinct from the classifier above,
	// which names the account on the OTHER side of a move. An import can land a row
	// on the wrong account, and every other fact about it was correctable here
	// while this one was not: the only fix was to delete the row and retype it,
	// discarding its links, splits, history and id.
	accountS := ui.UseState(txn.AccountID)
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
	onDebtPay := ui.UseEvent(func(e ui.Event) { debtPayS.Set(e.IsChecked()) })

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
		// The amount field holds a magnitude; the direction control holds the sign.
		//
		// C547: rejecting a typed minus was the last live piece of Cam's original
		// report. C520 gave the form a Direction control, but the obvious way to
		// turn an income back into a spend — type a minus in front of the number —
		// still hit `amt <= 0` and produced "Enter an amount greater than zero",
		// which reads as the app refusing to let you fix your own mistake. A typed
		// minus is now read as intent: it sets the direction to OUT and the control
		// follows, so the form never shows "Money in" over a stored expense.
		want := amountmath.Out
		if directionS.Get() == txnDirIn {
			want = amountmath.In
		}
		amt, resolved, err := amountmath.ParseSigned(amountS.Get(), currency.Decimals(t.Amount.Currency), want)
		if err != nil {
			// C561: the field is text now, so junk reaches here that type="number"
			// used to swallow. "Enter an amount greater than zero" is the right
			// answer for a zero and the WRONG one for "abc" — ParseSigned separates
			// the two exactly so each can say what it means.
			if errors.Is(err, amountmath.ErrZeroAmount) {
				errMsg.Set(uistate.T("transactions.positiveAmount"))
			} else {
				errMsg.Set(uistate.T("transactions.amountNotANumber"))
			}
			return
		}
		// A transfer keeps whatever sign it already had — that is which leg of the
		// move this row is, not a classification to flip. Its direction control is
		// not offered, so a typed sign has nothing to drive and is ignored here.
		if t.IsTransfer() {
			if amt < 0 {
				amt = -amt
			}
			if t.Amount.IsNegative() {
				amt = -amt
			}
		}
		// NOTE: the direction control is synced AFTER the write, not here. Calling
		// a state setter mid-handler re-renders the component while this closure is
		// still running, and the rest of the save (including PutTransaction) can be
		// lost — the save then does nothing at all, with no error and no toast.
		// Compute now, write, then sync the control.
		syncDirectionOut := !t.IsTransfer() && resolved != want
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateS.Get()))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		t.Desc = strings.TrimSpace(descS.Get())
		t.Payee = strings.TrimSpace(payeeS.Get())
		t.Amount = money.New(amt, t.Amount.Currency)
		// C579: picking a category by hand is the user taking ownership of the
		// classification, so record it. Without this the row they just filed keeps
		// its "auto" mark and goes on telling them a machine chose it — the
		// correction journey ends with the ledger contradicting the correction.
		// Narrow on purpose (see txnprov.ConfirmsCategory): only a CHANGE to a real
		// category counts, so fixing a typo in the amount does not silently vouch for
		// a category the user never looked at.
		confirmsCat := txnprov.ConfirmsCategory(txn, domain.Transaction{CategoryID: catS.Get()})
		if confirmsCat {
			t.Reviewed = true
		}
		t.CategoryID = catS.Get()
		t.Date = date
		if memberS.Get() != "" {
			t.MemberID = memberS.Get()
		}
		t.Tags = textutil.CommaFields(tagsS.Get())
		// C601: a confirmed category also leaves the review queue — exactly what the
		// same decision does in the review surface, where assignReviewCategory has
		// stripped this tag all along. The edit path did not, so correcting a flagged
		// charge here recorded the fix and left the charge flagged: still queued,
		// still in the "Flagged" quick filter, still counted in "Review inbox (N)",
		// permanently. Two paths for one decision have to agree on what it means.
		//
		// It runs AFTER the form's tags are applied, not beside the Reviewed flag
		// above — that line reassigns t.Tags wholesale from the field, so stripping
		// earlier would be silently undone. (And tagsS is deliberately not written
		// here: a state setter mid-handler re-renders the component while this
		// closure is still running, and the rest of the save can be lost with it.)
		if confirmsCat {
			t.Tags = removeReviewTag(t.Tags)
		}
		t = t.MarkCleared(clearedS.Get(), time.Now())
		t.Note = strings.TrimSpace(noteS.Get())
		t.ExcludeFromReports = excludeS.Get()
		// The movement classification is applied AFTER the amount, deliberately.
		// The sign of this row was decided above by the direction control (or, for
		// a row that was already a transfer, held to the leg it is). Classifying
		// changes what the row MEANS to income, spending and the debt pages — it
		// never touches the money, so it must not run before the money is settled.
		//
		// The debt-payment claim is only sent when the chosen account is actually a
		// debt; the checkbox is hidden otherwise, but a stale tick could survive a
		// counterparty change within one open modal, and txnclassify rejects that
		// combination rather than half-applying it.
		wantDebtPay := debtPayS.Get() && isLiabilityAccount(app, transferToS.Get())
		classified, cerr := txnclassify.Apply(t, transferToS.Get(), wantDebtPay, app.Accounts())
		if cerr != nil {
			errMsg.Set(cerr.Error())
			return
		}
		t = classified
		// C58: an amount edit must not silently desync an existing category breakdown —
		// budgets attribute per split line, so a mismatched total would misreport. Block
		// and point at the split section below instead of quietly dropping the split.
		if t.HasSplits() && !t.SplitsReconcile() {
			errMsg.Set(uistate.T("transactions.splitAmountMismatch"))
			return
		}
		// Reposting to another account goes through txnmove rather than assigning
		// AccountID here, because two things have to travel with it: a transfer
		// leg's partner names THIS row's account as its counterparty and would
		// otherwise be left pointing at an account the money is no longer on, and a
		// currency mismatch has to be refused rather than saved — balances drop the
		// whole account when a row does not fold into its currency.
		var movedPartner domain.Transaction
		haveMovedPartner := false
		if accountS.Get() != "" && accountS.Get() != t.AccountID {
			plan := txnmove.PlanMove(t, accountS.Get(), app.Accounts(), app.Transactions())
			if plan.Err != nil {
				// Same reasoning as the inline note: say it in the user's terms, not
				// the package's. This path is reachable when the picker changed and the
				// save came before the preview re-rendered.
				switch plan.Reason {
				case txnmove.ReasonCurrency:
					errMsg.Set(uistate.T("transactions.accountMoveCurrency", t.Amount.Currency, plan.ToName))
				case txnmove.ReasonSelfTransfer:
					errMsg.Set(uistate.T("transactions.accountMoveSelf", plan.ToName))
				default:
					errMsg.Set(uistate.T("transactions.accountMoveRefused", plan.ToName))
				}
				return
			}
			moved, partner, hasPartner, merr := txnmove.Apply(t, accountS.Get(), app.Accounts(), app.Transactions())
			if merr != nil {
				errMsg.Set(uistate.T("transactions.accountMoveRefused", plan.ToName))
				return
			}
			t, movedPartner, haveMovedPartner = moved, partner, hasPartner
		}
		// C629: a transfer is two rows. Writing only the one the user opened left the
		// other account stale and the pair no longer summing to zero, so pass the
		// pre-edit copy too and let appstate keep the counterpart in step.
		if err := app.PutTransactionWithTransferPair(txn, t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// The partner follows the move. Written after the row itself so a failure
		// on the first leaves the pair as it was rather than half-relocated.
		if haveMovedPartner {
			if err := app.PutTransaction(movedPartner); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		// C33-style learn tally: record the payee→category correction on an explicit set.
		if t.CategoryID != "" {
			learn := t.Payee
			if learn == "" {
				learn = t.Desc
			}
			uistate.IncrementLearnTally(learn, t.CategoryID)
		}
		// C547: now that the write has landed, bring the control into agreement
		// with what was stored. Doing this earlier re-rendered the form mid-save
		// and lost the write.
		if syncDirectionOut {
			directionS.Set(txnDirOut)
		}
		uistate.BumpDataRevision()
		// #63: the saved confirmation names the consequence — where the account's
		// balance now stands — instead of a bare "updated".
		saveMsg := uistate.T("toast.txnUpdated")
		if acc, ok := domain.AccountByID(app.Accounts(), t.AccountID); ok {
			if b, berr := ledger.Balance(acc, app.Transactions()); berr == nil {
				saveMsg = uistate.T("transactions.savedImpact", acc.Name,
					money.FormatMinor(b.Amount, currency.Decimals(acc.Currency)))
			}
		}
		uistate.PostNotice(saveMsg, false)

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
			// C579: pressing "Recategorize them" IS the person deciding, for every row
			// it touches — so each one is confirmed, exactly as a single hand-edit is.
			// Without this the rows the user just filed by hand keep their "auto" mark
			// and go on crediting a rule, which is the contradiction ConfirmsCategory
			// exists to prevent, reproduced one code path over.
			if txnprov.ConfirmsCategory(t, domain.Transaction{CategoryID: target.CategoryID}) {
				t.Reviewed = true
				// C601, third path: and it leaves the review queue. The single-row save
				// above strips this tag and assignReviewCategory always has; this bulk
				// sibling did not, so a flagged charge swept up by "Recategorize them"
				// kept its flag — still queued, still counted in "Review inbox (N)" —
				// after the user had explicitly decided it. Three code paths write this
				// one decision and all three now mean the same thing by it.
				t.Tags = removeReviewTag(t.Tags)
			}
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
		// A STACK, not a form-grid. As a grid this block became three auto-fit columns
		// — the question in the first, the preview in the second, and all three answer
		// buttons crushed into a 150px third, rendering as "categori them" / "No
		// thanks" clipped mid-word. It is a question followed by evidence followed by
		// answers; that is a column, and the answers wrap rather than shrink.
		return Div(ClassStr("txn-recat-offer"), Attr("data-testid", "txn-recat-offer"), Attr("role", "status"),
			P(css.Class("t-body"), offer),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), rows),
			Div(css.Class("txn-recat-actions", tw.Flex, tw.ItemsCenter, tw.Gap2),
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
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), OnInput(onDesc), uiw.FieldValue(descS.Get()))),
		labeledField(uistate.T("transactions.payeeLabel"),
			Fragment(
				Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.payeeLabel")), Placeholder(uistate.T("transactions.payeeLabel")), OnInput(onPayee), uiw.FieldValue(payeeS.Get())),
				// Parity: the ORIGINAL statement text stays visible beside the
				// cleaned merchant — when the resolver (alias table + cleanup)
				// displays this transaction under a different name, say what the
				// bank actually printed, so categorization stays debuggable.
				func() ui.Node {
					raw := strings.TrimSpace(firstNonEmpty(txn.Payee, txn.Desc))
					shown := strings.TrimSpace(app.PayeeResolver().Resolve(raw))
					if raw == "" || shown == "" || strings.EqualFold(raw, shown) {
						return Fragment()
					}
					return P(css.Class("muted", tw.TextXs, tw.Mt05), Attr("data-testid", "txn-original-statement"),
						uistate.T("transactions.originalStatement", raw))
				}(),
			)),
		// C561: a TEXT field with inputmode=decimal, NOT type=number with min=0.
		//
		// This input silently defeated C547. The Save button is a submit button bound
		// to this form (`FormID: "txn-edit-form"`), so the browser runs constraint
		// validation before submitting — and a typed "-32" is out of range for
		// min="0". Submission was blocked, `OnSubmit` never fired, and the app's save
		// handler never ran: no error banner, no toast, no change, nothing to see.
		// The form appeared to accept the edit and silently discard it, which is
		// worse than the rejection C547 removed.
		//
		// The magnitude rule now lives in amountmath.ParseSigned, which understands a
		// sign; expressing it as an HTML range constraint contradicted that and could
		// only fail silently. type=number was wrong for a second reason too: browsers
		// report value === "" for a partially-typed number (a lone "-"), so the
		// keystroke never reaches the Go state. Quick-add already used text +
		// inputmode=decimal, which is exactly why C546 worked there and C547 did not
		// work here.
		labeledField(uistate.T("transactions.amountPlaceholder"),
			Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"),
				Attr("data-testid", "txn-edit-amount"),
				Placeholder(uistate.T("transactions.amountPlaceholder")), OnInput(onAmount), uiw.FieldValue(amountS.Get()))),
		// Direction sits beside the amount because together they ARE the amount.
		If(!txn.IsTransfer(), labeledField(uistate.T("transactions.directionLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: []uiw.SelectOption{
					{Value: txnDirOut, Label: uistate.T("transactions.directionOut")},
					{Value: txnDirIn, Label: uistate.T("transactions.directionIn")},
				},
				Selected:  directionS.Get(),
				OnChange:  func(v string) { directionS.Set(v) },
				AriaLabel: uistate.T("transactions.directionLabel"),
				TestID:    "txn-edit-direction",
			}))),
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
							Placeholder(uistate.T("transactions.newCategoryName")), OnInput(onNewCatName), OnKeyDown(onNewCatKey), uiw.FieldValue(newCatName.Get())),
						Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txn-edit-newcat-add"),
							OnClick(addCat), uistate.T("transactions.newCategoryAdd")),
						Button(css.Class("btn btn-ghost"), Type("button"),
							OnClick(toggleAddCat), uistate.T("transactions.newCategoryCancel")))))),
		labeledField(uistate.T("transactions.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), OnInput(onDate), uiw.FieldValue(dateS.Get()))),
		labeledField(uistate.T("transactions.tagsLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.tagsLabel")), Placeholder(uistate.T("transactions.tagsPlaceholder")), OnInput(onTags), uiw.FieldValue(tagsS.Get()))),
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
		// Which account the row is POSTED to. Sits above the movement classifier
		// because it answers a more basic question — whose ledger is this even on —
		// and because confusing the two is easy: one names this row's account, the
		// other names the account on the far side of a move.
		accountField(app, txn, accountS.Get(), func(v string) { accountS.Set(v) }),
		// The movement classifier sits directly above "exclude from budgets and
		// reports" because the two answer the same question — should this row count
		// as income or spending — and a person who reaches for the blunt exclude
		// checkbox to silence a transfer should see the precise control first.
		classifyField(app, txn, transferToS.Get(), debtPayS.Get(),
			func(v string) {
				transferToS.Set(v)
				// A counterparty that cannot be paid down cannot carry a payment
				// claim; drop the tick with the choice so the two never disagree.
				if !isLiabilityAccount(app, v) {
					debtPayS.Set(false)
				}
			}, onDebtPay),
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
		// C520(b): this used to render at the bottom of a scrolling body, out of
		// sight of the Save button that produced it — an error the user cannot see
		// is the same as no error at all. Sticky to the foot of the body, so it is
		// adjacent to the action whatever the scroll position. Sticky rather than an
		// effect-driven scrollIntoView because UseEffect does not run for a
		// component passed as a FlipPanel prop.
		If(errMsg.Get() != "", Div(css.Class("form-err-sticky"), errText("txn-edit-err", errMsg.Get()))),
		// Delete stays in the body (left-aligned, apart from the panel footer's
		// Cancel/Save): the FlipPanel's pinned .set-foot owns Cancel + Save now (via
		// FormID → native submit), fixing the old floating mid-panel button row.
		//
		// C580: but it is now STICKY to the foot of the scrolling body, the same
		// treatment the error message got. In a plain block it sat at whatever depth
		// the form happened to end — below the fold on a transaction with a merchant
		// story, above it on a bare one — so the one destructive action in the dialog
		// had no fixed place, while the two safe actions were pinned and always
		// visible. Now it is always in view, always in the same spot, and separated
		// from Save by the full width of the dialog.
		Div(css.Class("txn-edit-danger"),
			Button(css.Class("btn btn-del"), Type("button"), Attr("data-testid", "txn-edit-delete"), OnClick(del), uistate.T("action.delete")),
		),
	)
}

// Transaction direction values for the edit form's sign control. They are the
// form's own vocabulary, not persisted — the stored truth is the amount's sign.
const (
	txnDirOut = "out"
	txnDirIn  = "in"
)

// isLiabilityAccount reports whether accountID names a card or loan, so the edit
// modal offers the debt-payment claim only where paying something down is a
// thing that can happen. An empty or unknown id is not a liability.
func isLiabilityAccount(app *appstate.App, accountID string) bool {
	if accountID == "" {
		return false
	}
	acc, ok := domain.AccountByID(app.Accounts(), accountID)
	return ok && acc.Class == domain.ClassLiability
}

// accountField renders the "which account is this on" picker — the correction
// for a row that arrived on the wrong one.
//
// Every other fact about an imported row was already editable here; this was not,
// so the only fix for a mis-import was to delete the row and retype it, throwing
// away its links, splits, history and id.
//
// It previews rather than only validating on save, because the two consequences
// worth knowing about are both invisible from the picker: that a transfer's other
// leg comes along, and that moving a cleared row disturbs a reconciled balance. A
// refusal (a currency the account cannot hold) also shows here rather than after
// the button.
func accountField(app *appstate.App, txn domain.Transaction, selected string, onSelect func(string)) ui.Node {
	accounts := acctsort.ForPicker(app.Accounts())
	if len(accounts) < 2 {
		// One account is not a choice, and a picker with a single option is a
		// question with one answer.
		return Fragment()
	}
	opts := make([]uiw.SelectOption, 0, len(accounts))
	for _, a := range accounts {
		opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
	}

	note := Fragment()
	if selected != "" && selected != txn.AccountID {
		p := txnmove.PlanMove(txn, selected, accounts, app.Transactions())
		switch {
		case p.Err != nil:
			// The REASON, not the error. p.Err says "txnmove: row is USD, account
			// Travel Card (EUR) is EUR" — which names a Go package at somebody who
			// only wanted to fix a mis-import, and explains the failure in terms of
			// the code rather than of their money.
			msg := uistate.T("transactions.accountMoveRefused", p.ToName)
			switch p.Reason {
			case txnmove.ReasonCurrency:
				msg = uistate.T("transactions.accountMoveCurrency", txn.Amount.Currency, p.ToName)
			case txnmove.ReasonSelfTransfer:
				msg = uistate.T("transactions.accountMoveSelf", p.ToName)
			}
			note = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-account-note"),
				Attr("role", "status"), msg)
		default:
			lines := []ui.Node{Span(uistate.T("transactions.accountMoveEffect", p.FromName, p.ToName))}
			if p.HasPartner {
				lines = append(lines, Span(" "+uistate.T("transactions.accountMovePartner")))
			}
			if p.BreaksReconciliation {
				lines = append(lines, Span(" "+uistate.T("transactions.accountMoveReconciled", p.FromName)))
			}
			note = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-account-note"),
				Attr("role", "status"), lines)
		}
	}

	return Div(css.Class("txn-account-field", tw.FlexCol, tw.Gap1), Attr("data-testid", "txn-edit-account-field"),
		uiw.FormField(uistate.T("transactions.accountLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   opts,
				Selected:  selected,
				AriaLabel: uistate.T("transactions.accountLabel"),
				OnChange:  onSelect,
				TestID:    "txn-edit-account",
			})),
		note,
	)
}

// classifyField renders the movement classifier: the counterparty picker, the
// consequence of the current choice, and — for a card or loan — the claim that
// this row IS the payment.
//
// It is a plain function rather than a component because every interactive child
// gets its handler from the caller's stable hooks; introducing a component here
// would buy nothing and add a render boundary between the control and the form
// state it writes.
func classifyField(app *appstate.App, txn domain.Transaction, selected string, debtPay bool,
	onSelect func(string), onDebtPay ui.Handler) ui.Node {
	// Sorted for the PICKER, not left in store order. txnclassify.Counterparties
	// preserves whatever order it is given, so the ordering decision belongs here.
	choices := txnclassify.Counterparties(txn, acctsort.ForPicker(app.Accounts()))
	if len(choices) == 0 {
		// Nothing to move money to or from. A picker whose only option is "none"
		// is a question with one answer, so it is not asked.
		return Fragment()
	}
	opts := make([]uiw.SelectOption, 0, len(choices)+1)
	opts = append(opts, uiw.SelectOption{Value: "", Label: uistate.T("transactions.classifyNone")})
	for _, c := range choices {
		opts = append(opts, uiw.SelectOption{Value: c.AccountID, Label: c.Name})
	}

	// C674: the far side, and what this save is actually worth.
	//
	// Classifying an imported leg never creates its counterpart — inventing a third
	// transaction would invent money — so the far side is usually another imported
	// row waiting for the same treatment. Whether one was FOUND changes what the
	// user should expect, and saying nothing is how a one-sided transfer gets
	// discovered later, from a balance that disagrees with the bank.
	//
	// The preview runs Apply's own validation without writing, so it can never
	// promise a save that will be refused.
	pairNote := Fragment()
	if selected != "" {
		pv := txnclassify.PreviewApply(txn, selected, debtPay && isLiabilityAccount(app, selected),
			app.Accounts(), app.Transactions())
		switch {
		case pv.Err != nil:
			// Shown here rather than only on submit: a control that looks available
			// and fails on click is worse than one that explains itself first.
			pairNote = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-pair"),
				Attr("role", "status"), pv.Err.Error())
		case pv.HasPartner && pv.Partner.Exact:
			pairNote = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-pair"),
				Attr("role", "status"), uistate.T("transactions.classifyPairFound", pv.CounterpartyName))
		case pv.HasPartner:
			// Same two accounts and dates, different magnitude — a fee usually. Worth
			// showing as a near match rather than claiming a clean pair.
			pairNote = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-pair"),
				Attr("role", "status"), uistate.T("transactions.classifyPairNear", pv.CounterpartyName,
					fmtMoney(pv.Partner.Txn.Amount)))
		default:
			pairNote = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-pair"),
				Attr("role", "status"), uistate.T("transactions.classifyPairNone", pv.CounterpartyName))
		}
	}

	// Name the consequence of the CURRENT selection, so what the save will do is
	// readable before it happens rather than inferable afterwards.
	effect := Fragment()
	debtClaim := Fragment()
	if acc, ok := domain.AccountByID(app.Accounts(), selected); ok {
		if acc.Class == domain.ClassLiability {
			effect = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-effect"),
				uistate.T("transactions.classifyEffectDebt", acc.Name))
			debtClaim = Div(css.Class(tw.FlexCol, tw.Gap1),
				Label(css.Class("txn-check"),
					Input(Type("checkbox"), Attr("data-testid", "txn-edit-classify-debt"),
						Attr("aria-label", uistate.T("transactions.classifyDebtLabel", acc.Name)),
						CheckedIf(debtPay), OnChange(onDebtPay)),
					Span(uistate.T("transactions.classifyDebtLabel", acc.Name))),
				Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyDebtHint")))
		} else {
			effect = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-effect"),
				uistate.T("transactions.classifyEffectNeutral"))
		}
	}

	// C673: a titled GROUP, not a lone field. "Other account this money moved to or
	// from" named a field and left the reader to work out what filling it in would
	// do — so it read as optional metadata sitting beside the category, which is
	// precisely how a row ends up filed under a category called "Transfer" while
	// still counting as spending. The heading names the action; the picker is the
	// question that action needs answered; the sign note is here because the minus
	// on this leg is not evidence of spending.
	return Div(css.Class("txn-classify-field", tw.FlexCol, tw.Gap1), Attr("data-testid", "txn-edit-classify-field"),
		Attr("role", "group"), Attr("aria-label", uistate.T("transactions.classifyGroup")),
		H4(css.Class("set-label"), Attr("data-testid", "txn-edit-classify-heading"),
			uistate.T("transactions.classifyGroup")),
		Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyGroupHint")),
		uiw.FormField(uistate.T("transactions.classifyLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   opts,
				Selected:  selected,
				AriaLabel: uistate.T("transactions.classifyLabel"),
				OnChange:  onSelect,
				TestID:    "txn-edit-classify",
			})),
		effect,
		pairNote,
		Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyHint")),
		// Shown only once the row IS a transfer: before that there is no leg to
		// explain, and the note would be answering a question nobody asked.
		If(selected != "", Span(css.Class("muted", tw.Text12),
			Attr("data-testid", "txn-edit-classify-sign"),
			uistate.T("transactions.classifySignHint"))),
		debtClaim,
	)
}
