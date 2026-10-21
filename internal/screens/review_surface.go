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
	"fmt"
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/reviewview"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Review modes. The values live in uistate because the session that remembers
// which one you are in has to outlive this surface (C559).
const (
	reviewModeSingle = uistate.ReviewModeSingle
	reviewModeBulk   = uistate.ReviewModeBulk
)

const recurringLoanPaymentDismissalsKey = "review:recurring-loan-payment-dismissals"

func loadRecurringLoanPaymentDismissals() map[string]bool {
	return reviewview.DecodeDismissals(uistate.KVGet(recurringLoanPaymentDismissalsKey))
}

func saveRecurringLoanPaymentDismissals(dismissed map[string]bool) {
	uistate.KVSet(recurringLoanPaymentDismissalsKey, reviewview.EncodeDismissals(dismissed))
	uistate.RequestPersist()
}

func visibleRecurringLoanPaymentSuggestions(all []txnclassify.PaymentSuggestion, dismissed map[string]bool) []txnclassify.PaymentSuggestion {
	return reviewview.VisibleLoanPayments(all, dismissed)
}

type recurringLoanPaymentSuggestionMemo struct {
	memo reviewview.Memo[*appstate.App, []txnclassify.PaymentSuggestion]
}

func (memo *recurringLoanPaymentSuggestionMemo) get(app *appstate.App, revision uint64, load func() []txnclassify.PaymentSuggestion) []txnclassify.PaymentSuggestion {
	return memo.memo.Get(app, revision, load)
}

func pruneRecurringLoanPaymentDismissals(dismissed map[string]bool, transactions []domain.Transaction) (map[string]bool, bool) {
	return reviewview.PruneDismissals(dismissed, transactions)
}

func reviewEmptyCopy(scope reviewscope.Scope, queueTotal int, suggestionCount int) (string, string) {
	return reviewview.Text(uistate.T).EmptyCopy(scope, queueTotal, suggestionCount)
}

func classifyRecurringLoanPayment(app *appstate.App, suggestion txnclassify.PaymentSuggestion) error {
	if app == nil {
		return fmt.Errorf("review: app is not ready")
	}
	txn, found := app.TransactionByID(suggestion.TransactionID)
	classified, err := reviewview.LoanPayment(suggestion, app.RecurringPaymentSuggestions(), txn, found, app.Accounts())
	if err != nil {
		return err
	}
	if err := app.PutTransactionWithTransferPair(txn, classified); err != nil {
		return fmt.Errorf("review: save loan payment transfer: %w", err)
	}
	return nil
}

// recordRecurringLoanPayment is the durable, non-UI adapter for callers holding
// an explicit App. The visible review action uses CommitUndoable below.
func recordRecurringLoanPayment(app *appstate.App, suggestion txnclassify.PaymentSuggestion, done func(error)) {
	if err := classifyRecurringLoanPayment(app, suggestion); err != nil {
		done(err)
		return
	}
	awaitTxnSave(app, []string{suggestion.TransactionID}, done)

}

type reviewLoanPaymentGroupProps struct {
	Suggestions []txnclassify.PaymentSuggestion
	OnRecord    func(txnclassify.PaymentSuggestion)
	OnDismiss   func(string)
}

func reviewLoanPaymentGroup(props reviewLoanPaymentGroupProps) ui.Node {
	if len(props.Suggestions) == 0 {
		return Fragment()
	}
	return Div(css.Class("rvs-loan-payments"), Attr("data-testid", "review-loan-payment-group"),
		H3(css.Class("rvs-loan-payments-title"), uistate.T("r4loan1.groupTitle")),
		Div(css.Class("rvs-loan-payments-list"), MapKeyed(props.Suggestions,
			func(s txnclassify.PaymentSuggestion) any { return s.TransactionID },
			func(s txnclassify.PaymentSuggestion) ui.Node {
				return ui.CreateElement(reviewLoanPaymentRow, reviewLoanPaymentRowProps{
					Suggestion: s, OnRecord: props.OnRecord, OnDismiss: props.OnDismiss,
				})
			})),
	)
}

type reviewLoanPaymentRowProps struct {
	Suggestion txnclassify.PaymentSuggestion
	OnRecord   func(txnclassify.PaymentSuggestion)
	OnDismiss  func(string)
}

func reviewLoanPaymentRow(props reviewLoanPaymentRowProps) ui.Node {
	record := ui.UseEvent(Prevent(func() { props.OnRecord(props.Suggestion) }))
	dismiss := ui.UseEvent(Prevent(func() { props.OnDismiss(props.Suggestion.TransactionID) }))
	return Div(css.Class("rvs-loan-payment-row"), Attr("data-testid", "review-loan-payment-row"),
		P(css.Class("rvs-loan-payment-message"), uistate.T("r4loan1.suggestionMessage", props.Suggestion.AccountName)),
		Div(css.Class("rvs-loan-payment-actions"),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "review-loan-payment-record"), OnClick(record), uistate.T("r4loan1.record")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-loan-payment-dismiss"), OnClick(dismiss), uistate.T("r4loan1.dismiss")),
		),
	)
}

// reviewTier buckets a merchant group by the confidence of its suggestion.
type reviewTier = reviewview.Tier

const (
	tierNone  = reviewview.TierNone
	tierLook  = reviewview.TierLook
	tierReady = reviewview.TierReady
)

type reviewRow = reviewview.Row

