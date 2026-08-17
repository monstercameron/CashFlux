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
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Review modes. The values live in uistate because the session that remembers
// which one you are in has to outlive this surface (C559).
const (
	reviewModeSingle = uistate.ReviewModeSingle
	reviewModeBulk   = uistate.ReviewModeBulk
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
	Total int // charges still queued IN THE ACTIVE SCOPE
	// QueueTotal and ViewTotal are the two denominators the scope selector offers
	// (C554): everything awaiting review, and the part of it the ledger's own
	// search/filters/period currently show. Both are computed every build so the
	// selector can state the count of the scope you are NOT in — a choice between
	// two options where only one has a number attached is not really a choice.
	QueueTotal int
	ViewTotal  int
	Byid       map[string]reviewRow
	// TxnByID lets the context band resolve link members without rescanning
	// every transaction per row (it was an O(links x transactions) nested loop).
	TxnByID map[string]domain.Transaction
	// Dupes maps a transaction id to the ids it looks like a duplicate of.
	// Computed once here rather than re-running duplicate detection over the
	// whole ledger on every render of the single-charge view.
	Dupes map[string][]string
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

// ledgerVisibleIDs is the ids of the transactions the ledger's own filters
// currently select, in ledger order.
//
// C554: Review used to read app.Transactions() and nothing else, so a ledger
// narrowed to one payee and one month opened a Review of the whole world. Rather
// than teach Review about search, tags, categories, members, periods and
// money-in/out, it asks txnfilter — the same code the ledger itself uses — and
// intersects by id. One implementation of "what am I looking at", so the two can
// never disagree.
//
// Pagination is deliberately dropped: paging is a property of the ledger's
// SCREEN, not of the working set. Someone who filtered to 300 charges and pressed
// Review means all 300, not the 25 on page one. (Apply does not paginate, so this
// is a statement of intent as much as a change — Page/PageSize are cleared so a
// future paginating Apply cannot silently narrow the queue.)
func ledgerVisibleIDs(txns []domain.Transaction, c txnfilter.Criteria) []string {
	c.Page, c.PageSize = 1, txnfilter.PageSizeAll
	shown := txnfilter.Apply(txns, c)
	out := make([]string, 0, len(shown))
	for _, t := range shown {
		out = append(out, t.ID)
	}
	return out
}

// restrictQueue keeps only the queued charges the ledger is currently showing,
// in queue order.
func restrictQueue(queue []domain.Transaction, visible []string) []domain.Transaction {
	ids := make([]string, 0, len(queue))
	for _, t := range queue {
		ids = append(ids, t.ID)
	}
	keep := make(map[string]bool, len(queue))
	for _, id := range reviewscope.Restrict(ids, visible) {
		keep[id] = true
	}
	out := make([]domain.Transaction, 0, len(keep))
	for _, t := range queue {
		if keep[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

// buildReviewIndex resolves every queued charge ONCE, groups by merchant, and
// ranks the groups. Suggestions are resolved per merchant rather than per charge
// — every charge in a group shares a payee, so the answer is the same and the
// work is O(groups) instead of O(charges).
//
// scope decides which charges are in play; ledger is the ledger's effective
// filter, used both to narrow the queue under ScopeCurrentView and to count the
// other scope's denominator under either (C554).
func buildReviewIndex(app *appstate.App, scope reviewscope.Scope, ledger txnfilter.Criteria) reviewIndex {
	idx := reviewIndex{Byid: map[string]reviewRow{}}
	if app == nil {
		return idx
	}
	res := uistate.LoadReviewResolutions()
	txns := app.Transactions()
	queue := reviewqueue.QueueOpen(txns, res, time.Now())
	idx.QueueTotal = len(queue)
	inView := restrictQueue(queue, ledgerVisibleIDs(txns, ledger))
	idx.ViewTotal = len(inView)
	if reviewscope.Normalize(scope) == reviewscope.ScopeCurrentView {
		queue = inView
	}
	idx.Total = len(queue)

	// Load the expensive inputs ONCE. Each of these is a KV read plus a JSON
	// unmarshal, and they used to happen once per merchant.
	tally := uistate.LoadLearnTally()
	threshold := uistate.LoadLearnThreshold()
	cats := app.Categories()

	idx.TxnByID = make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		idx.TxnByID[t.ID] = t
	}
	idx.Dupes = map[string][]string{}
	for _, g := range dedupe.FindDuplicates(txns) {
		if len(g.IDs) < 2 {
			continue
		}
		for _, id := range g.IDs {
			idx.Dupes[id] = g.IDs
		}
	}

	// Rank each charge by the confidence of its merchant's suggestion.
	suggByKey := map[string]catsuggest.Suggestion{}
	okByKey := map[string]bool{}
	ranked := make([]reviewqueue.Ranked, 0, len(queue))
	for _, t := range queue {
		k := reviewqueue.MerchantKey(t)
		if _, seen := okByKey[k]; !seen {
			s, ok := reviewSuggestionWith(app, t, tally, threshold, cats)
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
	rev := uistate.UseDataRevision().Get()
	open := uistate.UseReviewInbox()
	pr := uistate.UsePrefs().Get()
	// C554: the ledger's own working set, so Review can offer to inherit it. The
	// member lens is layered in exactly as the ledger layers it, or "Current view"
	// would mean something subtly different on the two screens.
	ledgerOwn := uistate.UseTxFilter().Get()
	ledgerEff, _ := ledgerOwn.WithOwnerLens(uistate.UseMemberLens())
	// C559: mode, card position and scope live in an atom so a trip to Categories
	// or Rules can come back to the same place.
	sessAtom := uistate.UseReviewSession()

	// All hooks unconditional, before any early return.
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
	focusRow := ui.UseState(0) // bulk: which merchant row the keyboard is on
	notice := ui.UseState("")
	// C653: the merchant-wide commit is a second, explicit step. This holds the
	// merchant key awaiting that confirmation, so the footer can state exactly what
	// is about to happen instead of a button doing it on the first click.
	confirmAll := ui.UseState("")
	// SMART+ scan state (C504). Proposals live apart from `manual` so a model
	// answer is never mistaken for something the user chose by hand.
	scanState := ui.UseState("idle")
	scanErr := ui.UseState("")
	aiProposals := ui.UseState(map[string]string{})
	scanStats := ui.UseState([2]int{})

	// The session is read once per render and written through one mutator, so the
	// scope/mode/cursor triple can never be half-updated.
	sess := sessAtom.Get().Normalized()
	setSession := func(mut func(*uistate.ReviewSession)) {
		s := sessAtom.Get().Normalized()
		mut(&s)
		sessAtom.Set(s.Normalized())
	}
	// C554: an unchosen scope FOLLOWS the ledger rather than being written to the
	// session on open. Deriving it keeps a filter the user changes between visits
	// honest, and means nothing is persisted until they actually decide.
	ledgerFiltered := len(ledgerEff.ActiveFilters()) > 0
	scope := reviewscope.Default(ledgerFiltered)
	if sess.ScopeChosen {
		scope = sess.Scope
	}
	setScope := func(s string) {
		// Both halves of the card position reset: index 3 of the old population is
		// not a meaningful place in the new one, and a merchant that was in view may
		// not be in the whole queue's first card either.
		setSession(func(x *uistate.ReviewSession) {
			x.ScopeChosen, x.Scope = true, reviewscope.Normalize(reviewscope.Scope(s))
			x.Cursor, x.CursorKey = 0, ""
		})
		confirmAll.Set("")
		notice.Set("")
	}
	setMode := func(m string) {
		setSession(func(x *uistate.ReviewSession) { x.Mode = m })
		confirmAll.Set("")
		notice.Set("")
	}
	setCursorTo := func(n int, key string) {
		setSession(func(x *uistate.ReviewSession) { x.Cursor, x.CursorKey = n, key })
		confirmAll.Set("")
	}
	onSingle := ui.UseEvent(func() { setMode(reviewModeSingle) })
	onBulk := ui.UseEvent(func() { setMode(reviewModeBulk) })
	// Closing the surface with the queue cleared ENDS the session; closing it to go
	// and create a category does not (C559), which is why the reset lives here and
	// not in CloseReviewInbox.
	closeSurface := ui.UseEvent(func() {
		uistate.ResetReviewSession()
		uistate.CloseReviewInbox()
	})

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

	// Rebuild ONLY when the data actually changes. Selecting a merchant, expanding
	// a row or switching mode are pure view state, and re-deriving the whole queue
	// for them is what made every interaction cost ~100-300ms. The scope and the
	// ledger filter join the keys because both change WHICH charges are in play.
	idx := ui.UseMemo(func() reviewIndex { return buildReviewIndex(app, scope, ledgerEff) },
		rev, open.Get(), string(scope), ledgerEff)

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
				// C554: the ids the ROW drew, not everything the ledger holds under
				// this merchant key. Under "Current view" the group already excludes
				// charges outside the filter, and a write that re-derived its own
				// population from the key would quietly reach past the scope the
				// user chose — the same overreach C653 fixes one card at a time.
				total += assignReviewToCharges(app, groupChargeIDs(r), cat)
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
	// The card's position is held by MERCHANT first, index second. The index alone
	// is not a place in this list: GroupByMerchant ranks groups by (confidence,
	// size, key), so categorizing one charge shrinks its group and can drop it below
	// a same-confidence neighbour — and a cursor that stayed at index 3 would then
	// be showing a merchant the user has never seen, which is the "one click, an
	// unpredictable outcome" C653 exists to end. Falling back to the index when the
	// key no longer matches is right, not a compromise: the merchant is gone from
	// the queue, so the work at that position IS the next thing to look at.
	rowKeys := make([]string, 0, len(singleRows))
	for _, r := range singleRows {
		rowKeys = append(rowKeys, r.Group.Key)
	}
	at := reviewscope.CardIndex(rowKeys, sess.Cursor, sess.CursorKey)
	var cur domain.Transaction
	var curRow reviewRow
	if len(singleRows) > 0 && len(singleRows[at].Group.Items) > 0 {
		curRow = singleRows[at]
		cur = curRow.Group.Items[0]
	}
	// gotoCard moves the card and records BOTH halves of where it landed.
	gotoCard := func(n int) {
		if len(singleRows) == 0 {
			return
		}
		if n < 0 {
			n = 0
		}
		if n >= len(singleRows) {
			n = len(singleRows) - 1
		}
		setCursorTo(n, singleRows[n].Group.Key)
	}
	groupIDs := groupChargeIDs(curRow)

	// C653: which charges one click changes, and it is the CARD's answer, not the
	// write path's. The card depicts a single charge, so a single charge is the
	// default; the merchant-wide option has to be chosen, and the choice expires
	// with the card it was made on (CommitKey) so it can never be inherited by a
	// merchant the user has not looked at.
	commit := reviewscope.CommitCharge
	if sess.CommitKey != "" && sess.CommitKey == curRow.Group.Key {
		commit = reviewscope.NormalizeCommit(sess.Commit)
	}
	if len(groupIDs) <= 1 {
		// A merchant with one queued charge has no second scope; offering "all 1"
		// would be a control with nothing behind it.
		commit = reviewscope.CommitCharge
	}
	setCommit := func(v string) {
		setSession(func(x *uistate.ReviewSession) {
			x.Commit, x.CommitKey = reviewscope.NormalizeCommit(reviewscope.CommitScope(v)), curRow.Group.Key
		})
		confirmAll.Set("")
		notice.Set("")
	}
	targets := reviewscope.Targets(commit, cur.ID, groupIDs)

	// The queue itself moves when an action lands — the charge (or the whole
	// merchant) leaves it, so the card at this cursor position is already the next
	// piece of work. Incrementing the cursor here as well is how the old code
	// SKIPPED a merchant per confirmation: rows shifted down by one and the cursor
	// stepped over the shift. Staying put is what "and next" actually means.
	settle := func() {
		// Pin the position to THIS merchant and this slot. If the group survived the
		// action (a per-charge commit on a merchant with siblings left) the next
		// render finds it by key and stays put, whatever the re-ranking did to its
		// index. If the group is gone (a merchant-wide commit, a snooze, the last
		// charge) the key matches nothing and the slot takes over, which is the next
		// piece of work. Both cases without moving the cursor by hand.
		setCursorTo(at, curRow.Group.Key)
		notice.Set("")
	}
	commitTargets := func(ids []string) {
		cat := catFor(curRow)
		if cat == "" {
			notice.Set(uistate.T("review.chooseFirst"))
			return
		}
		// C553: settle ONLY on a write that landed. This used to advance
		// unconditionally, so a failed write moved to the next card while the
		// transaction stayed uncategorized — the queue count fell, the card moved
		// on, and a reload brought the item straight back. Holding position with a
		// stated reason is the honest behavior.
		if !applyReviewChoice(app, ids, cat) {
			notice.Set(uistate.T("review.commitFailed"))
			return
		}
		settle()
	}
	doConfirm := func() {
		if cur.ID == "" {
			return
		}
		// A merchant-wide write is a second, explicit step: the first click asks,
		// naming the count, the category, the total and the dates it covers; the
		// second one writes. An inline strip rather than a modal, because this
		// surface is already a modal and one stacked on another is where "no
		// confirmation appeared" reports come from.
		if len(targets) > 1 && confirmAll.Get() != curRow.Group.Key {
			if catFor(curRow) == "" {
				notice.Set(uistate.T("review.chooseFirst"))
				return
			}
			confirmAll.Set(curRow.Group.Key)
			return
		}
		commitTargets(targets)
	}
	confirmOne := ui.UseEvent(doConfirm)
	cancelAll := ui.UseEvent(func() { confirmAll.Set("") })
	// The single card's category <select>. Registered HERE, unconditionally, rather
	// than inside reviewSingleView — that function is a plain call, so a hook in it
	// belongs to this fiber and would exist only in single mode.
	pickCategory := ui.UseEvent(func(e ui.Event) { setManual(curRow.Group.Key, e.GetValue()) })
	// C556: the single card's own path to a category that does not exist yet. The
	// picker is separate state, so the card, its cursor and any pending choice are
	// all still here when it closes — cancelled or saved.
	newCatSingle := ui.UseEvent(func() {
		if curRow.Group.Key != "" {
			picker.Set(curRow.Group.Key)
		}
	})
	// Snooze and dismiss follow the SAME scope as the confirm, so one control
	// describes every action on the card. Acting on one charge of a 122-charge
	// merchant used to leave 121 behind and read as a broken button; acting on all
	// 122 from a card showing one is the C653 defect in a different button.
	doSkip := func() {
		if cur.ID == "" {
			return
		}
		// C493: a durable snooze, not an in-memory skip that vanishes on reload.
		until := time.Now().AddDate(0, 0, 7)
		for _, id := range targets {
			uistate.SnoozeReviewItem(id, until)
		}
		uistate.BumpDataRevision()
		settle()
	}
	skipOne := ui.UseEvent(doSkip)
	doDismiss := func() {
		if cur.ID == "" {
			return
		}
		for _, id := range targets {
			uistate.DismissReviewItem(id)
		}
		uistate.BumpDataRevision()
		settle()
	}
	dismissOne := ui.UseEvent(doDismiss)

	// C559: the two corrections that legitimately leave Review. Each names its
	// destination, leaves a restorable crumb (route + working set + row + "reopen
	// Review"), and returns to this same card because the session outlives the
	// surface. Nothing is replayed: the ledger filter and the session are both
	// still where the user left them.
	leaveReview := func(to string) {
		uistate.SetReturnTo(uistate.ReturnTo{
			From: "/transactions", To: to, Label: reviewReturnLabel(scope, idx),
			TxnID: cur.ID, ReopenReview: true,
		})
		uistate.CloseReviewInbox()
		nav.Navigate(uistate.RoutePath(to))
	}
	toCategories := ui.UseEvent(func() { leaveReview("/categories") })
	toRules := ui.UseEvent(func() {
		// Hand the rule form the merchant and the category already on the card, so
		// the trip starts from the decision rather than from an empty form (C622's
		// contract, from the other surface).
		uistate.SetRuleDraft(strings.TrimSpace(curRow.Group.Merchant), catFor(curRow))
		leaveReview("/rules")
	})

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
	if isBulkMode := sess.Mode == reviewModeBulk; isBulkMode {
		keys.Next = func() { focusRow.Set(clampFocus(focusRow.Get() + 1)) }
		keys.Prev = func() { focusRow.Set(clampFocus(focusRow.Get() - 1)) }
		keys.Toggle = func() {
			if r := clampFocus(focusRow.Get()); r < len(idx.Rows) {
				toggleSelect(idx.Rows[r].Group.Key)
			}
		}
		keys.Confirm = doApply
	} else {
		keys.Next = func() { gotoCard(at + 1) }
		keys.Prev = func() { gotoCard(at - 1) }
		keys.Confirm = doConfirm
		keys.Skip = doSkip
		keys.Dismiss = doDismiss
	}
	setReviewKeys(keys)

	// ---- render --------------------------------------------------------------
	// C554: the scope strip. It states which population is being reviewed AND the
	// size of the other one, so switching is an informed decision rather than a
	// guess, and it is rendered above the empty state as well — "nothing here"
	// means something quite different when 251 charges are waiting one click away.
	scopeStrip := ui.CreateElement(reviewScopeStrip, reviewScopeStripProps{
		Scope: scope, ViewTotal: idx.ViewTotal, QueueTotal: idx.QueueTotal,
		LedgerFiltered: ledgerFiltered, Working: reviewWorkingSetPhrase(ledgerOwn),
		OnSelect: setScope,
	})

	if idx.Total == 0 {
		// An empty SCOPE is not an empty queue. Saying "all caught up" while the
		// entire-queue count sits at 251 is the app congratulating the user for a
		// filter they may not remember setting.
		title, sub := uistate.T("review.emptyTitle"), uistate.T("review.emptySub")
		if scope == reviewscope.ScopeCurrentView && idx.QueueTotal > 0 {
			title = uistate.T("review.emptyViewTitle")
			sub = uistate.T("review.emptyViewSub", reviewLeftPhrase(idx.QueueTotal))
		}
		return Div(css.Class("rvs rvs-done"), Attr("data-testid", "review-inbox"),
			scopeStrip,
			P(css.Class("rvw-done-title"), title),
			P(css.Class("rvw-done-sub"), sub),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "review-done"),
				OnClick(closeSurface), uistate.T("review.done")),
		)
	}

	isBulk := sess.Mode == reviewModeBulk
	kbdHint := uistate.T("review.keyboardHintSingle")
	if isBulk {
		kbdHint = uistate.T("review.keyboardHintBulk")
	}
	head := Div(css.Class("rvs-head"),
		uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.modeLabel"),
			Selected: sess.Mode,
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
		body, foot = reviewSingleView(reviewSingleArgs{
			App: app, Index: idx, Row: curRow, Txn: cur, CatFor: catFor, PickCategory: pickCategory,
			Confirm: confirmOne, Skip: skipOne, Dismiss: dismissOne, NewCat: newCatSingle,
			CancelAll: cancelAll, ToCategories: toCategories, ToRules: toRules,
			Notice: notice.Get(), Prefs: pr,
			Commit: commit, GroupSize: len(groupIDs), Targets: len(targets),
			// The strip is only ever the second step of a MULTI-charge write. Guarding
			// on len(targets) as well as the key covers the case where the group shrank
			// under a pending confirmation (a sibling categorized elsewhere) —
			// otherwise it would sit there asking "Categorize all 1 charges?".
			Confirming:    curRow.Group.Key != "" && confirmAll.Get() == curRow.Group.Key && len(targets) > 1,
			OnCommitScope: setCommit,
		})
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
		scopeStrip,
		head,
		Div(css.Class("rvs-body"), body),
		Div(css.Class("rvs-foot modal-foot"), foot),
		pick,
	)
}

// groupChargeIDs is the ids of the charges a merchant row is standing in for —
// the row's OWN items, already narrowed to the active scope. Every write in this
// surface is aimed with this rather than with a merchant key, so the number the
// user was shown and the number written are the same list (C554/C653).
func groupChargeIDs(r reviewRow) []string {
	out := make([]string, 0, len(r.Group.Items))
	for _, t := range r.Group.Items {
		out = append(out, t.ID)
	}
	return out
}

// reviewReturnLabel names the review task a cross-page trip is stepping out of,
// for the return crumb: which scope, and how much of it is left. "Back to Review
// (current view, 12 left)" tells the user which job they are returning to;
// "Back" alone is what C581 already found wanting on the ledger.
func reviewReturnLabel(scope reviewscope.Scope, idx reviewIndex) string {
	if scope == reviewscope.ScopeCurrentView {
		return uistate.T("review.returnLabelView", idx.Total)
	}
	return uistate.T("review.returnLabelQueue", idx.Total)
}

// reviewScopeStripProps drive the scope strip.
type reviewScopeStripProps struct {
	Scope                 reviewscope.Scope
	ViewTotal, QueueTotal int
	LedgerFiltered        bool
	Working               string // the ledger's working set in a few words
	OnSelect              func(string)
}

// reviewScopeStrip states which population Review is working through, and offers
// the other one with its count (C554).
//
// It is its own component so its Segmented hook sits at a stable position
// regardless of which branch the surface renders — the strip shows above the
// queue AND above the empty state, and a hook that appears in one and not the
// other is the framework's cardinal sin.
//
// An unfiltered ledger gets a plain sentence instead of a control: offering a
// choice between "current view" and "entire queue" when the two are the same set
// is a control that cannot do anything.
func reviewScopeStrip(props reviewScopeStripProps) ui.Node {
	if !props.LedgerFiltered {
		return Div(css.Class("rvs-scope"), Attr("data-testid", "review-scope"),
			Attr("data-scope", string(props.Scope)),
			Span(css.Class("rvs-scope-note"), uistate.T("review.scopeAllNote", plural(props.QueueTotal, "charge"))))
	}
	return Div(css.Class("rvs-scope"), Attr("data-testid", "review-scope"),
		Attr("data-scope", string(props.Scope)),
		uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.scopeLabel"),
			Selected: string(props.Scope),
			Options: []uiw.SegOption{
				{Value: string(reviewscope.ScopeCurrentView),
					Label: uistate.T("review.scopeView", props.ViewTotal), TestID: "review-scope-view"},
				{Value: string(reviewscope.ScopeEntireQueue),
					Label: uistate.T("review.scopeQueue", props.QueueTotal), TestID: "review-scope-queue"},
			},
			OnSelect: props.OnSelect,
		}),
		Span(css.Class("rvs-scope-note"), Attr("data-testid", "review-scope-note"),
			Attr("role", "status"), Attr("aria-live", "polite"),
			reviewScopeNote(props.Scope, props.ViewTotal, props.QueueTotal, props.Working)),
	)
}

// reviewWorkingSetPhrase names what the ledger is narrowed BY, as a fragment that
// reads inside a sentence.
//
// txnWorkingSetLabel exists for the same job on the return crumb, but it is built
// to follow the word "Back to" — "Transactions, filtered to \"coffee\"" — and drops
// into "…are in Transactions, filtered to \"coffee\"" as a grammatical wreck. Same
// fact, different grammatical slot, so a second phrasing rather than a shared one
// that is wrong in both places.
func reviewWorkingSetPhrase(c txnfilter.Criteria) string {
	if q := strings.TrimSpace(c.Text); q != "" {
		return uistate.T("review.scopeBySearch", q)
	}
	if n := len(c.ActiveFilters()); n > 0 {
		return uistate.T("review.scopeByFilters", plural(n, "filter"))
	}
	return uistate.T("review.scopeByView")
}

// reviewScopeNote is the sentence under the scope control: what the selected
// scope covers, and — crucially — how much sits OUTSIDE it. A scope that states
// only its own size leaves the reader unable to tell a small queue from a narrow
// filter.
func reviewScopeNote(scope reviewscope.Scope, view, queue int, working string) string {
	outside := queue - view
	if outside < 0 {
		outside = 0
	}
	if scope == reviewscope.ScopeCurrentView {
		if outside == 0 {
			return uistate.T("review.scopeViewNoteAll", plural(view, "charge"), working)
		}
		return uistate.T("review.scopeViewNote", plural(view, "charge"), working, reviewWaitingPhrase(outside))
	}
	return uistate.T("review.scopeQueueNote", plural(queue, "charge"), plural(view, "charge"), working)
}

// reviewWaitingPhrase is the "N more are" / "1 more is" fragment. Literal keys on
// both branches, not a key held in a variable — the i18n coverage scan only sees
// literal keys passed to T, and a dynamic key is a missing string nobody notices
// until it renders as its own name.
func reviewWaitingPhrase(n int) string {
	if n == 1 {
		return uistate.T("review.waitingOne")
	}
	return uistate.T("review.waitingMany", n)
}

// reviewLeftPhrase is the "N are" / "1 is" fragment for the empty-view sentence.
func reviewLeftPhrase(n int) string {
	if n == 1 {
		return uistate.T("review.leftOne")
	}
	return uistate.T("review.leftMany", n)
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
			// C602: the relationship is WRITTEN, not implied by an arrow. Carrying
			// "grouped into" as the arrow's tooltip and accessible name left the
			// sighted reader — the one who reported the header as unclear — with two
			// numbers and a glyph between them. The words are on the line now, quiet
			// enough not to compete with the figures they join.
			Span(css.Class("rvs-arrow"), Attr("aria-hidden", "true"), "→"),
			Span(css.Class("rvs-joiner"), uistate.T("review.groupedInto")),
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

// reviewSingleArgs is reviewSingleView's argument bag. It grew past the point
// where a positional list was readable, and a mislabelled `any` handler in a list
// of four is not a compile error.
type reviewSingleArgs struct {
	App    *appstate.App
	Index  reviewIndex
	Row    reviewRow
	Txn    domain.Transaction
	CatFor func(reviewRow) string
	// Handlers. `any` because these are framework event handles, not plain funcs.
	// PickCategory is the category <select>'s OnChange. It is threaded in rather
	// than registered here because this function is not a component: a hook called
	// inside it belongs to the surface's fiber and would exist only in single mode.
	Confirm, Skip, Dismiss, NewCat, CancelAll, ToCategories, ToRules, PickCategory any
	Notice                                                                         string
	Prefs                                                                          prefs.Prefs
	// C653: the commit scope, the size of the merchant group behind this card, and
	// how many charges the primary action will actually write. GroupSize and
	// Targets are separate because a one-charge merchant has a group of 1 and no
	// scope control, and the copy must not offer a choice that does not exist.
	Commit        reviewscope.CommitScope
	GroupSize     int
	Targets       int
	Confirming    bool // the merchant-wide write is awaiting its second click
	OnCommitScope func(string)
}

// reviewSingleView renders one charge with its context band and pinned actions.
func reviewSingleView(a reviewSingleArgs) (ui.Node, ui.Node) {
	app, idx, row, cur, catFor := a.App, a.Index, a.Row, a.Txn, a.CatFor
	notice, pr := a.Notice, a.Prefs
	if cur.ID == "" {
		return P(css.Class("rvs-note"), uistate.T("review.emptySub")), Fragment()
	}
	kind := domain.KindExpense
	if cur.Amount.Amount > 0 {
		kind = domain.KindIncome
	}
	// The OnChange handler is built by the SURFACE and threaded in, like every other
	// handler in this struct. It used to be `ui.UseEvent(...)` inline here — and
	// this function is a plain call, not a component, so that hook belonged to
	// ReviewSurfaceBody's fiber and ran only in single mode. Hooks are matched
	// across renders by call order per fiber, so a hook that appears in one mode and
	// not the other is the framework's cardinal sin; it happened to be the last hook
	// in the render, which made it harmless right up until the next hook was added
	// after it. Same defect reviewConfirmAll was extracted to avoid, three functions
	// down, in the same change.
	selArgs := []any{css.Class("field"), Attr("data-testid", "review-category-select"),
		Attr("aria-label", uistate.T("review.categoryLabel")), OnChange(a.PickCategory)}
	selArgs = append(selArgs, categorySelectNodes(app.Categories(), kind, catFor(row))...)

	// C653: the scope control. It only exists when there is genuinely a second
	// scope, it defaults to the charge the card is showing, and both options carry
	// their count — "1 selected charge" versus "122 merchant charges" — because the
	// complaint was never the merchant-wide write itself, it was that one button
	// silently meant either.
	// The scope line is ALWAYS here, control or not. A merchant with one queued
	// charge has no second scope to offer, but dropping the whole row on those
	// cards made the card's lower half jump as you paged the queue, and — worse —
	// left the one card where the scope is unambiguous as the one card that does
	// not state it. So the label stays and the control is replaced by the answer.
	scopeLine := []any{css.Class("rvs-cscope"), Attr("data-testid", "review-commit-scope")}
	if a.GroupSize > 1 {
		scopeLine = append(scopeLine, uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.commitScopeLabel"),
			Selected: string(a.Commit),
			Options: []uiw.SegOption{
				{Value: string(reviewscope.CommitCharge), Label: uistate.T("review.scopeThisCharge"),
					TestID: "review-scope-charge"},
				{Value: string(reviewscope.CommitMerchant),
					Label:  uistate.T("review.scopeAllMerchant", a.GroupSize, strings.TrimSpace(row.Group.Merchant)),
					TestID: "review-scope-merchant"},
			},
			OnSelect: a.OnCommitScope,
		}))
	} else {
		scopeLine = append(scopeLine,
			Span(css.Class("rvs-cscope-label"), uistate.T("review.commitScopeLabel")),
			Span(css.Class("rvs-cscope-fixed"), Attr("data-testid", "review-scope-only"),
				uistate.T("review.scopeThisCharge")))
	}
	scopePicker := Div(scopeLine...)

	// C559: the corrections that legitimately leave this surface, each naming
	// where it goes. The return crumb they leave brings back this card, this
	// scope and this ledger filter.
	// The ↗ and the accessible name both say these LEAVE. They sit a few
	// millimetres from "New category", which looks identical and does not leave, so
	// a user who has just created a category without losing their place has every
	// reason to expect the same here — and would find out otherwise only after the
	// modal closed under them.
	leaveGlyph := Span(css.Class("rvs-links-out"), Attr("aria-hidden", "true"),
		uistate.T("review.leavesReviewGlyph"))
	crossLinks := Div(css.Class("rvs-links"), Attr("data-testid", "review-cross-links"),
		Span(css.Class("rvs-links-lead"), uistate.T("review.linksLead")),
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "review-goto-categories"),
			Attr("aria-label", uistate.T("review.leavesReviewAria", uistate.T("review.gotoCategories"))),
			Attr("title", uistate.T("review.leavesReviewAria", uistate.T("review.gotoCategories"))),
			OnClick(a.ToCategories), uistate.T("review.gotoCategories"), leaveGlyph),
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "review-goto-rules"),
			Attr("aria-label", uistate.T("review.leavesReviewAria", uistate.T("review.gotoRules"))),
			Attr("title", uistate.T("review.leavesReviewAria", uistate.T("review.gotoRules"))),
			OnClick(a.ToRules), uistate.T("review.gotoRules"), leaveGlyph),
	)

	reason := reviewqueue.ReasonFor(cur)
	body := Div(css.Class("rvs-single"),
		Div(
			Div(css.Class("rvw-card"),
				Div(css.Class("rvw-card-top"),
					// C600: "Queued: no category yet" — a reason for being here, not a
					// claim about the row. The bare word sat directly above a Category
					// control that could already read "Dining" (a suggestion), so the card
					// asserted two contradictory things about one charge with nothing to
					// mark one as stored state and the other as a pending decision.
					Span(css.Class("rvw-reason is-uncat"), Attr("data-testid", "review-reason"),
						uistate.T("review.reasonLead", reviewReasonLabel(reason))),
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
				Div(css.Class("rvs-assign-row"),
					Select(selArgs...),
					// C556: the single card's own escape hatch to a category that does
					// not exist yet. The control was written and wired months ago and
					// simply never rendered here, so creating "Housing › Home
					// maintenance" mid-review meant leaving Review, visiting Categories,
					// returning to Transactions and reopening the queue. A button, not a
					// <select> sentinel, for the reason catSelectNew documents.
					Button(css.Class("btn btn-sm rvs-newcat"), Type("button"),
						Attr("data-testid", "review-new-category"),
						Attr("aria-label", uistate.T("review.newCategoryAria")),
						OnClick(a.NewCat), uistate.T("review.newCategory")),
				),
				If(row.HasSugg, Div(css.Class("rvs-why"), Attr("data-testid", "review-suggest-why"),
					reviewWhy(row.Suggestion))),
				// C600: the third axis. "Queued: no category yet" is the stored state
				// and the select is the pending decision; this says the decision has
				// not landed, which is what separates "has no category" from "has one
				// nobody has confirmed". Shown only when something is actually pending.
				If(catFor(row) != "", P(css.Class("rvs-pending"), Attr("data-testid", "review-pending-note"),
					uistate.T("review.pendingNote"))),
				If(reason == reviewqueue.ReasonSplitUnbalanced,
					P(css.Class("rvs-warn"), Attr("data-testid", "review-split-warn"),
						uistate.T("review.splitNeeded"))),
				If(notice != "", P(css.Class("rvs-notice"), Attr("role", "alert"),
					Attr("data-testid", "review-commit-err"), notice)),
			),
		),
		scopePicker,
		crossLinks,
		ui.CreateElement(reviewContextBand, reviewContextBandProps{App: app, Index: idx, Row: row, Txn: cur, Prefs: pr}),
	)

	// C653: every action label carries its own count, so nothing on this card
	// promises one scope and performs another.
	confirmLabel := uistate.T("review.categorizeNext")
	snoozeLabel, dismissLabel := uistate.T("review.snooze"), uistate.T("review.dismiss")
	if a.Targets > 1 {
		confirmLabel = uistate.T("review.categorizeAllN", a.Targets)
		snoozeLabel = uistate.T("review.snoozeN", a.Targets)
		dismissLabel = uistate.T("review.dismissN", a.Targets)
	}
	acts := Span(css.Class("rvs-foot-acts"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-dismiss"),
			OnClick(a.Dismiss), dismissLabel),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-skip"),
			OnClick(a.Skip), snoozeLabel),
		Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-commit"), OnClick(a.Confirm), confirmLabel),
	)
	if a.Confirming {
		acts = ui.CreateElement(reviewConfirmAll, reviewConfirmAllProps{
			Targets:  a.Targets,
			Merchant: strings.TrimSpace(row.Group.Merchant),
			Category: reviewCatName(app, catFor(row)),
			Facts:    reviewGroupFacts(row, pr),
			OnCancel: a.CancelAll, OnGo: a.Confirm,
		})
	}
	foot := Fragment(
		Span(css.Class("rvs-foot-sub"), Attr("data-testid", "review-progress"),
			uistate.T("review.leftCount", idx.Total)),
		Span(css.Class("rvs-hidden"), Attr("data-testid", "review-total"), strconv.Itoa(idx.Total)),
		acts,
	)
	return body, foot
}

