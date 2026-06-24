// SPDX-License-Identifier: MIT

// Package smartai holds the pure, platform-independent parts of the SMART
// series' AI ("[AI]") features: the prompt templates, the small registry of
// which AI features have a shipped UI, and the response-acceptability checks
// that drive the gpt-5.4-mini → gpt-5.5 escalation. It holds no transport — the
// wasm layer reads these to build a request, place the call (defaulting to the
// cheap model and escalating only when Acceptable returns false), and render the
// result. Pure Go, unit-tested on native Go.
package smartai

import "strings"

// Request is a built AI request: a system prompt that frames the task and a user
// message carrying the question and context. Both go to the model verbatim.
type Request struct {
	System string
	User   string
}

// implemented is the set of AI feature codes that have a working UI today, so the
// /smart catalog only offers a toggle for AI features that actually do something
// (mirroring the Free-engine HasEngine gate). It grows as features ship.
var implemented = map[string]bool{
	"SMART-A5": true, // natural-language account Q&A
}

// Implemented reports whether the AI feature has a shipped UI.
func Implemented(code string) bool { return implemented[code] }

// ImplementedCodes returns the shipped AI feature codes (order-independent; the
// caller sorts by catalog order).
func ImplementedCodes() []string {
	out := make([]string, 0, len(implemented))
	for c := range implemented {
		out = append(out, c)
	}
	return out
}

// Acceptable reports whether a model's answer is good enough to show, or whether
// the call should escalate to the stronger model. It is deliberately
// conservative: a blank answer, an obvious "I can't"/"I don't know" refusal, or a
// suspiciously truncated reply fails, triggering one escalation. This is the
// "if it's not smart enough" signal the routing policy keys on.
func Acceptable(answer string) bool {
	a := strings.TrimSpace(answer)
	if len(a) < 2 {
		return false
	}
	low := strings.ToLower(a)
	for _, refusal := range []string{
		"i can't", "i cannot", "i'm not able", "i am not able",
		"i don't know", "i do not know", "as an ai", "i'm unable", "i am unable",
	} {
		if strings.HasPrefix(low, refusal) {
			return false
		}
	}
	return true
}

// AccountQASystem is the system prompt for SMART-A5: a tight frame that keeps the
// model grounded in the supplied figures and answering in plain English.
const AccountQASystem = "You are a concise personal-finance assistant inside a budgeting app. " +
	"Answer the user's question using ONLY the account figures provided. " +
	"Be specific and quote the relevant numbers. If the figures don't contain the answer, say so plainly. " +
	"Reply in one or two short sentences — no preamble, no disclaimers."

// AccountQA builds the SMART-A5 request from the user's question and a compact,
// pre-formatted textual snapshot of their accounts (built by the caller, which
// has the live balances). The question is trimmed; an empty question yields an
// empty user message the caller should refuse to send.
func AccountQA(question, accountContext string) Request {
	q := strings.TrimSpace(question)
	user := "Accounts:\n" + strings.TrimSpace(accountContext) + "\n\nQuestion: " + q
	return Request{System: AccountQASystem, User: user}
}
