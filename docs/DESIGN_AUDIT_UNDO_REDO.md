# Design Proposal — Audit Log + Timeline Undo/Redo

**Status:** proposal / research (not yet specced for build — confirm scope before implementing).
**Author:** agent research pass.
**Goal:** a full-fledged, persistent **audit system** ("what changed, when, by whom") and a
**traversable timeline** that powers **undo / redo** and point-in-time restore.

---

## 1. TL;DR — the recommended idea

Add one pure-Go package, `internal/history`, and a single **commit seam** in `appstate`. Every data
mutation is wrapped so the app:

1. snapshots the dataset **before** the mutation,
2. runs the mutation,
3. **diffs** before→after into a minimal, entity-keyed **ChangeSet** (forward patch + inverse patch),
4. appends it to an **append-only audit log** and pushes it on the **undo stack**.

- **Undo** = apply the top entry's *inverse* patch. **Redo** = re-apply its *forward* patch.
- **Timeline traversal / restore-to-point** = move a cursor along the log, applying patches in order.
- **Audit view** = render the log: timestamp, actor, entity, operation, human summary, before→after diff.

This reuses the machinery that already exists (`Snapshot`/`Load`, lossless serialization,
`triggersSuspended`) and matches the project's "capture state, don't instrument every write" philosophy.
**Diffing — not hand-written inverse commands — is what makes cascades (transfer pairs, reassign-on-
delete, budget cover) reversible for free.**

---

## 2. Why this fits CashFlux's architecture

Grounded in the current code:

| Existing capability | File | Why it matters here |
|---|---|---|
| `store.Snapshot() → Dataset`, `store.Load(ds)` | `internal/store/*` | A whole-state memento is already a one-liner; restore is `Load`. |
| `Dataset` = plain slices of value structs + `Settings` | `internal/store/dataset.go:43-67` | Trivially diffable by `id`; deterministic; serializable. |
| Lossless, schema-versioned JSON export/import | `internal/store/dataset.go:69-106` | History entries serialize with the same guarantees. |
| Single mutation seam (`appstate.App`, ~40 `Put*`/`Delete*`, shared `del()`) | `internal/appstate/appstate.go:1403` | One place to thread a `commit()` wrapper. |
| `triggersSuspended` + `WithoutTriggers` | `appstate.go:41-45,1221-1233` | Reuse to **replay without re-firing workflows** or re-recording history. |
| Money is integer minor units; deterministic logic | project-wide | Snapshots/diffs are exact — no float drift across undo. |
| Whole-dataset autosave that "catches every mutation regardless of code path" | `internal/app/persist.go:90-141` | Precedent for state-capture over per-write hooks; the commit seam becomes the natural, *synchronous* save point. |

The one gap: writes do **not** currently pass through a single choke point (each `Put*` calls the store
directly). The proposal adds that choke point — see §5.

---

## 3. Approaches considered (and why diff-based wins)

| Approach | How undo works | Pros | Cons | Verdict |
|---|---|---|---|---|
| **A. Full-snapshot memento** | keep a `Dataset` per step; undo = `Load(prev)` | dead simple; perfect correctness | memory = full dataset × depth; audit log can't say *what* changed without diffing anyway | partial |
| **B. Command log (inverse ops)** | each of ~40 ops writes its own inverse | tiny memory; rich semantics | must hand-write & test an inverse for every op **including cascades** (transfer-pair delete, reassign-on-delete, cover-budget, apply-rules-bulk) — exactly the error-prone ones | rejected (effort/risk) |
| **C. Diff-based change log (RECOMMENDED)** | diff before/after snapshots → minimal inverse patch | minimal memory (only changed rows); **cascades reverse automatically**; audit summary falls out of the diff; one implementation covers all ops | needs a generic Dataset differ + an apply-patch routine | **chosen** |

C is B's memory profile with A's correctness, and the diff *is* the audit record. The differ is written
**once** and table-tested; no per-operation inverse logic.

