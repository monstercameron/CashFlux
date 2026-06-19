# CashFlux

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![WebAssembly](https://img.shields.io/badge/WebAssembly-654FF0.svg?logo=webassembly&logoColor=white)](https://webassembly.org)
[![Live demo](https://img.shields.io/badge/demo-GitHub_Pages-181717.svg?logo=github)](https://monstercameron.github.io/CashFlux/)

A **local-first, household-aware budgeting suite** — budgeting, planning, goals, a to-do list, and
optional AI insights — written in **pure Go compiled to WebAssembly** on the
[GoWebComponents](https://github.com/monstercameron/GoWebComponents) framework. Your data lives on
your device; nothing leaves it except an explicit AI call to OpenAI with your own key.

**▶ Live demo:** [monstercameron.github.io/CashFlux](https://monstercameron.github.io/CashFlux/) —
the latest `main` build, redeployed on every push. (It starts empty; use Settings → "Load sample" to
explore with realistic data. Your changes stay in your browser's local storage.)

## Highlights

- **Reconfigurable bento dashboard** (the "candidate C" design): drag-to-reorder and resize widgets,
  with the layout persisted to `localStorage`. KPIs, recent transactions, budgets, goals, to-do,
  accounts, cash flow, net-worth trend (SVG area chart), upcoming bills, savings rate, spending
  breakdown, and a freshness nudge.
- **Accounts** — assets and liabilities with live balances, net worth, archive/restore, a per-row
  "Mark updated", staleness badges, credit utilization, and liability/allocation sub-forms.
- **Transactions** — income/expense + account-to-account transfers (paired entries), tags,
  auto-suggested categories, "Repeat last", and a full filter set (search, account, category, member,
  date range) with sorting.
- **Budgets / Goals / To-do** — monthly budget tracking with a month stepper and health summary;
  savings goals with contribute + progress; tasks with priority, notes, and a hide-done filter.
- **Planning** — a debt-payoff calculator (with an extra-payment scenario) and a 12-month net-worth
  projection (with a trim-spending what-if).
- **Allocate** — ranks where to put new capital (accounts, high-interest debts, goals) by a chosen
  profile, with an explainable per-criterion breakdown and an optional AI narrative.
- **Customize** — a sandboxed formula calculator over your live figures.
- **AI (bring-your-own OpenAI key, client-side)** — "Explain my month", natural-language Q&A, and
  CSV transaction import.
- **PWA** — installable, offline-capable (service worker), dark and fast.

## Stack

- **Go** (target `GOOS=js GOARCH=wasm`, Go 1.26+), GoWebComponents (`html/shorthand` + `ui`/`state`/
  `router` hooks) consumed as a versioned module — no local `replace`.
- **Storage:** pure-Go in-memory SQLite (`ncruces/go-sqlite3`, no cgo). JSON/CSV are the portable
  import/export and sync payloads.
- **Styling:** Tailwind (CDN) + a small bespoke design-system stylesheet; Fraunces + Inter fonts.

## Build & run

```sh
# Inner loop (live reload):
./.tools/gwc.exe dev -app ./main.go -root .

# Build the wasm bundle:
GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .

# Test the pure logic packages (js/wasm view packages are build-tagged out of native):
go test ./...
```

Serve `web/` (it contains `index.html`, the manifest, and the service worker) with the wasm bundle in
`web/bin/`.

### Optional backend self-hosting

CashFlux can also run an optional backend for multi-device sync and encrypted BYO OpenAI key proxying.
The browser app talks to the server through the GoGRPCBridge `/grpc` tunnel; AI proxy calls are not
exposed as direct browser HTTP routes. See [`docs/SELF_HOSTING.md`](./docs/SELF_HOSTING.md) for the
Docker Compose quickstart, `.env` template, TLS notes, backup/restore runbook, upgrades, and optional
OAuth provider setup.

### Hosting (SPA history fallback)

CashFlux uses clean (non-hash) client-side routes, so any static host must **rewrite unknown
non-asset paths to `index.html`** — otherwise a deep link or refresh at e.g. `/accounts` 404s before
the app loads. The app itself then routes to the right screen. (The installed/offline PWA is already
covered: the service worker serves the cached shell for navigations.)

- **GitHub Pages** (no rewrite support): the deploy workflow generates a `404.html` copy of the shell,
  which Pages serves for unknown paths — handled automatically here.
- **Netlify:** add `/* /index.html 200` to a `_redirects` file.
- **Vercel:** a rewrite of `/(.*)` → `/index.html` in `vercel.json`.
- **nginx:** `try_files $uri $uri/ /index.html;`.
- **Caddy / `caddy file-server`:** `try_files {path} /index.html`.

**Local development:** the `gwc dev` server does not yet serve the app shell for history routes (a
deep link / hard refresh at e.g. `/accounts` returns 404; only built assets like `/bin/main.wasm` are
served). Until the framework grows an SPA history fallback, start from the root (`/`) and navigate
in-app, or run a production build behind one of the rewrite rules above. The deployed PWA is unaffected
(the GitHub Pages `404.html` shell and the service-worker navigation fallback both cover deep links).

## Architecture

Business logic is **platform-independent and table-driven tested** — `internal/money`, `currency`,
`ledger`, `budgeting`, `goals`, `freshness`, `validate`, `dateutil`, `id`, plus the Phase-2 engines
`payoff`, `allocate`, `forecast`, `formula` (tokenizer → parser → evaluator), `rules`, `chart`,
`dashlayout`, and the `ai` codec. None import `syscall/js`, so they run and test on native Go.

The wasm/UI layer (`internal/screens`, `internal/ui`, `internal/app`, `internal/uistate`) is a thin
shell over those packages. `internal/store` is the SQLite persistence; `internal/appstate` is the
single validated read/write seam between UI and store.

## Project docs

- [`SPEC.md`](./SPEC.md) — product specification
- [`TODOS.md`](./TODOS.md) — priority-ordered backlog and status
- [`CLAUDE.md`](./CLAUDE.md) — engineering rules and quick reference
- [`docs/GOWEBCOMPONENTS.md`](./docs/GOWEBCOMPONENTS.md) — framework notes
- [`CHANGELOG.md`](./CHANGELOG.md) / [`DEVLOG.md`](./DEVLOG.md) — history and decisions

## License

Released under the [MIT License](./LICENSE) — © 2026 monstercameron. Do what you like with it;
keep the copyright notice.