// sharedReviewSuggestion promotes a per-charge result to a merchant-wide
// suggestion only when every charge resolves to the same category. Rules can
// depend on account, amount and date, so merchant identity alone is insufficient.
func sharedReviewSuggestion(g reviewqueue.Group, byID map[string]catsuggest.Suggestion) (catsuggest.Suggestion, bool, bool) {
	return reviewview.SharedSuggestion(g, byID)
}

// reviewIndex is the whole surface's data for one render.
type reviewIndex = reviewview.Index

// buildReviewIndex resolves every queued charge once in its own account, amount,
// and date context, then promotes a result to the merchant group only when all
// charges agree. Shared lookup inputs are loaded once for the whole index.
//
// scope decides which charges are in play; ledger is the ledger's effective
// filter, used both to narrow the queue under ScopeCurrentView and to count the
// other scope's denominator under either (C554).
func buildReviewIndex(app *appstate.App, scope reviewscope.Scope, ledger txnfilter.Criteria) reviewIndex {
	return buildReviewIndexScoped(app, scope, ledger, false)
}

func buildReviewIndexScoped(app *appstate.App, scope reviewscope.Scope, ledger txnfilter.Criteria, fallbackEmptyView bool) reviewIndex {
	if app == nil {
		return reviewview.BuildIndex(reviewview.Input{Scope: scope})
	}
	txns := app.Transactions()
	in := reviewview.Input{
		Transactions: txns, Resolutions: uistate.LoadReviewResolutions(),
		Scope: scope, Ledger: ledger, FallbackEmptyView: fallbackEmptyView, Now: time.Now(),
	}
	suggestions := make(map[string]catsuggest.Suggestion, len(txns))
	tally, threshold, cats := uistate.LoadLearnTally(), uistate.LoadLearnThreshold(), app.Categories()
	for _, t := range reviewview.Candidates(in) {
		if suggestion, ok := reviewSuggestionWith(app, t, tally, threshold, cats); ok {
			suggestions[t.ID] = suggestion
		}
	}
	in.Suggestions = suggestions
	return reviewview.BuildIndex(in)
}

// buildReviewIndexForOpen keeps an unchosen default from opening on an empty
// filtered view while review items wait outside it. An explicitly chosen scope
// remains in force, including an intentionally empty current view.
func buildReviewIndexForOpen(app *appstate.App, scope reviewscope.Scope, chosen bool, ledger txnfilter.Criteria) reviewIndex {
	return buildReviewIndexScoped(app, scope, ledger, !chosen)
}

// ReviewSurfaceBody is the body of the review modal. The panel is NoFooter +
// FlushBody, so this owns both the scrolling region and the pinned action bar.
func ReviewSurfaceBody(_ struct{}) ui.Node {
	app := appstate.Default
	open := uistate.UseReviewInbox()
	if app == nil || !open.Get() {
		if !open.Get() {
			ClearReviewKeys()
		}
		return Fragment()
	}
	return ui.CreateElement(reviewSurfaceReady, struct{}{})
}

// reviewController contains browser state and persistence adapters only. Domain
// decisions are derived by reviewview from explicit snapshots.
type reviewController struct {
	app                                            *appstate.App
	pr                                             prefs.Prefs
	session                                        state.Atom[uistate.ReviewSession]
	sess                                           uistate.ReviewSession
	ledgerOwn, ledgerEff                           txnfilter.Criteria
	scope                                          reviewscope.Scope
	ledgerFiltered                                 bool
	idx                                            reviewIndex
	card                                           reviewview.Card
	selected, openGroups                           ui.State[map[string]bool]
	manual, aiProposals                            ui.State[map[string]string]
	picker, notice, confirmAll, scanState, scanErr ui.State[string]
	pickerRev, focusRow                            ui.State[int]
	applying, confirming                           ui.State[bool]
	scanStats                                      ui.State[[2]int]
	gapRows                                        []reviewRow
	gapCharges                                     int
	hasProvider                                    bool
	navigate                                       func(string)
	scan                                           func()
	loanPaymentGroup                               ui.Node
	loanPaymentSuggestions                         int
}

func useReviewController() *reviewController {
	c := &reviewController{app: appstate.Default}
	rev := uistate.UseDataRevision().Get()
	open := uistate.UseReviewInbox()
	c.pr = uistate.UsePrefs().Get()
	c.ledgerOwn = uistate.UseTxFilter().Get()
	c.ledgerEff, _ = c.ledgerOwn.WithOwnerLens(uistate.UseMemberLens())
	c.session = uistate.UseReviewSession()
	c.selected = ui.UseState(map[string]bool{})
	c.openGroups = ui.UseState(map[string]bool{})
	c.picker = ui.UseState("")
	c.pickerRev = ui.UseState(0)
	c.manual = ui.UseState(map[string]string{})
	c.focusRow = ui.UseState(0)
	c.notice = ui.UseState("")
	c.confirmAll = ui.UseState("")
	c.scanState = ui.UseState("idle")
	c.scanErr = ui.UseState("")
	c.aiProposals = ui.UseState(map[string]string{})
	c.scanStats = ui.UseState([2]int{})
	c.applying = ui.UseState(false)
	c.confirming = ui.UseState(false)
	c.navigate = router.UseNavigate().Navigate
	c.sess = c.session.Get().Normalized()
	c.scope, c.ledgerFiltered = reviewview.ActiveScope(c.ledgerEff, c.sess.ScopeChosen, c.sess.Scope)
	c.idx = ui.UseMemo(func() reviewIndex {
		return buildReviewIndexForOpen(c.app, c.scope, c.sess.ScopeChosen, c.ledgerEff)
	}, c.app, rev, c.app.Rev(), open.Get(), string(c.scope), c.sess.ScopeChosen, c.ledgerEff)
	c.scope = c.idx.Scope
	c.loanPaymentGroup = c.useLoanPayments()
	c.gapRows, c.gapCharges = reviewview.Gaps(c.idx.Rows)
	c.scan = c.useScan()
	c.card = reviewview.CurrentCard(c.idx.Rows, c.sess.Cursor, c.sess.CursorKey, c.sess.CommitKey, c.sess.Commit)
	c.installKeys()
	return c
}

