# Design Proposal — Multi-Provider AI Inference

**Status:** proposal / research (confirm scope before building).
**Goal:** let users choose their inference provider — **OpenAI, Anthropic (Claude), Cerebras,
OpenRouter, DeepSeek, GLM (Zhipu), Kimi (Moonshot)** — for CashFlux's AI features, keeping the
local-first, bring-your-own-key model.

---

## 1. TL;DR

CashFlux's AI layer is **already ~80% provider-agnostic**. `postCompletions` takes `baseURL` as a
parameter; the request/response shaping is pure and isolated. So **every OpenAI-compatible provider
works today by swapping base URL + key + model** — the missing pieces are a *provider registry*, a
*settings model that holds more than one key + an active (provider, model) choice*, *capability
awareness* (vision / structured outputs vary), and **one new wire dialect for Anthropic**.

**Highest-leverage move:** add **OpenRouter** first. It's OpenAI-compatible and itself an aggregator —
one integration unlocks Claude, DeepSeek, GLM, Kimi, Gemini, Llama, etc. Native Anthropic is the only
provider needing new wire code, and direct-from-browser Claude has a **CORS caveat** (§4) that makes
"Claude via OpenRouter or the backend proxy" the pragmatic default.

---

## 2. Current state (what the code already does)

| Piece | File | Notes |
|---|---|---|
| Pure request/response shaping (OpenAI chat-completions) | `internal/ai/ai.go` | `BuildRequest`, `BuildStructuredRequest`, `ParseResponse`, `ParseUsage`, `ErrorMessage`, pricing table — **OpenAI dialect, but pure & isolated**. |
| Vision request shaping | `internal/ai/vision.go` | `image_url` content parts (OpenAI multimodal). |
| Network transport (browser) | `internal/ai/transport.go` | `SendChat`/`SendVisionChat`/`SendStructuredVisionChat` → `postCompletions(apiKey, **baseURL**, …)`. **`baseURL` is already a parameter**; `DefaultBaseURL = https://api.openai.com/v1`. Header is `Authorization: Bearer`; path is `baseURL + "/chat/completions"`. Has retries/abort. |
| Backend proxy (hosted/self-host) | `internal/ai/proxy_transport.go` | gRPC streaming so keys can live server-side. Already a second transport. |
| Settings | `internal/store/dataset.go` (`Settings.OpenAIKey`, `OpenAIModel`) | **single** key + model. Redacted on export (`ExportJSONRedacted`). |
| Settings UI | `internal/app/settings.go` | key field, model select, "remember key on device" toggle. |

**Implication:** the seam to generalize is small and already isolated. We're not rewriting the AI
layer — we're parameterizing what's hardcoded (dialect, auth header, endpoint path) and enriching
settings.

---

## 3. Provider landscape (research)

> Endpoints/capabilities drift — **verify at build time**. "Dialect" = wire format.

| Provider | Base URL (typical) | Dialect | Auth header | Browser CORS | Vision | Structured outputs |
|---|---|---|---|---|---|---|
| **OpenAI** | `https://api.openai.com/v1` | openai | `Authorization: Bearer` | ✅ | ✅ (gpt-4o…) | ✅ native `json_schema` (strict) |
| **OpenRouter** | `https://openrouter.ai/api/v1` | openai | `Authorization: Bearer` (+ optional `HTTP-Referer`, `X-Title`) | ✅ | per chosen model | per chosen model |
| **Cerebras** | `https://api.cerebras.ai/v1` | openai | `Authorization: Bearer` | ✅ (verify) | ✗ (text: Llama/Qwen) | ✅ (advertised) |
| **DeepSeek** | `https://api.deepseek.com/v1` | openai | `Authorization: Bearer` | ✅ | ✗ | partial (`json_object`, not full schema) |
| **GLM / Zhipu** | `https://open.bigmodel.cn/api/paas/v4` | openai | `Authorization: Bearer` | verify | ✅ GLM-4V | partial |
| **Kimi / Moonshot** | `https://api.moonshot.cn/v1` (also `.ai`) | openai | `Authorization: Bearer` | verify | ✅ vision variants | partial |
| **Anthropic (Claude)** | `https://api.anthropic.com/v1` | **anthropic** | `x-api-key` + `anthropic-version` | **⚠ blocked by default** (needs `anthropic-dangerous-direct-browser-access: true`) | ✅ (base64 `source`) | via tool-use (no `json_schema`) |

**Two wire dialects only:**
- **openai** — `POST {base}/chat/completions`, Bearer auth, `messages[]`, `response_format` for
  structured, `image_url` parts for vision. Covers **6 of 7** providers (OpenAI, OpenRouter, Cerebras,
  DeepSeek, GLM, Kimi).