---

## 4. The `internal/history` package (pure Go, native-tested)

Built bottom-up per the SDLC rule — no `syscall/js`, fully unit-tested before any UI.

```go
package history

// RowChange is one entity row that changed within a collection.
type RowChange struct {
    Coll   string          // "transactions", "accounts", … (Dataset field key)
    ID     string          // entity id
    Before json.RawMessage // nil = row did not exist (an insert)
    After  json.RawMessage // nil = row was deleted
}

// ChangeSet is one undoable unit: all rows touched by a single user action,
// plus a settings delta and a human-readable label.
type ChangeSet struct {
    Label    string      // "Deleted transaction · Groceries $42.10"
    At       time.Time   // when committed (passed in; no Date.now in logic)
    Actor    string      // member id / "you" / "workflow:<id>" / "import"
    Rows     []RowChange
    Settings *SettingsDelta // nil when settings unchanged
}

// Diff computes the ChangeSet between two datasets (keyed by id per collection).
func Diff(before, after store.Dataset) ChangeSet

// Invert returns the ChangeSet that undoes c (swap Before/After).
func (c ChangeSet) Invert() ChangeSet

// Apply returns ds with the change set's "After" rows applied (forward),
// used for redo and timeline replay.
func Apply(ds store.Dataset, c ChangeSet) store.Dataset

// Stack is the bounded undo/redo history with a cursor for time travel.
type Stack struct { /* entries []ChangeSet; cursor int; capBytes int */ }
func (s *Stack) Push(c ChangeSet)         // truncates redo tail, enforces cap
func (s *Stack) Undo() (ChangeSet, bool)  // returns inverse to apply, moves cursor
func (s *Stack) Redo() (ChangeSet, bool)
func (s *Stack) Coalesce(window) // merge rapid same-entity edits (see §6)
```

Notes:
- Rows are stored as `json.RawMessage` so the differ is **generic over all 20 collections** — no
  per-type code. Comparison is byte-equality of canonical JSON (deterministic marshaling).
- `Apply`/`Invert` are total and pure → exhaustively table-testable (insert, update, delete, cascade,
  no-op, settings-only, bulk).

---

## 5. The `appstate` commit seam

Introduce one internal method every mutation funnels through:

```go
// commit runs mutate, records the resulting diff as one undoable/audited unit,
// and (unless replaying) persists. label/actor describe the action for the log.
func (a *App) commit(label, actor string, mutate func() error) error {
    if a.replaying {            // undo/redo/import replay: mutate only, no record
        return mutate()
    }
    before, _ := a.store.Snapshot()
    if err := mutate(); err != nil { return err }   // validation failure → no entry
    after, _ := a.store.Snapshot()
    cs := history.Diff(before, after)
    if cs.Empty() { return nil }                     // edit that changed nothing
    cs.Label, cs.Actor, cs.At = label, actor, a.now()
    a.hist.Push(cs)
    a.audit.Append(cs)                               // append-only log (see §7)
    return nil
}
```

Refactor the existing methods to wrap their store call, e.g.:

```go
func (a *App) PutAccount(ac domain.Account) error {
    if is := validate.ValidateAccount(ac); !is.OK() { return is }
    return a.commit(labelPutAccount(ac), a.actor(), func() error { return a.store.PutAccount(ac) })
}
```

- **Undo/redo** call `commit`-bypassing apply with `a.replaying = true` and `triggersSuspended = true`,
  so replaying restores data **without** re-firing workflows, re-running rules, or recording new history.
- **Bulk operations** (`ImportJSON`, `ApplyRules`, `ReassignCategory`, CSV import) wrap the *whole*
  operation in one `commit`, producing a single "Imported 532 transactions" undo step — this maps
  directly onto the existing `WithoutTriggers` bulk pattern.
- A failed validation returns before the second snapshot → **no audit noise from rejected edits**.

