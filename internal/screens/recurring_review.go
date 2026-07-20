// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// buildDiscoverTxns converts the store's transactions into the discovery input:
// resolved payees, positive magnitudes in the base currency, and the in/out
// direction. Transfers between the household's own accounts are excluded (they
// aren't recurring commitments).
func buildDiscoverTxns(app *appstate.App, rates currency.Rates) []recurdiscover.Txn {
	txns := app.Transactions()
	out := make([]recurdiscover.Txn, 0, len(txns))
	for _, t := range txns {
		if t.TransferAccountID != "" || t.Amount.Amount == 0 {
			continue
		}
		dir := recurdiscover.Out
		mag := t.Amount.Amount
		if mag > 0 {
			dir = recurdiscover.In
		} else {
			mag = -mag
		}
		if conv, err := rates.Convert(money.New(mag, t.Amount.Currency), rates.Base); err == nil {
			mag = conv.Amount
		}
		out = append(out, recurdiscover.Txn{
			ID: t.ID, Date: t.Date, Payee: app.ResolvePayee(t.Payee),
			AmountMinor: mag, AccountID: t.AccountID, Direction: dir, Currency: rates.Base,
		})
	}
	return out
}

// rhyCommitments describes every existing recurring flow to the discovery engine
// well enough that it is never re-proposed as a "new" candidate. Matching on the
// display label alone fails badly — the household calls it "Mortgage payment"
// while the bank posts "MERIDIAN DATA" — so each commitment also declares:
//
//   - the signatures it has ACTUALLY been paying, harvested from the bill-match
//     TxnLinks and BillAccountID-tagged transactions that settled it; and
//   - an amount + cadence fingerprint, for the (common) case where nothing has
//     been linked to it yet.
func rhyCommitments(app *appstate.App, rates currency.Rates) []recurdiscover.Commitment {
	payeeOf := map[string]string{}
	sigsFor := map[string]map[string]bool{}
	addSig := func(recurringID, payee string) {
		if recurringID == "" || strings.TrimSpace(payee) == "" {
			return
		}
		sig := recurdiscover.Signature(app.ResolvePayee(payee))
		if sig == "" {
			return
		}
		if sigsFor[recurringID] == nil {
			sigsFor[recurringID] = map[string]bool{}
		}
		sigsFor[recurringID][sig] = true
	}
	for _, t := range app.Transactions() {
		payeeOf[t.ID] = t.Payee
		// A transaction tagged as this flow's bill payment is direct evidence of
		// the payee text the flow settles.
		if rid, ok := bills.RecurringIDFromAccount(t.BillAccountID); ok {
			addSig(rid, t.Payee)
		}
	}
	for _, l := range app.TxnLinks() {
		if l.Kind != domain.TxnLinkBillMatch || l.RecurringID == "" {
			continue
		}
		for _, tid := range l.TxnIDs {
			addSig(l.RecurringID, payeeOf[tid])
		}
	}

	recs := app.Recurring()
	out := make([]recurdiscover.Commitment, 0, len(recs))
	for _, r := range recs {
		dir := recurdiscover.Out
		if !r.Amount.IsNegative() {
			dir = recurdiscover.In
		}
		var sigs []string
		for s := range sigsFor[r.ID] {
			sigs = append(sigs, s)
		}
		sort.Strings(sigs) // deterministic input to a deterministic engine
		amt := r.Amount.Abs()
		if conv, err := rates.Convert(amt, rates.Base); err == nil {
			amt = conv
		}
		out = append(out, recurdiscover.Commitment{
			ID: r.ID, Payee: r.Label, AccountID: r.AccountID, Direction: dir,
			Signatures:  sigs,
			AmountMinor: amt.Amount,
			Cadence:     recurdiscover.FromDomainCadence(r.Cadence),
		})
	}
	return out
}

