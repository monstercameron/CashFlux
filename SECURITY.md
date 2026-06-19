# Security Policy

## Supported Versions

CashFlux is pre-release software. Security fixes are made on `main` and included in the next published build.

## Reporting a Vulnerability

Please report suspected vulnerabilities through GitHub private vulnerability reporting:

https://github.com/monstercameron/CashFlux/security/advisories/new

Do not open a public issue for an unpatched vulnerability. Include the affected route, feature, or package; reproduction steps; impact; and any relevant logs with secrets removed.

## Handling

Reports are triaged as soon as practical. Confirmed issues that affect user data, authentication, encrypted AI keys, sync isolation, or backend availability are treated as release-blocking until fixed or explicitly deferred with a documented mitigation.

## Scope Notes

CashFlux is local-first, with an optional backend for sync and AI proxying. Never send real financial data, access tokens, OAuth secrets, OpenAI keys, or production database files in a report.
