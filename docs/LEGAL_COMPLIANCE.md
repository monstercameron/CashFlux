# Legal Compliance Pack

This document is an engineering launch artifact, not legal advice. Counsel should review the final public copy before CashFlux Cloud accepts paid subscriptions.

## Privacy Policy Draft

CashFlux Cloud is optional. The local-first app remains usable without a Cloud account.

Data CashFlux Cloud may process:

- Account identity from OAuth providers or self-host token mode.
- Workspace metadata, current sync snapshots, snapshot history, and blob metadata.
- Encrypted BYO AI-key ciphertext and nonce; decrypted keys are never returned by export endpoints.
- Usage counters for rate limits, billing support, and abuse controls.
- Audit events for security-relevant actions.
- Billing identifiers and subscription status when Stripe billing is enabled.

CashFlux does not sell personal data. Payment-card data is handled by Stripe-hosted payment flows, not by CashFlux servers.

## Terms Of Service Draft

CashFlux provides budgeting, sync, backup, and AI proxy tooling. It does not provide financial, investment, legal, or tax advice.

Users are responsible for:

- The data they import or sync.
- Provider keys they add.
- Self-hosted server configuration, backups, and access tokens.
- Reviewing AI-generated output before acting on it.

Cloud subscriptions, trials, plan changes, cancellations, and payment method updates should run through
Stripe-hosted Checkout and customer portal surfaces when billing is enabled. CashFlux enforces one Cloud trial
per account/identity before creating Checkout sessions; payment-fraud screening stays in Stripe Checkout/Radar
so card data and fraud outcomes do not touch product behavior.

## Cookie And Consent Note

CashFlux should avoid marketing cookies for launch. Required cookies are limited to OAuth/session security cookies:

- Refresh-token cookie: httpOnly, Secure in production, SameSite.
- CSRF cookie: paired with the CSRF request header for cookie-authenticated session routes.
- OAuth state cookie: short-lived login CSRF/PKCE/nonce binding.

If analytics or marketing pixels are added later, they must be opt-in where required and documented here before release.

## DPA Template Outline

Use this outline for a Data Processing Addendum before selling to organizations:

- Roles: customer as controller, CashFlux as processor for Cloud-hosted data.
- Processing subject: sync, backup, AI proxy, support, billing, abuse prevention, and security logging.
- Data categories: account identity, financial snapshots supplied by the user, blob metadata, audit records, usage counters, and billing identifiers.
- Security measures: TLS, authenticated routes, tenant isolation, encrypted BYO AI keys, audit logs, backups, retention windows, and least-privilege operations.
- Subprocessor notice: maintain the public subprocessor list below and provide notice before material changes.
- Data-subject requests: export/delete endpoints plus documented support workflow and SLA.
- Return/delete: customer can export server data and delete the account; backups expire under the published retention schedule.

## Public Subprocessors

| Provider | Purpose | Data shared |
| --- | --- | --- |
| Stripe | Checkout, subscription billing, customer portal | Billing identifiers, payment status, customer contact fields needed for billing |
| Google OAuth | Optional login provider | OAuth subject, email/profile claims returned during login |
| GitHub OAuth | Optional login provider | OAuth subject, email/profile claims returned during login |
| OpenAI | Optional BYO-key AI proxy | User-selected prompt/image content sent through the proxy using the user's key |
| DigitalOcean or chosen host | Optional Cloud/self-host infrastructure | Server runtime, database, backups, and logs depending on deployment |

## Data Subject Request Workflow

1. Authenticate the requester through their Cloud account or verified support channel.
2. Use `GET /v1/account/export` for self-serve export whenever possible.
3. Use `DELETE /v1/account` for self-serve deletion whenever possible.
4. For manual requests, acknowledge within 5 business days and target completion within 30 days unless law requires a shorter window.
5. Record request id, requester, action, operator, completion time, and exceptions in the support/audit system.
6. For correction requests, prefer in-app edits so sync state remains consistent.
