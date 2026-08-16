// SPDX-License-Identifier: MIT

//go:build js && wasm

// The dual-mode review surface (C500–C512): one place to categorize, replacing
// two modals that wrote the same field through two different undo mechanisms.
//
// Bulk mode is the thesis: a queue of 250 charges is not 250 decisions, it is
// ~30 merchant decisions, tiered by how confident the matcher is. Single mode is
// the same queue one charge at a time, for when the context matters more than
// the throughput. Both share one panel size so switching never resizes the
// container, and one pinned footer so the primary action is always reachable.
package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Review modes.
const (
	reviewModeSingle = "single"
	reviewModeBulk   = "bulk"
)

// reviewTier buckets a merchant group by the confidence of its suggestion.
type reviewTier int

const (
	tierNone  reviewTier = iota // nothing local could answer
	tierLook                    // suggested, but worth a glance
	tierReady                   // safe to confirm in bulk
)

// reviewRow is one merchant group with everything the UI needs, computed once
// per render. C510: building this ONCE and passing it down is what keeps a
// 250-row list from being O(n²) — sibling counts, suggestions and merchant
// lookups all used to be recomputed per row.
type reviewRow struct {
	Group      reviewqueue.Group
	Tier       reviewTier
	Suggestion catsuggest.Suggestion
	HasSugg    bool
}

// reviewIndex is the whole surface's data for one render.
type reviewIndex struct {
	Rows  []reviewRow
	Total int // charges still queued
	Byid  map[string]reviewRow
}

// tierOf maps a resolver confidence onto a display tier.
func tierOf(s catsuggest.Suggestion, ok bool) reviewTier {
	if !ok || s.CategoryID == "" {
		return tierNone
	}
	if s.Confidence >= catsuggest.ConfHigh {
		return tierReady
	}
	return tierLook
}

// buildReviewIndex resolves every queued charge ONCE, groups by merchant, and
// ranks the groups. Suggestions are resolved per merchant rather than per charge
// — every charge in a group shares a payee, so the answer is the same and the
// work is O(groups) instead of O(charges).
func buildReviewIndex(app *appstate.App) reviewIndex {
	idx := reviewIndex{Byid: map[string]reviewRow{}}
	if app == nil {
		return idx
	}
	res := uistate.LoadReviewResolutions()
	txns := app.Transactions()
	queue := reviewqueue.QueueOpen(txns, res, time.Now())
	idx.Total = len(queue)

	// Rank each charge by the confidence of its merchant's suggestion.
	suggByKey := map[string]catsuggest.Suggestion{}
	okByKey := map[string]bool{}
	ranked := make([]reviewqueue.Ranked, 0, len(queue))
	for _, t := range queue {
		k := reviewqueue.MerchantKey(t)
		if _, seen := okByKey[k]; !seen {
			s, ok := reviewSuggestion(app, t)
			suggByKey[k], okByKey[k] = s, ok
		}
		conf := 0
		if okByKey[k] {
			conf = int(suggByKey[k].Confidence)
		}
		ranked = append(ranked, reviewqueue.Ranked{Txn: t, Confidence: conf})
	}

	for _, g := range reviewqueue.GroupByMerchant(ranked) {
		s, ok := suggByKey[g.Key], okByKey[g.Key]
		row := reviewRow{Group: g, Tier: tierOf(s, ok), Suggestion: s, HasSugg: ok}
		idx.Rows = append(idx.Rows, row)
		idx.Byid[g.Key] = row
	}
	return idx
}