// rhyDemoteNoise lowers candidates that do not read as household commitments to
// the Silent tier, so they surface only in the opt-in deeper lane rather than
// leading the review queue. Nothing is dropped — an uncertain class belongs in
// Silent, not page 1.
//
// Outbound candidates are judged by the subscriptions package's existing
// classification (liability payments, essential spend like groceries and fuel,
// and anything already planned). Inbound candidates are only proposed when they
// read like a genuine paycheck — a pay-like cadence with enough history — so an
// employer deposit is an income commitment but a one-off refund is not.
func rhyDemoteNoise(app *appstate.App, cands []recurdiscover.Candidate) []recurdiscover.Candidate {
	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	liability := map[string]bool{}
	for _, a := range app.Accounts() {
		if a.Class == domain.ClassLiability || a.Type.IsLiability() {
			liability[a.ID] = true
		}
	}
	txnByID := map[string]domain.Transaction{}
	for _, t := range app.Transactions() {
		txnByID[t.ID] = t
	}
	planned := map[string]bool{}
	for _, r := range app.Recurring() {
		planned[strings.ToLower(strings.TrimSpace(r.Label))] = true
	}

	out := make([]recurdiscover.Candidate, 0, len(cands))
	for _, c := range cands {
		keep := true
		switch {
		case c.Direction == recurdiscover.In:
			keep = rhyLooksLikePaycheck(c)
		case planned[strings.ToLower(strings.TrimSpace(c.Payee))]:
			keep = false
		case rhyIsEssentialOrLender(c, txnByID, catName, liability):
			keep = false
		case rhyIsHabitual(c, txnByID, catName):
			// Eating and drinking out is the case the amount tests cannot reach: a
			// daily coffee costs the same $7.35 every time, so it passes every
			// consistency bar a real subscription passes. Only the merchant and the
			// category know it is a habit rather than an obligation.
			keep = false
		case recurdiscover.OverFrequent(c.Evidence):
			// And the case the merchant list cannot reach: a cluster carrying far
			// more charges than its own cadence explains over the span it was seen
			// is describing how often the household goes somewhere, not what it owes.
			keep = false
		case rhyAmountTooVariable(c.Evidence):
			// A commitment is something owed on a schedule, not a merchant visited
			// regularly. Groceries, restaurants and fuel repeat with a wildly
			// varying amount; a real bill or subscription does not.
			keep = false
		}
		if !keep {
			c.Tier = recurdiscover.TierSilent
		}
		out = append(out, c)
	}
	return out
}

// rhyIsEssentialOrLender reuses the subscriptions package's phrase judgment
// against everything the candidate actually carries — its payee, and the
// category / payee / description of the transactions that evidence it — so
// groceries, fuel, utilities, and loan payments never get proposed as new
// commitments. (IsEssentialSpend itself matches on an exact Desc comparison,
// which does not fit payee-keyed candidates.)
func rhyIsEssentialOrLender(c recurdiscover.Candidate, txnByID map[string]domain.Transaction,
	catName map[string]string, liability map[string]bool) bool {
	if subscriptions.IsEssentialName(c.Payee) || subscriptions.IsLenderName(c.Payee) {
		return true
	}
	for _, id := range c.Evidence.TxnIDs {
		t, ok := txnByID[id]
		if !ok {
			continue
		}
		// Money moving to a liability is a transfer against debt, not a new
		// subscription.
		if liability[t.AccountID] {
			return true
		}
		if subscriptions.IsEssentialName(t.Payee) || subscriptions.IsEssentialName(t.Desc) ||
			subscriptions.IsLenderName(t.Payee) || subscriptions.IsLenderName(t.Desc) {
			return true
		}
		if n := catName[t.CategoryID]; n != "" && (subscriptions.IsEssentialName(n) || subscriptions.IsLenderName(n)) {
			return true
		}
	}
	return false
}

// rhyIsHabitual applies the subscriptions package's habitual-spend judgment
// across everything the candidate carries — its payee, and the payee /
// description / category name of each evidencing transaction — so a coffee shop
// or a takeaway habit never leads the review queue.
//
// It mirrors rhyIsEssentialOrLender deliberately: the two judgments answer
// different questions ("must the household buy this?" vs "does the household
// simply buy this often?") and only agree on the answer that matters here —
// neither is a commitment.
func rhyIsHabitual(c recurdiscover.Candidate, txnByID map[string]domain.Transaction,
	catName map[string]string) bool {
	if subscriptions.IsHabitualName(c.Payee) {
		return true
	}
	for _, id := range c.Evidence.TxnIDs {
		t, ok := txnByID[id]
		if !ok {
			continue
		}
		if subscriptions.IsHabitualName(t.Payee) || subscriptions.IsHabitualName(t.Desc) {
			return true
		}
		if n := catName[t.CategoryID]; n != "" && subscriptions.IsHabitualName(n) {
			return true
		}
	}
	return false
}

