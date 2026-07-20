// SPDX-License-Identifier: MIT

// Package recurdiscover is the deterministic recurring-charge discovery engine
// behind the Bills & Recurring surface. Given a stream of already-normalized
// transactions (the caller resolves noisy processor payees with
// internal/payeealias first), it finds the household's recurring commitments —
// subscriptions, bills, paychecks — without any network or AI call, and produces
// evidence-carrying candidates the UI can render verbatim.
//
// The pipeline is four deterministic stages:
//
//  1. Signature  — quarantine per-transaction reference noise (order ids, auth
//     codes, hashes) to a '#' placeholder, then cluster transactions
//     by an exact-or-fuzzy signature match within the hard keys of
//     account and direction (this file).
//  2. Rhythm     — score each cluster's inter-arrival gaps against candidate
//     cadences (weekly … annual) and infer an anchor day + posting
//     window (rhythm.go).
//  3. Cost       — model the amount as fixed, banded, or stepped (one price
//     change), split a same-signature cluster that actually holds two
//     subscriptions, and reject incoherent noise (cost.go).
//  4. Confidence — combine rhythm fit × cost stability × occurrence count ×
//     liveness into a Likely / NeedsReview / Silent tier, and flag a
//     stable inbound flow as an income (paycheck) candidate
//     (confidence.go).
//
// A cluster that matches an existing commitment's matcher is deduped up front and
// returned as that commitment's cycle-matches rather than a new candidate
// (discover.go). Death detection and local re-verification of a claimed (e.g.
// Smart+/AI) pattern round out the surface (death.go, verify.go).
//
// The package is pure Go with NO syscall/js dependency and imports only the
// standard library plus internal/domain; it is exhaustively table-tested on
// native Go. Money is integer minor units throughout; amounts on Txn are positive
// magnitudes and Direction carries the in/out meaning.
package recurdiscover

import (
	"sort"
	"strings"
	"time"
)

// fuzzyJoinThreshold is the hard-coded token-set similarity at or above which a
// newcomer joins an existing cluster's canonical signature. It is deliberately a
// constant (not user-tunable) per the discovery spec.
const fuzzyJoinThreshold = 0.90

// minFuzzySignatureLen is the length floor for fuzzy matching: signatures shorter
// than this join by EXACT equality only, so short brand tokens ("UBER") never
// fuzz into a longer neighbour ("UBER EATS").
const minFuzzySignatureLen = 8

// Direction is whether a cash flow leaves (Out) or enters (In) the household. It
// is a hard clustering key: an inbound and an outbound charge never merge even
// with an identical signature.
type Direction int8

const (
	// Out is money leaving — a bill, subscription, or transfer to a liability.
	Out Direction = iota
	// In is money arriving — a paycheck or other inbound deposit.
	In
)

// String renders a Direction for logs and evidence.
func (d Direction) String() string {
	if d == In {
		return "in"
	}
	return "out"
}

// Txn is one already-normalized transaction fed to discovery. Payee is the
// resolved clean display name (the caller runs internal/payeealias). AmountMinor
// is a POSITIVE magnitude in minor units; the sign meaning lives in Direction.
// AccountID and Direction are hard clustering keys — discovery never merges
// across either. Currency is informational (mixed-currency streams are the
// caller's responsibility to pre-convert).
type Txn struct {
	ID          string
	Date        time.Time
	Payee       string
	AmountMinor int64
	AccountID   string
	Direction   Direction
	Currency    string
}

// Pins are the user's clustering overrides, surfaced in Detection preferences.
// All signature keys are compared against the CANONICAL quarantined signature of
// a cluster (see Signature); pairwise keys are order-independent.
type Pins struct {
	// Suppressed holds canonical signatures the user marked "not recurring". A
	// suppressed cluster still forms (so dedupe and history stay correct) but is
	// never proposed as a candidate.
	Suppressed map[string]bool
	// NeverMerge lists pairs of canonical signatures that must stay apart even
	// when they would otherwise fuzzy-join.
	NeverMerge [][2]string
	// ForceMerge lists pairs of canonical signatures that must join, overriding
	// the fuzzy threshold. Hard keys (account + direction) still apply — a forced
	// pair only merges within the same account and direction.
	ForceMerge [][2]string
}

// pairKey normalizes an unordered signature pair to a stable comparison key.
func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

func (p Pins) neverMerge(a, b string) bool {
	k := pairKey(a, b)
	for _, pr := range p.NeverMerge {
		if pairKey(pr[0], pr[1]) == k {
			return true
		}
	}
	return false
}

func (p Pins) forceMerge(a, b string) bool {
	k := pairKey(a, b)
	for _, pr := range p.ForceMerge {
		if pairKey(pr[0], pr[1]) == k {
			return true
		}
	}
	return false
}

// SignatureCluster is a set of transactions that share a recurring identity:
// same account, same direction, and an exact-or-fuzzy signature match. Txns are
// held in chronological order (ties broken by ID). Canonical is the cluster's
// stable identity — the most common quarantined signature among its members.
type SignatureCluster struct {
	Canonical string
	AccountID string
	Direction Direction
	Txns      []Txn
}

// Signature quarantines per-transaction reference noise from a resolved payee and
// returns the cluster signature. It upper-cases and splits the payee into tokens,
// replaces every token that looks like a reference id — a token mixing letters
// and digits, a run of six or more consecutive digits, or a high-entropy
// consonant run — with a single '#', and rejoins. So "SPOTIFY P1A2B3" and
// "SPOTIFY K9X2M1" both collapse to "SPOTIFY #", while real words ("SPOTIFY",
// "UBER EATS") survive untouched.
func Signature(payee string) string {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(payee)))
	if len(fields) == 0 {
		return ""
	}
	out := make([]string, 0, len(fields))
	for _, tok := range fields {
		tok = strings.Trim(tok, ".,:;/\\|()[]{}\"'*")
		if tok == "" {
			continue
		}
		if isHashToken(tok) {
			out = append(out, "#")
			continue
		}
		out = append(out, tok)
	}
	return strings.Join(out, " ")
}