func (c *reviewController) setScope(scope string) {
	c.session.Set(uistate.ReviewSession(reviewview.Session(c.session.Get()).WithScope(scope)))
	c.confirmAll.Set("")
	c.notice.Set("")
}

func (c *reviewController) setMode(mode string) {
	c.session.Set(uistate.ReviewSession(reviewview.Session(c.session.Get()).WithMode(mode)))
	c.confirmAll.Set("")
	c.notice.Set("")
}

func (c *reviewController) setCursorTo(position int, key string) {
	c.session.Set(uistate.ReviewSession(reviewview.Session(c.session.Get()).WithCursor(position, key)))
	c.confirmAll.Set("")
}

func (c *reviewController) setCommit(scope string) {
	c.session.Set(uistate.ReviewSession(reviewview.Session(c.session.Get()).WithCommit(scope, c.card.Row.Group.Key)))
	c.confirmAll.Set("")
	c.notice.Set("")
}

func (c *reviewController) toggleGroup(key string) {
	c.openGroups.Set(reviewview.Toggle(c.openGroups.Get(), key))
}
func (c *reviewController) toggleSelect(key string) {
	c.selected.Set(reviewview.Toggle(c.selected.Get(), key))
}
func (c *reviewController) clearSelection() { c.selected.Set(map[string]bool{}) }
func (c *reviewController) catFor(row reviewRow) string {
	return reviewview.Category(row, c.manual.Get(), c.aiProposals.Get())
}

func (c *reviewController) setManual(key, category string) {
	if category == catSelectNew {
		c.picker.Set(key)
		return
	}
	c.manual.Set(reviewview.Choose(c.manual.Get(), key, category))
}

func (c *reviewController) selectTier(tier reviewTier) {
	c.selected.Set(reviewview.SelectTier(c.idx.Rows, c.selected.Get(), tier, c.manual.Get(), c.aiProposals.Get()))
}

func (c *reviewController) useLoanPayments() ui.Node {
	app := c.app
	memo := ui.UseState(&recurringLoanPaymentSuggestionMemo{}).Get()
	all := memo.get(app, app.Rev(), app.RecurringPaymentSuggestions)
	seed := ui.UseMemo(loadRecurringLoanPaymentDismissals, app)
	dismissed := ui.UseState(seed)
	ui.UseEffect(func() func() {
		pruned, changed := pruneRecurringLoanPaymentDismissals(dismissed.Get(), app.Transactions())
		if changed {
			dismissed.Set(pruned)
			saveRecurringLoanPaymentDismissals(pruned)
		}
		return nil
	}, app, app.Rev())
	record := func(suggestion txnclassify.PaymentSuggestion) {
		_ = CommitUndoable(app, uistate.T("r4loan1.recordedUndo"), []string{suggestion.TransactionID}, func() error { return classifyRecurringLoanPayment(app, suggestion) }, func(err error) {
			if err != nil {
				uistate.PostNotice(uistate.T("tc3w4.changesNotSaved"), true)
				return
			}
		})
	}
	dismiss := func(transactionID string) {
		active := reviewview.Dismiss(dismissed.Get(), transactionID)
		pruned, _ := pruneRecurringLoanPaymentDismissals(active, app.Transactions())
		dismissed.Set(pruned)
		saveRecurringLoanPaymentDismissals(pruned)
	}
	c.loanPaymentSuggestions = len(visibleRecurringLoanPaymentSuggestions(all, dismissed.Get()))
	return ui.CreateElement(reviewLoanPaymentGroup, reviewLoanPaymentGroupProps{
		Suggestions: visibleRecurringLoanPaymentSuggestions(all, dismissed.Get()), OnRecord: record, OnDismiss: dismiss,
	})
}

func (c *reviewController) apply() {
	if c.applying.Get() {
		return
	}
	assignments := reviewview.Assignments(c.idx.Rows, c.selected.Get(), c.manual.Get(), c.aiProposals.Get())
	c.applying.Set(true)
	accepted := commitReviewAssignments(c.app, assignments, func(total int, err error) {
		c.applying.Set(false)
		if err != nil {
			c.notice.Set(uistate.T("review.commitFailed"))
			return
		}
		c.notice.Set(uistate.T("review.bulkApplied", total))
		c.selected.Set(map[string]bool{})
	})
	if accepted == 0 {
		c.applying.Set(false)
	}
}

