// SPDX-License-Identifier: MIT

//go:build js && wasm

// Transaction Review inbox (CG-S2): a focused triage flow — opened from the
// transactions toolbar — that steps through the transactions still needing a
// human look (uncategorized, or flagged #needs-review) one at a time. For each,
// the user picks a category and confirms ("Categorize & next"), optionally
// applying the same category to every other queued charge from the same merchant
// in one go; accepts a deterministic suggestion; or skips it for now. Choosing a
// category does NOT auto-commit — a deliberate confirm avoids the classic
// select-slip footgun. The pure selection lives in internal/reviewqueue.
package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/payeeclean"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/rulesuggest"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// firstToReview returns the newest transaction still needing review that hasn't
// been skipped this session, so the flow always shows the next actionable item.
func firstToReview(txns []domain.Transaction, skips []string) (domain.Transaction, bool) {
	skset := make(map[string]bool, len(skips))
	for _, s := range skips {
		skset[s] = true
	}
	for _, t := range reviewqueue.Queue(txns) {
		if !skset[t.ID] {
			return t, true
		}
	}
	return domain.Transaction{}, false
}

// workingCount is how many still-reviewable (non-skipped) transactions remain.
func workingCount(txns []domain.Transaction, skips []string) int {
	skset := make(map[string]bool, len(skips))
	for _, s := range skips {
		skset[s] = true
	}
	n := 0
	for _, t := range reviewqueue.Queue(txns) {
		if !skset[t.ID] {
			n++
		}
	}
	return n
}

// sameMerchantQueued counts OTHER queued (still-needing-review) transactions that
// share a merchant key with the given one — the "N others from this payee" the
// user can categorize in one action.
// C498: both the COUNT shown and the batch that acts on it go through
// reviewqueue.MerchantKey. They used to compare the raw descriptor with
// strings.EqualFold while the card displayed the CLEANED name, so two charges
// shown as "Amazon" did not batch together and the hint promised a grouping the
// action would not honour.
func sameMerchantQueued(txns []domain.Transaction, key, exceptID string) int {
	if strings.TrimSpace(key) == "" {
		return 0
	}
	n := 0
	for _, t := range reviewqueue.Queue(txns) {
		if t.ID != exceptID && reviewqueue.MerchantKey(t) == key {
			n++
		}
	}
	return n
}

// removeReviewTag returns tags without the review flag.
func removeReviewTag(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != reviewqueue.ReviewTag {
			out = append(out, t)
		}
	}
	return out
}

// assignReviewCategory sets one transaction's category (clearing the review flag,
// since categorizing resolves it) and persists. A failed write (e.g. a read-only
// Viewer identity) is surfaced as a notice — swallowing it left the inbox frozen
// on the same item with zero feedback (QA CF-02). Returns whether the write
// landed, so callers can offer an undo toast only for real changes.
func assignReviewCategory(app *appstate.App, txnID, catID string) bool {
	for _, t := range app.Transactions() {
		if t.ID == txnID {
			t.CategoryID = catID
			t.Tags = removeReviewTag(t.Tags)
			if err := app.PutTransaction(t); err != nil {
				uistate.PostNotice(err.Error(), true)
				uistate.BumpDataRevision()
				return false
			}
			// Learn from the confirmation, so the next charge from this merchant
			// is suggested locally instead of costing a SMART+ call (C513).
			rememberReviewChoice(t, catID)
			// C553: PutTransaction writes the in-memory store; RequestPersist is
			// what puts it in the dataset. Without this the card advanced and the
			// queue count dropped, but a reload brought the transaction back
			// Uncategorized — the write looked like it landed and had not. Same
			// defect as C543 on the categories page, in a second surface.
			uistate.RequestPersist()
			uistate.BumpDataRevision()
			return true
		}
	}
	return false
}

// assignReviewByMerchant categorizes every queued transaction sharing the merchant
// key (the current one included) in one pass, so a repeated charge clears in a
// single action. Returns how many transactions were written (0 when the write
// failed), so callers can offer an undo toast only for real changes.
func assignReviewByMerchant(app *appstate.App, key, catID string) int {
	var writeErr error
	n := 0
	app.BulkMutate(func() {
		for _, t := range app.Transactions() {
			if !reviewqueue.Needs(t) {
				continue
			}
			if reviewqueue.MerchantKey(t) == key {
				t.CategoryID = catID
				t.Tags = removeReviewTag(t.Tags)
				if err := app.PutTransaction(t); err != nil {
					if writeErr == nil {
						writeErr = err
					}
					continue
				}
				rememberReviewChoice(t, catID)
				n++
			}
		}
	})
	if writeErr != nil {
		uistate.PostNotice(writeErr.Error(), true)
	}
	// C553: persist the batch for the same reason the single path does — an
	// in-memory write that never reaches the dataset is undone by a reload.
	if n > 0 {
		uistate.RequestPersist()
	}
	uistate.BumpDataRevision()
	return n
}

