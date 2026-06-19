# Backend Legal Endpoints

CashFlux Cloud exposes public legal-document discovery endpoints for onboarding and billing surfaces:

- `GET /legal/privacy`
- `GET /legal/terms`

Both endpoints return JSON with `slug`, `title`, `version`, `effectiveAt`, and a short `summary` array. They are public because they contain no user data or secrets and must be reachable before login.

The current documents are launch drafts. Self-serve account export and deletion remain tracked separately before public Cloud launch.