func (c *reviewController) useScan() func() {
	backend := uistate.BackendAIActive(c.pr)
	c.hasProvider = aiProviderConfigured(c.app, backend)
	connection := resolveAIConn(c.app, backend, c.pr.ServerURL, c.pr.ServerToken)
	catalog := smartCatalog(c.app.Categories())
	return func() {
		if !c.hasProvider {
			uistate.CloseReviewInbox()
			c.navigate(uistate.RoutePath("/settings/ai"))
			return
		}
		if !reviewview.CanScan(len(c.gapRows), c.scanState.Get()) {
			return
		}
		c.scanStats.Set([2]int{})
		draft := reviewview.DraftScan(c.gapRows, smartCatScanCap, reviewMoneyText)
		c.scanState.Set("scanning")
		c.scanErr.Set("")
		runSmartAI(connection, smartai.AutoCategorize(draft.Lines, catalog.Prompt()), func(answer string) {
			proposals, stats := reviewview.ScanResult(draft, answer, catalog, c.aiProposals.Get())
			c.aiProposals.Set(proposals)
			c.scanStats.Set(stats)
			c.scanState.Set("done")
			uistate.BumpDataRevision()
		}, func(err string) { c.scanErr.Set(err); c.scanState.Set("done"); uistate.BumpDataRevision() })
	}
}

func (c *reviewController) acceptScan() {
	proposals := c.aiProposals.Get()
	if len(proposals) == 0 {
		return
	}
	manual, selected := reviewview.AcceptScan(c.manual.Get(), c.selected.Get(), proposals)
	c.manual.Set(manual)
	c.selected.Set(selected)
	c.notice.Set(uistate.T("review.filledN", len(proposals)))
}

func (c *reviewController) makeRules() {
	if len(c.selected.Get()) == 0 {
		return
	}
	made := 0
	for _, draft := range reviewview.RuleDrafts(c.idx.Rows, c.selected.Get(), c.manual.Get(), c.aiProposals.Get()) {
		draft.ID, draft.Order = id.New(), 1000+made
		if err := c.app.PutRule(draft); err != nil {
			uistate.PostNotice(err.Error(), true)
			continue
		}
		made++
	}
	if made > 0 {
		c.notice.Set(uistate.T("review.rulesMade", made))
		uistate.RequestPersist()
	}
	c.apply()
}

func (c *reviewController) gotoCard(position int) {
	if len(c.idx.Rows) == 0 {
		return
	}
	at, key := reviewview.MoveCard(c.idx.Rows, position)
	c.setCursorTo(at, key)
}

func (c *reviewController) settle() {
	c.setCursorTo(c.card.At, c.card.Row.Group.Key)
	c.notice.Set("")
}

func (c *reviewController) commitTargets(ids []string) {
	if c.confirming.Get() {
		return
	}
	if !reviewqueue.CanAssignCategory(c.card.Txn) {
		c.notice.Set(uistate.T("review.splitNeeded"))
		return
	}
	category := c.catFor(c.card.Row)
	if category == "" {
		c.notice.Set(uistate.T("review.chooseFirst"))
		return
	}
	c.confirming.Set(true)
	if !applyReviewChoice(c.app, ids, category, func(ok bool) {
		c.confirming.Set(false)
		if !ok {
			c.notice.Set(uistate.T("review.commitFailed"))
			return
		}
		c.settle()
	}) {
		c.confirming.Set(false)
		c.notice.Set(uistate.T("review.commitFailed"))
	}
}

func (c *reviewController) confirm() {
	decision := reviewview.Confirm(c.card.Txn, c.catFor(c.card.Row), len(c.card.Targets), c.confirmAll.Get(), c.card.Row.Group.Key)
	if decision.NoticeKey != "" {
		c.notice.Set(uistate.T(decision.NoticeKey))
		return
	}
	if decision.Ask {
		c.confirmAll.Set(c.card.Row.Group.Key)
		return
	}
	if decision.Commit {
		c.commitTargets(c.card.Targets)
	}
}

func (c *reviewController) skip() {
	if c.card.Txn.ID == "" {
		return
	}
	until := reviewview.SnoozeUntil(time.Now())
	for _, id := range c.card.Targets {
		uistate.SnoozeReviewItem(id, until)
	}
	uistate.BumpDataRevision()
	c.settle()
}

func (c *reviewController) dismiss() {
	if c.card.Txn.ID == "" {
		return
	}
	for _, id := range c.card.Targets {
		uistate.DismissReviewItem(id)
	}
	uistate.BumpDataRevision()
	c.settle()
}

func (c *reviewController) leaveReview(to string) {
	uistate.SetReturnTo(uistate.ReturnTo{From: "/transactions", To: to,
		Label: reviewReturnLabel(c.scope, c.idx), TxnID: c.card.Txn.ID, ReopenReview: true})
	uistate.CloseReviewInbox()
}

func (c *reviewController) toCategories(event ui.Event) {
	if isPlainLeftClick(event) {
		c.leaveReview("/categories")
	}
}

func (c *reviewController) toRules(event ui.Event) {
	if !isPlainLeftClick(event) {
		return
	}
	uistate.SetRuleDraft(reviewview.MerchantName(c.card.Row), c.catFor(c.card.Row))
	c.leaveReview("/rules")
}

