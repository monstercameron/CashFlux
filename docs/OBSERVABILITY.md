# CashFlux Observability

Use `/metrics` with a backend bearer token and scrape it from Prometheus or an equivalent collector. Logs go to stdout and are collected by the host or container platform; keep access limited because logs include operational identifiers.

## Logs

The self-host Compose stack uses Docker's `local` log driver with 10 files of 10 MB each per service. Forward stdout logs from Docker, journald, or your platform collector to a central sink when running production. Retain operational logs for the shortest useful window, start with 30 days, and restrict access to operators who can handle request ids, trace ids, user ids, workspace ids, and audit event ids.

Use `CASHFLUX_SERVER_LOG_FORMAT=json` in production so collectors can parse fields without scraping text. Search incidents by `request_id`, `trace_id`, `user_id`, `workspace_id`, route/RPC, status, and `cause`.

## Traces

Set `CASHFLUX_SERVER_OTLP_ENDPOINT` to an OTLP/HTTP collector URL, for example
`http://otel-collector:4318`, to enable OpenTelemetry trace export. If the CashFlux-specific variable is unset,
the server falls back to the standard `OTEL_EXPORTER_OTLP_ENDPOINT`. The exported resource uses
`service.name=cashflux-server`; request and RPC logs continue to carry `trace_id` for correlation.

## Service-Level Objectives

- Availability: 99.9% monthly successful health and request handling.
- Error rate: less than 1% HTTP 5xx and less than 1% non-OK gRPC responses over 10 minutes.
- Latency: p99 HTTP request latency under 1 second over 10 minutes; investigate route-level outliers.

## Dashboard Queries

- Backend up: `cashflux_server_up`
- HTTP request rate by route/status: `sum by (route, status) (rate(cashflux_http_requests_total[5m]))`
- HTTP p99 latency: `histogram_quantile(0.99, sum by (le) (rate(cashflux_http_request_duration_seconds_bucket[5m])))`
- gRPC request rate by method/status: `sum by (method, status) (rate(cashflux_grpc_requests_total[5m]))`
- Active streams: `cashflux_grpc_streams_active`
- Sync conflicts: `rate(cashflux_sync_lww_rejects_total[5m])`
- Blob throughput: `rate(cashflux_blob_bytes_transferred_total[5m])`
- Blob GC: `increase(cashflux_blob_gc_deleted_total[24h])` and `increase(cashflux_blob_gc_sweeps_total[24h])`
- AI proxy usage: `rate(cashflux_ai_proxy_requests_total[5m])` and `rate(cashflux_ai_proxy_tokens_total[5m])`
- DB operation rate/latency: `sum by (operation) (rate(cashflux_db_queries_total[5m]))`
- Billing funnel: `increase(cashflux_billing_events_total[24h])` by event/plan/status, plus
  `cashflux_billing_mrr_cents` for estimated active MRR without user labels.

## Alerts And Routing

Load `deploy/prometheus-rules.yml` into Prometheus. Route `severity=page` alerts to the on-call channel and `severity=ticket` alerts to the normal operational backlog. Every alert should include the current deployment, recent request logs, and the relevant dashboard link when your collector supports annotations.

Start incident handling with the runbooks in `docs/SELF_HOSTING.md`: check `/livez`, `/readyz`, `/metrics`, container health, SQLite disk space, and recent structured error logs by `request_id` or `trace_id`.
