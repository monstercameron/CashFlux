# Contributing to CashFlux

Thanks for being here! CashFlux is a **pure-Go, local-first budgeting suite that runs in the browser
via WebAssembly**, built on [GoWebComponents](https://github.com/monstercameron/GoWebComponents).
Contributions of all sizes are welcome — bug reports, docs, tests, and code.

By participating you agree to the [Code of Conduct](./CODE_OF_CONDUCT.md).

> **Heads-up: this is an agent-built, agent-maintained project.** The codebase is 100% AI-authored, and
> issues / PRs / feature requests are triaged and implemented by **Claude and Codex agents** with a human in
> the loop. Practically, that means: write clear, reproducible issues (the agents act on detail), and expect
> that even human PRs may be reviewed or refined by an agent before merge. [`CLAUDE.md`](./CLAUDE.md) is the
> agents' rulebook — following it is the fastest path to a merge.

## Ways to help

- 🐛 **Report a bug** — [open an issue](https://github.com/monstercameron/CashFlux/issues/new/choose).
  The [live demo](https://monstercameron.github.io/CashFlux/) + **Settings → "Load sample"** makes
  reproductions easy.
- 💡 **Request a feature** — file a feature-request issue. The roadmap is public in
  [`TODOS.md`](./TODOS.md), so check whether it's already planned and add your thoughts.
- 🔧 **Send a pull request** — see below.

## Project shape (read this first)

The golden rule: **business logic is platform-independent and table-tested; the WASM/UI layer is a thin
shell over it.**

```
internal/<domain>     pure Go logic — money, currency, ledger, budgeting, goals, payoff,
                      allocate, forecast, formula, rules, reports, …  (NO syscall/js, unit-tested)
internal/store        in-tab SQLite persistence
internal/appstate     the single validated read/write seam
internal/screens|ui|app|uistate   the WASM/UI shell (build-tagged js,wasm)
```

**Good first issues** usually live in the pure logic packages — no browser needed, just `go test`.

## Dev setup

Requires **Go 1.26+**.

```sh
# Live-reload dev server:
./.tools/gwc.exe dev -app ./main.go -root .

# Build the wasm bundle:
GOOS=js GOARCH=wasm go build -o ./web/bin/main.wasm .

# Run the pure-logic tests on native Go:
go test ./...

# Format + vet before committing:
gofmt -l -w . && go vet ./...
```

(Optional) regenerate README screenshots with the dev server running: `node e2e/readme_shots.mjs`.

## Pull request guidelines

The full engineering bar lives in [`CLAUDE.md`](./CLAUDE.md). The short version:

1. **Build bottom-up:** data model → tested logic → persistence → state → UI **last**. Never UI-first.
2. **Pure, idiomatic, beautiful Go.** Small composable packages, clear names, doc comments on exported
   symbols, errors wrapped with context. `gofmt`/`go vet` clean. No dead code.
3. **New logic ships with table-driven tests** in the same change. Export→import must stay lossless.
4. **One feature per commit**, with a clear [Conventional Commit](https://www.conventionalcommits.org)
   subject (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`). Tests for that feature go in the same commit.
5. **Update [`CHANGELOG.md`](./CHANGELOG.md) and [`DEVLOG.md`](./DEVLOG.md)** in the same commit.
6. **Money is integer minor units** (cents) with an explicit currency — never a float in domain logic.
7. **`log/slog` only** (via the project `logging` package) — no `fmt.Println` debugging.
8. **No `syscall/js` in logic packages** — keep them native-testable.

Make sure `go build` (with `GOOS=js GOARCH=wasm`) and `go test ./...` pass before you push. Keep PRs
focused; if a change grows, split it.

## Reporting security issues

Please **don't** open a public issue for vulnerabilities — see [`SECURITY.md`](./SECURITY.md).

Happy hacking. 💸
