// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Discover runs the full deterministic pipeline over the transactions and returns
// new candidates plus the cluster matches that belong to existing commitments.
// The caller supplies already-resolved payees on each Txn (via internal/payeealias),
// the existing commitments to dedupe against, the user's clustering pins, and
// options. The result is deterministic and independent of transaction insertion
// order.
func Discover(txns []Txn, existing []Commitment, pins Pins, opts Options) Result {
	now := opts.Now
	if now.IsZero() {
		now = latestDate(txns)
	}

	clusters := Cluster(txns, pins)

	var res Result
	for _, c := range clusters {
		// Evidence is computed up front because dedupe needs it: a commitment can
		// be recognised by its amount+cadence fingerprint, not just by name.
		ev, hasEv := buildEvidence(c.Txns, now, opts)
		// Dedupe first: a cluster matching an existing commitment's matcher is a
		// set of cycles for that commitment, never a new candidate.
		if id, ok := matchExisting(c, ev, hasEv, existing); ok {
			if !hasEv {
				// No coherent rhythm/cost, but the identity match still means these
				// transactions belong to the commitment; report them as cycles with a
				// minimal evidence shell.
				ev = minimalEvidence(c.Txns)
			}
			res.CycleMatches = append(res.CycleMatches, CommitmentCycle{
				CommitmentID: id,
				Evidence:     ev,
				TxnIDs:       txnIDs(c.Txns),
			})
			continue
		}

		if pins.Suppressed != nil && pins.Suppressed[c.Canonical] {
			continue // user marked this signature "not recurring"
		}

		for _, cand := range candidatesFromCluster(c, now, opts) {
			res.Candidates = append(res.Candidates, cand)
		}
	}

	sortCandidates(res.Candidates)
	sortCycles(res.CycleMatches)
	return res
}

// candidatesFromCluster turns one signature cluster into zero, one, or two
// candidates: it first tries to split a same-signature cluster that holds two
// concurrent subscriptions, then models each (sub)group's rhythm and cost, keeping
// only groups with a usable rhythm, coherent cost, and enough occurrences.
func candidatesFromCluster(c SignatureCluster, now time.Time, opts Options) []Candidate {
	txns := append([]Txn(nil), c.Txns...)
	sort.SliceStable(txns, func(i, j int) bool {
		if !txns[i].Date.Equal(txns[j].Date) {
			return txns[i].Date.Before(txns[j].Date)
		}
		return txns[i].ID < txns[j].ID
	})

	// Determine the split floor from the fastest plausible cadence (monthly).
	splitFloor := CadenceMonthly.evidenceFloor()
	if opts.MinOccurrences > 0 {
		splitFloor = opts.MinOccurrences
	}

	var groups [][]Txn
	if parts := splitAmounts(txns, splitFloor); parts != nil {
		for _, idx := range parts {
			g := make([]Txn, len(idx))
			for i, k := range idx {
				g[i] = txns[k]
			}
			groups = append(groups, g)
		}
	} else {
		groups = [][]Txn{txns}
	}

	var out []Candidate
	for _, g := range groups {
		if cand, ok := candidateFromGroup(c, g, now, opts); ok {
			out = append(out, cand)
		}
	}
	return out
}

// candidateFromGroup builds a candidate from one coherent group of transactions,
// or reports ok=false when the group has no usable rhythm, incoherent cost, or too
// few occurrences for its cadence.
func candidateFromGroup(c SignatureCluster, g []Txn, now time.Time, opts Options) (Candidate, bool) {
	ev, ok := buildEvidence(g, now, opts)
	if !ok {
		return Candidate{}, false
	}
	rh := detectRhythm(datesOf(g))
	cost := analyzeCost(amountsOf(g), datesOf(g))
	floor := effectiveFloor(rh.cadence, opts)
	conf, tier := scoreConfidence(rh, cost, len(g), floor, ev.LastSeen, now)

	return Candidate{
		Signature:  c.Canonical,
		Payee:      representativePayee(g),
		Direction:  c.Direction,
		AccountID:  c.AccountID,
		IsIncome:   c.Direction == In,
		Confidence: conf,
		Tier:       tier,
		Evidence:   ev,
	}, true
}

