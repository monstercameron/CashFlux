# Report Export Design

CashFlux reports use live in-app chart rendering, but exported files must be
self-contained. Shared reports can contain sensitive household financial data, so
they should open offline without fetching code, fonts, images, or chart libraries.

## Chart Snapshots

In-app reports may use D3 through the typed chart layer. Exported PDF,
standalone HTML, PNG, CSV, or JSON artifacts must not depend on live D3 at open
time. For visual formats, snapshot the already-rendered static SVG markup from
the report chart and embed that SVG in the exported artifact.

This keeps exports:

- offline-capable, because the shared artifact does not need JavaScript or a CDN;
- deterministic, because it captures the chart the user saw;
- safer to share, because the file does not execute a report-specific script.

CSV and JSON exports should use the same typed report data that produced the
screen, not scrape formatted DOM text.

## D3 Runtime Policy

The app runtime pins D3 at `7.9.0` and includes that URL in the service-worker
core cache so installed/offline app sessions can render charts after the first
successful load. If D3 is vendored locally later, update the service worker and
this note together.

## Privacy Guardrail

Before any visual report export, show copy that makes it clear the exported file
contains financial data. Redacted or aggregates-only exports should be added as a
separate option rather than changing the default report calculations.