func (c *reviewController) installKeys() {
	keys := reviewKeyActions{Bulk: func() { c.setMode(reviewModeBulk) }, Single: func() { c.setMode(reviewModeSingle) }}
	if c.sess.Mode == reviewModeBulk {
		keys.Next = func() { c.focusRow.Set(reviewview.Focus(c.focusRow.Get()+1, len(c.idx.Rows))) }
		keys.Prev = func() { c.focusRow.Set(reviewview.Focus(c.focusRow.Get()-1, len(c.idx.Rows))) }
		keys.Toggle = func() {
			if at := reviewview.Focus(c.focusRow.Get(), len(c.idx.Rows)); at < len(c.idx.Rows) {
				c.toggleSelect(c.idx.Rows[at].Group.Key)
			}
		}
		keys.Confirm = c.apply
	} else {
		keys.Next = func() { c.gotoCard(c.card.At + 1) }
		keys.Prev = func() { c.gotoCard(c.card.At - 1) }
		keys.Confirm, keys.Skip, keys.Dismiss = c.confirm, c.skip, c.dismiss
	}
	setReviewKeys(keys)
}

func reviewSurfaceReady(_ struct{}) ui.Node {
	c := useReviewController()
	return c.render()
}

func (c *reviewController) singleArgs() reviewSingleArgs {
	return reviewSingleArgs{App: c.app, Index: c.idx, Row: c.card.Row, Txn: c.card.Txn,
		CatFor: c.catFor, PickCategory: func(event ui.Event) { c.setManual(c.card.Row.Group.Key, event.GetValue()) },
		Confirm: c.confirm, Skip: c.skip, Dismiss: c.dismiss,
		NewCat: func() {
			if c.card.Row.Group.Key != "" {
				c.picker.Set(c.card.Row.Group.Key)
			}
		},
		CancelAll: func() { c.confirmAll.Set("") }, ToCategories: c.toCategories, ToRules: c.toRules,
		Notice: c.notice.Get(), Prefs: c.pr, Commit: c.card.Commit, GroupSize: len(c.card.IDs), Targets: len(c.card.Targets),
		Confirming: reviewview.PendingConfirmation(c.card.Row.Group.Key, c.confirmAll.Get(), len(c.card.Targets)), OnCommitScope: c.setCommit,
	}
}

func (c *reviewController) render() ui.Node {
	scopeStrip := ui.CreateElement(reviewScopeStrip, reviewScopeStripProps{
		Scope: c.scope, ViewTotal: c.idx.ViewTotal, QueueTotal: c.idx.QueueTotal, LedgerFiltered: c.ledgerFiltered,
		Working: reviewWorkingSetPhrase(c.ledgerOwn), OnSelect: c.setScope,
	})
	if c.idx.Total == 0 {
		title, sub := reviewEmptyCopy(c.scope, c.idx.QueueTotal, c.loanPaymentSuggestions)
		return ui.CreateElement(reviewEmptyView, reviewEmptyViewProps{Scope: scopeStrip, Loans: c.loanPaymentGroup, Title: title, Sub: sub})
	}
	head := ui.CreateElement(reviewModeHeader, reviewModeHeaderProps{Mode: c.sess.Mode, OnSelect: c.setMode})
	var body, foot ui.Node
	if c.sess.Mode == reviewModeBulk {
		var setupLink ui.Node = Fragment()
		if !c.hasProvider {
			setupLink = ui.CreateElement(reviewAISetup, reviewAISetupProps{OnClick: c.scan})
		}
		body, foot = reviewBulkView(c.app, c.idx, c.selected.Get(), c.openGroups.Get(), c.manual.Get(),
			c.catFor, c.toggleGroup, c.toggleSelect, c.setManual,
			func() { c.selectTier(tierReady) }, func() { c.selectTier(tierLook) }, c.apply, c.clearSelection, c.makeRules,
			c.notice.Get(), c.pr, reviewview.Focus(c.focusRow.Get(), len(c.idx.Rows)),
			ui.CreateElement(reviewScanStrip, reviewScanStripProps{
				Gaps: len(c.gapRows), GapCharges: c.gapCharges, State: c.scanState.Get(),
				Filled: c.scanStats.Get()[0], Skipped: c.scanStats.Get()[1], Err: c.scanErr.Get(),
				HasProvider: c.hasProvider, OnScan: c.scan, OnUse: c.acceptScan, CanUse: len(c.aiProposals.Get()) > 0,
			}), setupLink, c.pickerRev.Get())
	} else {
		body, foot = reviewSingleView(c.singleArgs())
	}
	body = Div(css.Class("rvs-with-loan-payments"), c.loanPaymentGroup, body)
	var pick ui.Node = Fragment()
	if key := c.picker.Get(); key != "" {
		pick = ui.CreateElement(CategoryPicker, CategoryPickerProps{Kind: reviewview.PickerKind(c.idx, key), Selected: c.manual.Get()[key],
			OnPick: func(category string) { c.setManual(key, category); c.picker.Set("") }, OnClose: func() { c.picker.Set("") },
		})
	}
	return Div(css.Class("rvs"), Attr("data-testid", "review-inbox"),
		Attr("data-focus", strconv.Itoa(reviewview.Focus(c.focusRow.Get(), len(c.idx.Rows)))), scopeStrip, head,
		Div(css.Class("rvs-body"), body), Div(css.Class("rvs-foot modal-foot"), foot), pick,
	)
}

type reviewEmptyViewProps struct {
	Scope, Loans ui.Node
	Title, Sub   string
}

func reviewEmptyView(p reviewEmptyViewProps) ui.Node {
	close := ui.UseEvent(func() { uistate.ResetReviewSession(); uistate.CloseReviewInbox() })
	return Div(css.Class("rvs rvs-done"), Attr("data-testid", "review-inbox"), p.Scope, p.Loans,
		P(css.Class("rvw-done-title"), p.Title), P(css.Class("rvw-done-sub"), p.Sub),
		Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "review-done"), OnClick(close), uistate.T("review.done")),
	)
}