- **anthropic** — `POST {base}/messages`, `x-api-key`+`anthropic-version`, top-level `system`, content
  blocks, **required `max_tokens`**, base64 image `source`, tool-use for structured JSON. Only native
  Claude needs this.

**Cross-cutting gotchas:**
- **Structured outputs are not universal.** The vision import relies on schema-constrained JSON. Only
  OpenAI (and some) support strict `json_schema`; others do `json_object` or nothing → need a
  capability flag + **prompt-coerced-JSON fallback** (ask for JSON, validate, repair/retry).
- **Vision is model-specific**, not provider-specific (e.g. `deepseek-chat` no, `glm-4v` yes).
- **Token usage/pricing fields** mostly match OpenAI's `usage`; the pricing table needs per-provider
  entries (and OpenRouter returns model-specific costs).

---

## 4. The CORS reality (important for a browser-first app)

Keys are entered client-side and calls go **from the browser**. Most providers send permissive CORS
(OpenAI, OpenRouter, DeepSeek, Cerebras work direct). **Anthropic blocks browser calls by default**;
the opt-in `anthropic-dangerous-direct-browser-access` header exposes the user's key to page scripts and
is fragile. Chinese providers (GLM/Kimi) — verify CORS per region.

**Therefore:** route **direct** browser calls only to CORS-friendly providers; offer **Claude via
OpenRouter** (OpenAI dialect, CORS-ok) as the default Claude path, and/or **via the existing backend
proxy** (`proxy_transport.go`) where the key stays server-side. This reuses infrastructure that already
exists and sidesteps the CORS/key-exposure problem.

---

## 5. Proposed design

### 5.1 Provider registry (pure Go)

A new pure package, e.g. `internal/aiprovider` (or extend `internal/ai`), no `syscall/js`:

```go
type Dialect int // DialectOpenAI | DialectAnthropic

type Capabilities struct { Chat, Vision, StructuredOutputs, Streaming bool }

type Provider struct {
    ID          string        // "openai", "anthropic", "openrouter", "cerebras", "deepseek", "glm", "kimi"
    Name        string        // "OpenAI", "Anthropic (Claude)", …
    DefaultBase string        // base URL (user-overridable)
    Dialect     Dialect
    AuthHeader  string        // "Authorization: Bearer" | "x-api-key"
    ExtraHeaders map[string]string // e.g. anthropic-version; OpenRouter X-Title
    Models      []Model       // curated list: id, label, caps, pricing
    SignupURL   string        // where to get a key (shown in settings)
    DirectBrowserOK bool      // false → recommend proxy/OpenRouter (Anthropic)
}
```

Curated default model lists per provider (free-text override allowed, essential for OpenRouter's huge
catalog). Pricing folds into the existing `EstimateCostUSD` path, keyed by `(provider, model)`.

### 5.2 Two dialect implementations

- Keep today's builders/parsers as the **openai** dialect (no change).
- Add **anthropic** dialect: `buildAnthropicRequest`/`parseAnthropicResponse`/vision base64/usage
  mapping/error mapping. `Send*` dispatches on `provider.Dialect`; `postCompletions` takes the auth
  scheme, extra headers, and endpoint path from the provider instead of hardcoding.

`transport.go` change is small: replace the hardcoded header/path with provider-supplied values; the
fetch/retry/abort machinery is untouched.

### 5.3 Settings model + migration

Replace the single key/model with a provider-aware config (additive, migrated):

```go
type AIConfig struct {
    ActiveProvider string                  // "openai"
    ActiveModel    string                  // "gpt-4o-mini"
    Keys           map[string]string       // providerID -> key (redacted on export)
    BaseOverrides  map[string]string       // providerID -> custom base URL (self-host/proxy)
}
```

- **Migration:** existing `Settings.OpenAIKey`/`OpenAIModel` → `Keys["openai"]` + `ActiveProvider=openai`.
  Schema bump + migrate step (`store.migrate`, `dataset.go:96`).
- **Redaction:** `ExportJSONRedacted` must now strip **all** `Keys`, not just the one OpenAI key
  (security — current code only redacts `OpenAIKey`).
- **Stretch:** per-feature provider (cheap model for auto-categorization, strong for insights) — the
  registry makes this a later, additive change, not a rewrite.

### 5.4 UI (settings)

Provider dropdown → key field with **"Get a key" link** (`SignupURL`) → model dropdown (curated, or
free-text for OpenRouter) → live **capability badges** (Vision / Structured / Streaming) and a price
estimate. A "Test connection" button (a 1-token ping) confirms the key/base before saving. For
Anthropic, a note + a one-click "use via OpenRouter / backend proxy" suggestion (the CORS guidance).

