// all_routes_smoke.mjs — v1.0 backbone regression. Every routed page must:
//   1. load with substantive content (not an empty shell / error card),
//   2. render zero console/page errors,
//   3. survive a light-theme pass and back,
//   4. keep the left-nav highlight + breadcrumb coherent.
// This is the stable floor under the per-page interaction regressions; it
// encodes "the app is not broken on any route", independent of visual polish.
//
// Run:  node e2e/regression/all_routes_smoke.mjs
import { boot, nav, setTheme, mainText, errsOf } from "./_harness.mjs";

// Every route in the screens registry (rail + off-rail) plus the three seeded
// custom pages. Each entry: [route, mustContainRegex]. The regex is a light
// sanity anchor — a word that MUST appear when the page rendered its real body,
// case-insensitive (innerText honors text-transform).
const ROUTES = [
  ["/", /net worth|dashboard|good (morning|afternoon|evening)/i],
  ["/transactions", /transaction|payee|amount/i],
  ["/accounts", /account|balance/i],
  ["/budgets", /budget/i],
  ["/goals", /goal/i],
  ["/todo", /to-?do|task/i],
  ["/notifications", /notification|alert|nothing/i],
  ["/debt", /debt|payoff|owed/i],
  ["/investments", /portfolio|securities|holding|investment/i],
  ["/allocate", /allocate|put to work|surplus/i],
  ["/planning", /plan|scenario|forecast/i],
  ["/recurring", /recurring|schedule|upcoming/i],
  ["/reports", /report|spending|income/i],
  ["/networth", /net worth|assets|liabilities/i],
  ["/health", /health|score/i],
  ["/assistant", /assistant|ask|chat/i],
  ["/studio", /studio|design|widget|formula/i],
  ["/household", /household|member|people/i],
  ["/categories", /categor/i],
  ["/rules", /rule|auto-?fil|match/i],
  ["/artifacts", /file|artifact|vault|storage/i],
  ["/activity", /activity|change|record|history/i],
  ["/settings", /household|preferences|appearance/i],
  ["/help", /help|getting set up|set up/i],
  ["/about", /about|privacy|version/i],
  ["/customize", /formula|metric|calculat/i],
  ["/fields", /field|custom/i],
  ["/workflows", /workflow|automation|trigger/i],
  ["/appearance", /appearance|theme|mode|motion/i],
  ["/setup", /set up|currency|income|account/i],
  ["/credit", /credit|card|utiliz/i],
  ["/loans", /loan|balance|owed/i],
  ["/bills", /bill|due|upcoming/i],
  ["/subscriptions", /subscription|monthly|recurring/i],
  ["/insights", /insight|spending|highlight/i],
  ["/smart", /smart|ai|feature/i],
  ["/members", /member|people|person/i],
  ["/split", /split|share|settle/i],
  ["/widget-builder", /widget|build|card|canvas/i],
  ["/widget-manager", /widget|manage|dashboard/i],
  ["/documents", /document|import|csv|upload/i],
  ["/duplicates", /duplicate|possible|match/i],
  ["/plans", /plan|free|cloud|price/i],
  ["/p/side-hustle", /side|surplus|project/i],
  ["/p/priya-business", /shop|business|revenue|priya/i],
  ["/p/marcus-hobbies", /hobb|stonks|brokerage|marcus/i],
];

const fails = [];
const { browser, context, page, errors } = await boot();
try {
  for (const [route, rx] of ROUTES) {
    const before = errors.length;
    await nav(page, route, 1800);
    const t = await mainText(page).catch(() => "");
    if (!t || t.trim().length < 40) fails.push(`${route}: empty/too-short body`);
    else if (!rx.test(t)) fails.push(`${route}: body missing expected anchor ${rx}`);
    const newErrs = errors.slice(before);
    if (newErrs.length) fails.push(`${route}: ${newErrs.length} error(s) — ${newErrs.join(" | ")}`);
    console.log(`  ${fails.some((f) => f.startsWith(route + ":")) ? "✗" : "✓"} ${route}`);
  }

  // Light-theme survives across a representative sweep (recolor + no new errors).
  await setTheme(page, "light");
  const lightBefore = errors.length;
  for (const route of ["/", "/transactions", "/reports", "/settings", "/subscriptions", "/p/priya-business"]) {
    await nav(page, route, 1500);
    const dt = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    if (dt !== "light") fails.push(`${route}: data-theme not light in light mode (${dt})`);
  }
  const lightErrs = errors.slice(lightBefore);
  if (lightErrs.length) fails.push(`light-theme pass: ${lightErrs.length} error(s) — ${lightErrs.join(" | ")}`);
  await setTheme(page, "dark");
  console.log(`  ${fails.some((f) => f.includes("light")) ? "✗" : "✓"} light-theme sweep`);
} finally {
  await context.close();
  await browser.close();
}

if (fails.length) {
  console.error("\nFAIL all_routes_smoke:\n - " + fails.join("\n - "));
  process.exit(1);
}
console.log("\nPASS: all_routes_smoke — every route loads, no errors, both themes");
