# CashFlux Incident Response

This runbook covers the optional CashFlux backend: sync, blob storage, OAuth sessions, and AI proxying. Keep the web app local-first during incidents; the safest degradation is to preserve local use and pause backend writes when needed.

## Status Page

Expose `GET /status` through the same TLS hostname as the backend. It returns JSON with `status`, component health, and `updatedAt`; a 200 means the process and SQLite are ready, while 503 means the process is up but the database readiness check failed.

External status pages should poll:

```sh
curl -fsS https://<domain>/status
curl -fsS https://<domain>/readyz
```

Publish updates when `/status` is degraded for more than 5 minutes, when sync/AI is unavailable for paying users, or when data integrity is at risk.

## Severity Levels

- SEV1: confirmed data loss, cross-tenant data exposure, leaked secrets, or sustained backend outage affecting most users. Page immediately, disable risky paths if needed, and update the status page within 15 minutes.
- SEV2: sync, OAuth, blob, or AI proxy failure affecting a subset of users for more than 15 minutes. Triage immediately and update the status page within 30 minutes.
- SEV3: elevated errors, latency, capacity pressure, failed scheduled backup, or isolated user impact. Create a tracked issue and update during business hours unless it worsens.

## First 15 Minutes

1. Confirm scope with `/status`, `/livez`, `/readyz`, `/metrics`, structured logs, and recent deploy history.
2. Assign an incident lead. Keep one timeline in the issue or incident channel.
3. Stabilize first: rollback, scale down writes, disable the AI proxy, or move the service into maintenance if data correctness is uncertain.
4. Preserve evidence: request IDs, trace IDs, deploy SHA, logs, metrics snapshots, and any failed backup manifest.
5. Post the first status update with scope, user impact, and next update time.

## Diagnosis Checklist

- Health: `/livez` process check, `/readyz` SQLite check, `/status` public component status.
- Capacity: HTTP max-in-flight rejections, gRPC active connections, stream caps, queue depth metrics, host CPU/memory/PID/FD pressure.
- Storage: SQLite disk free, WAL checkpoint behavior, blob directory permissions, backup freshness, manifest digests.
- Security: audit stream around the event window, auth failures, unusual AI key writes, token/session revocations.
- Upstream: OAuth provider errors, OpenAI 429/5xx/timeout rates, reverse-proxy websocket upgrade failures.

## Communication

Use plain language. Include impact, affected features, current mitigation, and next update time. Do not include user financial data, tokens, AI keys, raw datasets, or blob bytes in public or shared channels.

Update cadence:

- SEV1: every 15 minutes until mitigated, then every 30 minutes until resolved.
- SEV2: every 30 minutes until mitigated, then hourly until resolved.
- SEV3: at creation and resolution, plus meaningful state changes.

## Recovery

1. Prefer rollback for bad deploys when the schema is compatible.
2. Prefer forward fixes for migrations; never downgrade a database in place.
3. For data restore, follow `docs/SELF_HOSTING.md`: stop the stack, restore `cashflux-server.db` and `blobs/`, start the stack, verify `/readyz`, and spot-check sync.
4. Rotate tokens or revoke sessions when auth material may be exposed.
5. Confirm metrics and logs return to baseline before resolving.

## Postmortem

Write a postmortem for every SEV1/SEV2 and any repeated SEV3. Include:

- Timeline with detection, mitigation, and resolution times.
- Customer impact and affected features.
- Root cause and contributing factors.
- What worked and what slowed the response.
- Follow-up actions with owners and due dates.

Postmortems are blameless but specific. Track action items in `TODOS.md` or issues until completed.
