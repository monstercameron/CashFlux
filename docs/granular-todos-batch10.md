# Granular todo decomposition — batch 10 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` once the in-progress `origin/main` merge
> resolves and commits unblock. No code written.

## R10 no-key receipt import (#441 → atomic)

ALREADY SHIPPED:
- **C94** Camera capture — DONE: `pickImageDataURL` sets `input.capture="environment"` (`internal/screens/documents.go:1086`); browser opens rear camera, no separate button needed.
- Pure helpers ready: `ai.EstimateCostUSD` + `ai.FormatCostUSD` (`internal/ai/ai.go:192,203`); `ai.Usage` already returned by vision callbacks (only the *display* wiring for C99 is missing).

Remaining atomic todos:
- [ ] **[C93][BLOCKER]** No-key manual fallback — `internal/screens/documents_image_import.go:32-85` — when `NeedsKey` && image chosen, add an "Enter manually" CTA → `uistate.RoutePath("/transactions")` (Tesseract.js local OCR is ~20MB, out of scope for one commit). New `OnManual ui.Handler` prop declared with `ui.UseEvent` at top of `Documents()` (unconditional), passed down.
- [ ] **[C95][MAJOR]** Swap key-check/image-check order in `readAI` — `documents.go:393-400` — image-empty guard must fire BEFORE the no-key guard so "Choose an image first" shows instead of the misleading needs-key notice.
- [ ] **[C96][MAJOR]** Unreadable-image error path — `documents.go:405-417` — distinguish 0-rows-parsed from API error; add i18n `documents.unreadableImage` ("couldn't read any transactions — try a clearer photo").
- [ ] **[C97][MAJOR]** Image size/format validation in `pickImageDataURL` onLoad — `documents.go:1094-1108` — reject >20MB + non jpeg/png/webp/gif via a new `onErr func(string)` param threaded into `chooseImage` (~l377); add 2 i18n keys.
- [ ] **[C98][MINOR]** Persist chosen image across the Settings round-trip — `documents.go:95` — `imageURL` is component-local `ui.UseState` (lost on nav); move to a `state.UseAtom("doc:pendingImageURL")` (or browserstore), clear on successful import (~l574/594).
- [ ] **[C99][MINOR]** Show token count + est. cost after vision call — `documents.go:404-417` — capture the `ai.Usage` (currently `_`), call `ai.EstimateCostUSD(aiModel,u)`+`FormatCostUSD`, set new `aiCostMsg` state, render muted line in `ImageImportCard` (`documents_image_import.go:53-85`); pattern at `insights.go:1077-1078`.
- [ ] **[C100][DESIGN]** Inline OpenAI-key explainer in `ImageImportCard` NeedsKey block — `documents_image_import.go:73-84` — what/where (platform.openai.com)/cost (~$0.002/receipt)/privacy (image goes browser→OpenAI, never to CashFlux); new i18n `documents.keyExplainer`.
- Gotchas: declare all new hooks at top of `Documents()` unconditionally; cost-estimation logic in the `onResult` closure (not the card); `onErr` is a pure-Go param (no alert()); use `state.UseAtom`/browserstore not `ui.UseState` for cross-nav persistence.

## F44 data ownership / backup (#469 → atomic)

ALREADY SHIPPED:
- **C298 (nav part)** Settings→Data jump-nav — DONE: `"settings.data"` in `settingsNavKeys` (`internal/app/settingssectionnav.go:34`).
- Pure helpers ready: `ExportJSONWithBlobs`/`ExportJSONRedactedWithBlobs`/`ImportJSONWithBlobs` (`internal/appstate/artifact_ops.go:110/122/136`); `recordBackupNow`/`loadLastBackup` (`internal/app/notifyrun.go:243/249`).

Remaining atomic todos:
- [ ] **[C294a][MAJOR]** `exportJSON()` → `app.ExportJSONWithBlobs()` — `internal/app/settings.go:1303` (callback, IDB-safe).
- [ ] **[C294b][MAJOR]** `activeDataset()` → `app.ExportJSONRedactedWithBlobs()` — `internal/app/backupall.go:55`.
- [ ] **[C294c][MAJOR]** `importJSON()` → `app.ImportJSONWithBlobs()` — `internal/app/settings.go:1360`.
- [ ] **[C295a][MAJOR]** Wrap `importJSON()` body in `confirmModal(...)` gated on ack — `settings.go:1354` — mirror `wipeData()` at l1386.
- [ ] **[C295b][MAJOR]** i18n `settings.importConfirm` ("Replace all current data with this file? This can't be undone.") — `internal/i18n/en.go` ~l1095.
- [ ] **[C296a][MINOR]** Add partial-CSV hint under the CSV export button — `internal/app/settings_section.go` ~l272 — muted `P` (pure helper, no hooks).
- [ ] **[C296b][MINOR]** i18n `settings.exportCSVHint` ("Exports your transactions only — not accounts, budgets, or attachments.") — en.go ~l1060.
- [ ] **[C297a-d][MINOR]** Surface "Back up everything"/"Restore" in Settings→Data — add `OnBackupEverything`/`OnRestoreBackup` to `settingsRightProps` (`settings_section.go` ~l124), 2 `dataBtn` calls (~l268-283), wire in `globalSettingsForm()` (`settings.go` ~l958-963), + 2 i18n keys.
- [ ] **[C298a-b][MINOR]** Destructive wipe-confirm label — `settings.go:1386` — add `ConfirmLabel` to the confirm-dialog request (check `internal/uistate` + `dialoghost.go:140`), pass `settings.wipeConfirmLabel` ("Erase data").
- [ ] **[C299a-d][MAJOR]** "Last backed up" timestamp — call `recordBackupNow()` at end of `backupEverything()` (`backupall.go:85`); add `LastBackupAt time.Time` to `settingsRightProps`; render muted `pr.FormatDate(p.LastBackupAt)` line (`settings_section.go` after l283); wire `LastBackupAt: loadLastBackup()` in `globalSettingsForm()` (`settings.go` ~l900).
- Gotchas: `*WithBlobs` variants block on IDB — only call from event/callback handlers (all listed sites are); `settingsRightColumn` is hook-free, derive values in `globalSettingsForm()` and pass via props.
