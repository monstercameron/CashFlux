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
	"SMART-A5":  true, // natural-language account Q&A
	"SMART-A10": true, // account health explanation
	"SMART-G4":  true, // goal drafting from a wish
	"SMART-P2":  true, // plain-language scenario draft
	"SMART-P3":  true, // narrated forecast/outlook summary
	"SMART-AL4": true, // plain-language allocation intent
	"SMART-SU2": true, // overlapping-service detection
	"SMART-D4":  true, // natural-language to-do quick-add
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

// OutlookSystem is the system prompt for SMART-P3: narrate the figures into one
// short, calm paragraph a person can act on.
const OutlookSystem = "You are a calm personal-finance assistant. Given the figures below, write ONE short " +
	"paragraph (2–4 sentences) summarizing the person's financial outlook in plain English: what's going " +
	"well, what to watch, and the single most useful next step. Use the actual numbers. No headings, no lists, no disclaimers."

// Outlook builds the SMART-P3 request from a pre-formatted snapshot of the
// household's position (net worth, this-month flows, runway, upcoming bills).
func Outlook(context string) Request {
	return Request{System: OutlookSystem, User: "Figures:\n" + strings.TrimSpace(context)}
}

// goalDraftSystem frames SMART-G4: turn a plain-English wish into a concrete plan.
const goalDraftSystem = "You help set savings goals. From the user's wish, draft a concrete goal: a target amount, " +
	"a sensible deadline, and the monthly contribution it implies. If they gave an amount or date, use it; otherwise " +
	"propose reasonable ones and say they're estimates. Two or three short sentences, plain English, no lists."

// GoalDraft builds the SMART-G4 request from the user's plain-language wish and a
// short snapshot of what they can afford (e.g. typical monthly surplus).
func GoalDraft(wish, financialContext string) Request {
	return Request{System: goalDraftSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nWish: " + strings.TrimSpace(wish)}
}

// healthSystem frames SMART-A10: explain account health in plain language.
const healthSystem = "You are a finance assistant. Given the accounts and balances, give a brief plain-English read " +
	"of the person's account health — what looks healthy, what to watch (idle cash, high utilization, thin buffers). " +
	"Two or three sentences, specific numbers, no lists, no disclaimers."

// AccountHealth builds the SMART-A10 request from an account/balance snapshot.
func AccountHealth(accountContext string) Request {
	return Request{System: healthSystem, User: "Accounts:\n" + strings.TrimSpace(accountContext)}
}

// overlapSystem frames SMART-SU2: spot redundant subscriptions.
const overlapSystem = "You review subscriptions for redundancy. Given the list, name any that overlap in purpose " +
	"(e.g. two music services, several streaming, overlapping cloud storage) and suggest keeping one. If none overlap, " +
	"say so. Two or three sentences, plain English, no lists."

// OverlapDetect builds the SMART-SU2 request from a subscription list snapshot.
func OverlapDetect(subscriptionContext string) Request {
	return Request{System: overlapSystem, User: "Subscriptions:\n" + strings.TrimSpace(subscriptionContext)}
}

// allocationSystem frames SMART-AL4: turn intent into allocation settings.
const allocationSystem = "You help allocate spare cash. From the user's plain-English intent, recommend an allocation " +
	"approach: which profile (debt, safety, goals, balanced), how much to hold in reserve, and any per-destination cap. " +
	"Two or three short sentences, plain English, no lists."

// AllocationIntent builds the SMART-AL4 request from intent + a money snapshot.
func AllocationIntent(intent, financialContext string) Request {
	return Request{System: allocationSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nIntent: " + strings.TrimSpace(intent)}
}

// scenarioSystem frames SMART-P2: draft a what-if scenario in plain English.
const scenarioSystem = "You help plan what-if scenarios. From the user's sentence, describe the scenario as a concrete " +
	"plan: the recurring changes (raises, extra payments) and any one-time items, and a one-line read of the likely effect. " +
	"Two or three short sentences, plain English, no lists."

// ScenarioDraft builds the SMART-P2 request from the sentence + a money snapshot.
func ScenarioDraft(sentence, financialContext string) Request {
	return Request{System: scenarioSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nScenario: " + strings.TrimSpace(sentence)}
}

// todoSystem frames SMART-D4: parse a sentence into a clean to-do.
const todoSystem = "You turn a sentence into a single concise financial to-do. Reply with just the to-do text " +
	"(imperative, includes any amount and date the user gave). One line, no quotes, no preamble."

// TodoParse builds the SMART-D4 request from the user's sentence.
func TodoParse(sentence string) Request {
	return Request{System: todoSystem, User: strings.TrimSpace(sentence)}
}