// postCategorizedUndo captures an undo point for the categorization that just
// landed and shows an undoable toast naming the category, so a slip in the
// review flow is reversible in one click (report item: review loop needs undo).
func postCategorizedUndo(app *appstate.App, catID string, batch int) {
	auditview.CaptureNow()
	name := reviewCatName(app, catID)
	if batch > 1 {
		uistate.PostUndoable(uistate.T("review.categorizedBatchUndo", batch, name))
		return
	}
	uistate.PostUndoable(uistate.T("review.categorizedUndo", name))
}

func reviewCatName(app *appstate.App, id string) string {
	if id == "" {
		return uistate.T("review.uncategorized")
	}
	for _, c := range app.Categories() {
		if c.ID == id {
			return c.Name
		}
	}
	return uistate.T("review.uncategorized")
}

func reviewAcctName(app *appstate.App, id string) string {
	for _, a := range app.Accounts() {
		if a.ID == id {
			return a.Name
		}
	}
	return ""
}

// reviewRulesReady is the "N ready-made rules could file many of these" lead-in for
// the inbox's cross-link to /rules, correctly singular/plural.
func reviewRulesReady(n int) string {
	if n == 1 {
		return uistate.T("review.rulesReadyOne")
	}
	return uistate.T("review.rulesReadyMany", n)
}

