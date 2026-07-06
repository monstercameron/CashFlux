// a11y.spec.mjs — automated accessibility RATCHET (axe-core, WCAG 2 A/AA). The
// app carries pre-existing critical/serious violations (mostly color-contrast on
// muted text, plus a few aria/svg-alt issues); clearing them is a separate design
// effort. So rather than a red gate that blocks everything, this locks in the
// CURRENT state as a baseline and fails only when a route gains a NEW rule
// violation or MORE nodes for an existing rule — no a11y regressions slip in, and
// the baseline documents the debt to burn down.
//
// Regenerate after an intentional a11y change (ideally a REDUCTION):
//   UPDATE_A11Y=1 npx playwright test a11y.spec.mjs --project=chromium
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, nav, settle } from "./fixtures.mjs";

const require = createRequire(import.meta.url);
const AXE_PATH = require.resolve("axe-core/axe.min.js");
const BASELINE = path.join(path.dirname(fileURLToPath(import.meta.url)), "a11y-baseline.json");
const UPDATE = !!process.env.UPDATE_A11Y;

// One representative route per major surface family.
const A11Y_ROUTES = [
  "/", "/transactions", "/accounts", "/budgets", "/goals", "/todo",
  "/reports", "/settings", "/household", "/investments", "/recurring", "/about",
];

// color-contrast is excluded from the automated gate: axe re-evaluates it against
// live computed backgrounds, so charts/images/fonts settling a frame later flip
// the count run-to-run (non-deterministic → flaky). Contrast is instead guarded by
// visual regression + review. The remaining rules (aria-*, svg-img-alt,
// nested-interactive, labels, focus) are structural and stable.
const EXCLUDED_RULES = new Set(["color-contrast"]);

// scanRoute returns { ruleId: blockingNodeCount } for critical/serious rules.
async function scanRoute(app, route) {
  await nav(app, route);
  await settle(app); // axe scans once with no retry — settle first so parallel CPU load can't skew counts
  await app.addScriptTag({ path: AXE_PATH });
  const results = await app.evaluate(async () => {
    // eslint-disable-next-line no-undef
    return await axe.run(document, {
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa"] },
      resultTypes: ["violations"],
    });
  });
  const counts = {};
  for (const v of results.violations) {
    if (v.impact !== "critical" && v.impact !== "serious") continue;
    if (EXCLUDED_RULES.has(v.id)) continue;
    counts[v.id] = (counts[v.id] || 0) + v.nodes.length;
  }
  return counts;
}

if (UPDATE) {
  test("regenerate a11y baseline", async ({ app }) => {
    test.setTimeout(300_000);
    const baseline = {};
    for (const route of A11Y_ROUTES) baseline[route] = await scanRoute(app, route);
    writeFileSync(BASELINE, JSON.stringify(baseline, null, 2) + "\n");
  });
} else {
  let baseline = {};
  try {
    baseline = JSON.parse(readFileSync(BASELINE, "utf8"));
  } catch {
    /* first run before generation — every route reports as new */
  }
  for (const route of A11Y_ROUTES) {
    test(`a11y ratchet: ${route} gains no new critical/serious violations`, async ({ app }) => {
      const now = await scanRoute(app, route);
      const base = baseline[route] || {};
      const regressions = [];
      for (const [rule, count] of Object.entries(now)) {
        const allowed = base[rule] || 0;
        if (count > allowed) regressions.push(`${rule}: ${allowed} → ${count} nodes (new/worse)`);
      }
      expect(regressions, `${route} a11y regressions vs baseline:\n${regressions.join("\n")}`).toEqual([]);
    });
  }
}