// rhyVariableAmountShare is how wide a candidate's observed amount spread may be,
// as a fraction of its typical amount, before it reads as variable SPENDING
// rather than a commitment. A subscription is fixed or tightly banded; a weekly
// grocery run is not.
const rhyVariableAmountShare = 0.40

// rhyAmountTooVariable reports whether the observed amounts swing too widely for
// the pattern to be a commitment. A stepped amount (one durable price change) is
// explicitly NOT volatile — that is a price rise on the same commitment.
func rhyAmountTooVariable(ev recurdiscover.Evidence) bool {
	if ev.Amount.Kind != recurdiscover.AmountBanded || ev.Amount.Typical <= 0 {
		return false
	}
	spread := ev.Amount.HighMinor - ev.Amount.LowMinor
	return float64(spread) > float64(ev.Amount.Typical)*rhyVariableAmountShare
}

// rhyLooksLikePaycheck reports whether an inbound candidate reads like real
// income: a pay-like rhythm with at least three sightings. Everything else
// (refunds, transfers, one-off deposits) belongs in the Silent tier.
func rhyLooksLikePaycheck(c recurdiscover.Candidate) bool {
	switch c.Evidence.Cadence {
	case recurdiscover.CadenceWeekly, recurdiscover.CadenceBiweekly,
		recurdiscover.CadenceSemimonthly, recurdiscover.CadenceMonthly:
		return c.Evidence.Count >= 3
	default:
		return false
	}
}

// loadRecurPins reads the persisted discovery pins as recurdiscover.Pins.
func loadRecurPins() recurdiscover.Pins {
	p := uistate.LoadRecurPins()
	supp := map[string]bool{}
	for _, s := range p.Suppressed {
		supp[s] = true
	}
	return recurdiscover.Pins{Suppressed: supp, NeverMerge: p.NeverMerge, ForceMerge: p.ForceMerge}
}

// rhyReviewProps configures the review section (no inputs today).
type rhyReviewProps struct{}

