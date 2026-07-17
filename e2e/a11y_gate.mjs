// a11y_gate.mjs — automated accessibility release gate (#67).
//
// Injects axe-core into every core route and FAILS (exit 1) on any serious or
// critical violation. Moderate/minor findings are reported but do not gate, so
// the gate stays green-or-broken, never noisy. Run against any served webroot:
//
//   node e2e/a11y_gate.mjs [port]           # default 8080
//
// The scan runs in BOTH color schemes' default theme (dark) plus a light-theme
// pass on the dashboard, since contrast findings differ per theme.
import { createRequire } from "module";
import { readFileSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const here = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const AXE_SRC = readFileSync(join(here, "node_modules", "axe-core", "axe.min.js"), "utf8");

const PORT = process.argv[2] || "8080";
const BASE = `http://127.0.0.1:${PORT}`;

// Every routed surface a user lives in. Deliberately broad — the gate is the
// release check, not a spot check.
const ROUTES = [
  "/", "/transactions", "/accounts", "/budgets", "/goals", "/todo",
  "/notifications", "/assistant", "/reports", "/subscriptions", "/recurring",
  "/settings",
];

// Known-issue allowlist: rule id → substring of the target selector. Add ONLY
// with a tracking note; an empty list is the goal.
const ALLOW = [
  // frameworks + third-party vendored widgets are out of this gate's scope
];

const allowed = (v, node) =>
  ALLOW.some((a) => v.id === a.rule && node.target.join(" ").includes(a.target));

const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

let gateFailures = 0;
let reported = 0;

async function scan(label) {
  await page.evaluate(AXE_SRC);
  const result = await page.evaluate(async () => {
    return await window.axe.run(document, {
      resultTypes: ["violations"],
      // Best-practice-only rules (region/landmark nesting) produce framework-
      // level noise; the gate enforces the WCAG A/AA tags.
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"] },
    });
  });
  for (const v of result.violations) {
    const nodes = v.nodes.filter((n) => !allowed(v, n));
    if (nodes.length === 0) continue;
    const gating = v.impact === "serious" || v.impact === "critical";
    if (gating) gateFailures += nodes.length;
    reported += nodes.length;
    console.log(`${gating ? "FAIL" : "warn"} [${label}] ${v.id} (${v.impact}) ×${nodes.length} — ${v.help}`);
    for (const n of nodes.slice(0, 3)) console.log(`       ${n.target.join(" ")}`);
    if (nodes.length > 3) console.log(`       … +${nodes.length - 3} more`);
  }
}

for (const route of ROUTES) {
  await page.evaluate((r) => { history.pushState({}, "", r); dispatchEvent(new PopStateEvent("popstate")); }, route);
  await page.waitForTimeout(1800);
  await scan(route);
}

// Light-theme contrast pass on the dashboard (findings differ per theme).
await page.evaluate(() => {
  localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
});
await page.evaluate(() => { history.pushState({}, "", "/"); dispatchEvent(new PopStateEvent("popstate")); });
await page.reload({ waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);
await scan("/ (light)");

await b.close();
console.log(`\nA11Y GATE: ${gateFailures} gating (serious/critical) · ${reported} total findings across ${ROUTES.length + 1} scans`);
process.exit(gateFailures === 0 ? 0 : 1);