// isHashToken reports whether a token reads as a transaction reference rather
// than a real word: letters AND digits mixed, six-plus consecutive digits, or a
// vowelless letter run of length five or more (a proxy for "high char entropy"
// that leaves ordinary brand words — which almost always carry a vowel — alone).
func isHashToken(tok string) bool {
	var hasLetter, hasDigit bool
	var digitRun, maxDigitRun, vowels, letters int
	for _, r := range tok {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
			digitRun++
			if digitRun > maxDigitRun {
				maxDigitRun = digitRun
			}
		case r >= 'A' && r <= 'Z':
			hasLetter = true
			letters++
			digitRun = 0
			switch r {
			case 'A', 'E', 'I', 'O', 'U', 'Y':
				vowels++
			}
		default:
			digitRun = 0
		}
	}
	if hasLetter && hasDigit {
		return true
	}
	if maxDigitRun >= 6 {
		return true
	}
	if letters >= 5 && vowels == 0 && letters == len([]rune(tok)) {
		return true
	}
	return false
}

// Cluster groups transactions into signature clusters deterministically. It sorts
// the input chronologically (ties broken by transaction ID) so the result is
// independent of insertion order, then processes each transaction in turn,
// joining it to the best matching existing cluster within its account+direction —
// exact signature first, then a token-set fuzzy match at or above the 0.90
// threshold when both signatures clear the length floor — and honouring the
// user's never-merge and force-merge pins. Each cluster keeps a canonical
// signature (the most common member signature) that newcomers compare against.
func Cluster(txns []Txn, pins Pins) []SignatureCluster {
	ordered := append([]Txn(nil), txns...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].Date.Equal(ordered[j].Date) {
			return ordered[i].Date.Before(ordered[j].Date)
		}
		return ordered[i].ID < ordered[j].ID
	})

	type cluster struct {
		accountID string
		direction Direction
		txns      []Txn
		sigCounts map[string]int
		canonical string
	}
	var clusters []*cluster

	for _, t := range ordered {
		sig := Signature(t.Payee)
		var best *cluster
		var bestScore float64
		for _, c := range clusters {
			if c.accountID != t.AccountID || c.direction != t.Direction {
				continue // hard keys never merge across
			}
			if pins.neverMerge(sig, c.canonical) {
				continue
			}
			// Exact match always wins outright.
			if sig == c.canonical {
				best = c
				bestScore = 1
				break
			}
			// Forced merge joins regardless of fuzzy score.
			if pins.forceMerge(sig, c.canonical) {
				if 1.0 > bestScore || best == nil {
					best, bestScore = c, 1
				}
				continue
			}
			// Fuzzy join, gated by the length floor on both signatures.
			if len(sig) < minFuzzySignatureLen || len(c.canonical) < minFuzzySignatureLen {
				continue
			}
			s := tokenSetSimilarity(sig, c.canonical)
			if s >= fuzzyJoinThreshold && s > bestScore {
				best, bestScore = c, s
			}
		}
		if best == nil {
			best = &cluster{
				accountID: t.AccountID,
				direction: t.Direction,
				sigCounts: map[string]int{},
			}
			clusters = append(clusters, best)
		}
		best.txns = append(best.txns, t)
		best.sigCounts[sig]++
		best.canonical = canonicalSignature(best.sigCounts)
	}

	out := make([]SignatureCluster, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, SignatureCluster{
			Canonical: c.canonical,
			AccountID: c.accountID,
			Direction: c.direction,
			Txns:      c.txns,
		})
	}
	return out
}

// canonicalSignature returns the most common signature in the tally, breaking a
// frequency tie by choosing the lexicographically smallest form so the result is
// independent of processing order.
func canonicalSignature(counts map[string]int) string {
	var bestSig string
	var bestCount int
	for sig, n := range counts {
		if n > bestCount || (n == bestCount && (bestSig == "" || sig < bestSig)) {
			bestSig, bestCount = sig, n
		}
	}
	return bestSig
}

// tokenSetSimilarity scores two signatures in [0,1] by pairing their tokens
// greedily (largest per-token edit ratio first) and normalizing the matched
// ratios by the larger token count — so extra unmatched tokens ("UBER" vs
// "UBER EATS") drag the score down. Identical signatures score 1.
func tokenSetSimilarity(a, b string) float64 {
	ta := strings.Fields(a)
	tb := strings.Fields(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	// Ensure ta is the smaller set for the greedy pass.
	if len(ta) > len(tb) {
		ta, tb = tb, ta
	}
	used := make([]bool, len(tb))
	var sum float64
	for _, x := range ta {
		bestRatio := -1.0
		bestIdx := -1
		for i, y := range tb {
			if used[i] {
				continue
			}
			r := editRatio(x, y)
			if r > bestRatio {
				bestRatio, bestIdx = r, i
			}
		}
		if bestIdx >= 0 {
			used[bestIdx] = true
			sum += bestRatio
		}
	}
	return sum / float64(len(tb))
}

// editRatio is 1 - levenshtein(a,b)/max(len). Equal strings give 1; totally
// different strings approach 0.
func editRatio(a, b string) float64 {
	if a == b {
		return 1
	}
	d := levenshtein(a, b)
	m := len(a)
	if len(b) > m {
		m = len(b)
	}
	if m == 0 {
		return 1
	}
	return 1 - float64(d)/float64(m)
}

// levenshtein is the standard edit distance between two ASCII strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