// rhyReviewSection is the "Waiting for your review" strip: the provenance trust
// ladder. Group A is the deterministic Smart candidates (evidence sentence +
// expandable transactions); Group B is the opt-in Smart+ lane (only after the
// user invokes it), each re-verified locally. Its own component so the AI +
// disclosure hooks stay isolated.
func rhyReviewSection(_ rhyReviewProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	now := time.Now()

	aiOpen := ui.UseState(false)
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	aiText := ui.UseState("")
	// Paging state per provenance lane. Confirming/rejecting NEVER resets these —
	// the page is only clamped (pagination.Clamp), so emptying the last page falls
	// back to the previous one instead of bouncing the user to page 1.
	pageA := ui.UseState(1)
	sizeA := ui.UseState(rhyReviewPageSize)
	pageB := ui.UseState(1)
	sizeB := ui.UseState(rhyReviewPageSize)

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := buildDiscoverTxns(app, rates)
	groupA, leftovers := rhySplitCandidates(app, now, base)

	// Evidence transactions by ref: a confirm back-claims each as a prior cycle,
	// and the evidence list renders each one's raw descriptor and real amount.
	txnByID := map[string]domain.Transaction{}
	for _, t := range app.Transactions() {
		txnByID[t.ID] = t
	}

	onConfirm := func(c recurdiscover.Candidate) {
		mag := c.Evidence.Amount.Typical
		m := money.New(mag, base)
		if c.Direction == recurdiscover.Out {
			m = m.Neg()
		}
		cad := recurdiscover.DomainCadence(c.Evidence.Cadence)
		next := rhyNextDue(cad, c.Evidence.LastSeen, now)
		r := domain.Recurring{ID: id.New(), Label: c.Payee, Amount: m, Cadence: cad, NextDue: next, AccountID: c.AccountID}
		if err := app.PutRecurring(r); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		for _, tid := range c.Evidence.TxnIDs {
			_ = app.PutTxnLink(domain.TxnLink{
				ID: id.New(), Kind: domain.TxnLinkBillMatch, TxnIDs: []string{tid},
				RecurringID: r.ID, OccurrenceDate: txnByID[tid].Date, CreatedAt: now,
			})
		}
		uistate.BumpDataRevision()
	}
	onReject := func(c recurdiscover.Candidate) {
		uistate.SuppressSignature(c.Signature)
		uistate.BumpDataRevision()
	}

	var groups []any
	if len(groupA) > 0 {
		psA := sizeA.Get()
		curA := pagination.Clamp(pageA.Get(), len(groupA), psA)
		sA, eA := pagination.Bounds(curA, len(groupA), psA)
		cands := []any{css.Class("rhy-review-page")}
		for _, c := range groupA[sA:eA] {
			cand := c
			cands = append(cands, ui.CreateElement(rhyReviewCand, rhyReviewCandProps{
				Cand: cand, Base: base, Txns: txnByID, OnConfirm: onConfirm, OnReject: onReject,
			}))
		}
		groups = append(groups, Div(css.Class("rhy-review-group"),
			Div(css.Class("rhy-group-head"),
				Span(css.Class("rhy-smark"), uistate.T("rhythm.smartMark")),
				Span(uistate.T("rhythm.reviewSmartGroup", plural(len(groupA), "repeating charge"))),
			),
			Div(cands...),
			uiw.Pager(uiw.PagerProps{
				Page: curA, Total: len(groupA), PageSize: psA,
				PageSizes: []int{5, 10, 25},
				OnPage:    func(n int) { pageA.Set(n) },
				// Changing the page size restarts at page 1 (the standard idiom).
				OnPageSize: func(s int) { sizeA.Set(s); pageA.Set(1) },
				IDPrefix:   "rhy-review",
			}),
		))
	}

	// ── Group B — Smart+ (opt-in) ──
	settings := app.Settings()
	pr := uistate.LoadPrefs().Normalize()
	hasKey := settings.OpenAIKey != "" || pr.BackendActive()
	payload := rhyLeftoverPayload(leftovers, base)
	estTokens := len(payload) / 4

	runDeeper := ui.UseEvent(Prevent(func() {
		aiOpen.Set(true)
		if !hasKey {
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		aiText.Set("")
		model := settings.OpenAIModel
		if model == "" {
			model = "gpt-5.4-mini"
		}
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise personal-finance assistant. From the leftover charge patterns, name any that look like real recurring subscriptions or bills. 2-3 sentences, plain English, never invent amounts."},
			{Role: ai.RoleUser, Content: payload},
		}
		done := func(c string, _ ai.Usage) { aiLoading.Set(false); aiText.Set(c) }
		fail := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if pr.BackendActive() {
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0.4, done, fail)
		} else {
			ai.SendChat(settings.OpenAIKey, ai.DefaultBaseURL, model, messages, 0.4, done, fail)
		}
	}))

	if aiOpen.Get() {
		gb := []any{css.Class("rhy-review-group"),
			Div(css.Class("rhy-group-head"),
				Span(css.Class("rhy-smark is-plus"), uistate.T("rhythm.smartPlusMark")),
				Span(uistate.T("rhythm.reviewPlusGroup", plural(len(leftovers), "pattern"))),
			),
		}
		switch {
		case aiLoading.Get():
			gb = append(gb, P(css.Class("muted"), Attr("data-testid", "rhy-smartplus-loading"), uistate.T("rhythm.lookDeeperBusy")))
		case aiErr.Get() != "":
			gb = append(gb, P(css.Class("err"), Attr("role", "alert"), aiErr.Get()))
		case aiText.Get() != "":
			gb = append(gb, P(css.Class("rhy-cand-reason"), Attr("data-testid", "rhy-smartplus-ai"), aiText.Get()))
		}
		// Each leftover is re-scored locally; verified ones become confirmable, the
		// rest carry an honest "no local way to confirm". Paged in its own lane so
		// the Smart / Smart+ provenance grouping stays legible.
		psB := sizeB.Get()
		curB := pagination.Clamp(pageB.Get(), len(leftovers), psB)
		sB, eB := pagination.Bounds(curB, len(leftovers), psB)
		pageItems := []any{css.Class("rhy-review-page")}
		for _, c := range leftovers[sB:eB] {
			cand := c
			v := recurdiscover.Verify(rhyClaimOf(cand), txns, recurdiscover.Options{Now: now})
			note := uistate.T("rhythm.noLocalConfirm")
			if v.Verified {
				note = uistate.T("rhythm.verifiedLocally")
			}
			pageItems = append(pageItems, ui.CreateElement(rhyReviewCand, rhyReviewCandProps{
				Cand: cand, Base: base, Txns: txnByID, IsPlus: true, Verified: v.Verified, Note: note,
				OnConfirm: onConfirm, OnReject: onReject,
			}))
		}
		gb = append(gb, Div(pageItems...))
		if len(leftovers) > 0 {
			gb = append(gb, uiw.Pager(uiw.PagerProps{
				Page: curB, Total: len(leftovers), PageSize: psB,
				PageSizes:  []int{5, 10, 25},
				OnPage:     func(n int) { pageB.Set(n) },
				OnPageSize: func(s int) { sizeB.Set(s); pageB.Set(1) },
				IDPrefix:   "rhy-review-plus",
			}))
		}
		groups = append(groups, Div(gb...))
	}

	// The opt-in footer: token estimate up front; disabled with an explanation
	// when no key is configured.
	// ONE coherent state: with a key the control invites the deeper look and
	// quotes the cost; without one it is visibly disabled and the sentence
	// explains why, pointing at Settings. Never both at once.
	var footer ui.Node
	if !aiOpen.Get() && len(leftovers) > 0 {
		if hasKey {
			footer = Div(css.Class("rhy-review-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-smartplus-optin"),
					OnClick(runDeeper),
					"✦ "+uistate.T("rhythm.lookDeeper", estTokens)),
			)
		} else {
			footer = Div(css.Class("rhy-review-foot is-disabled"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("disabled", "true"), Attr("aria-disabled", "true"),
					Attr("data-testid", "rhy-smartplus-optin"),
					"✦ "+uistate.T("rhythm.lookDeeperLabel")),
				Span(css.Class("muted"),
					uistate.T("rhythm.lookDeeperNeedsKey", estTokens),
					" ",
					A(css.Class("link"), Href(uistate.RoutePath("/settings")), uistate.T("rhythm.lookDeeperSettings")),
				),
			)
		}
	}

	// Nothing to review and the deeper lane is idle → hide the strip entirely
	// (all hooks above have already run, so this early return is hook-safe).
	if len(groups) == 0 && footer == nil {
		return Fragment()
	}
	secBody := append([]any{}, groups...)
	if footer != nil {
		secBody = append(secBody, footer)
	}
	// ONE arithmetic for the whole strip. The header counts what is actually
	// REVIEWABLE here — the number the lane header and the pager also count — so
	// the three figures agree. The demoted signals are still acknowledged, but as
	// their own honest figure with somewhere to go, never folded into a headline
	// total that leads to a queue five items long.
	title := uistate.T("rhythm.reviewTitleCount", len(groupA))
	var action ui.Node
	if len(leftovers) > 0 {
		action = ui.CreateElement(rhyWeakSignalsLink, rhyWeakSignalsProps{Count: len(leftovers)})
	}
	return rhySection("sec-review", title, uistate.T("rhythm.reviewNote"), action, Fragment(secBody...))
}