// ReviewSurfaceBody is the body of the review modal. The panel is NoFooter +
// FlushBody, so this owns both the scrolling region and the pinned action bar.
func ReviewSurfaceBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	open := uistate.UseReviewInbox()
	pr := uistate.UsePrefs().Get()

	// All hooks unconditional, before any early return.
	mode := ui.UseState(reviewModeBulk)
	selected := ui.UseState(map[string]bool{})
	openGroups := ui.UseState(map[string]bool{})
	picker := ui.UseState("") // merchant key whose picker is open, "" = closed
	// Bumped whenever the picker closes. A <select> is a CONTROLLED input the
	// browser also mutates: choosing the "+ New category" sentinel sets the live
	// value, and re-rendering the same node does not put it back, so the row was
	// left displaying a sentinel that is not a category. Re-keying the rows on
	// this counter recreates them and restores the real value.
	pickerRev := ui.UseState(0)
	manual := ui.UseState(map[string]string{})
	cursor := ui.UseState(0)
	focusRow := ui.UseState(0) // bulk: which merchant row the keyboard is on
	notice := ui.UseState("")
	// SMART+ scan state (C504). Proposals live apart from `manual` so a model
	// answer is never mistaken for something the user chose by hand.
	scanState := ui.UseState("idle")
	scanErr := ui.UseState("")
	aiProposals := ui.UseState(map[string]string{})
	scanStats := ui.UseState([2]int{})

	setMode := func(m string) { mode.Set(m); notice.Set("") }
	onSingle := ui.UseEvent(func() { setMode(reviewModeSingle) })
	onBulk := ui.UseEvent(func() { setMode(reviewModeBulk) })
	closeSurface := ui.UseEvent(func() { uistate.CloseReviewInbox() })

	toggleGroup := func(k string) {
		m := openGroups.Get()
		nm := make(map[string]bool, len(m)+1)
		for kk, v := range m {
			nm[kk] = v
		}
		nm[k] = !nm[k]
		openGroups.Set(nm)
	}
	toggleSelect := func(k string) {
		m := selected.Get()
		nm := make(map[string]bool, len(m)+1)
		for kk, v := range m {
			nm[kk] = v
		}
		nm[k] = !nm[k]
		selected.Set(nm)
	}
	setManual := func(k, catID string) {
		if catID == catSelectNew {
			picker.Set(k)
			return
		}
		m := manual.Get()
		nm := make(map[string]string, len(m)+1)
		for kk, v := range m {
			nm[kk] = v
		}
		nm[k] = catID
		manual.Set(nm)
	}
	clearSel := ui.UseEvent(func() { selected.Set(map[string]bool{}) })
	nav := router.UseNavigate()

	if app == nil {
		return Fragment()
	}
	if !open.Get() {
		ClearReviewKeys()
		return Fragment()
	}

	idx := buildReviewIndex(app)

	// catFor resolves what a group will be categorized as: a hand edit wins over
	// the suggestion, always.
	catFor := func(r reviewRow) string {
		if v, ok := manual.Get()[r.Group.Key]; ok {
			return v
		}
		if r.HasSugg {
			return r.Suggestion.CategoryID
		}
		// SMART+ is consulted LAST — only for what the free sources could not
		// answer, which is what keeps the paid pass small (C515 precedence).
		if v, ok := aiProposals.Get()[r.Group.Key]; ok {
			return v
		}
		return ""
	}

	// applySelected writes every selected group in ONE bulk mutation with a
	// single undo point, so a 40-merchant confirm is one reversible step.
	doApply := func() {
		sel := selected.Get()
		if len(sel) == 0 {
			return
		}
		// Seal whatever came before into its own undo entry, so this batch does
		// not merge with it.
		auditview.CaptureNow()
		total := 0
		app.BulkMutate(func() {
			for _, r := range idx.Rows {
				if !sel[r.Group.Key] {
					continue
				}
				cat := catFor(r)
				if cat == "" {
					continue
				}
				total += assignReviewByMerchant(app, r.Group.Key, cat)
			}
		})
		if total > 0 {
			// C508: seal THIS batch immediately too. Undo points are otherwise
			// captured on the autosave tick, so two confirms inside one 4s window
			// collapsed into a single entry and the earlier batch became
			// unreachable — one-level undo wearing a multi-step coat.
			auditview.CaptureNow()
			uistate.PostUndoable(uistate.T("review.bulkApplied", total))
			notice.Set(uistate.T("review.bulkApplied", total))
		}
		selected.Set(map[string]bool{})
		uistate.BumpDataRevision()
	}
	applySelected := ui.UseEvent(doApply)

	// ---- SMART+ scan (C504/C509) ---------------------------------------------
	// Scanned set = merchants the LOCAL sources could not answer. Paying a model
	// to re-derive what a rule already knows is the waste this avoids.
	gapRows := make([]reviewRow, 0)
	for _, r := range idx.Rows {
		if !r.HasSugg {
			gapRows = append(gapRows, r)
		}
	}
	gapCharges := 0
	for _, r := range gapRows {
		gapCharges += len(r.Group.Items)
	}

	backendAI := pr.Normalize().BackendActive()
	hasProvider := aiProviderConfigured(app, backendAI)
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	catalog := smartCatalog(app.Categories())

	doScan := func() {
		if !hasProvider {
			// Land on the AI tab, not the settings default. Sending someone to a
			// page that says nothing about keys, after closing the surface they
			// were working in, is why "Connect a key" read as a button that does
			// nothing. Settings tabs are routable as /settings/<tab>.
			uistate.CloseReviewInbox()
			nav.Navigate(uistate.RoutePath("/settings/ai"))
			return
		}
		if len(gapRows) == 0 || scanState.Get() == "scanning" {
			return
		}
		// Clear a previous verdict so "Scan again" visibly restarts rather than
		// leaving the old result on screen.
		scanStats.Set([2]int{})
		// One representative charge per MERCHANT, not per charge: every charge in
		// a group shares a payee, so asking about all of them buys nothing and
		// costs linearly.
		sample := gapRows
		if len(sample) > smartCatScanCap {
			sample = sample[:smartCatScanCap]
		}
		var lines strings.Builder
		incomeByRef := make(map[int]bool, len(sample))
		for i, r := range sample {
			t := r.Group.Items[0]
			lines.WriteString(strconv.Itoa(i+1) + " | " + strings.TrimSpace(r.Group.Merchant) +
				" | " + fmtMoney(t.Amount) + "\n")
			incomeByRef[i+1] = t.Amount.Amount > 0
		}
		scanState.Set("scanning")
		scanErr.Set("")
		runSmartAI(aiConn, smartai.AutoCategorize(lines.String(), catalog.Prompt()),
			func(text string) {
				parsed := smartai.RejectSignMismatches(
					smartai.ParseCategoryAssignments(text, len(sample), catalog), incomeByRef)
				out := map[string]string{}
				for k, v := range aiProposals.Get() {
					out[k] = v
				}
				filled := 0
				for _, a := range parsed {
					if a.Ref < 1 || a.Ref > len(sample) || a.CategoryID == "" {
						continue
					}
					out[sample[a.Ref-1].Group.Key] = a.CategoryID
					filled++
				}
				aiProposals.Set(out)
				scanStats.Set([2]int{filled, len(sample) - filled})
				scanState.Set("done")
				uistate.BumpDataRevision()
			},
			func(e string) { scanErr.Set(e); scanState.Set("done"); uistate.BumpDataRevision() })
	}
	scan := ui.UseEvent(doScan)

	// Accept the scan's answers: they move into `manual` (so they are applied and
	// marked) and their merchants are selected, leaving Confirm as the next click.
	useScan := ui.UseEvent(func() {
		props := aiProposals.Get()
		if len(props) == 0 {
			return
		}
		nm := make(map[string]string, len(manual.Get())+len(props))
		for k, v := range manual.Get() {
			nm[k] = v
		}
		ns := make(map[string]bool, len(selected.Get())+len(props))
		for k, v := range selected.Get() {
			ns[k] = v
		}
		for k, v := range props {
			// Never overwrite a hand edit with a model answer.
			if _, edited := manual.Get()[k]; edited {
				continue
			}
			nm[k], ns[k] = v, true
		}
		manual.Set(nm)
		selected.Set(ns)
		notice.Set(uistate.T("review.filledN", len(props)))
	})

	// C506: the fastest way to clear a queue is to stop charges entering it.
	// One action both files the visible batch AND writes the rule that files the
	// next one — the user is already looking at a merchant-grouped set, so the
	// match text and the category are both already decided.
	doMakeRules := func() {
		sel := selected.Get()
		if len(sel) == 0 {
			return
		}
		made := 0
		for _, r := range idx.Rows {
			if !sel[r.Group.Key] || len(r.Group.Items) == 0 {
				continue
			}
			cat := catFor(r)
			if cat == "" {
				continue
			}
			// Match on the CLEANED merchant name: the raw descriptor carries a
			// per-charge reference, so a rule built from it would match exactly
			// one transaction and never fire again.
			match := strings.TrimSpace(r.Group.Merchant)
			if match == "" {
				continue
			}
			if err := app.PutRule(rules.Rule{
				ID: id.New(), Match: match, SetCategoryID: cat, Order: 1000 + made,
			}); err != nil {
				uistate.PostNotice(err.Error(), true)
				continue
			}
			made++
		}
		if made > 0 {
			notice.Set(uistate.T("review.rulesMade", made))
			uistate.RequestPersist()
		}
		// Apply the batch too: making a rule should not leave the charges that
		// prompted it sitting in the queue.
		doApply()
	}
	makeRules := ui.UseEvent(doMakeRules)

	selectTier := func(t reviewTier) {
		m := make(map[string]bool, len(idx.Rows))
		for k, v := range selected.Get() {
			m[k] = v
		}
		for _, r := range idx.Rows {
			if r.Tier == t && catFor(r) != "" {
				m[r.Group.Key] = true
			}
		}
		selected.Set(m)
	}
	selReady := ui.UseEvent(func() { selectTier(tierReady) })
	selLook := ui.UseEvent(func() { selectTier(tierLook) })

	// ---- single mode actions -------------------------------------------------
	singleRows := idx.Rows
	at := cursor.Get()
	if at >= len(singleRows) {
		at = 0
	}
	var cur domain.Transaction
	var curRow reviewRow
	if len(singleRows) > 0 && len(singleRows[at].Group.Items) > 0 {
		curRow = singleRows[at]
		cur = curRow.Group.Items[0]
	}
	advance := func() {
		cursor.Set(cursor.Get() + 1)
		notice.Set("")
	}
	doConfirm := func() {
		if cur.ID == "" {
			return
		}
		cat := catFor(curRow)
		if cat == "" {
			notice.Set(uistate.T("review.chooseFirst"))
			return
		}
		applyReviewChoice(app, cur, cat, true)
		advance()
	}
	confirmOne := ui.UseEvent(doConfirm)
	// Single mode's "new category" button targets whichever merchant is on
	// screen; curRow is settled by this point.
	newCatSingle := ui.UseEvent(func() {
		if curRow.Group.Key != "" {
			picker.Set(curRow.Group.Key)
		}
	})
	// Single mode acts on a MERCHANT, not one charge: confirm already applies to
	// the whole group, so snooze and dismiss do too. Acting on one charge of a
	// 122-charge merchant left 121 behind and read as a broken button.
	doSkip := func() {
		// C493: a durable snooze, not an in-memory skip that vanishes on reload.
		until := time.Now().AddDate(0, 0, 7)
		for _, t := range curRow.Group.Items {
			uistate.SnoozeReviewItem(t.ID, until)
		}
		uistate.BumpDataRevision()
		advance()
	}
	skipOne := ui.UseEvent(doSkip)
	doDismiss := func() {
		for _, t := range curRow.Group.Items {
			uistate.DismissReviewItem(t.ID)
		}
		uistate.BumpDataRevision()
		advance()
	}
	dismissOne := ui.UseEvent(doDismiss)

	// ---- keyboard (C507) -----------------------------------------------------
	// The hint in the header promises these; a surface that advertises keys it
	// does not implement is worse than one with none.
	clampFocus := func(n int) int {
		if len(idx.Rows) == 0 {
			return 0
		}
		if n < 0 {
			return len(idx.Rows) - 1
		}
		if n >= len(idx.Rows) {
			return 0
		}
		return n
	}
	keys := reviewKeyActions{
		Bulk:   func() { setMode(reviewModeBulk) },
		Single: func() { setMode(reviewModeSingle) },
	}
	if isBulkMode := mode.Get() == reviewModeBulk; isBulkMode {
		keys.Next = func() { focusRow.Set(clampFocus(focusRow.Get() + 1)) }
		keys.Prev = func() { focusRow.Set(clampFocus(focusRow.Get() - 1)) }
		keys.Toggle = func() {
			if r := clampFocus(focusRow.Get()); r < len(idx.Rows) {
				toggleSelect(idx.Rows[r].Group.Key)
			}
		}
		keys.Confirm = doApply
	} else {
		keys.Next = func() { cursor.Set(cursor.Get() + 1) }
		keys.Prev = func() {
			if cursor.Get() > 0 {
				cursor.Set(cursor.Get() - 1)
			}
		}
		keys.Confirm = doConfirm
		keys.Skip = doSkip
		keys.Dismiss = doDismiss
	}
	setReviewKeys(keys)

	// ---- render --------------------------------------------------------------
	if idx.Total == 0 {
		return Div(css.Class("rvs rvs-done"), Attr("data-testid", "review-inbox"),
			P(css.Class("rvw-done-title"), uistate.T("review.emptyTitle")),
			P(css.Class("rvw-done-sub"), uistate.T("review.emptySub")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "review-done"),
				OnClick(closeSurface), uistate.T("review.done")),
		)
	}

	isBulk := mode.Get() == reviewModeBulk
	kbdHint := uistate.T("review.keyboardHintSingle")
	if isBulk {
		kbdHint = uistate.T("review.keyboardHintBulk")
	}
	head := Div(css.Class("rvs-head"),
		uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.modeLabel"),
			Selected: mode.Get(),
			Options: []uiw.SegOption{
				{Value: reviewModeSingle, Label: uistate.T("review.modeSingle"), TestID: "review-mode-single"},
				{Value: reviewModeBulk, Label: uistate.T("review.modeBulk"), TestID: "review-mode-bulk"},
			},
			OnSelect: setMode,
		}),
		Span(css.Class("rvs-kbd"), Attr("data-testid", "review-kbd"), kbdHint),
	)
	_, _ = onSingle, onBulk

	var body, foot ui.Node
	if isBulk {
		body, foot = reviewBulkView(app, idx, selected.Get(), openGroups.Get(), manual.Get(),
			catFor, toggleGroup, toggleSelect, setManual, selReady, selLook,
			applySelected, clearSel, makeRules, notice.Get(), pr, clampFocus(focusRow.Get()),
			ui.CreateElement(reviewScanStrip, reviewScanStripProps{
				Gaps: len(gapRows), GapCharges: gapCharges, State: scanState.Get(),
				Filled: scanStats.Get()[0], Skipped: scanStats.Get()[1], Err: scanErr.Get(),
				HasProvider: hasProvider, OnScan: scan, OnUse: useScan,
				CanUse: len(aiProposals.Get()) > 0,
			}), pickerRev.Get())
	} else {
		body, foot = reviewSingleView(app, idx, curRow, cur, catFor, setManual,
			confirmOne, skipOne, dismissOne, newCatSingle, notice.Get(), pr)
	}

	var pick ui.Node = Fragment()
	if k := picker.Get(); k != "" {
		kind := domain.KindExpense
		if r, ok := idx.Byid[k]; ok && len(r.Group.Items) > 0 && r.Group.Items[0].Amount.Amount > 0 {
			kind = domain.KindIncome
		}
		pick = ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind:     kind,
			Selected: manual.Get()[k],
			OnPick:   func(catID string) { setManual(k, catID); picker.Set("") },
			OnClose:  func() { picker.Set("") },
		})
	}

	return Div(css.Class("rvs"), Attr("data-testid", "review-inbox"),
		// LOAD-BEARING, not diagnostic: without a changed prop on the root the
		// reconciler skipped diffing the keyed merchant rows, so moving the
		// keyboard focus updated no class. It doubles as the value a test can
		// assert on without reaching into the list.
		Attr("data-focus", strconv.Itoa(clampFocus(focusRow.Get()))),
		head,
		Div(css.Class("rvs-body"), body),
		Div(css.Class("rvs-foot modal-foot"), foot),
		pick,
	)
}

