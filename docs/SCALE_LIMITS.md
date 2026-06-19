# CashFlux Backend Scale Limits

CashFlux self-host uses one Go process, SQLite in WAL mode, and a content-addressed blob directory. This is intentionally simple and reliable for small teams, families, and early hosted deployments, but it has a clear write-scaling ceiling.

## Current Ceiling

SQLite allows many readers but still has one writer at a time. CashFlux configures one database connection, WAL mode, and a busy timeout so writes queue predictably instead of competing across many connections.

Expected comfortable use:

- Small household/team deployments.
- Low write frequency: sync snapshots, AI-key updates, OAuth sessions, blob metadata, audit events, and usage counters.
- Blob bytes on disk, with SQLite storing metadata only.

Watch these signals:

- `cashflux_db_query_duration_seconds_sum` rising faster than request volume.
- More HTTP 503/429 responses from in-flight or rate limits.
- Increasing `cashflux_queue_depth{queue="workspace_watch"}`.
- Slow WAL checkpoints or growing database/WAL files.
- Host disk, file-descriptor, CPU, or memory pressure.

## Do Not Guess Capacity

Before increasing tenant count or selling a hosted multi-tenant plan, run load and soak tests against the exact deployment shape. Use the results to set admission limits, quotas, and the migration trigger.

Recommended starting thresholds for migration planning:

- p99 write latency stays above the SLO for two consecutive business days.
- WAL checkpoints regularly exceed the maintenance window.
- Write conflicts or busy-timeout errors appear under normal load.
- A single tenant can materially affect another tenant's latency.

## Migration Path

Preferred migration order:

1. Keep SQLite but shard by tenant or deployment when operationally acceptable.
2. Move metadata tables to Postgres when hosted multi-tenant writes outgrow single-writer SQLite.
3. Move blobs to object storage such as S3, R2, or MinIO while preserving content-addressed hashes.
4. Keep the gRPC/HTTP API contracts stable; change storage behind the repository layer.

Postgres migration requirements:

- Forward-only schema migrations.
- Dual-write or export/import rehearsal in staging.
- Tenant-isolation tests carried over from SQLite.
- Backfill scripts that are idempotent and resumable.
- Rollback plan based on restoring from backup, not downgrading the live schema.

## Operator Guidance

Use `docs/OBSERVABILITY.md` for dashboard queries and alerts. Use `docs/OPERATIONS_RUNBOOK.md` before deploys and migrations. Treat scale migration as an incident-risk project: take backups, rehearse restore, publish a maintenance window if user-facing sync writes will pause, and record results in the devlog or release notes.