// reviewConfirmAllProps drive the merchant-wide confirmation strip.
type reviewConfirmAllProps struct {
	Targets            int
	Merchant, Category string
	Facts              string
	OnCancel, OnGo     any
}

// reviewConfirmAll is the second step of a merchant-wide write: the question,
// the facts to check it against, and the two ways out.
//
// It is its OWN component for one reason — it needs an effect, and an effect in
// a conditionally-rendered branch of reviewSingleView would be a hook that
// appears and disappears between renders of the surface.
//
// The effect moves focus onto the confirm button. Without it this design has a
// hole exactly where its safety is supposed to be: the strip REPLACES the
// primary button in place, so a screen-reader user who activates "Categorize &
// next" hears nothing change — the button they pressed is gone, the accessible
// name they were on has not been updated, and `role=alertdialog` without a focus
// move is not reliably announced. That user could press Enter twice and never
// perceive the intermediate question, which is precisely the group of people the
// second step exists to protect. (`autofocus` is not an option here: C615 records
// that it does nothing anywhere in this app.)
func reviewConfirmAll(props reviewConfirmAllProps) ui.Node {
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil
		}
		if el := doc.Call("querySelector", `[data-testid="review-confirm-all-go"]`); el.Truthy() {
			el.Call("focus")
		}
		return nil
	}, "review-confirm-all")

	return Span(css.Class("rvs-foot-acts rvs-confirm-all"), Attr("data-testid", "review-confirm-all"),
		Attr("role", "alertdialog"), Attr("aria-modal", "false"),
		Attr("aria-label", uistate.T("review.applyAllTitle", props.Targets)),
		Span(css.Class("rvs-confirm-q"), Attr("id", "review-confirm-all-q"),
			uistate.T("review.applyAllAsk", props.Targets, props.Merchant, props.Category)),
		Span(css.Class("rvs-confirm-facts"), props.Facts),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-confirm-all-cancel"),
			OnClick(props.OnCancel), uistate.T("action.cancel")),
		Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-confirm-all-go"),
			// The button's own accessible name carries the question, so focusing it
			// announces what is being asked rather than a bare "Yes, categorize 122".
			Attr("aria-describedby", "review-confirm-all-q"),
			OnClick(props.OnGo), uistate.T("review.applyAllGo", props.Targets)),
	)
}

// reviewGroupFacts is the one-line preview a merchant-wide confirmation shows:
// what the batch adds up to and the span of dates it reaches across. Both are
// facts a user checks a scope against — a total an order of magnitude off, or a
// date range stretching back further than they expected, is how you notice the
// button is aimed at more than you meant.
func reviewGroupFacts(row reviewRow, pr prefs.Prefs) string {
	items := row.Group.Items
	if len(items) == 0 {
		return ""
	}
	var total int64
	first, last := items[0].Date, items[0].Date
	cur := items[0].Amount.Currency
	for _, t := range items {
		total += t.Amount.Amount
		if t.Date.Before(first) {
			first = t.Date
		}
		if t.Date.After(last) {
			last = t.Date
		}
	}
	span := pr.FormatDate(first)
	if !first.Equal(last) {
		span = uistate.T("review.dateRange", pr.FormatDate(first), pr.FormatDate(last))
	}
	return uistate.T("review.applyAllFacts", fmtMoney(money.Money{Amount: total, Currency: cur}), span)
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