// fmtTierCount renders "N merchants · M charges" for a tier header.
func fmtTierCount(rows []reviewRow, t reviewTier) (merchants, charges int) {
	for _, r := range rows {
		if r.Tier != t {
			continue
		}
		merchants++
		charges += len(r.Group.Items)
	}
	return
}

func tierLabel(t reviewTier) string {
	switch t {
	case tierReady:
		return uistate.T("review.tierReady")
	case tierLook:
		return uistate.T("review.tierLook")
	}
	return uistate.T("review.tierNone")
}

func tierMod(t reviewTier) string {
	switch t {
	case tierReady:
		return "is-ready"
	case tierLook:
		return "is-look"
	}
	return "is-none"
}

// reviewBulkView renders the tiered merchant list and its pinned footer.
func reviewBulkView(
	app *appstate.App, idx reviewIndex,
	sel map[string]bool, openG map[string]bool, manual map[string]string,
	catFor func(reviewRow) string,
	toggleGroup, toggleSelect func(string), setManual func(string, string),
	selReady, selLook, applySel, clearSel, makeRules any, notice string, pr prefs.Prefs, focused int,
	scanStrip ui.Node, rev int,
) (ui.Node, ui.Node) {
	decisions := len(idx.Rows)

	var tiers []ui.Node
	for _, t := range []reviewTier{tierReady, tierLook, tierNone} {
		m, c := fmtTierCount(idx.Rows, t)
		if m == 0 {
			continue
		}
		rows := make([]reviewRow, 0, m)
		focusKey := ""
		if focused >= 0 && focused < len(idx.Rows) {
			focusKey = idx.Rows[focused].Group.Key
		}
		for _, r := range idx.Rows {
			if r.Tier == t {
				rows = append(rows, r)
			}
		}
		var act ui.Node = Fragment()
		switch t {
		case tierReady:
			act = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-selectall-ready"), OnClick(selReady),
				uistate.T("review.selectAll", m))
		case tierLook:
			act = Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "review-selectall-look"), OnClick(selLook),
				uistate.T("review.selectAll", m))
		}
		tiers = append(tiers, Div(css.Class("rvs-tier "+tierMod(t)), Attr("data-tier", tierMod(t)),
			Attr("role", "group"), Attr("aria-label", tierLabel(t)),
			Div(css.Class("rvs-tier-head"),
				Span(css.Class("rvs-tier-mark")),
				Span(css.Class("rvs-tier-name"), tierLabel(t)),
				Span(css.Class("rvs-tier-count"), uistate.T("review.tierCount", plural(m, "merchant"), plural(c, "charge"))),
				Span(css.Class("rvs-tier-act"), act),
			),
			Div(css.Class("rvs-groups"),
				MapKeyed(rows, func(r reviewRow) any { return r.Group.Key + "#" + strconv.Itoa(rev) }, func(r reviewRow) ui.Node {
					return ui.CreateElement(reviewGroupRow, reviewGroupRowProps{
						Row: r, App: app, Selected: sel[r.Group.Key], Open: openG[r.Group.Key], Focused: r.Group.Key == focusKey,
						CategoryID: catFor(r), Manual: manual[r.Group.Key] != "",
						Prefs: pr, OnToggleOpen: toggleGroup, OnToggleSelect: toggleSelect,
						OnCategory: setManual,
					})
				}),
			),
		))
	}

	body := Div(
		Div(css.Class("rvs-collapse"),
			Span(css.Class("rvs-big"), Attr("data-testid", "review-total"), strconv.Itoa(idx.Total)),
			Span(css.Class("rvs-unit"), uistate.T("review.charges")),
			Span(css.Class("rvs-arrow"), "→"),
			Span(css.Class("rvs-big is-accent"), strconv.Itoa(decisions)),
			Span(css.Class("rvs-unit"), uistate.T("review.decisions")),
		),
		P(css.Class("rvs-note"), uistate.T("review.collapseNote")),
		scanStrip,
		Div(css.Class("rvs-legend"),
			Span(Span(css.Class("rvs-dot is-ready")), uistate.T("review.legendHigh")),
			Span(Span(css.Class("rvs-dot is-look")), uistate.T("review.legendMid")),
			Span(Span(css.Class("rvs-dot is-none")), uistate.T("review.legendNone")),
		),
		If(notice != "", P(css.Class("rvs-notice"), Attr("data-testid", "review-notice"),
			Attr("role", "status"), Attr("aria-live", "polite"), notice)),
		Fragment(nodesToAny(tiers)...),
	)

	// Footer: selection summary + the actions.
	selMerchants, selCharges := 0, 0
	for _, r := range idx.Rows {
		if sel[r.Group.Key] {
			selMerchants++
			selCharges += len(r.Group.Items)
		}
	}
	label := uistate.T("review.nothingPicked")
	sub := uistate.T("review.pickHint")
	confirm := uistate.T("review.confirm")
	if selMerchants > 0 {
		label = uistate.T("review.selCount", plural(selMerchants, "merchant"), plural(selCharges, "charge"))
		sub = uistate.T("review.undoStays", "")
		confirm = uistate.T("review.confirmN", selCharges)
	}
	foot := Fragment(
		Span(css.Class("rvs-foot-n"), Attr("data-testid", "review-selection"),
			Attr("role", "status"), Attr("aria-live", "polite"), label),
		Span(css.Class("rvs-foot-sub"), sub),
		Span(css.Class("rvs-foot-acts"),
			If(selMerchants > 0,
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-makerule"),
					OnClick(makeRules), uistate.T("review.makeRule"))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-clear"),
				OnClick(clearSel), uistate.T("review.clear")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-apply"), OnClick(applySel), confirm),
		),
	)
	return body, foot
}