### 5.5 Capability-aware features

The AI features must read capabilities, not assume OpenAI:
- **Vision import** (receipts/statements) requires `Capabilities.Vision` — disable/explain when the
  active model is text-only ("Pick a vision-capable model to scan receipts").
- **Structured features** prefer native `StructuredOutputs`; when absent, fall back to
  prompt-coerced JSON with validate-and-repair (the schema is already defined — reuse it as a prompt
  contract).
- Surface a clear, plain-English message when a feature needs a capability the chosen model lacks.

### 5.6 Backend proxy

Extend the gRPC proxy contract (`backendrpc`) to carry `provider` + optional `baseURL` so hosted/
self-host users get the same provider choice with the key held server-side — the natural home for
Claude (no CORS issue) and for users who don't want keys in the browser.

---

## 6. Security & privacy

- **Redact every provider key** on export and in autosave (extend `ExportJSONRedacted`); keys stay
  session-only unless the user opts into "remember on device" (per-provider toggle).
- **Same data-minimization** as today: only the aggregate `FinancialContext` leaves the device, and
  only on an explicit action (`ai.go` already enforces this — keep it dialect-agnostic).
- **Don't enable Anthropic's dangerous-direct-browser header silently** — prefer proxy/OpenRouter;
  if offered at all, gate it behind an explicit "I understand my key is exposed to page scripts" opt-in.
- Feeds the security review (C45) and the prod-hardening items (C44).

---

## 7. Phased plan (bottom-up, one feature per commit)

1. **`internal/aiprovider` registry (pure, native-tested):** Provider/Model/Capabilities + curated
   defaults + pricing; table tests. No UI, no transport change.
2. **Generalize the openai dialect transport:** thread provider auth header/extra headers/base/path
   through `postCompletions`; settings `AIConfig` + migration + redaction. **Ships OpenAI, OpenRouter,
   Cerebras, DeepSeek, GLM, Kimi** (all openai-dialect). Tests for build/parse/redaction/migration.
3. **Anthropic dialect:** `buildAnthropicRequest`/parse/vision/usage/errors; dispatch on dialect;
   default Claude to OpenRouter/proxy with a CORS note. Table tests for the messages format.
4. **Settings UI:** provider/model pickers, key + signup link, capability badges, price estimate,
   "Test connection". Playwright story.
5. **Capability-aware features:** gate vision/structured per active model; prompt-coerced-JSON fallback.
6. **Backend proxy provider passthrough** (optional, for hosted/self-host + Claude-without-CORS).

Phase 1+2 alone deliver six providers with minimal code, because the wire layer is already parameterized.

---

## 8. Open decisions

1. **Anthropic direct vs OpenRouter/proxy-only?** Recommend: ship Claude via OpenRouter/proxy first;
   direct Anthropic behind an explicit risk opt-in (or proxy-only).
2. **Per-feature provider routing** now or later? Recommend later (registry makes it additive).
3. **How much model curation vs free-text?** Recommend curated lists + free-text override (mandatory
   for OpenRouter).
4. **Default provider/model** out of the box (stay OpenAI, or offer a free/cheap default like a
   DeepSeek/Cerebras model)?
5. **Remember-key scope** — one global toggle or per-provider?

---

## 9. Agent harness (agentic tool-calling)

**Goal:** an agent loop (model calls tools, observes results, repeats until done) that works across the
providers in §3 and integrates **seamlessly** into CashFlux's pure-Go/wasm, local-first, client-side
app.

### 9.1 Landscape researched — and why off-the-shelf doesn't fit

The hard constraint is **`GOOS=js GOARCH=wasm`, in-browser, no heavy deps, data stays local.** Every
mainstream Go agent framework is built for a **server** runtime and pulls transitive deps (HTTP servers,
file/OS, telemetry, sometimes cgo) that aren't wasm-friendly and duplicate our already-isolated
transport:

| Option | What it is | Why it doesn't fit (here) |
|---|---|---|
| **tmc/langchaingo** | LangChain port for Go | Large surface, server-oriented, abstraction overhead, wasm not a goal; would wrap (not replace) our transport. |
| **cloudwego/eino** (ByteDance) | Powerful graph/chain agent framework | Heavyweight, server runtime, many deps; overkill for a browser app. |
| **Firebase Genkit (Go)** | Google flows/tracing/tooling | Runtime + tooling oriented; deps; not wasm-first. |
| **swarmgo / misc Swarm ports** | OpenAI Swarm-style handoffs | Thin but assume server net/deps; immature; still need our wire layer. |
| **Official SDKs** (`openai-go`, `anthropic-sdk-go`) | Vendor clients | wasm compat uncertain; would replace our isolated transport; **don't provide an agent loop** anyway. |
| **MCP Go SDK** (`modelcontextprotocol/go-sdk`) | Protocol to expose tools to *external* agents | Solves a different problem (see §9.4) — not an in-app loop. |

