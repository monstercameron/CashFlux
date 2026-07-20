# CashFlux end-to-end / regression suite

The trusted, CI-gated tests live in [`regression/`](./regression) and run on the
[Playwright Test](https://playwright.dev) runner. Everything else at the top of
`e2e/` is archived scratch (see [`_archive/`](./_archive)) and is **not** run.

## Run it

```bash
cd e2e
npm ci                        # first time only
npx playwright install chromium
npm test                      # runs the whole regression suite
npx playwright test smoke.spec.mjs   # one file
npm run report                # open the last HTML report
```

The runner is hermetic: a `globalSetup` builds the wasm app + copies `wasm_exec.js`,
and Playwright starts the static server (`serve.mjs`) itself on port 8099 — you do
**not** need a `gwc dev` running. Every wait keys off a real signal
(`data-app-ready`, `data-route`), never a sleep.

## The layers

| Spec | Guards |
|------|--------|
| `smoke.spec.mjs` | every route loads with on-topic content + no app errors, both themes |
| `interactions.spec.mjs` | real user actions (add a to-do, Settings tabs) assert their result |
| `rhythm.spec.mjs` | the unified Bills & recurring surface (`/recurring`, `/bills`, `/subscriptions`): hero, overdue strip, review paging, agenda + calendar, roster |
| `coverage.spec.mjs` | **ratchet** — the set of interactive controls per route can't drift; new/removed controls fail until acknowledged |
| `invariants.spec.mjs` | theme tokens defined in both themes; no route scrolls the body horizontally |
| `a11y.spec.mjs` | **ratchet** — axe (WCAG 2 A/AA) structural violations can't regress past the baseline |
| `visual.spec.mjs` | pixel baselines for content-stable pages (Windows-native) |

### The test pyramid

- **Base — pure logic** (`go test ./...`): the `internal/*` logic packages (money,
  ledger, budgeting, widgetdata, bills, allocate, …) are table-tested natively.
- **Middle — component tests** (`tools/wasm-test.ps1`): real components mounted
  through GoWebComponents' `testkit/render` (mock DOM, no browser), plus the
  js/wasm-tagged registry/browserstore tests. These are **wasm-target** Go tests
  that `go test ./...` skips (build tag), so they run via a dedicated lane:

  ```bash
  pwsh tools/wasm-test.ps1        # Windows (uses tools/go_js_wasm_exec.bat)
  # or, on Unix: PATH="$PATH:$(go env GOROOT)/lib/wasm" GOOS=js GOARCH=wasm go test ./internal/ui/ ./internal/screens/ ./internal/browserstore/
  ```

  See `internal/ui/meter_wasm_test.go` for the reference example. (Scope to specific
  packages — `./...` won't wasm-compile because `internal/server` is server-only.)
- **Top — end-to-end** (`regression/*.spec.mjs`): the Playwright layers above.

## Regenerating baselines (only after an intentional change)

```bash
# Interactive-element inventory:
UPDATE_COVERAGE=1 npx playwright test coverage.spec.mjs --project=chromium
# a11y baseline (ideally after REDUCING violations):
UPDATE_A11Y=1 npx playwright test a11y.spec.mjs --project=chromium
# Visual snapshots (run on Windows — baselines are -win32):
npm run visual:update
```

On Windows/PowerShell, set the env var separately: `$env:UPDATE_COVERAGE=1; npx playwright ...`.

## Notes

- **Visual is Windows-native.** Pixel baselines are committed as `-win32` snapshots
  and the visual spec skips on non-Windows, so Linux CI does not run it (the other
  five layers gate CI).
- **color-contrast** is excluded from the a11y gate — axe re-scores it against live
  backgrounds and it flakes run-to-run; it's covered by visual review instead.
