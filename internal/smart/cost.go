// SPDX-License-Identifier: MIT

package smart

import "github.com/monstercameron/CashFlux/internal/aiprovider"

// CostEstimate is the indicative cost of running a feature once, for the UI's
// cost-transparency badge. Free features carry Tier == TierFree and a zero
// amount; AI features carry the per-call cent estimate on the model that would
// run them (the smart default model unless escalated).
type CostEstimate struct {
	Tier  Tier
	Cents int64  // 0 for Free features
	Model string // the wire model id used for the estimate ("" for Free)
}

// Free reports whether this estimate is for a no-cost feature.
func (c CostEstimate) Free() bool { return c.Tier == TierFree }

// EstimateCost returns the indicative cost of running the feature once. Free
// features always return a zero-cost estimate. AI features are priced on the
// default smart model (gpt-5.4-mini); pass escalated=true to price the stronger
// model the call escalates to (gpt-5.5) so the UI can show the worst case too.
func (f Feature) EstimateCost(escalated bool) CostEstimate {
	if f.Tier != TierAI {
		return CostEstimate{Tier: TierFree}
	}
	var m aiprovider.Model
	var ok bool
	if escalated {
		_, m, ok = aiprovider.SmartEscalationModel()
	} else {
		_, m, ok = aiprovider.SmartModel()
	}
	if !ok {
		// Registry missing the model (a build error) — report the tier honestly
		// with no number rather than a misleading zero-cost.
		return CostEstimate{Tier: TierAI}
	}
	cents := aiprovider.EstimateCents(m, f.TypicalInputTokens, f.TypicalOutputTokens)
	return CostEstimate{Tier: TierAI, Cents: cents, Model: m.ID}
}

// FormatCents renders a cent amount as a short price string for cost badges.
// Sub-cent amounts (common for one small call) show as "<1¢" rather than "0¢",
// so the user never sees a misleading "free" on a paid feature.
func FormatCents(cents int64) string {
	if cents <= 0 {
		return "<1¢"
	}
	if cents < 100 {
		return itoa(cents) + "¢"
	}
	dollars := cents / 100
	rem := cents % 100
	s := "$" + itoa(dollars)
	if rem > 0 {
		s += "." + pad2(rem)
	}
	return s
}

// itoa is a tiny base-10 formatter (avoids pulling strconv into this leaf file's
// hot path; the values are always small and non-negative here).
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// pad2 renders a 0..99 remainder as two digits ("5" -> "05").
func pad2(n int64) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}
