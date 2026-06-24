// SPDX-License-Identifier: MIT

package aiprovider

// This file pins the model-routing policy for the optional SMART-series AI
// features (the "[AI]" items). It is a product decision, per the user: every
// smart AI call starts on the cheap, fast reasoning model (gpt-5.4-mini); when
// that model is not strong enough for a given task — the caller judges this from
// a failed structured-output validation or a low self-reported confidence — the
// call escalates to the flagship reasoning model (gpt-5.5) run at LOW effort, so
// the escalation buys capability without paying for full-effort reasoning.
//
// The package only models the policy (which model, which request profile) so it
// stays pure and testable; the wasm/AI layer reads these to place the call and
// decides when to escalate.

// SmartModelID is the default model for SMART-series AI features: cheap, fast,
// reasoning-capable, vision-capable. Resolved against the OpenAI provider.
const SmartModelID = "gpt-5.4-mini"

// SmartEscalationModelID is the stronger model a smart AI call escalates to when
// the default model proves insufficient for the task.
const SmartEscalationModelID = "gpt-5.5"

// SmartModel returns the default SMART-feature model and its provider. The bool
// is false only if the curated registry is missing the model (a build error),
// in which case callers should fall back to Default.
func SmartModel() (Provider, Model, bool) {
	p, ok := Get("openai")
	if !ok {
		return Provider{}, Model{}, false
	}
	m, ok := p.Model(SmartModelID)
	return p, m, ok
}

// SmartEscalationModel returns the stronger model a smart AI call escalates to,
// with its provider.
func SmartEscalationModel() (Provider, Model, bool) {
	p, ok := Get("openai")
	if !ok {
		return Provider{}, Model{}, false
	}
	m, ok := p.Model(SmartEscalationModelID)
	return p, m, ok
}

// SmartProfile is the request shape for the default smart model: the app's
// standard Responses-over-websocket streaming call at medium effort. The default
// model is light enough that medium effort stays cheap.
func SmartProfile() Profile { return DefaultProfile() }

// SmartEscalationProfile is the request shape for an escalated smart call: the
// stronger model, deliberately held to LOW reasoning effort so the escalation
// pays for capability, not for full-effort reasoning.
func SmartEscalationProfile() Profile { return LowEffortProfile() }
