# Archived e2e scratch (not run, not gated)

These are the ad-hoc `node`/Playwright scripts and screenshot dumps produced
during development before the suite was consolidated onto the Playwright Test
runner. They are kept for historical reference only.

**They are NOT run by CI and are NOT maintained.** Many were one-shot probes tied
to a specific fix or dataset and have bit-rotted (e.g. they read `localStorage`
directly, which the app moved off after the IndexedDB migration).

The trusted, CI-gated regression suite lives in [`../regression/`](../regression) —
see [`../README.md`](../README.md). If you need to reproduce something one of these
scripts did, port the check into a `*.spec.mjs` there instead of reviving the
script.