// buildEvidence runs rhythm + cost over a group and, when both cohere and the
// occurrence count clears the cadence's evidence floor, assembles the Evidence the
// UI renders. It reports ok=false otherwise.
func buildEvidence(g []Txn, now time.Time, opts Options) (Evidence, bool) {
	if len(g) < 2 {
		return Evidence{}, false
	}
	dates := datesOf(g)
	rh := detectRhythm(dates)
	if rh.cadence == CadenceUnknown || rh.fit < minRhythmFit {
		return Evidence{}, false
	}
	if len(g) < effectiveFloor(rh.cadence, opts) {
		return Evidence{}, false
	}
	cost := analyzeCost(amountsOf(g), dates)
	if !cost.coherent {
		return Evidence{}, false
	}

	first, last := dates[0], dates[0]
	for _, d := range dates {
		if d.Before(first) {
			first = d
		}
		if d.After(last) {
			last = d
		}
	}
	cur := ""
	if len(g) > 0 {
		cur = g[0].Currency
	}
	return Evidence{
		Count:        len(g),
		Cadence:      rh.cadence,
		AnchorDay:    rh.anchorDay,
		WindowSpread: rh.spread,
		PostsBy:      rh.postsBy,
		Amount:       cost.model,
		FirstSeen:    dayOf(first),
		LastSeen:     dayOf(last),
		TxnIDs:       txnIDs(g),
		Currency:     cur,
	}, true
}

// effectiveFloor is the cadence's evidence floor with the MinOccurrences override
// applied to monthly-or-faster cadences (the long-cadence floor of 2 stands).
func effectiveFloor(c Cadence, opts Options) int {
	floor := c.evidenceFloor()
	if opts.MinOccurrences > 0 && floor > 2 {
		return opts.MinOccurrences
	}
	return floor
}

// minimalEvidence is the evidence shell for a deduped cluster whose rhythm/cost
// did not cohere — enough to report the matched cycles.
func minimalEvidence(g []Txn) Evidence {
	dates := datesOf(g)
	first, last := dates[0], dates[0]
	for _, d := range dates {
		if d.Before(first) {
			first = d
		}
		if d.After(last) {
			last = d
		}
	}
	cur := ""
	if len(g) > 0 {
		cur = g[0].Currency
	}
	return Evidence{
		Count:     len(g),
		Cadence:   CadenceUnknown,
		FirstSeen: dayOf(first),
		LastSeen:  dayOf(last),
		TxnIDs:    txnIDs(g),
		Currency:  cur,
	}
}

// matchExisting reports the id of an existing commitment whose matcher covers
// this cluster (same direction, compatible account, matching identity), if any.
//
// Identity is established by any of three signals, strongest first:
//
//  1. the signatures the commitment is KNOWN to match — derived by the caller
//     from the transactions already settled/linked to it. This is what makes a
//     "Mortgage payment" flow recognise its own "MERIDIAN DATA" charges.
//  2. its display name, compared on the CORE signature (the non-'#' tokens) so a
//     clean "Spotify" bridges a cluster canonical of "SPOTIFY #".
//  3. an amount+cadence fingerprint on the same account, for the common case
//     where nothing has been linked yet.
//
// ev/hasEv are the cluster's evidence, needed only by the fingerprint signal.
func matchExisting(c SignatureCluster, ev Evidence, hasEv bool, existing []Commitment) (string, bool) {
	cCore := coreSignature(c.Canonical)
	for _, e := range existing {
		if e.Direction != c.Direction {
			continue
		}
		if e.AccountID != "" && c.AccountID != "" && e.AccountID != c.AccountID {
			continue
		}
		// (1) signatures this commitment has actually been paying.
		matched := false
		for _, s := range e.Signatures {
			if coreMatches(coreSignature(s), cCore) {
				matched = true
				break
			}
		}
		// (2) the display name.
		if !matched && coreMatches(coreSignature(Signature(e.Payee)), cCore) {
			matched = true
		}
		// (3) the amount+cadence fingerprint, only on a shared account.
		if !matched && hasEv && e.AccountID != "" && e.AccountID == c.AccountID && e.matchesFingerprint(ev) {
			matched = true
		}
		if matched {
			return e.ID, true
		}
	}
	return "", false
}

