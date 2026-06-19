# SOC 2 Readiness Checklist

This is an engineering readiness checklist, not a certification claim. Use it to keep CashFlux Cloud and self-host support work aligned with audit-friendly controls before a formal SOC 2 program exists.

## Access Control

- Require authenticated access for every route that returns or mutates customer data.
- Keep production secrets in environment/secret-manager storage, not source control.
- Record privileged support actions in audit logs with actor, request id, reason, target, and timestamp.
- Review admin/support access at least monthly and remove stale access immediately.

## Change Management

- Ship one atomic feature per commit with `CHANGELOG.md`, `DEVLOG.md`, and `TODOS.md` updates.
- Require `go test ./...`, wasm build or `gwc verify`, and relevant browser/server checks before deploy.
- Run `cashflux-server backup` and `cashflux-server migrate-check` before backend deploys.
- Prefer forward fixes. Roll back stateless code only; restore from backup rather than downgrading a migrated database.

## Monitoring And Availability

- Expose `/livez`, `/readyz`, `/status`, and `/metrics` through the production TLS host.
- Alert on readiness failures, high HTTP/gRPC error rates, p99 latency, disk pressure, failed backups, and websocket upgrade failures.
- Keep structured JSON logs in production and search by `request_id`, `trace_id`, route/RPC, user id, and cause.
- Rehearse restore and retention jobs on the cadence in `docs/OPERATIONS_RUNBOOK.md`.

## Vendor Management

- Maintain the public subprocessor list in `docs/LEGAL_COMPLIANCE.md`.
- Track Stripe, OAuth providers, OpenAI, and infrastructure host ownership, data shared, and operational contact paths.
- Review vendor status pages and security notices during incidents and monthly operational review.
- Do not add analytics, payment, AI, or hosting subprocessors until the legal compliance pack is updated.

## Incident Response

- Use `docs/INCIDENT_RESPONSE.md` for severity, ownership, update cadence, recovery, and postmortems.
- Preserve request IDs, trace IDs, deploy SHA, logs, metrics snapshots, and backup manifests during incidents.
- Record customer impact, detection time, mitigation time, root cause, and follow-up owners.
- Tie incidents that affect data integrity or availability to a restore rehearsal or mitigation test before closure.