// reviewSingleView renders one charge with its context band and pinned actions.
func reviewSingleView(
	app *appstate.App, idx reviewIndex, row reviewRow, cur domain.Transaction,
	catFor func(reviewRow) string, setManual func(string, string),
	confirmOne, skipOne, dismissOne, newCatBtn any, notice string, pr prefs.Prefs,
) (ui.Node, ui.Node) {
	if cur.ID == "" {
		return P(css.Class("rvs-note"), uistate.T("review.emptySub")), Fragment()
	}
	kind := domain.KindExpense
	if cur.Amount.Amount > 0 {
		kind = domain.KindIncome
	}
	selArgs := []any{css.Class("field"), Attr("data-testid", "review-category-select"),
		Attr("aria-label", uistate.T("review.categoryLabel")),
		OnChange(ui.UseEvent(func(e ui.Event) { setManual(row.Group.Key, e.GetValue()) }))}
	selArgs = append(selArgs, categorySelectNodes(app.Categories(), kind, catFor(row))...)

	reason := reviewqueue.ReasonFor(cur)
	body := Div(css.Class("rvs-single"),
		Div(
			Div(css.Class("rvw-card"),
				Div(css.Class("rvw-card-top"),
					Span(css.Class("rvw-reason is-uncat"), Attr("data-testid", "review-reason"),
						reviewReasonLabel(reason)),
					Span(css.Class("rvw-date"), pr.FormatDate(cur.Date)),
				),
				Div(css.Class("rvw-payee"), Attr("data-testid", "review-payee"), row.Group.Merchant),
				Div(css.Class("rvw-rawpayee"), rawPayeeOf(cur)),
				Div(css.Class("rvw-meta"),
					Span(css.Class("rvw-amount is-expense"), fmtMoney(cur.Amount)),
					If(reviewAcctName(app, cur.AccountID) != "",
						Span(css.Class("rvw-acct"), reviewAcctName(app, cur.AccountID))),
				),
			),
			Div(css.Class("rvs-assign"),
				Div(css.Class("rvw-assign-label"), uistate.T("review.categoryLabel")),
				Select(selArgs...),
				If(row.HasSugg, Div(css.Class("rvs-why"), Attr("data-testid", "review-suggest-why"),
					reviewWhy(row.Suggestion))),
				If(reason == reviewqueue.ReasonSplitUnbalanced,
					P(css.Class("rvs-warn"), Attr("data-testid", "review-split-warn"),
						uistate.T("review.splitNeeded"))),
				If(notice != "", P(css.Class("rvs-notice"), Attr("role", "alert"),
					Attr("data-testid", "review-commit-err"), notice)),
			),
		),
		ui.CreateElement(reviewContextBand, reviewContextBandProps{App: app, Row: row, Txn: cur, Prefs: pr}),
	)

	foot := Fragment(
		Span(css.Class("rvs-foot-sub"), Attr("data-testid", "review-progress"),
			uistate.T("review.leftCount", idx.Total)),
		Span(css.Class("rvs-hidden"), Attr("data-testid", "review-total"), strconv.Itoa(idx.Total)),
		Span(css.Class("rvs-foot-acts"),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-dismiss"),
				OnClick(dismissOne), uistate.T("review.dismiss")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-skip"),
				OnClick(skipOne), uistate.T("review.snooze")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-commit"), OnClick(confirmOne),
				uistate.T("review.categorizeNext")),
		),
	)
	return body, foot
}

func reviewReasonLabel(r reviewqueue.Reason) string {
	switch r {
	case reviewqueue.ReasonFlagged:
		return uistate.T("review.reasonFlagged")
	case reviewqueue.ReasonSplitUnbalanced:
		return uistate.T("review.reasonSplit")
	}
	return uistate.T("review.reasonUncategorized")
}
