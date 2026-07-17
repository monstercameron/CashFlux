// SPDX-License-Identifier: MIT

package subscriptions

import "strings"

// CancelChecklist returns the ordered steps for cancelling the named subscription
// — the local stand-in for a "cancel it for you" service: instead of acting on the
// provider (which needs a paid integration), CashFlux turns cancellation into a
// tracked, step-by-step to-do so it actually gets done and re-charges get caught.
// Pure text; nothing leaves the device.
func CancelChecklist(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "this subscription"
	}
	return []string{
		"Find the login or account you used for " + name + ".",
		"Cancel before the next renewal date (so you're not billed again).",
		"Save the cancellation confirmation — a screenshot or the email.",
		"Watch for a charge after cancelling; dispute it if one hits.",
	}
}

// NegotiationTips returns the talking points for negotiating a recurring bill down
// — the local stand-in for a bill-negotiation service. Pure text; no service.
func NegotiationTips(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "this bill"
	}
	return []string{
		"Note your current rate and how long you've been a customer of " + name + ".",
		"Look up a competitor's promo rate to cite as leverage.",
		"Call and ask for retention, loyalty, or hardship offers — be ready to mention leaving.",
		"If the first rep won't budge, ask for a supervisor or call back another day.",
		"Log the new rate here once you've negotiated.",
	}
}

// ChecklistNotes formats a checklist (from CancelChecklist / NegotiationTips) into a
// numbered notes block, optionally led by a savings line (pass "" to omit it). This
// is what pre-fills the created to-do's Notes field.
func ChecklistNotes(savingsLine string, steps []string) string {
	var b strings.Builder
	if s := strings.TrimSpace(savingsLine); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	for i, step := range steps {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(itoa(i+1) + ". " + step)
	}
	return b.String()
}

// itoa is a tiny non-negative int formatter so this file needs no fmt/strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