type reviewModeHeaderProps struct {
	Mode     string
	OnSelect func(string)
}

func reviewModeHeader(p reviewModeHeaderProps) ui.Node {
	return Div(css.Class("rvs-head"), uiw.Segmented(uiw.SegmentedProps{
		Label: uistate.T("review.modeLabel"), Selected: p.Mode,
		Options: []uiw.SegOption{
			{Value: reviewModeSingle, Label: uistate.T("review.modeSingle"), TestID: "review-mode-single"},
			{Value: reviewModeBulk, Label: uistate.T("review.modeBulk"), TestID: "review-mode-bulk"},
		}, OnSelect: p.OnSelect,
	}), reviewKeyboardDisclosure(reviewview.Text(uistate.T).KeyboardHint(p.Mode == reviewModeBulk)))
}

type reviewAISetupProps struct{ OnClick func() }

func reviewAISetup(p reviewAISetupProps) ui.Node { return reviewAISetupLink(p.OnClick) }

func reviewKeyboardDisclosure(hint string) ui.Node {
	return Details(css.Class("rvs-kbd"), Attr("data-testid", "review-kbd"),
		Summary(uistate.T("shortcuts.title")),
		Span(Attr("data-testid", "review-kbd-hint"), hint),
	)
}

// reviewReturnLabel names the review task a cross-page trip is stepping out of,
// for the return crumb: which scope, and how much of it is left. "Back to Review
// (current view, 12 left)" tells the user which job they are returning to;
// "Back" alone is what C581 already found wanting on the ledger.
func reviewReturnLabel(scope reviewscope.Scope, idx reviewIndex) string {
	return reviewview.Text(uistate.T).ReturnLabel(scope, idx)
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
			Attr("data-scope", string(props.Scope)))
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
	return reviewview.Text(uistate.T).WorkingSetPhrase(c)
}

// reviewScopeNote is the sentence under the scope control: what the selected
// scope covers, and — crucially — how much sits OUTSIDE it. A scope that states
// only its own size leaves the reader unable to tell a small queue from a narrow
// filter.
func reviewScopeNote(scope reviewscope.Scope, view, queue int, working string) string {
	return reviewview.Text(uistate.T).ScopeNote(scope, view, queue, working)
}

// fmtTierCount renders "N merchants · M charges" for a tier header.
func fmtTierCount(rows []reviewRow, t reviewTier) (merchants, charges int) {
	return reviewview.TierCount(rows, t)
}

func tierLabel(t reviewTier) string {
	return reviewview.Text(uistate.T).TierLabel(t)
}

func tierMod(t reviewTier) string {
	return reviewview.TierClass(t)
}

func reviewIntro(charges, merchants int) ui.Node {
	return Div(css.Class("rvs-intro"), Attr("data-testid", "review-intro"),
		H2(css.Class("rvs-intro-title"), uistate.T("reviewInbox.categoriesTitle")),
		P(css.Class("rvs-intro-count"), Attr("data-testid", "review-total-line"),
			uistate.T("reviewInbox.countLine", charges, merchants)),
	)
}

// reviewBulkView renders the tiered merchant list and its pinned footer.
func reviewBulkView(
	app *appstate.App, idx reviewIndex,
	sel map[string]bool, openG map[string]bool, manual map[string]string,
	catFor func(reviewRow) string,
	toggleGroup, toggleSelect func(string), setManual func(string, string),
	selReady, selLook, applySel, clearSel, makeRules func(), notice string, pr prefs.Prefs, focused int,
	scanStrip, setupLink ui.Node, rev int,
) (ui.Node, ui.Node) {
	props := reviewBulkProps{App: app, Index: idx, Selected: sel, Open: openG, Manual: manual,
		CatFor: catFor, ToggleGroup: toggleGroup, ToggleSelect: toggleSelect, SetManual: setManual,
		SelectReady: selReady, SelectLook: selLook, Apply: applySel, Clear: clearSel, MakeRules: makeRules,
		Notice: notice, Prefs: pr, Focused: focused, ScanStrip: scanStrip, SetupLink: setupLink, Revision: rev,
	}
	return ui.CreateElement(reviewBulkBody, props), ui.CreateElement(reviewBulkFooter, props)
}

type reviewBulkProps struct {
	App                                              *appstate.App
	Index                                            reviewIndex
	Selected, Open                                   map[string]bool
	Manual                                           map[string]string
	CatFor                                           func(reviewRow) string
	ToggleGroup, ToggleSelect                        func(string)
	SetManual                                        func(string, string)
	SelectReady, SelectLook, Apply, Clear, MakeRules func()
	Notice                                           string
	Prefs                                            prefs.Prefs
	Focused, Revision                                int
	ScanStrip, SetupLink                             ui.Node
}

