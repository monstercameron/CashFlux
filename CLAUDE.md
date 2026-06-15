# CashFlux — Project Instructions

CashFlux is a **local-first, household-aware budgeting suite** written in **Go compiled to
WebAssembly** on the **GoWebComponents** framework. It is an owned platform held to a high quality
bar. The full product spec is in [`SPEC.md`](./SPEC.md) — read it before implementing features.

> **Process rule:** Agree the spec before writing feature code. Do not infer requirements; confirm
> scope, then build. Scaffolding/tooling setup may proceed without asking.

## Stack & layout

- **Language:** Go (target `GOOS=js GOARCH=wasm`). Requires Go 1.26+.
- **Framework:** local checkout at `../GoWebComponents` (consumed via `replace` in `go.mod`).
- **UI:** `html/shorthand` (dot-imported) element + control-flow funcs (`If`, `IfElse`, `Map`,
  `MapKeyed`). State via `ui` hooks and `state` atoms; pages via `router` (history).
- **Local store:** IndexedDB via the framework `interop` package.
- **AI (Phase 2):** OpenAI, called client-side with the user's own key (from Settings).

## Build / run / test

```powershell
# Inner loop (live reload) — run via the framework's gwc runner:
go run ../GoWebComponents/tools/gwc dev -app .\main.go -root .

# Build wasm directly:
$env:GOOS="js"; $env:GOARCH="wasm"; go build -o .\static\bin\main.wasm .

# Tests — native logic packages (no wasm needed) MUST pass here:
go test ./...

# Wasm / browser lanes via the runner:
go run ../GoWebComponents/tools/gwc test -lane unit -lane wasm
```

Build artifacts (`bin/`, `static/bin/`, `static/wasm_exec.js`) are git-ignored.

## Code-quality rules (non-negotiable)

1. **Pure, idiomatic, beautiful Go.** Small composable packages; clear, intention-revealing names;
   doc comments on every exported symbol; errors wrapped with context (`fmt.Errorf("...: %w", err)`).
   Run `gofmt`/`go vet` clean. No dead code, no commented-out blocks, no hacks.
2. **Clean architecture — logic is platform-independent.** Domain types and all business logic
   (money/FX, balances, freshness, allocation scoring, formula evaluation, import/export) live in
   plain Go packages with **NO `syscall/js`**, so they unit-test on native Go. The wasm/UI layer is
   a thin shell that calls into them. Never put computation in view code.
3. **Thorough testing.** Table-driven unit tests for every logic package; test edge cases and
   round-trips (export→import must be lossless). UI flows covered by wasm/browser tests. New logic
   ships with tests in the same change.
4. **Structured logging with `log/slog`.** Use the project `logging` package (custom `slog.Handler`
   → browser console + in-app ring buffer). Levelled, contextual (`slog.With(...)`). **Never**
   `fmt.Println`-debug.
5. **Determinism & explainability.** User-facing computations (allocations, forecasts, formulas,
   budgets) must expose their breakdown. No black boxes.
6. **Money is never a float in domain logic.** Represent money as integer minor units (e.g. cents)
   with an explicit currency; format only at the edge. Multi-currency aggregation goes through the
   FX table with the chosen base currency.
7. **Strong core schema + additive extensibility.** Core entity types stay strongly typed; user
   flexibility comes from validated custom fields and the sandboxed formula engine — never by
   loosening core types into untyped maps.

## UI & writing rules

- **Modern, clean, exceptionally readable UI.** Strong typography hierarchy, generous spacing and
  contrast, calm color. Prefer clarity over density.
- **Plain, friendly English everywhere.** No jargon, no abbreviations users must decode. Labels,
  empty states, errors, and nudges read like a helpful person wrote them.
- **Accessible by default** — use the framework a11y primitives; every control is labelled and
  keyboard-reachable.
- **Friendly, never naggy.** Freshness nudges and AI suggestions are dismissible and low-pressure.

## Hooks & framework gotchas (learned from the framework source)

- **`On*` prop options register hooks on the wasm build.** Only call them at stable render
  positions — **never** inside a variable-length loop. For per-row interactive elements, make the
  row its own component (`ui.CreateElement(Row, props)`) and pass plain `func` callbacks down as
  props; the row owns its handler hook.
- Build keyed lists with `MapKeyed(items, keyFn, render)`; results flatten into element children
  automatically (so `Div(Class(...), MapKeyed(...))` is fine).
- Components return `ui.Node`; mount the router with `router.NewHistoryRouter(...).Mount("#app")`.

## Configuration

The app is heavily configurable (see SPEC §12): layered defaults → household → member → screen.
Every option needs a plain-English label and a sane default. Nothing should require editing code to
reconfigure a workflow/modality.

## Version control & journaling (STRICT — aggressively enforced)

These rules are non-negotiable and apply to every change:

- **One feature per commit.** Each commit is exactly one logical feature or fix — no bundling
  unrelated changes, no mega-commits. If a change grows, split it. Tests for that feature go in the
  same commit, and the build/tests must pass before committing.
- **Conventional, descriptive commit messages** (e.g. `feat: add multi-currency FX table`,
  `fix: …`, `test: …`, `docs: …`, `refactor: …`). The subject says what and why in plain English.
- **Maintain `CHANGELOG.md`.** Every commit is recorded there under an `Unreleased` section
  (Keep-a-Changelog style: Added / Changed / Fixed / Removed). Update it in the same commit as the
  change — never retroactively.
- **Maintain `DEVLOG.md`** — a developer journal/devlog: dated entries capturing what was worked
  on, decisions and trade-offs made, problems hit and how they were solved, and what's next. Append
  an entry whenever meaningful work happens; it is the narrative companion to the changelog.
- Do not let work accumulate uncommitted. Commit each completed feature immediately, with its
  CHANGELOG + DEVLOG updates.
