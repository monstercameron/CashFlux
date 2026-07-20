// SPDX-License-Identifier: MIT

package recurdiscover

import "fmt"

// Claim is a claimed recurring pattern to be verified locally against the actual
// transactions — the shape a Smart+/AI proposal takes so the deterministic engine
// can re-score it ("verified locally ✓") or return an honest "no local
// confirmation". Signatures are the (possibly merged) canonical signatures the
// claim covers; AmountMinor ± BandMinor is the claimed cost band.
type Claim struct {
	Signatures  []string
	Direction   Direction
	AccountID   string
	Cadence     Cadence
	AmountMinor int64
	BandMinor   int64
}

// Verification is the result of re-scoring a Claim: whether the local engine
// confirms it, the confidence, the evidence gathered, and a plain-English reason.
type Verification struct {
	Verified   bool
	Confidence float64
	Evidence   Evidence
	Reason     string
}

// Verify re-scores a claimed pattern against actual transactions. It gathers the
// transactions whose signature matches the claim (same direction and, when the
// claim names one, account), then checks that the detected rhythm matches the
// claimed cadence and the amounts sit within the claimed band. It confirms only
// when there is enough matching history AND the rhythm and cost agree with the
// claim — otherwise it returns an honest, unconfirmed result the UI can show as
// "no local way to confirm".
func Verify(claim Claim, txns []Txn, opts Options) Verification {
	now := opts.Now
	if now.IsZero() {
		now = latestDate(txns)
	}

	var matched []Txn
	for _, t := range txns {
		if t.Direction != claim.Direction {
			continue
		}
		if claim.AccountID != "" && t.AccountID != "" && t.AccountID != claim.AccountID {
			continue
		}
		sig := Signature(t.Payee)
		for _, cs := range claim.Signatures {
			if sigMatches(sig, cs) {
				matched = append(matched, t)
				break
			}
		}
	}

	floor := effectiveFloor(claim.Cadence, opts)
	if len(matched) < floor {
		return Verification{
			Reason: fmt.Sprintf("only %d matching transactions — not enough to confirm a %s pattern locally", len(matched), claim.Cadence),
		}
	}

	ev, ok := buildEvidence(matched, now, opts)
	if !ok {
		return Verification{
			Reason: "the matching transactions have no coherent rhythm or amount locally",
		}
	}
	rh := detectRhythm(datesOf(matched))
	cost := analyzeCost(amountsOf(matched), datesOf(matched))
	conf, _ := scoreConfidence(rh, cost, len(matched), floor, ev.LastSeen, now)

	cadenceOK := rh.cadence == claim.Cadence
	amountOK := withinBand(ev.Amount.Typical, claim.AmountMinor, claim.BandMinor)

	switch {
	case cadenceOK && amountOK:
		return Verification{
			Verified:   true,
			Confidence: conf,
			Evidence:   ev,
			Reason:     fmt.Sprintf("verified locally: %d %s charges near %s", ev.Count, rh.cadence, formatBand(claim.AmountMinor, claim.BandMinor)),
		}
	case !cadenceOK:
		return Verification{
			Confidence: conf,
			Evidence:   ev,
			Reason:     fmt.Sprintf("claimed %s but the transactions read as %s", claim.Cadence, rh.cadence),
		}
	default:
		return Verification{
			Confidence: conf,
			Evidence:   ev,
			Reason:     fmt.Sprintf("cadence matches but amounts (~%d) fall outside the claimed band", ev.Amount.Typical),
		}
	}
}

// withinBand reports whether an amount is within center ± band. A non-positive
// band collapses to exact equality.
func withinBand(amount, center, band int64) bool {
	if band < 0 {
		band = 0
	}
	return absInt64(amount-center) <= band
}

// formatBand renders a claimed band for a reason string (minor units).
func formatBand(center, band int64) string {
	if band <= 0 {
		return fmt.Sprintf("%d", center)
	}
	return fmt.Sprintf("%d±%d", center, band)
}
