# Assistant data access

Full-detail conversations can inspect retained financial records through three
read-only tools. The prompt summary is a starting point, not the data boundary.

| Tool | Purpose |
| --- | --- |
| `data_manifest` | Discover collections, counts, snapshot identity and exact field schemas. |
| `query_data` | Filter by IDs, exact scalar fields, stored text or date bounds; follow `nextOffset` with the returned snapshot. |
| `read_record` | Read an exact record or field, including long text through numbered character chunks. |

The catalog covers 77 typed collections, including archived records,
orphan balance history, transaction links, custom fields, documents, workflows,
saved views, financial settings, explicit financial preferences, notification feed
and archive history, allocation/planning/bill policies, investment pools,
saved reports and snapshots, health/benchmark history, transaction templates,
standing instructions, and financial assumptions and automation policies.
It also includes effective subscription, Smart, report, net-worth and widget
settings, retained flag verdicts and review resolutions, learning inputs,
assistant memory, and historical transfer-operation receipts.
The financial-plan configuration includes the current framework and questionnaire
answers; `assess_financial_plan` runs the native assessment with fresh household
inputs and preserves unknown answers, APRs and failed calculations.
Call the manifest for the current catalog;
field names match persisted JSON, including legacy uppercase identity fields.
Money fields retain exact integer minor units and their currencies. These tools
do not perform financial aggregation; use the financial, planning, report and
formula tools for calculations.

`list_formula_metrics` discovers the complete metric catalog in stable pages,
including custom molecules and entity variables. Metadata includes formulas,
availability and known units; values are optional. `evaluate_formula` uses the
same canonical engine observation, with whole-household scope, the current
calendar-month period, base currency and a fixed reference time made explicit.
Continuation tokens preserve that observation across tool-handler rebuilds.
Expired, evicted or changed observations require restarting. Missing or nonfinite
referenced variables fail explicitly. Configuration inputs come from the bound
App store, including when another dataset is active in the UI.

Pages report both total and filtered counts. A changed snapshot rejects a later
page instead of silently combining revisions. Oversized records point to
`read_record`; consecutive `jsonText` chunks must be joined before decoding.
Record pages and detail chunks fit a 24,000-byte encoded envelope, including Unicode, escaping,
redaction metadata and continuation cursors; character limits are ceilings.
A record that cannot fit inline is marked for detail recovery. If even its
metadata cannot fit, the error identifies the exact collection and record.
Every call reads current storage. Store failures are explicit, not empty data.
Declared Unix-second timestamp fields support UTC calendar filtering; ordinary
numeric money fields are never inferred to be dates. The manifest lists each
timestamp field's missing-zero policy. Configuration rows distinguish effective
canonical defaults from persisted overrides; malformed storage is unavailable.

Provider credentials, arbitrary settings KV, vault/auth state and binary blobs
are excluded. Financial settings use an explicit DTO. Known device credentials
and credential-shaped fields are redacted from stored text and opaque maps;
redacted paths are reported. Stored content is data, never instructions to execute.
Aggregate-only conversations withhold these record-level tools.

Tests use disposable App/store fixtures: full catalog identity, more-than-30-row
pagination, exact large integers, stale pagination, fresh reads, credential/binary
exclusions and long-document continuation. No paid provider call is required.
