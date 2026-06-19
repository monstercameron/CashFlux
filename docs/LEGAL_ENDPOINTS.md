# Backend Legal Endpoints

See `docs/LEGAL_COMPLIANCE.md` for the launch draft privacy/terms text, cookie note, DPA outline, subprocessor list, and data-subject request workflow.

CashFlux Cloud exposes public legal-document discovery endpoints for onboarding and billing surfaces:

- `GET /legal/privacy`
- `GET /legal/terms`

Both endpoints return JSON with `slug`, `title`, `version`, `effectiveAt`, and a short `summary` array. They are public because they contain no user data or secrets and must be reachable before login.

The current documents are launch drafts.

Authenticated account data controls:

- `GET /v1/account/export`
- `DELETE /v1/account`

Account export returns the authenticated user's server-side Cloud data: user row, workspaces, current snapshots, blob metadata, usage rows, AI-key provider names, audit events, and refresh-session count. It does not decrypt or return BYO AI keys, and it does not inline blob bytes.

Account deletion purges the authenticated user's relational rows through SQLite cascades, then sweeps unreferenced blob metadata and files.