// ReviewInboxBody is the body of the review-inbox flip modal, mounted at the
// shell root by app.ReviewInboxHost. It owns its controls (the FlipPanel is
// NoFooter), stepping through the live queue.
func ReviewInboxBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	open := uistate.UseReviewInbox()
	pr := uistate.UsePrefs().Get()

	// SMART+ availability: a configured AI provider (bring-your-own-key or backend
	// proxy). When present, the modal offers an "AI category" button that asks the
	// model to pick from the user's EXISTING categories.
	backendAI := pr.Normalize().BackendActive()
	hasProvider := app != nil && aiProviderConfigured(app, backendAI)
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)

	// All hooks declared unconditionally, before any early return.
	skipped := ui.UseState([]string{})
	total := ui.UseState(0)
	opened := ui.UseState(false)
	// startedAt stamps when this review session opened (#63): with a few
	// reviews done, the progress line adds a remaining-time estimate.
	startedAt := ui.UseState(int64(0))
	seededFor := ui.UseState("~none~")
	selVal := ui.UseState("")
	alsoSimilar := ui.UseState(false)
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	commitErr := ui.UseState("")

	// aiCategorize (SMART+): send just the current transaction + the existing
	// category list to the model, parse the answer against the REAL categories (so
	// it can't invent one), and apply + advance. Explicit click, so instant-apply
	// is fine (no select-slip risk).
	aiCategorize := ui.UseEvent(func() {
		if aiLoading.Get() {
			return
		}
		cur, ok := firstToReview(app.Transactions(), skipped.Get())
		if !ok {
			return
		}
		catalog := smartCatalog(app.Categories())
		// Capture the transaction AND the batch flag at CLICK time: the callback
		// runs later, and the user's intent is what they set before pressing the
		// button, not whatever the checkbox happens to hold when the reply lands.
		curTxn := cur
		batch := alsoSimilar.Get()
		lines := "1 | " + strings.TrimSpace(cur.Payee+" — "+cur.Desc) + " | " + fmtMoney(cur.Amount)
		aiLoading.Set(true)
		aiErr.Set("")
		// Reject a sign-mismatched answer before it can be written (C490).
		incomeByRef := map[int]bool{1: curTxn.Amount.Amount > 0}
		runSmartAI(aiConn, smartai.AutoCategorize(lines, catalog.Prompt()),
			func(text string) {
				parsed := smartai.RejectSignMismatches(
					smartai.ParseCategoryAssignments(text, 1, catalog), incomeByRef)
				if len(parsed) > 0 && parsed[0].CategoryID != "" {
					// C553: an AI-proposed category that failed to save must not
					// advance either — same rule as the manual confirm.
					if !applyReviewChoice(app, curTxn, parsed[0].CategoryID, batch) {
						commitErr.Set(uistate.T("review.commitFailed"))
						return
					}
					alsoSimilar.Set(false)
					seededFor.Set("~none~")
				} else {
					aiErr.Set(uistate.T("review.aiNoMatch"))
				}
				aiLoading.Set(false)
			},
			func(e string) { aiErr.Set(e); aiLoading.Set(false) })
	})

	onSelect := ui.UseEvent(func(e ui.Event) { selVal.Set(e.GetValue()); commitErr.Set("") })
	toggleSimilar := ui.UseEvent(func() { alsoSimilar.Set(!alsoSimilar.Get()) })
	commit := ui.UseEvent(func() {
		cur, ok := firstToReview(app.Transactions(), skipped.Get())
		if !ok {
			return
		}
		v := selVal.Get()
		if v == "" {
			// QA CF-02: an unarmed confirm used to no-op with zero feedback — an
			// automation tool (or a missed change event) then saw a "stuck" inbox.
			commitErr.Set(uistate.T("review.chooseFirst"))
			return
		}
		commitErr.Set("")
		// C553: advance only on a write that landed. A failed write used to move
		// the card on anyway, so the item looked resolved and came back on the
		// next reload. assignReviewCategory posts the underlying reason as a
		// notice; this states it beside the action that produced it.
		if !applyReviewChoice(app, cur, v, alsoSimilar.Get()) {
			commitErr.Set(uistate.T("review.commitFailed"))
			return
		}
		alsoSimilar.Set(false)
		seededFor.Set("~none~")
	})
	applySuggest := ui.UseEvent(func() {
		cur, ok := firstToReview(app.Transactions(), skipped.Get())
		if !ok {
			return
		}
		sug, has := reviewSuggestion(app, cur)
		if !has || sug.CategoryID == "" {
			return
		}
		// Same rule as the manual confirm (C553): accepting a suggestion that
		// failed to save must not advance.
		if !applyReviewChoice(app, cur, sug.CategoryID, alsoSimilar.Get()) {
			commitErr.Set(uistate.T("review.commitFailed"))
			return
		}
		alsoSimilar.Set(false)
		seededFor.Set("~none~")
	})
	skip := ui.UseEvent(func() {
		if cur, ok := firstToReview(app.Transactions(), skipped.Get()); ok {
			seededFor.Set("~none~")
			skipped.Set(append(append([]string{}, skipped.Get()...), cur.ID))
		}
	})
	closeInbox := ui.UseEvent(func() { uistate.CloseReviewInbox() })
	// CG-S2 cross-link: jump to /rules where the ready-made rule suggestions live,
	// closing the inbox first so we don't return to a stale modal.
	nav := router.UseNavigate()
	gotoRules := ui.UseEvent(func() {
		uistate.CloseReviewInbox()
		nav.Navigate(uistate.RoutePath("/rules"))
	})

	if app == nil {
		return Fragment()
	}
	if open.Get() && !opened.Get() {
		total.Set(reviewqueue.Count(app.Transactions()))
		skipped.Set(nil)
		seededFor.Set("~none~")
		startedAt.Set(time.Now().Unix())
		opened.Set(true)
	}
	if !open.Get() && opened.Get() {
		opened.Set(false)
	}
	if !open.Get() {
		return Fragment()
	}

	cur, has := firstToReview(app.Transactions(), skipped.Get())

	// All caught up.
	if !has {
		skips := len(skipped.Get())
		sub := uistate.T("review.allDoneClean")
		if skips > 0 {
			sub = uistate.T("review.allDoneSkipped", skips)
		}
		return Div(css.Class("rvw rvw-done"), Attr("data-testid", "review-inbox"),
			Div(css.Class("rvw-done-icon"), uiw.Icon(icon.CheckCircle, css.Class(tw.W8, tw.H8))),
			P(css.Class("rvw-done-title"), uistate.T("review.allDoneTitle")),
			P(css.Class("rvw-done-sub"), sub),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "review-done"), OnClick(closeInbox),
				uistate.T("review.done")),
		)
	}

	// Reseed per-item controls when the current item changes.
	if seededFor.Get() != cur.ID {
		selVal.Set(cur.CategoryID)
		alsoSimilar.Set(false)
		aiErr.Set("")
		commitErr.Set("")
		seededFor.Set(cur.ID)
	}

	// C600: phrased as the REASON this charge is queued ("Queued: no category
	// yet"), so it cannot be read as a claim about the row while the Category
	// control beside it already shows a suggestion.
	reason := reviewqueue.ReasonFor(cur)
	reasonLabel := uistate.T("review.reasonLead", uistate.T("review.reasonUncategorized"))
	reasonMod := "is-uncat"
	if reason == reviewqueue.ReasonFlagged {
		reasonLabel = uistate.T("review.reasonLead", uistate.T("review.reasonFlagged"))
		reasonMod = "is-flagged"
	}

	rawPayee := strings.TrimSpace(rawPayeeOf(cur))
	cleanPayee := payeeclean.Suggest(rawPayee)
	if cleanPayee == "" {
		cleanPayee = rawPayee
	}
	amountMod := "is-expense"
	if cur.Amount.Amount >= 0 {
		amountMod = "is-income"
	}

	work := workingCount(app.Transactions(), skipped.Get())
	// C497: recount live rather than trusting the snapshot taken on open — a rule
	// firing, a sync pull or another tab all change the queue underneath us.
	liveTotal := reviewqueue.Count(app.Transactions()) + len(skipped.Get())
	if liveTotal < work {
		liveTotal = work
	}
	pos := liveTotal - work + 1
	if pos < 1 {
		pos = 1
	}
	if pos > liveTotal {
		pos = liveTotal
	}
	left := work - 1
	if left < 0 {
		left = 0
	}

	// Category picker (choosing arms the confirm button; it does NOT auto-commit).
	catOpts := []any{css.Class("field"), Attr("data-testid", "review-category-select"),
		Attr("aria-label", uistate.T("review.categoryLabel")), OnChange(onSelect),
		Option(Value(""), SelectedIf(selVal.Get() == ""), uistate.T("review.choose"))}
	for _, c := range app.Categories() {
		catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(selVal.Get() == c.ID), c.Name))
	}

	// One-click SMART suggestion (local: rule → history → merchant dictionary),
	// with the evidence behind it so the user can judge rather than trust it.
	var suggNode ui.Node
	if sug, ok := reviewSuggestion(app, cur); ok && sug.CategoryID != cur.CategoryID {
		why := reviewWhy(sug)
		suggNode = Button(css.Class("rvw-suggest"), Type("button"), Attr("data-testid", "review-suggest"),
			Attr("data-source", sug.Source.String()), OnClick(applySuggest),
			uiw.Icon(icon.Check, css.Class(tw.W4, tw.H4)),
			Span(uistate.T("review.suggested", reviewCatName(app, sug.CategoryID))),
			If(why != "", Span(css.Class("rvw-suggest-why"), Attr("data-testid", "review-suggest-why"), why)))
	}

	// SMART+ AI category button — asks the model to pick an existing category for
	// this transaction. Shown only when an AI provider is configured.
	var aiBtn ui.Node
	if hasProvider {
		label := uistate.T("review.aiCategory")
		if aiLoading.Get() {
			label = uistate.T("review.aiThinking")
		}
		aiArgs := []any{css.Class("rvw-ai"), Type("button"), Attr("data-testid", "review-ai"), OnClick(aiCategorize)}
		if aiLoading.Get() {
			aiArgs = append(aiArgs, Attr("aria-disabled", "true"))
		}
		aiArgs = append(aiArgs, smartGlyph(aiLoading.Get(), tw.Fold(tw.W4, tw.H4)), Span(label))
		aiBtn = Button(aiArgs...)
	}

	// "Also apply to N others from this merchant" — turns a repeated charge into a
	// single action, so a 200-item backlog of the same payee clears fast.
	var similarNode ui.Node
	if sc := sameMerchantQueued(app.Transactions(), reviewqueue.MerchantKey(cur), cur.ID); sc > 0 {
		cbArgs := []any{Type("checkbox"), OnChange(toggleSimilar)}
		if alsoSimilar.Get() {
			cbArgs = append(cbArgs, Attr("checked", "checked"))
		}
		similarNode = Label(css.Class("rvw-similar"), Attr("data-testid", "review-similar"),
			Input(cbArgs...),
			Span(uistate.T("review.alsoApply", sc, cleanPayee)),
		)
	}

	commitCls := "btn btn-primary rvw-commit"
	if selVal.Get() == "" {
		commitCls += " is-disabled"
	}

	// A quiet cross-link to /rules when ready-made rule suggestions exist: triaging
	// 250 charges one at a time is slow if a handful of rules could file most of
	// them. Same deterministic source as the /rules "Suggestions ready" stat.
	var rulesLink ui.Node = Fragment()
	if n := len(rulesuggest.Suggest(app.Transactions(), app.Rules(), 3)); n > 0 {
		rulesLink = P(css.Class("rvw-rules-link"),
			Span(reviewRulesReady(n)),
			Text(" — "),
			Button(css.Class("rvw-rules-link-btn"), Type("button"),
				Attr("data-testid", "review-inbox-rules-link"), OnClick(gotoRules),
				uistate.T("review.rulesReadyLink")),
		)
	}

	return Div(css.Class("rvw"), Attr("data-testid", "review-inbox"),
		// Progress: count + "N left" and a slim track.
		Div(css.Class("rvw-progress"),
			Span(css.Class("rvw-progress-count"), Attr("data-testid", "review-progress"),
				uistate.T("review.progress", pos, liveTotal)+" · "+uistate.T("review.leftCount", left)+
					reviewPaceSuffix(startedAt.Get(), pos-1, left)),
			Div(css.Class("rvw-progress-track"),
				Div(css.Class("rvw-progress-fill"), Attr("style", progressWidth(pos, liveTotal))),
			),
		),
		rulesLink,
		// The transaction under review.
		Div(css.Class("rvw-card"),
			Div(css.Class("rvw-card-top"),
				Span(css.Class("rvw-reason "+reasonMod), reasonLabel),
				Span(css.Class("rvw-date"), pr.FormatDate(cur.Date)),
			),
			Div(css.Class("rvw-payee"), Attr("data-testid", "review-payee"), cleanPayee),
			If(cleanPayee != rawPayee, Div(css.Class("rvw-rawpayee"), rawPayee)),
			Div(css.Class("rvw-meta"),
				Span(css.Class("rvw-amount "+amountMod), fmtMoney(cur.Amount)),
				If(reviewAcctName(app, cur.AccountID) != "", Span(css.Class("rvw-acct"), reviewAcctName(app, cur.AccountID))),
			),
		),
		// Category picker + suggestion (SMART) + AI (SMART+) + apply-to-similar.
		Div(css.Class("rvw-assign"),
			Div(css.Class("rvw-assign-label"), uistate.T("review.categoryLabel")),
			Select(catOpts...),
			If(suggNode != nil || aiBtn != nil,
				Div(css.Class("rvw-sugg-row"),
					If(suggNode != nil, suggNode),
					If(aiBtn != nil, aiBtn),
				)),
			If(aiErr.Get() != "", P(css.Class("rvw-ai-err"), Attr("role", "alert"), Attr("data-testid", "review-ai-err"), aiErr.Get())),
			If(similarNode != nil, similarNode),
		),
		// Actions: primary confirm, then skip; close lives in the header X. The
		// confirm stays fully clickable while unarmed — no disabled/aria-disabled,
		// which automation tools refuse to click — so an unarmed click can explain
		// itself (the alert below) instead of silently doing nothing (QA CF-02).
		If(commitErr.Get() != "", P(css.Class("rvw-ai-err"), Attr("role", "alert"), Attr("data-testid", "review-commit-err"), commitErr.Get())),
		Div(css.Class("rvw-actions"),
			Button(css.Class(commitCls), Type("button"), Attr("data-testid", "review-commit"), OnClick(commit),
				uistate.T("review.categorizeNext")),
			Button(css.Class("btn btn-ghost"), Type("button"), Attr("data-testid", "review-skip"), OnClick(skip),
				uistate.T("review.skip")),
		),
	)
}

// progressWidth is the inline style for the progress fill bar.
// reviewPaceSuffix estimates the remaining review time from this session's own
// pace (#63): after at least three reviews it appends "≈ N min left at this
// pace". Silent before that — an estimate from one datapoint would be noise.
func reviewPaceSuffix(startedUnix int64, reviewed, left int) string {
	if startedUnix <= 0 || reviewed < 3 || left <= 0 {
		return ""
	}
	elapsed := time.Now().Unix() - startedUnix
	if elapsed <= 0 {
		return ""
	}
	mins := (float64(elapsed) / float64(reviewed) * float64(left)) / 60
	est := int(mins + 0.5)
	if est < 1 {
		est = 1
	}
	return " · " + uistate.T("review.paceEstimate", est)
}

func progressWidth(pos, total int) string {
	pct := 0
	if total > 0 {
		pct = pos * 100 / total
	}
	if pct > 100 {
		pct = 100
	}
	return "width:" + strconv.Itoa(pct) + "%"
}