// rhyWeakSignalsProps carries the demoted-signal count to the header link.
type rhyWeakSignalsProps struct{ Count int }

// rhyWeakSignalsLink is the route from the review strip's header to the demoted
// signals: it opens Detection preferences, which lists them. Its own component
// so the modal-open hook sits at a stable position.
func rhyWeakSignalsLink(props rhyWeakSignalsProps) ui.Node {
	open := uistate.UseSubsPrefsOpen()
	click := ui.UseEvent(Prevent(func() { open.Set(true) }))
	return Button(css.Class("strip-toggle btn-sm"), Type("button"), Attr("data-testid", "rhy-weak-signals"),
		Title(uistate.T("rhythm.weakSignalsTitle")), OnClick(click),
		uistate.T("rhythm.weakSignalsLink", props.Count))
}

// rhyReviewPageSize is the default candidates-per-page in the review strip.
const rhyReviewPageSize = 5

// rhySplitCandidates runs discovery and splits the result the way the review
// strip does: the candidates actually reviewable in the strip, and the demoted
// Silent ones (the "weaker signals"). Both are ordered most-valuable-first.
//
// It exists so the strip's headline count, its lane headers, its pager, and the
// weak-signals list in Detection preferences all derive from ONE split. They
// previously each counted something different, and the page ended up stating
// three totals — 57, 6 and 5 — of which only the last was reachable.
func rhySplitCandidates(app *appstate.App, now time.Time, base string) (review, weak []recurdiscover.Candidate) {
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	res := recurdiscover.Discover(buildDiscoverTxns(app, rates), rhyCommitments(app, rates),
		loadRecurPins(), recurdiscover.Options{Now: now})
	for _, c := range rhyDemoteNoise(app, res.Candidates) {
		// Every candidate must be able to justify itself. One whose evidence
		// sentence would render empty has nothing to show the user, so it is never
		// proposed — and never counted, in either bucket.
		if rhyEvidenceSentence(c.Evidence, base) == "" {
			continue
		}
		if c.Tier == recurdiscover.TierSilent {
			weak = append(weak, c)
		} else {
			review = append(review, c)
		}
	}
	rhyOrderCandidates(review)
	rhyOrderCandidates(weak)
	return review, weak
}