**Conclusion:** none are "seamless" for wasm + local-first. The agent loop is ~a few hundred lines of
pure Go; **build a thin in-house harness on top of the §5 provider abstraction** and borrow the concepts
(tool schemas, bounded loop, handoffs) rather than the frameworks.

### 9.2 Recommended — a minimal in-house harness on the provider abstraction

- **Tool-call dialect = the same two-dialect split as §5.2.** OpenAI **function calling**
  (`tools`/`tool_calls`) is supported by OpenAI, OpenRouter, DeepSeek, Cerebras, Kimi/Moonshot, GLM;
  Anthropic uses **tool-use** content blocks. One internal `Tool`/`ToolCall`/`ToolResult` type, mapped
  per dialect. Reuses the JSON-schema machinery already present for structured outputs.
- **Tool registry = typed Go tools over `appstate`** (the existing validated seam): *read* tools
  (query transactions/budgets/net-worth/insights), *write* tools (add/categorize transaction, create
  budget/goal/task) — each a Go func + a JSON schema. No new data path; tools call the same `Put*`/
  query methods the UI does.
- **Agent loop (pure logic, `internal/agent`):** bounded by **max steps + token budget**; `model →
  tool_calls → execute → append results → repeat` until a final message or a stop condition; **cancelable**
  (reuse the existing abort/cancel pattern). Orchestration is pure and table-testable with a fake
  model; the wasm transport stays the only network seam.
- **Capability-gated:** requires the active model to support tool-calling (a `Capabilities.Tools` flag
  added to the §5.1 registry). For non-tool models, fall back to **plan-only mode** — the model
  proposes a list of actions and the user approves them (still useful, no tool API needed).

### 9.3 Safety — the audit/undo system is the agent's seatbelt

This is the strongest reason to do it in-house: agent actions ride the same guarantees as user actions.

- **Every agent mutation flows through `appstate` validation** (no raw store access).
- **Every agent mutation is recorded by the audit/undo system (C78) with `actor = "agent"`** → fully
  reversible with one `⌘Z`, and visible in the activity timeline. An agent that mis-categorizes 30 txns
  is one undo away.
- **Destructive/bulk tools require explicit confirmation** — a FlipPanel approval step (ties to C76)
  before execution; read tools run freely.
- **Data-minimization preserved:** only what a tool returns is sent back to the model; the existing
  "only aggregate context leaves the device on an explicit action" rule (`ai.go`) extends to tools.
- **Transparency (determinism/explainability rule):** render the agent's steps/tool-calls as a
  transcript so the user sees exactly what it did and why.

### 9.4 Complementary, later: expose CashFlux as an MCP server

Inverse direction — let **external** agents (Claude Code, etc.) drive CashFlux via the **Model Context
Protocol**: expose the same tool registry over MCP. Natural fit for the self-host backend (the gRPC
proxy already exists). Out of scope for the in-app harness; noted as a future surface.

### 9.5 Tie-ins

- **C78** (audit/undo) — the agent safety net (above).
- **Workflow engine** (`internal/workflow`) — the agent can *propose/author* workflows and rules, not
  just one-off edits; the trigger/action model already exists.
- **C76** (AI modal) / **C81** (provider registry) — shared surface and capability flags.
- **Formula sandbox** (`internal/formula`) — a safe, sandboxed compute tool the agent can call.

### 9.6 Phased plan (extends §7)

1. **Registry: add `Capabilities.Tools`** + per-dialect tool-call mapping (extends Phase 1/2/3).
2. **`internal/agent` (pure, native-tested):** `Tool`/`ToolCall`/`ToolResult`, a tool registry, the
   bounded loop; tests with a fake model (multi-step, stop conditions, budget caps, tool errors).
3. **Bind tools to `appstate`** (read first, then guarded writes) — actor=`agent`, routed through C78.
4. **wasm wiring + UI:** a chat/agent surface with a step transcript + approval prompts for destructive
   tools; capability gating + plan-only fallback. Playwright story.
5. **(Later) MCP server** over the self-host backend (§9.4).

**Dependency note:** the agent harness should land **after** C81 Phase 1–3 (needs the provider/dialect
abstraction) and is dramatically safer **after** C78 (undo) — sequence accordingly.
