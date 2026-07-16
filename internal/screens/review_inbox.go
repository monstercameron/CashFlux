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

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/payeeclean"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
func sameMerchantQueued(txns []domain.Transaction, key, exceptID string) int {
	if strings.TrimSpace(key) == "" {
		return 0
	}
	n := 0
	for _, t := range reviewqueue.Queue(txns) {
		if t.ID != exceptID && strings.EqualFold(strings.TrimSpace(rawPayeeOf(t)), key) {
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
// since categorizing resolves it) and persists.
func assignReviewCategory(app *appstate.App, txnID, catID string) {
	for _, t := range app.Transactions() {
		if t.ID == txnID {
			t.CategoryID = catID
			t.Tags = removeReviewTag(t.Tags)
			_ = app.PutTransaction(t)
			uistate.BumpDataRevision()
			return
		}
	}
}

// assignReviewByMerchant categorizes every queued transaction sharing the merchant
// key (the current one included) in one pass, so a repeated charge clears in a
// single action.
func assignReviewByMerchant(app *appstate.App, key, catID string) {
	app.BulkMutate(func() {
		for _, t := range app.Transactions() {
			if !reviewqueue.Needs(t) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(rawPayeeOf(t)), key) {
				t.CategoryID = catID
				t.Tags = removeReviewTag(t.Tags)
				_ = app.PutTransaction(t)
			}
		}
	})
	uistate.BumpDataRevision()
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
	seededFor := ui.UseState("~none~")
	selVal := ui.UseState("")
	alsoSimilar := ui.UseState(false)
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")

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
		cats := app.Categories()
		catIDByName := make(map[string]string, len(cats))
		var catList strings.Builder
		for _, c := range cats {
			catIDByName[c.Name] = c.ID
			catList.WriteString(c.Name + "\n")
		}
		curID := cur.ID
		lines := "1 | " + strings.TrimSpace(cur.Payee+" — "+cur.Desc) + " | " + fmtMoney(cur.Amount)
		aiLoading.Set(true)
		aiErr.Set("")
		runSmartAI(aiConn, smartai.AutoCategorize(lines, catList.String()),
			func(text string) {
				parsed := smartai.ParseCategoryAssignments(text, 1, catIDByName)
				if len(parsed) > 0 && parsed[0].CategoryID != "" {
					assignReviewCategory(app, curID, parsed[0].CategoryID)
					seededFor.Set("~none~")
				} else {
					aiErr.Set(uistate.T("review.aiNoMatch"))
				}
				aiLoading.Set(false)
			},
			func(e string) { aiErr.Set(e); aiLoading.Set(false) })
	})

	onSelect := ui.UseEvent(func(e ui.Event) { selVal.Set(e.GetValue()) })
	toggleSimilar := ui.UseEvent(func() { alsoSimilar.Set(!alsoSimilar.Get()) })
	commit := ui.UseEvent(func() {
		cur, ok := firstToReview(app.Transactions(), skipped.Get())
		if !ok {
			return
		}
		v := selVal.Get()
		if v == "" {
			return
		}
		if alsoSimilar.Get() {
			assignReviewByMerchant(app, strings.TrimSpace(rawPayeeOf(cur)), v)
		} else {
			assignReviewCategory(app, cur.ID, v)
		}
		alsoSimilar.Set(false)
		seededFor.Set("~none~")
	})
	applySuggest := ui.UseEvent(func() {
		if cur, ok := firstToReview(app.Transactions(), skipped.Get()); ok {
			if sug := app.AutoCategorizeTransaction(cur); sug.CategoryID != "" {
				assignReviewCategory(app, cur.ID, sug.CategoryID)
				seededFor.Set("~none~")
			}
		}
	})
	skip := ui.UseEvent(func() {
		if cur, ok := firstToReview(app.Transactions(), skipped.Get()); ok {
			seededFor.Set("~none~")
			skipped.Set(append(append([]string{}, skipped.Get()...), cur.ID))
		}
	})
	closeInbox := ui.UseEvent(func() { uistate.CloseReviewInbox() })

	if app == nil {
		return Fragment()
	}
	if open.Get() && !opened.Get() {
		total.Set(reviewqueue.Count(app.Transactions()))
		skipped.Set(nil)
		seededFor.Set("~none~")
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
		seededFor.Set(cur.ID)
	}

	reason := reviewqueue.ReasonFor(cur)
	reasonLabel := uistate.T("review.reasonUncategorized")
	reasonMod := "is-uncat"
	if reason == reviewqueue.ReasonFlagged {
		reasonLabel = uistate.T("review.reasonFlagged")
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
	pos := total.Get() - work + 1
	if pos < 1 {
		pos = 1
	}
	if pos > total.Get() {
		pos = total.Get()
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

	// One-click deterministic suggestion, when it beats the current state.
	var suggNode ui.Node
	if sug := app.AutoCategorizeTransaction(cur); sug.CategoryID != "" && sug.CategoryID != cur.CategoryID {
		suggNode = Button(css.Class("rvw-suggest"), Type("button"), Attr("data-testid", "review-suggest"), OnClick(applySuggest),
			uiw.Icon(icon.Check, css.Class(tw.W4, tw.H4)),
			Span(uistate.T("review.suggested", reviewCatName(app, sug.CategoryID))))
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
	if sc := sameMerchantQueued(app.Transactions(), rawPayee, cur.ID); sc > 0 {
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

	return Div(css.Class("rvw"), Attr("data-testid", "review-inbox"),
		// Progress: count + "N left" and a slim track.
		Div(css.Class("rvw-progress"),
			Span(css.Class("rvw-progress-count"), Attr("data-testid", "review-progress"),
				uistate.T("review.progress", pos, total.Get())+" · "+uistate.T("review.leftCount", left)),
			Div(css.Class("rvw-progress-track"),
				Div(css.Class("rvw-progress-fill"), Attr("style", progressWidth(pos, total.Get()))),
			),
		),
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
		// Actions: primary confirm, then skip; close lives in the header X.
		Div(css.Class("rvw-actions"),
			Button(css.Class(commitCls), Type("button"), Attr("data-testid", "review-commit"), OnClick(commit),
				uistate.T("review.categorizeNext")),
			Button(css.Class("btn btn-ghost"), Type("button"), Attr("data-testid", "review-skip"), OnClick(skip),
				uistate.T("review.skip")),
		),
	)
}

// progressWidth is the inline style for the progress fill bar.
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