// rhySlug turns a candidate signature into a selector-friendly testid suffix:
// lowercase, non-alphanumerics collapsed to single hyphens, trimmed. A raw
// signature carries spaces and the '#' reference placeholder ("MSFT XBOX GAME
// PASS #"), which makes for awkward test selectors.
func rhySlug(s string) string {
	var b strings.Builder
	lastDash := true // suppress a leading hyphen
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// rhyOrderCandidates puts the most valuable candidates on the first page:
// strongest confidence tier first, then largest monthly cost impact, with the
// payee as a deterministic tie-break.
func rhyOrderCandidates(cs []recurdiscover.Candidate) {
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].Tier != cs[j].Tier {
			return cs[i].Tier > cs[j].Tier
		}
		mi, mj := rhyMonthlyImpact(cs[i].Evidence), rhyMonthlyImpact(cs[j].Evidence)
		if mi != mj {
			return mi > mj
		}
		return cs[i].Payee < cs[j].Payee
	})
}

// rhyMonthlyImpact normalizes a candidate's typical amount to a per-month figure
// so cadences are comparable when ranking.
func rhyMonthlyImpact(ev recurdiscover.Evidence) int64 {
	a := ev.Amount.Typical
	switch ev.Cadence {
	case recurdiscover.CadenceWeekly:
		return a * 52 / 12
	case recurdiscover.CadenceBiweekly:
		return a * 26 / 12
	case recurdiscover.CadenceSemimonthly:
		return a * 2
	case recurdiscover.CadenceEvery4Weeks:
		return a * 13 / 12
	case recurdiscover.CadenceQuarterly:
		return a / 3
	case recurdiscover.CadenceSemiannual:
		return a / 6
	case recurdiscover.CadenceAnnual:
		return a / 12
	default: // monthly / unknown
		return a
	}
}

// rhyClaimOf turns a Silent candidate into a re-verification claim.
func rhyClaimOf(c recurdiscover.Candidate) recurdiscover.Claim {
	band := c.Evidence.Amount.ToleranceMinor
	if band == 0 {
		band = c.Evidence.Amount.Typical / 20 // ~5% default band
	}
	return recurdiscover.Claim{
		Signatures: []string{c.Signature}, Direction: c.Direction, AccountID: c.AccountID,
		Cadence: c.Evidence.Cadence, AmountMinor: c.Evidence.Amount.Typical, BandMinor: band,
	}
}

