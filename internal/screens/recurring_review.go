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
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
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
	existing := make([]recurdiscover.Commitment, 0)
	for _, r := range app.Recurring() {
		dir := recurdiscover.Out
		if !r.Amount.IsNegative() {
			dir = recurdiscover.In
		}
		existing = append(existing, recurdiscover.Commitment{ID: r.ID, Payee: r.Label, AccountID: r.AccountID, Direction: dir})
	}
	res := recurdiscover.Discover(txns, existing, loadRecurPins(), recurdiscover.Options{Now: now})

	// txn date lookup so a confirm can back-claim each evidence transaction as its
	// own prior cycle.
	dateOf := map[string]time.Time{}
	for _, t := range app.Transactions() {
		dateOf[t.ID] = t.Date
	}

	var groupA, leftovers []recurdiscover.Candidate
	for _, c := range res.Candidates {
		if c.Tier == recurdiscover.TierSilent {
			leftovers = append(leftovers, c)
		} else {
			groupA = append(groupA, c)
		}
	}
	// Most valuable first: confidence tier, then monthly cost impact.
	rhyOrderCandidates(groupA)
	rhyOrderCandidates(leftovers)

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
				RecurringID: r.ID, OccurrenceDate: dateOf[tid], CreatedAt: now,
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
				Cand: cand, Base: base, OnConfirm: onConfirm, OnReject: onReject,
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
				Cand: cand, Base: base, IsPlus: true, Verified: v.Verified, Note: note,
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
	var footer ui.Node
	if !aiOpen.Get() && len(leftovers) > 0 {
		if hasKey {
			footer = Div(css.Class("rhy-review-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-smartplus-optin"),
					OnClick(runDeeper),
					"✦ "+uistate.T("rhythm.lookDeeper", estTokens)),
			)
		} else {
			footer = Div(css.Class("rhy-review-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("disabled", "true"), Attr("data-testid", "rhy-smartplus-optin"),
					"✦ "+uistate.T("rhythm.lookDeeper", estTokens)),
				Span(css.Class("muted"), uistate.T("rhythm.lookDeeperNoKey")),
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
	// The header states the honest total up front, so paging never hides scale.
	title := uistate.T("rhythm.reviewTitleCount", len(groupA)+len(leftovers))
	return rhySection("sec-review", title, uistate.T("rhythm.reviewNote"), nil, Fragment(secBody...))
}

// rhyReviewPageSize is the default candidates-per-page in the review strip.
const rhyReviewPageSize = 5

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
	Cand      recurdiscover.Candidate
	Base      string
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

	sig := recurdiscover.Signature(c.Payee)
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
		If(show.Get(), rhyEvidenceList(c.Evidence, props.Base)),
	)
}

// rhyEvidenceList renders the expandable per-transaction evidence.
func rhyEvidenceList(ev recurdiscover.Evidence, base string) ui.Node {
	items := []any{css.Class("rhy-ev-list")}
	for i, tid := range ev.TxnIDs {
		amt := fmtMoney(money.New(ev.Amount.Typical, base))
		items = append(items, Li(
			Span(strconv.Itoa(i+1)+". "+tid),
			Span(amt),
		))
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