func reviewBulkBody(p reviewBulkProps) ui.Node {
	idx, notice, scanStrip := p.Index, p.Notice, p.ScanStrip
	decisions := len(idx.Rows)

	var tiers []ui.Node
	for _, t := range []reviewTier{tierReady, tierLook, tierNone} {
		m, c := fmtTierCount(idx.Rows, t)
		if m == 0 {
			continue
		}
		tiers = append(tiers, ui.CreateElement(reviewBulkTier, reviewBulkTierProps{Bulk: p, Tier: t, Merchants: m, Charges: c}))
	}

	return Div(
		reviewIntro(idx.Total, decisions),
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

}

type reviewBulkTierProps struct {
	Bulk               reviewBulkProps
	Tier               reviewTier
	Merchants, Charges int
}

func reviewBulkTier(props reviewBulkTierProps) ui.Node {
	p, t, m, c := props.Bulk, props.Tier, props.Merchants, props.Charges
	app, idx, focused, rev, pr := p.App, p.Index, p.Focused, p.Revision, p.Prefs
	sel, openG, manual, catFor := p.Selected, p.Open, p.Manual, p.CatFor
	toggleGroup, toggleSelect, setManual := p.ToggleGroup, p.ToggleSelect, p.SetManual
	rows, focusKey := reviewview.TierRows(idx.Rows, t, focused)
	selectReadyClick := OnClick(p.SelectReady)
	selectLookClick := OnClick(p.SelectLook)

	var act ui.Node = Fragment()
	switch t {
	case tierReady:
		act = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-selectall-ready"), selectReadyClick,
			uistate.T("review.selectAll", m))
	case tierLook:
		act = Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "review-selectall-look"), selectLookClick,
			uistate.T("review.selectAll", m))
	}
	return Div(css.Class("rvs-tier "+tierMod(t)), Attr("data-tier", tierMod(t)),
		Attr("role", "group"), Attr("aria-label", tierLabel(t)),
		Div(css.Class("rvs-tier-head"),
			Span(css.Class("rvs-tier-mark")),
			Span(css.Class("rvs-tier-name"), tierLabel(t)),
			Span(css.Class("rvs-tier-count"), uistate.T("review.tierCountPlain", plural(m, "merchant"), plural(c, "charge"))),
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
	)
}

func reviewBulkFooter(p reviewBulkProps) ui.Node {
	// Footer: selection summary + the actions.
	selMerchants, selCharges := reviewview.SelectionCount(p.Index.Rows, p.Selected)
	label, sub, confirm := reviewview.Text(uistate.T).SelectionLabels(selMerchants, selCharges)
	makeRulesClick := OnClick(p.MakeRules)

	var disabled any
	if selMerchants == 0 {
		disabled = Attr("disabled", "disabled")
	}
	return Fragment(
		Span(css.Class("rvs-foot-n"), Attr("data-testid", "review-selection"),
			Attr("role", "status"), Attr("aria-live", "polite"), label),
		Span(css.Class("rvs-foot-sub"), sub),
		p.SetupLink,
		Span(css.Class("rvs-foot-acts"),
			If(selMerchants > 0,
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-makerule"),
					makeRulesClick, uistate.T("review.makeRule"))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-clear"),
				disabled,
				OnClick(p.Clear), uistate.T("review.clear")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "review-apply"), disabled,
				OnClick(p.Apply), confirm),
		),
	)
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
	// Each child registers its plain callbacks at stable hook positions.
	Confirm, Skip, Dismiss, NewCat, CancelAll func()
	ToCategories, ToRules, PickCategory       func(ui.Event)
	Notice                                    string
	Prefs                                     prefs.Prefs
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
	if a.Txn.ID == "" {
		return P(css.Class("rvs-note"), uistate.T("review.emptySub")), Fragment()
	}
	return ui.CreateElement(reviewSingleBody, a), ui.CreateElement(reviewSingleFooter, a)
}

func reviewSingleBody(a reviewSingleArgs) ui.Node {
	return Div(css.Class("rvs-single"),
		Div(ui.CreateElement(reviewSingleCard, a), ui.CreateElement(reviewSingleAssignment, a)),
		ui.CreateElement(reviewCommitScope, a), ui.CreateElement(reviewSingleLinks, a),
		ui.CreateElement(reviewContextBand, reviewContextBandProps{App: a.App, Index: a.Index, Row: a.Row, Txn: a.Txn, Prefs: a.Prefs}),
	)
}

func reviewSingleCard(a reviewSingleArgs) ui.Node {
	app, row, cur, pr := a.App, a.Row, a.Txn, a.Prefs
	reason := reviewqueue.ReasonFor(cur)
	return Div(css.Class("rvw-card"),
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
			reviewMoneyNode("rvw-amount", cur.Amount),
			If(reviewAcctName(app, cur.AccountID) != "",
				Span(css.Class("rvw-acct"), reviewAcctName(app, cur.AccountID))),
		),
	)
}

func reviewSingleAssignment(a reviewSingleArgs) ui.Node {
	row, notice := a.Row, a.Notice
	canCategorize := reviewqueue.CanAssignCategory(a.Txn)
	catFor := a.CatFor
	selArgs := []any{css.Class("field"), Attr("data-testid", "review-category-select"),
		Attr("aria-label", uistate.T("review.categoryLabel")), OnChange(a.PickCategory)}
	if !canCategorize {
		selArgs = append(selArgs, Attr("disabled", "disabled"))
	}
	selArgs = append(selArgs, categorySelectNodes(a.App.Categories(), reviewview.Kind(a.Txn), a.CatFor(a.Row))...)

	return Div(css.Class("rvs-assign"),
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
				Attr("disabled", disabledIf(!canCategorize)),
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
		If(!canCategorize,
			P(css.Class("rvs-warn"), Attr("data-testid", "review-split-warn"),
				uistate.T("review.splitNeeded"))),
		If(notice != "", P(css.Class("rvs-notice"), Attr("role", "alert"),
			Attr("data-testid", "review-commit-err"), notice)),
	)
}

