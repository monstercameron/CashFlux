# Framework dependency upgrade — October 21, 2026

| Library | CashFlux before | Latest checked | Action |
| --- | --- | --- | --- |
| GoWebComponents | `/v5 v5.0.1` | `/v7 v7.0.0` | Migrate every application and test import, including both console entry points. |
| SchemaFlux | No module requirement or source imports | `/v2 v2.0.1` tag | No installed dependency to update. Keep the existing AI transport and strict import decoders. |

Sources: [GoWebComponents v7 release](https://github.com/monstercameron/GoWebComponents/releases/tag/v7.0.0),
[v7 migration guide](https://github.com/monstercameron/GoWebComponents/blob/v7.0.0/docs/MIGRATION_V7.md),
[SchemaFlux v2.0.1 tag](https://github.com/monstercameron/SchemaFlux/releases/tag/v2.0.1).
The versions were also resolved with `go list -m -json MODULE@latest`.

## Application migration

CashFlux now requires Go 1.27.0 and selects toolchain Go 1.27.3. The framework is a regular
versioned dependency; CashFlux has no framework replacement directive. Input-handler props
retain `ui.Handler` rather than erasing it to `any`. The framework's `CheckedIf` replaces the
identical local helper. Shared opacity utilities use `OpacityNum(Num(n))`, avoiding the v7
deprecation warning. Custom-page Markdown explicitly registers the bundled engine and has a
rendered-output regression test.

SchemaFlux v2's local parser does not replace CashFlux's strict rejection of unknown fields and
trailing JSON. Its typed extraction API performs a model operation and would need an intentional
adapter for CashFlux's cancellable browser transport. Adding an unused module would not upgrade
an existing integration, so it is not added by this dependency maintenance change.

## Local runner setup

The checked v7 Windows ARM64 binary matches `SHA256SUMS`, but it cannot locate its source root
because its paths were trimmed during the upstream build. The local runner was instead built
from the official source archive, whose SHA-256 is:

```text
2bd7feda5e8e4ac0fc79596e0ce5f2176d6620c34a2141dc45fd6dd8e87213cc
```

Reproduce from the CashFlux root, downloading the archive and its checksum from the v7 release:

```powershell
# Verify the downloaded archive against the published .sha256 file before extraction.
New-Item -ItemType Directory .tools/gwc-framework-v7
tar -xf /path/to/GoWebComponents-v7.0.0.tar.gz -C .tools/gwc-framework-v7
$env:GOTOOLCHAIN = 'go1.27.3'
$runner = Join-Path $PWD '.tools/gwc.exe'
go -C .tools/gwc-framework-v7/tools build -o $runner ./gwc
```

Keep `.tools/gwc-framework-v7` in place. These files are ignored development tooling; the app
continues to resolve the released framework from the Go module cache. The previous v5 runner
and its existing source directory were preserved.

Use `scripts/dev.ps1` on port 8080 with full-page reload (`-hot=false`). The v7 `gwc build`
command unconditionally disables JSON v2, conflicting with `go-sqlite3 v0.35.0` on Go 1.27.
The existing CashFlux direct Go build scripts retain Go's default JSON engine and work with
that SQLite version; production verification uses `go build -tags production` directly.

## Verification

- Native `go test -work -vet=all -p 4 ./...`: **370 packages passed** on Go 1.27.3.
- Screen WebAssembly suite: **1,090 tests passed**; its only failure was the new Markdown test
  using a heading-role query unsupported by the mock DOM. The corrected test checks actual
  heading/list/emphasis/link nodes and passes separately. The complete suite was not repeated
  after that test-only query correction.
- Shared UI WebAssembly: **90 tests passed**. Browser-store WebAssembly: **60 tests passed**.
- The expanded app suite exposed three existing test issues: a leaked encrypted-workspace marker,
  a legacy-plaintext assertion contradicting App Lock downgrade protection, and a recovery-copy
  assertion that did not allow an existing identical copy. Test-only corrections are separate
  follow-up changes. Their combined focused run passes **6 tests**; a complete app-suite rerun
  after those corrections is still pending.
- Canonical browser smoke: **2 tests passed** against both the live development build and the exact
  production artifact, covering all 46 fixture routes and a light-theme sweep, without retries.
  The production artifact SHA-256 is
  `610411af273f9783cbd2ec075ed0653c58e6099aeb8b48810e3f2a05ae890327`.
- Independent production-artifact checks passed nine routes with native View Transitions under
  both motion preferences, plus the Dashboard-period-change → Transactions → Accounts sequence.
  There were no page errors. Expected diagnostic resource noise included the forced raw-WASM 404
  used for gzip injection and optional local backend requests when no backend was running.
- Main app, services worker, admin console, and customer portal compile with `-tags production`.
- Port 8080 reports a successful build and `hotReloadEnabled: false` (full-page live reload).

One earlier production interaction benchmark recorded missed route transitions. The independent
route/transition checks could not reproduce them; retain that failed report as a failed capture,
not a passing interaction benchmark or performance claim. Browser diagnostics were run in
disposable sample contexts with external traffic blocked. No production release, tag, or
deployment is part of this change.
