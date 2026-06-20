// Package aiprovider is the pure, typed model of the AI inference providers
// CashFlux can talk to (C81 phase 1): who they are, which wire dialect they speak,
// their default endpoints, and the per-model capabilities and indicative pricing.
// It holds no transport — the wasm/AI layer reads this registry to build a request
// and pick an endpoint, so adding a provider is data, not code.
//
// Most providers are OpenAI-compatible (chat/completions, Bearer auth); only
// Anthropic needs its own dialect. Pricing and capabilities drift upstream — the
// values here are indicative defaults to seed the UI, not a contract.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package aiprovider

import "sort"

// Dialect is the wire protocol a provider speaks. openai covers chat/completions
// with Bearer auth (the large majority); anthropic is the /messages dialect.
type Dialect string

const (
	DialectOpenAI    Dialect = "openai"
	DialectAnthropic Dialect = "anthropic"
)

// AuthStyle is how the API key is presented on a request.
type AuthStyle string

const (
	AuthBearer  AuthStyle = "bearer"    // Authorization: Bearer <key>
	AuthXAPIKey AuthStyle = "x-api-key" // x-api-key: <key> (+ anthropic-version)
)

// Structured describes how strongly a model can be asked for machine-readable JSON.
// It isn't universal — features that need JSON (e.g. vision import) fall back to
// prompt-coerced JSON when a model only supports none/json_object.
type Structured string

const (
	StructuredNone       Structured = "none"        // no structured mode; coerce via prompt
	StructuredJSONObject Structured = "json_object" // response_format json_object
	StructuredJSONSchema Structured = "json_schema" // native schema-constrained output
)

// Capabilities are per-model, not per-provider (vision in particular is a model
// trait). Defaults are conservative so a feature gates off rather than failing.
type Capabilities struct {
	Vision     bool
	Streaming  bool
	ToolUse    bool
	Reasoning  bool // accepts a reasoning-effort parameter (a "thinking" model)
	Structured Structured
}

// Model is one selectable model on a provider, with indicative pricing in cents per
// one million tokens (input and output priced separately, as providers do).
type Model struct {
	ID                 string // the wire model id, e.g. "gpt-4o"
	Label              string // human label
	Caps               Capabilities
	InputCentsPerMTok  int64
	OutputCentsPerMTok int64
}

// Provider is one inference endpoint family: its dialect, default base URL, auth
// style, where to get a key, whether it accepts free-text model ids (aggregators
// like OpenRouter do), and its curated default models.
type Provider struct {
	ID       string // stable id, e.g. "openai"
	Label    string // human label
	Dialect  Dialect
	BaseURL  string // default API base; the user may override
	Auth     AuthStyle
	KeyURL   string // "Get a key" link
	FreeText bool   // accepts arbitrary model ids beyond Models (aggregators)
	Models   []Model
}

// Model returns the curated model with the given id on this provider.
func (p Provider) Model(id string) (Model, bool) {
	for _, m := range p.Models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// Get returns the provider with the given id from the curated registry.
func Get(id string) (Provider, bool) {
	for _, p := range registry {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// Providers returns the curated registry in stable id order, so pickers and tests
// are deterministic.
func Providers() []Provider {
	out := make([]Provider, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Default returns the recommended starting provider and model: OpenAI / gpt-5.5, a
// reasoning model run at medium effort over the streaming Responses API (see
// DefaultProfile). gpt-4o-mini remains for cheap, non-reasoning calls.
func Default() (Provider, Model) {
	p, _ := Get("openai")
	m, _ := p.Model("gpt-5.5")
	return p, m
}

// EstimateCents returns the indicative cost in whole cents for a request of the
// given input/output token counts on a model, rounded to the nearest cent. Negative
// token counts are treated as zero.
func EstimateCents(m Model, inputTokens, outputTokens int64) int64 {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	micro := inputTokens*m.InputCentsPerMTok + outputTokens*m.OutputCentsPerMTok
	return (micro + 500_000) / 1_000_000
}