**Cost:** two `Snapshot()` calls per user action. Snapshots are in-memory slice copies; a coarse user
action (save one transaction) is sub-millisecond even at thousands of rows. This is far less frequent
than the existing 4s autosave loop. If profiling ever shows pressure, snapshot only the touched
collections (the label already implies them).

---

## 6. Granularity, coalescing, and actor attribution

- **One user action = one entry.** Inline-edit save, delete, quick-add, drag-reorder each commit once.
- **Coalescing:** rapid successive edits to the *same field of the same entity* within a short window
  (e.g. typing then immediately re-saving) merge into one step so undo isn't death-by-a-thousand-cuts.
  Configurable; off for deletes/creates.
- **Actor:** `you` for direct UI actions; `workflow:<id>` when a workflow effect mutates; `import` for
  bulk loads; in multi-member households, attribute to the active member if/when a "current member"
  concept exists. This makes the audit log answer **"who changed this?"**, not just "what changed".

---

## 7. The audit log (persistence & retention)

Two surfaces, different lifetimes:

1. **Undo/redo stack** — in-memory `history.Stack`, bounded by a byte cap (localStorage-quota-aware,
   like autosave already worries about). **Open decision:** persist it so undo survives reload (§10).
2. **Audit log** — append-only, persisted, queryable record of every committed `ChangeSet`. Recommended
   home: a new `audit_log` table in the SQLite store (sortable, filter-by-entity/date/actor), plus a
   `SchemaVersion` bump and a migration step (the migrate hook already exists,
   `internal/store/dataset.go:96`). Exportable alongside the dataset; **redact secrets** (OpenAI key)
   exactly as `ExportJSONRedacted` does today. Retention is capped (N entries or M days, configurable
   per the "heavily configurable" rule), with older entries compacted/dropped.

This also lays groundwork for future **sync**: an op-stream/audit log is the natural basis for
op-based merge across devices (the dataset already anticipates sync — `dataset.go:42`).

---

## 8. UI / UX surfaces (built last)

Ordered by value-per-effort:

1. **Inline "Undo" on the toast** *(highest value, lowest cost)*. The `Notice` atom + `Toast` already
   exist (`internal/app/toast.go`); add an optional action button: *"Deleted transaction · Undo"*.
   Catches the 90% case (oops-delete) right where it happens.
2. **Global shortcuts** `Ctrl/⌘+Z` (undo) and `Ctrl/⌘+Shift+Z` / `Ctrl+Y` (redo), added to the existing
   keyboard layer (`internal/app/shortcuts.go`), suppressed while typing in a field (that guard already
   exists). Add **Undo/Redo** entries to the ⌘K command palette too.
3. **Activity / History timeline screen** (a registry-driven Tools screen, so it's auto-routed + railed
   per the B7 rule). Reverse-chronological list: time · actor · entity · summary, with an expandable
   **before→after diff** per entry and a **"Restore to this point"** time-travel action (applies all
   inverse patches from head back to that entry, itself recorded as one undoable "Restored to <time>").
4. **Per-entity "Recent changes"** inside inline editors (filter the audit log by entity id) — "this
   account was edited 3× this week."

Empty/disabled states and confirmation on destructive time-travel restores follow the usual UI rules.

---

## 9. Edge cases & risks (call out before building)

- **Side effects aren't undoable.** Undo restores *data*; it can't un-send a notification, un-call the
  OpenAI API, or reverse a backend push. Document this; only data state is guaranteed reversible.
- **Workflows on replay.** Must replay with `triggersSuspended` (and `replaying`) or undo could trigger
  cascades and corrupt history. Covered by §5; needs a regression test.
- **Large blobs.** `Artifact.Bytes` / `BlobRef` (`internal/store/dataset.go:61`) would bloat diffs.
  Treat artifact binary payloads specially: diff on `BlobRef`/hash, never copy bytes into a ChangeSet.
- **Secrets.** Never let `Settings.OpenAIKey` enter the audit log or persisted history (redact, as
  export already does).