func reviewCommitScope(a reviewSingleArgs) ui.Node {
	scopeLine := []any{css.Class("rvs-cscope"), Attr("data-testid", "review-commit-scope")}
	if a.GroupSize > 1 {
		scopeLine = append(scopeLine, uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("review.commitScopeLabel"),
			Selected: string(a.Commit),
			Options: []uiw.SegOption{
				{Value: string(reviewscope.CommitCharge), Label: uistate.T("review.scopeThisCharge"),
					TestID: "review-scope-charge"},
				{Value: string(reviewscope.CommitMerchant),
					Label:  uistate.T("review.scopeAllMerchant", a.GroupSize, reviewview.MerchantName(a.Row)),
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
	return Div(scopeLine...)

}

func reviewSingleLinks(a reviewSingleArgs) ui.Node {
	leaveGlyph := Span(css.Class("rvs-links-out"), Attr("aria-hidden", "true"),
		uistate.T("review.leavesReviewGlyph"))
	return Div(css.Class("rvs-links"), Attr("data-testid", "review-cross-links"),
		Span(css.Class("rvs-links-lead"), uistate.T("review.linksLead")),
		reviewDestinationLink("review-goto-categories", "/categories", uistate.T("review.gotoCategories"),
			uistate.T("review.leavesReviewAria", uistate.T("review.gotoCategories")), a.ToCategories, leaveGlyph),
		reviewDestinationLink("review-goto-rules", "/rules", uistate.T("review.gotoRules"),
			uistate.T("review.leavesReviewAria", uistate.T("review.gotoRules")), a.ToRules, leaveGlyph),
	)

}

func reviewSingleActions(a reviewSingleArgs) ui.Node {
	canCategorize := reviewqueue.CanAssignCategory(a.Txn)
	confirmLabel, snoozeLabel, dismissLabel := reviewview.Text(uistate.T).ActionLabels(a.Targets)
	confirmClick := OnClick(a.Confirm)
	var commitControl ui.Node
	if canCategorize {
		commitControl = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-commit"), confirmClick, confirmLabel)
	} else {
		commitControl = Span(css.Class("rvs-warn"), Attr("data-testid", "review-split-action"), uistate.T("review.splitNeeded"))
	}
	return Span(css.Class("rvs-foot-acts"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-dismiss"),
			OnClick(a.Dismiss), dismissLabel),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-skip"),
			OnClick(a.Skip), snoozeLabel),
		commitControl,
	)
}

func reviewSingleFooter(a reviewSingleArgs) ui.Node {
	app, idx, row, pr, catFor := a.App, a.Index, a.Row, a.Prefs, a.CatFor
	canCategorize := reviewqueue.CanAssignCategory(a.Txn)
	acts := ui.CreateElement(reviewSingleActions, a)
	if canCategorize && a.Confirming {
		acts = ui.CreateElement(reviewConfirmAll, reviewConfirmAllProps{
			Targets:  a.Targets,
			Merchant: reviewview.MerchantName(row),
			Category: reviewCatName(app, catFor(row)),
			Facts:    reviewGroupFacts(row, pr),
			OnCancel: a.CancelAll, OnGo: a.Confirm,
		})
	}
	return Fragment(
		Span(css.Class("rvs-foot-sub"), Attr("data-testid", "review-progress"),
			uistate.T("review.leftCount", idx.Total)),
		Span(css.Class("rvs-hidden"), Attr("data-testid", "review-total"), strconv.Itoa(idx.Total)),
		acts,
	)
}

// reviewAISetupLink keeps the Smart+ key invitation discoverable without taking
// space away from the charges being reviewed.
func reviewAISetupLink(onClick any) ui.Node {
	return A(css.Class("btn-link rvs-ai-setup-link"),
		Attr("data-testid", "review-ai-setup"),
		Attr("href", uistate.RoutePath("/settings/ai")),
		OnClick(onClick), uistate.T("review.connectAI"))
}

// isPlainLeftClick keeps link side effects scoped to the same clicks that the global
// router interceptor handles. Modified clicks remain native and leave this surface intact.
func isPlainLeftClick(e ui.Event) bool {
	event := e.JSValue()
	button := event.Get("button")
	numeric := button.Type() == js.TypeNumber
	value := 0
	if numeric {
		value = button.Int()
	}
	return reviewview.PlainLeftClick(value, numeric, event.Get("metaKey").Truthy(), event.Get("ctrlKey").Truthy(), event.Get("shiftKey").Truthy(), event.Get("altKey").Truthy())
}

// reviewDestinationLink renders a cross-surface destination as a real router link.
// Its click handler only records the return context; the shared anchor interceptor
// owns ordinary in-app navigation.
func reviewDestinationLink(testID, route, label, accessibleName string, onClick any, trailing ui.Node) ui.Node {
	clickOption := OnClick(onClick)
	args := []any{css.Class("btn-link"),
		Attr("data-testid", testID),
		Attr("href", uistate.RoutePath(route)),
		Attr("aria-label", accessibleName),
		Attr("title", accessibleName)}
	if onClick != nil {
		args = append(args, clickOption)
	}
	args = append(args, label, trailing)
	return A(args...)
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
	return reviewview.Text(uistate.T).GroupFacts(row, pr, reviewMoneyText)
}

func reviewReasonLabel(r reviewqueue.Reason) string {
	return reviewview.Text(uistate.T).ReasonLabel(r)
}