// rhyLeftoverPayload serializes the leftover candidates for the Smart+ request —
// only signatures + rhythm/amount series, no raw transactions.
func rhyLeftoverPayload(leftovers []recurdiscover.Candidate, base string) string {
	var b strings.Builder
	for _, c := range leftovers {
		fmt.Fprintf(&b, "%s · %s · ~%s · %d seen · last %s\n",
			c.Payee, rhyCadenceLabel(c.Evidence.Cadence), fmtMoney(money.New(c.Evidence.Amount.Typical, base)),
			c.Evidence.Count, c.Evidence.LastSeen.Format("Jan 2"))
	}
	return b.String()
}

// rhyNextDue steps a domain cadence forward from lastSeen to the first occurrence
// strictly after now.
func rhyNextDue(cad domain.RecurringCadence, lastSeen, now time.Time) time.Time {
	d := cad.Next(lastSeen)
	for i := 0; i < 400 && !d.After(now); i++ {
		d = cad.Next(d)
	}
	return d
}

// rhyReviewCandProps drives one review candidate.
type rhyReviewCandProps struct {
	Cand recurdiscover.Candidate
	Base string
	// Txns resolves an evidence transaction ref to the transaction itself, so the
	// evidence list can show the raw descriptor and the real per-charge amount.
	Txns      map[string]domain.Transaction
	IsPlus    bool
	Verified  bool
	Note      string
	OnConfirm func(recurdiscover.Candidate)
	OnReject  func(recurdiscover.Candidate)
}

// rhyReviewCand renders one candidate with its evidence sentence, an expandable
// transaction list, and the Confirm / Not-recurring verbs. Its own component so
// the disclosure + action hooks stay stable.
func rhyReviewCand(props rhyReviewCandProps) ui.Node {
	c := props.Cand
	show := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { show.Set(!show.Get()) }))
	confirm := ui.UseEvent(Prevent(func() {
		if props.OnConfirm != nil {
			props.OnConfirm(c)
		}
	}))
	reject := ui.UseEvent(Prevent(func() {
		if props.OnReject != nil {
			props.OnReject(c)
		}
	}))

	sig := rhySlug(recurdiscover.Signature(c.Payee))
	var plusNote ui.Node = Fragment()
	if props.IsPlus {
		plusNote = Span(css.Class("rhy-cand-reason"), Style(map[string]string{"margin-left": "0.4rem"}), props.Note)
	}
	evLabel := uistate.T("rhythm.seeEvidence")
	if show.Get() {
		evLabel = uistate.T("rhythm.hideEvidence")
	}
	return Div(css.Class("rhy-cand"), Attr("data-testid", "rhy-review-"+sig),
		Div(css.Class("rhy-cand-top"),
			Span(css.Class("rhy-cand-name"), c.Payee),
			plusNote,
			Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "rhy-review-confirm-"+sig),
				OnClick(confirm), uistate.T("rhythm.confirm")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-review-reject-"+sig),
				OnClick(reject), uistate.T("rhythm.notRecurring")),
		),
		P(css.Class("rhy-cand-ev"), rhyEvidenceSentence(c.Evidence, props.Base)),
		Button(css.Class("btn btn-sm strip-toggle", tw.Mt1), Type("button"), Attr("aria-expanded", ariaBool(show.Get())),
			Attr("data-testid", "rhy-review-evidence-"+sig), OnClick(toggle), evLabel),
		If(show.Get(), rhyEvidenceList(c.Evidence, props.Base, props.Txns)),
	)
}

