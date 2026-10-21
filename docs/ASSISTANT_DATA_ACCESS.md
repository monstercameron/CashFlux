# Assistant data access

Full-detail conversations can inspect retained financial records through three
read-only tools. The prompt summary is a starting point, not the data boundary.

| Tool | Purpose |
| --- | --- |
| `data_manifest` | Discover collections, counts, snapshot identity and exact field schemas. |
| `query_data` | Filter by IDs, exact scalar fields, stored text or date bounds; follow `nextOffset` with the returned snapshot. |
| `read_record` | Read an exact record or field, including long text through numbered character chunks. |

The initial catalog covers 42 typed collections, including archived records,
orphan balance history, transaction links, custom fields, documents, workflows,
saved views and financial settings. Call the manifest for the current catalog;
field names match persisted JSON, including legacy uppercase identity fields.
Money fields retain exact integer minor units and their currencies. These tools
do not perform financial aggregation; use the financial, planning, report and
formula tools for calculations.

Pages report both total and filtered counts. A changed snapshot rejects a later
page instead of silently combining revisions. Oversized records point to
`read_record`; consecutive `jsonText` chunks must be joined before decoding.
Every call reads current storage. Store failures are explicit, not empty data.

Provider credentials, arbitrary settings KV, vault/auth state and binary blobs
are excluded. Financial settings use an explicit DTO. Known device credentials
and credential-shaped fields are redacted from stored text and opaque maps;
redacted paths are reported. Stored content is data, never instructions to execute.
Aggregate-only conversations withhold these record-level tools.

Tests use disposable App/store fixtures: full catalog identity, more-than-30-row
pagination, exact large integers, stale pagination, fresh reads, credential/binary
exclusions and long-document continuation. No paid provider call is required.