// FromDomainCadence maps a persisted domain cadence onto discovery's richer
// rhythm so a caller can hand an existing commitment's declared cadence to the
// dedupe fingerprint. Daily has no discovery equivalent (nothing that frequent is
// modelled as a commitment) and maps to CadenceUnknown, which disables the
// fingerprint rather than guessing.
func FromDomainCadence(c domain.RecurringCadence) Cadence {
	switch c {
	case domain.CadenceWeekly:
		return CadenceWeekly
	case domain.CadenceBiweekly:
		return CadenceBiweekly
	case domain.CadenceSemimonthly:
		return CadenceSemimonthly
	case domain.CadenceMonthly:
		return CadenceMonthly
	case domain.CadenceQuarterly:
		return CadenceQuarterly
	case domain.CadenceYearly:
		return CadenceAnnual
	default:
		return CadenceUnknown
	}
}

// coreSignature drops the '#' reference placeholders from a signature, leaving the
// stable merchant tokens — the identity a clean commitment name shares with a
// noisy cluster canonical.
func coreSignature(sig string) string {
	toks := strings.Fields(sig)
	kept := make([]string, 0, len(toks))
	for _, t := range toks {
		if t == "#" {
			continue
		}
		kept = append(kept, t)
	}
	return strings.Join(kept, " ")
}

// coreMatches reports whether two core signatures identify the same merchant:
// exact (non-empty), or fuzzy at or above the join threshold when both clear the
// length floor.
func coreMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if len(a) < minFuzzySignatureLen || len(b) < minFuzzySignatureLen {
		return false
	}
	return tokenSetSimilarity(a, b) >= fuzzyJoinThreshold
}

// sigMatches reports whether two signatures identify the same merchant: exact, or
// fuzzy at or above the join threshold when both clear the length floor. Used by
// re-verification where both signatures are quarantined cluster canonicals.
func sigMatches(a, b string) bool {
	if a == b {
		return a != ""
	}
	if len(a) < minFuzzySignatureLen || len(b) < minFuzzySignatureLen {
		return false
	}
	return tokenSetSimilarity(a, b) >= fuzzyJoinThreshold
}

// representativePayee returns the most common raw payee among the group (ties
// broken lexicographically) for display.
func representativePayee(g []Txn) string {
	counts := map[string]int{}
	for _, t := range g {
		counts[t.Payee]++
	}
	best, bestN := "", -1
	for p, n := range counts {
		if n > bestN || (n == bestN && (best == "" || p < best)) {
			best, bestN = p, n
		}
	}
	return best
}

// sortCandidates orders candidates deterministically: strongest tier first, then
// by signature, direction, account, and typical amount.
func sortCandidates(cs []Candidate) {
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].Tier != cs[j].Tier {
			return cs[i].Tier > cs[j].Tier
		}
		if cs[i].Signature != cs[j].Signature {
			return cs[i].Signature < cs[j].Signature
		}
		if cs[i].Direction != cs[j].Direction {
			return cs[i].Direction < cs[j].Direction
		}
		if cs[i].AccountID != cs[j].AccountID {
			return cs[i].AccountID < cs[j].AccountID
		}
		return cs[i].Evidence.Amount.Typical < cs[j].Evidence.Amount.Typical
	})
}

// sortCycles orders cycle matches by commitment id for determinism.
func sortCycles(cs []CommitmentCycle) {
	sort.SliceStable(cs, func(i, j int) bool {
		return cs[i].CommitmentID < cs[j].CommitmentID
	})
}

func latestDate(txns []Txn) time.Time {
	var t time.Time
	for _, x := range txns {
		if x.Date.After(t) {
			t = x.Date
		}
	}
	return t
}

func datesOf(g []Txn) []time.Time {
	out := make([]time.Time, len(g))
	for i, t := range g {
		out[i] = t.Date
	}
	return out
}

func amountsOf(g []Txn) []int64 {
	out := make([]int64, len(g))
	for i, t := range g {
		out[i] = t.AmountMinor
	}
	return out
}

func txnIDs(g []Txn) []string {
	out := make([]string, len(g))
	for i, t := range g {
		out[i] = t.ID
	}
	return out
}
