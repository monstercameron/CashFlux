// SPDX-License-Identifier: MIT

package aiprovider

// This file pins the app's preferred request shape for AI calls (a product
// decision, per the user): OpenAI's Responses API, carried over a WebSocket when
// the provider/model supports it (HTTPS+SSE otherwise), with streaming text and a
// reasoning model (gpt-5.5) run at medium effort — plus a low-effort variant for
// lightweight chain-of-thought calls. The transport layer reads these to build the
// connection; this package only models the intent so it's pure and testable.

// APIStyle is the request/response shape a call uses.
type APIStyle string

const (
	// APIResponses is OpenAI's Responses API (the preferred default).
	APIResponses APIStyle = "responses"
	// APIChatCompletions is the older /chat/completions shape (fallback / other
	// OpenAI-compatible providers).
	APIChatCompletions APIStyle = "chat_completions"
)

// Transport is how the request is carried.
type Transport string

const (
	// TransportWebSocket is a persistent websocket (preferred when available — lower
	// latency for streaming + tool round-trips).
	TransportWebSocket Transport = "websocket"
	// TransportHTTPS is request-per-call HTTPS with SSE streaming (the universal
	// fallback).
	TransportHTTPS Transport = "https"
)

// Effort is the reasoning effort for a "thinking" model. Lower is cheaper/faster.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
)

// Profile is the resolved request configuration for a call.
type Profile struct {
	API       APIStyle
	Transport Transport
	Stream    bool
	Effort    Effort
}

// DefaultProfile is the app's main request shape: the Responses API over a websocket,
// streaming, at medium reasoning effort — for the primary Insights/agent calls.
func DefaultProfile() Profile {
	return Profile{API: APIResponses, Transport: TransportWebSocket, Stream: true, Effort: EffortMedium}
}

// LowEffortProfile is the lightweight variant for quick chain-of-thought calls
// (short classifications, suggestions): same Responses API + streaming, but low
// reasoning effort to keep latency and cost down.
func LowEffortProfile() Profile {
	p := DefaultProfile()
	p.Effort = EffortLow
	return p
}

// For resolves the effective profile for a provider/model: the app default, but the
// websocket transport and reasoning effort fall back when the target can't support
// them — a non-OpenAI (non-Responses) dialect drops to chat-completions over HTTPS,
// a non-reasoning model carries no effort, and a non-streaming model turns streaming
// off. base is the desired profile (DefaultProfile or LowEffortProfile).
func (p Provider) For(m Model, base Profile) Profile {
	out := base
	if p.Dialect != DialectOpenAI {
		out.API = APIChatCompletions
		out.Transport = TransportHTTPS
	}
	if !m.Caps.Streaming {
		out.Stream = false
	}
	if !m.Caps.Reasoning {
		out.Effort = ""
	}
	return out
}