- **Quota.** Persisted history competes with the dataset for localStorage; enforce the byte cap and
  degrade gracefully (drop oldest), mirroring the autosave quota guard.
- **Settings vs data scope.** Decide whether preference/appearance changes are undoable or excluded
  (recommend: audit them, but keep them out of data-undo by default — they're not "oops-delete" risks).
- **Schema migration of old history.** Stored ChangeSets reference a schema version; migrate or discard
  on version bump (simplest: discard undo stack on migration, keep audit log read-only).

---

## 10. Decisions

Resolved (2026-06-20):

1. **Undo survives reload — yes.** Persist a **bounded** undo stack (byte-capped, quota-aware) so
   `⌘Z`/`⌘⇧Z` work after a reload. Discard (don't migrate) the stack on a schema bump; the audit log
   stays read-only across versions.
2. **Undo scope — data entities only.** `⌘Z` reverts transactions/accounts/budgets/goals/etc. Settings,
   appearance, and layout are **audited but excluded from data-undo** (they aren't oops-delete risks
   and would pollute the data timeline).

Still open (decide during spec):

3. **Audit retention** — entry count vs age cap + default. Recommend: 500 entries or 90 days.
4. **Granularity of "restore to point"** — global only, or also per-entity rollback?
5. **Actor model** — is there (or will there be) a "current member" to attribute actions to?

---

## 11. Phased plan (bottom-up, one feature per commit)

1. **`internal/history`** — `Diff`, `Invert`, `Apply`, `Stack`, coalescing; exhaustive table tests.
   *(pure logic, native Go; no UI.)*
2. **`appstate` commit seam** — `commit()`, `replaying` flag; route all `Put*`/`Delete*`/bulk through
   it; tests asserting one entry per action, none on validation failure, cascades reverse, replay
   doesn't re-fire triggers.
3. **Persistence** — `audit_log` store table + schema bump + migration + redaction + export; optional
   persisted undo stack. Round-trip tests.
4. **UI** — toast Undo action → global shortcuts + palette → Activity timeline screen → per-entity
   history. Playwright stories for undo/redo and restore-to-point.

Each phase is independently shippable; Phase 1 + 2 already deliver working in-session undo/redo before
any new screen exists.

---

## 12. Per-feature undo stories — every stateful surface

The diff engine reverses *rows* for free; the **smartness** is in three editorial decisions the
`commit()` seam makes for each feature:

- **Unit of regret** — what set of rows must reverse *together* as one `⌘Z` (the cascade boundary).
- **Side-effect fence** — what must **not** be reverted (notifications sent, AI calls billed, backend
  pushes, money already moved at a bank). Undo restores *records*, never the outside world.
- **Label & actor** — what the timeline entry says, so the audit reads like a story, not a row dump.

Below: one story per stateful feature, the smart behavior, and the gotcha. (✅ = data-undo;
👁 = audited-only, excluded from `⌘Z` per §10 decision 2.) Every entity here is a `Dataset` collection
(`internal/store/dataset.go:43-67`) unless noted.

### A. Money & the ledger ✅

- **A1. Transaction add/edit/delete.** *"I fat-fingered $420 instead of $42 and saved."* One entry,
  label `Edited transaction · Groceries`. **Smart:** coalesce successive edits to the *same* txn within
  the window into one step; never coalesce an add or a delete. **Gotcha:** the "txn added" workflow
  trigger already fired on the original add — undo must **not** re-fire it (replay suspends triggers),
  and redo must not double-fire.
- **A2. Transfer (paired legs).** *"I deleted one side of a transfer."* The reciprocal leg
  (`DeleteTransactionWithTransferPair`, `appstate.go:1240`) is found and removed in the *same* commit →
  **both legs reverse as one** `⌘Z`. **Gotcha:** editing one leg's amount should pair-edit the other in
  one unit too, or undo leaves an unbalanced transfer.
- **A3. Bulk recategorize / bulk clear / bulk delete.** *"I bulk-recategorized 60 rows, wrong target."*
  The whole selection is **one** entry: `Recategorized 60 transactions → Dining`. **Smart:** the diff
  records only the 60 changed `categoryId` fields, so undo is tiny and exact even though the action was
  bulk. **Gotcha:** if some rows were already that category (no-op), they don't appear in the diff —
  the count in the label should reflect *actually changed*, not *selected*.
- **A4. Duplicate transaction.** Inverse of an add → undo removes the dup. Trivial, but label it
  `Duplicated transaction` so the timeline distinguishes it from a fresh add.
- **A5. CSV / statement import.** *"I imported the wrong file."* One entry: `Imported 532 transactions
  (chase-2026.csv)`, mapping to the existing `WithoutTriggers` bulk path. **Smart:** dedupe-skipped rows
  aren't in the diff; undo removes exactly what was inserted, leaving pre-existing rows untouched.
- **A6. Mark-all-updated / reconcile / update-balance.** Account freshness + reconciling adjustments
  reverse together with the balance row they changed.

### B. Budgeting & goals ✅

- **B1. Budget add/edit/delete.** Standard single-entity story.
- **B2. Cover budget (move money between envelopes).** *"I covered the wrong envelope."* `CoverBudget`
  (`appstate.go:1294`) touches **two** budgets (and may create an adjustment txn) — all in one commit →
  one `⌘Z` restores both balances. **Gotcha:** the cascade boundary must include any generated
  transfer/adjustment row, or undo leaves a dangling adjustment.
- **B3. Rollover.** Period rollover that writes carried-forward amounts reverses as one unit per period
  applied.
- **B4. Goal add/edit/delete + contribution.** *"I contributed to the wrong goal."* If a contribution
  creates a linked transaction, the goal update **and** the transaction are one unit. **Gotcha:** decide
  and document — does undoing the contribution also remove the money movement? (Recommend yes: they're
  one user intent.)
- **B5. Budget methodology switch** (`Settings.BudgetMethodology`). Changes how every budget computes.
  👁 audited; **excluded from data-undo** (it's a settings/config change, decision §10.2) — but the
  timeline should show it because it silently reshapes every budget figure.

### C. Planning, allocate, payoff ✅

- **C1. Allocation profile save/run.** Saving an `AllocationProfile` is a single entry. *Running* an
  allocation that creates/edits transactions groups all resulting movements into one
  `Allocated $2,000 across 5 buckets` step. **Smart:** the explainable breakdown (determinism rule) can
  be stashed in the entry's label/detail so the timeline shows *why* each amount moved.
- **C2. Plan save/delete.** Single-entity.
- **C3. Payoff baseline set/reset** (`Settings.PayoffBaseline`, `appstate.go` payoff_progress). *"I reset
  my debt baseline by accident — now 'paid off since' is wrong."* This is a settings field but it's
  **stateful progress data**, not a preference. **Decision needed:** treat baseline as data-undoable
  (recommend ✅, exception to the settings exclusion) since losing it loses progress history.

### D. Organization — categories, members, rules ✅

- **D1. Category delete with reassign-on-delete.** *The flagship cascade.* `ReassignCategory` +
  `DeleteCategory`: deleting "Coffee" reassigns its 40 transactions to "Dining" then removes the
  category. **All 41 rows are one commit** → `⌘Z` restores the category *and* moves the 40 txns back.
  **Gotcha:** without grouping, naive undo would resurrect the category but leave 40 txns mis-filed.
  This single story is the strongest argument for diff-based grouping over per-row undo.
- **D2. Subcategory tree edits.** Re-parenting / nesting changes reverse as the set of rows whose
  `parentId` changed.
- **D3. Member delete with owner-reassign** (`DeleteMemberAfterReassign`, `appstate.go:502`). Same
  cascade shape as D1 across accounts/budgets/goals owned by that member → one unit.
- **D4. Set default member** (`SetDefaultMember`). Small but real data change → ✅ one entry.
- **D5. Rule add/edit/delete + reorder.** Precedence reorder changes multiple rules' order fields → one
  entry. **Gotcha:** rules are *latent* — editing a rule doesn't change transactions until applied
  (D6), so undoing a rule edit must NOT try to un-apply past categorizations.
- **D6. Apply rules (bulk).** *"Apply rules recategorized 200 txns wrong."* One entry,
  `Applied rules · 200 transactions changed`. Reverses the 200 `categoryId` diffs only. Independent of
  D5 (the rule definitions are untouched).

### E. Documents, imports, AI ✅ / 👁

- **E1. Document add/delete.** Single-entity.
- **E2. Reviewed document-row import** (`ImportReviewedDocumentRows`). Like A5 — one entry for the batch;
  diff is the inserted txns + the document record.
- **E3. Receipt import** (`ImportReceipt`). Creates a transaction (+ maybe a document). One unit.
  **Side-effect fence:** the **AI vision call already happened and was billed** — undo removes the
  resulting transaction but cannot un-bill the API. Label notes it came from AI so the user understands
  redo won't re-call AI (it re-applies the saved result).
- **E4. Saved insight add/delete** (`SavedInsight`). ✅ single-entity. **Side-effect fence:** the
  Q&A/explain AI call is external; undo only removes the saved record.
- **E5. AI key / model / FX rates** (`Settings`). 👁 audited, excluded from data-undo; FX-rate edits
  silently change every multi-currency figure, so surfacing them in the timeline matters even though
  `⌘Z` won't touch them.

### F. Automation — workflows & recurring ✅ (with care)

- **F1. Workflow definition add/edit/delete** (`Workflow`). ✅ single-entity, latent like rules.
- **F2. Workflow *effects* (the hard one).** A workflow run mutates data (categorize, create txn,
  notify). **Smart attribution:** those mutations commit with `actor = workflow:<id>` and a label like
  `Workflow "Rent reminder" · created 1 task`, so the timeline distinguishes machine edits from yours.
  **Decision:** are workflow-made data changes user-undoable? Recommend **yes** (they're real data) —
  but the run record (`WorkflowRun`) is 👁 audit-only, never resurrected by undo. **Gotcha:** replaying
  an undo must keep `triggersSuspended` so undoing a workflow's effect doesn't re-trigger the workflow.
- **F3. Recurring rule add/edit/delete** (`Recurring`). ✅ latent. **Generated** transactions from a
  recurring rule are normal txns (A1) — undoing the *rule* must not retroactively delete already-
  materialized transactions (decouple definition from instances, like D5↔D6).
- **F4. Freshness reminder task** (`CreateFreshnessReminderTask`). System-created `Task`; actor =
  `system`. ✅ undoable as a normal task add; **side-effect fence:** any notification already shown isn't
  recalled.

### G. To-do & splitting ✅

- **G1. Task add/edit/complete/delete** (`Task`). ✅ Completing a task is a state change → undoable.
  **Smart:** coalesce rapid check/uncheck toggles; nested-subtask edits (C72) reverse with their parent
  set if a delete cascades to children.
- **G2. Shared expense + settle-up** (`SharedExpense`, `Settlement`, `SettleUp`/`RecordSettlement`,
  `appstate/settle.go`). *"I recorded a settlement payment from the wrong person."* The settlement record
  reverses as one entry. **Side-effect fence (critical):** undo reverses the **record**, not real money
  — if a roommate already Venmo'd you, undo doesn't claw it back. Label must make this unmistakable.

### H. Custom pages & artifacts ✅

- **H1. Custom page create/rename/hide/delete + widget grid** (`CustomPage`). Deleting a page with N
  widgets = one unit (page + its widget config). **Gotcha:** the widget bento layout lives partly in
  page data (✅) and partly in UI-state layout atoms (👁) — be explicit about which half undo restores.
- **H2. Artifact add/delete** (`Artifact`, has `Bytes`/`BlobRef`, `dataset.go:61`). **Smart:** the
  ChangeSet stores the `BlobRef`/hash, **never the bytes** — undoing an artifact delete restores the
  reference and re-points at the blob store; it does not snapshot megabytes into history. **Gotcha:** if
  the blob was hard-deleted, undo of a delete may need to keep blobs until they fall off the history
  cap (soft-delete blobs while referenced by the undo stack).

### I. 👁 Audited-but-not-data-undoable (UI / config state)

These persist via `uistate.*` localStorage atoms, **not** the `Dataset`. Per §10.2 they're **excluded
from `⌘Z`** but **should appear in the audit timeline** (they change what the user sees):
preferences/theme/accent/density/week-start/date-format (`prefs`), hidden modules, **nav drag-reorder**
(`navorder`), rail collapsed, **bento layout & widget configs** (`layout`/`widgetcfg`), period/resolution,
saved transaction filter (`txfilter`), custom fonts, banner, theme-editor changes.

- **I1. Smart exception candidates.** Two of these *feel* like data and users may expect undo:
  **nav reorder** and **bento layout drag**. **Decision:** keep them audit-only for v1 (simpler, avoids
  mixing config into the data timeline), but note them as the most likely "please make this undoable
  too" follow-ups. A separate, lightweight **layout-undo** scoped to the customize surface could be
  added later without touching data-undo.

### J. Workspace-level (meta) 👁

- **J1. Workspace create/rename/delete/switch/import** (`internal/app/workspace.go`, separate
  registries). **Scope rule:** undo history is **per-workspace** — switching workspaces swaps in that
  workspace's stack; `⌘Z` never crosses a workspace boundary. Workspace *deletion* is a destructive meta
  action guarded by confirm, **not** part of data-undo (recommend a separate "recently deleted
  workspaces" trash with its own retention, rather than wiring it into the per-workspace stack).

---

## 13. Cross-cutting smart-undo patterns (generalized from §12)

The stories above reduce to a small set of rules the `commit()` seam should encode:

1. **Group by user intent, not by row.** One click = one entry, however many rows or collections it
   touches (D1, D3, B2, A2). The diff captures the breadth; the commit boundary captures the intent.
2. **Definition vs instance are independent.** Editing a latent definition (rule, recurring, workflow,
   budget methodology) never retroactively rewrites the instances it already produced, and vice-versa
   (D5↔D6, F1↔F2, F3↔instances). Each is its own undoable unit.
3. **Replay is trigger-free and history-free.** Undo/redo set `replaying` + `triggersSuspended` so
   reversing a change never fires workflows/rules or records new audit entries (A1, F2).
4. **Side-effect fence.** Undo reverses *records*, never the outside world: sent notifications, billed
   AI calls, backend pushes, real money (E3, E4, F4, G2). Labels say so; redo re-applies saved results,
   it does not re-invoke external services.
5. **Big payloads by reference.** Blobs/bytes are diffed by hash and soft-deleted while referenced by
   the stack — history never copies large binaries (H2).
6. **Attribute the actor.** `you` / `workflow:<id>` / `import` / `system` so the timeline reads as a
   narrative and machine edits are distinguishable from human ones (A5, E3, F2, F4).
7. **Coalesce noise, never coalesce structural change.** Merge rapid same-field edits and toggle
   flip-flops; never merge an add, a delete, or a cascade (A1, A3, G1).
8. **Per-workspace, never cross-workspace.** The stack is scoped to the active workspace (J1).
9. **Label honestly with effective counts.** "Recategorized 60" should mean 60 rows *actually changed*,
   not 60 selected — the diff is the source of truth (A3).
10. **Excluded ≠ invisible.** Config/UI-state changes don't participate in `⌘Z` but still land in the
    audit timeline, because they change what the user sees (§I, B5, E5).