// rhyEvidenceList renders the expandable per-transaction evidence: the date, the
// RAW bank descriptor, and that transaction's own amount.
//
// The candidate's name is deliberately the cleaned-up merchant ("Xbox Game
// Pass"); the descriptor it was cleaned FROM ("MSFT * XBOX GAME PASS
// 425-6816830") is what the user will recognise on their statement, so the
// evidence list is exactly where it belongs. Per-transaction amounts also make a
// banded candidate legible — a list repeating one typical figure hid the spread
// that justifies the band.
func rhyEvidenceList(ev recurdiscover.Evidence, base string, txns map[string]domain.Transaction) ui.Node {
	items := []any{css.Class("rhy-ev-list")}
	pr := uistate.LoadPrefs()
	for i, tid := range ev.TxnIDs {
		label := strconv.Itoa(i+1) + ". "
		amt := fmtMoney(money.New(ev.Amount.Typical, base))
		if t, ok := txns[tid]; ok {
			label += pr.FormatDate(t.Date) + " · " + strings.TrimSpace(t.Payee)
			amt = fmtMoney(t.Amount.Abs())
		} else {
			label += uistate.T("rhythm.evTxnMissing")
		}
		items = append(items, Li(Span(label), Span(amt)))
	}
	return Ul(items...)
}

// rhyEvidenceSentence composes the plain-English justification from Evidence.
func rhyEvidenceSentence(ev recurdiscover.Evidence, base string) string {
	parts := []string{uistate.T("rhythm.evPayments", ev.Count)}
	parts = append(parts, rhyAnchorText(ev))
	if ev.WindowSpread > 0 && ev.PostsBy > 0 && !rhyWeeklyFamily(ev.Cadence) {
		parts = append(parts, uistate.T("rhythm.evPostsBy", rhyOrdinal(ev.PostsBy)))
	}
	amt := fmtMoney(money.New(ev.Amount.Typical, base))
	if ev.Amount.Kind == recurdiscover.AmountFixed {
		parts = append(parts, uistate.T("rhythm.evEvery", amt))
	} else {
		parts = append(parts, uistate.T("rhythm.evAbout", amt))
	}
	if !ev.LastSeen.IsZero() {
		parts = append(parts, uistate.T("rhythm.evLast", ev.LastSeen.Format("Jan 2")))
	}
	return strings.Join(parts, " · ")
}

// rhyAnchorText renders the cadence + anchor clause, choosing a weekday for the
// weekly family and a day-of-month ordinal otherwise.
func rhyAnchorText(ev recurdiscover.Evidence) string {
	cad := rhyCadenceLabel(ev.Cadence)
	if ev.AnchorDay <= 0 {
		return cad
	}
	if rhyWeeklyFamily(ev.Cadence) {
		return uistate.T("rhythm.evOn", cad, time.Weekday(ev.AnchorDay%7).String())
	}
	return uistate.T("rhythm.evAround", cad, rhyOrdinal(ev.AnchorDay))
}

// rhyWeeklyFamily reports whether the cadence anchors on a weekday rather than a
// day-of-month.
func rhyWeeklyFamily(c recurdiscover.Cadence) bool {
	switch c {
	case recurdiscover.CadenceWeekly, recurdiscover.CadenceBiweekly, recurdiscover.CadenceEvery4Weeks:
		return true
	default:
		return false
	}
}

// rhyCadenceLabel localizes a detected cadence (lowercase, for the sentence).
func rhyCadenceLabel(c recurdiscover.Cadence) string {
	switch c {
	case recurdiscover.CadenceWeekly:
		return uistate.T("rhythm.rcWeekly")
	case recurdiscover.CadenceBiweekly:
		return uistate.T("rhythm.rcBiweekly")
	case recurdiscover.CadenceSemimonthly:
		return uistate.T("rhythm.rcSemimonthly")
	case recurdiscover.CadenceMonthly:
		return uistate.T("rhythm.rcMonthly")
	case recurdiscover.CadenceEvery4Weeks:
		return uistate.T("rhythm.rcEvery4Weeks")
	case recurdiscover.CadenceQuarterly:
		return uistate.T("rhythm.rcQuarterly")
	case recurdiscover.CadenceSemiannual:
		return uistate.T("rhythm.rcSemiannual")
	case recurdiscover.CadenceAnnual:
		return uistate.T("rhythm.rcAnnual")
	default:
		return uistate.T("rhythm.rcUnknown")
	}
}

// rhyOrdinal renders an English ordinal ("9th"). The lowercase suffixes are not
// prose to the screenlint ratchet (digit-bearing / all-lowercase), so they stay
// in code rather than the catalog.
func rhyOrdinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}
