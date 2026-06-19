# Investments Scope Decision

CashFlux is a household budgeting app, not a brokerage portfolio tracker. The
core investment scope is therefore balance-only: investment, retirement, and
brokerage accounts are represented as accounts with manually updated balances.

## Decision

Keep investments in core as balance-only accounts. Do not add holdings,
cost-basis, tax-lot, performance attribution, or live market pricing to the core
app.

This keeps the product:

- local-first and useful offline;
- free of market-data licensing and backend price-feed requirements;
- focused on household cash flow, budgets, bills, goals, and net worth;
- simpler to validate for privacy and export/import round trips.

## Optional Future Extension

If investment detail is added later, build it as a lightweight manual extension:
symbol, quantity, manual price, and as-of date. Do not fetch live prices from a
public API in the browser, and do not require paid/licensed market data in the
local-first core.

Net-worth reports may continue to use account balances. Users who need full
portfolio analytics should keep using a dedicated brokerage or portfolio tool
alongside CashFlux.
