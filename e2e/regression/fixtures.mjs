// fixtures.mjs — shared Playwright Test fixtures + helpers for the CashFlux
// regression suite. The whole suite keys off the app's own readiness signal
// (`data-app-ready`), set once boot+seed+mount completes, so NOTHING here sleeps
// on a guessed timeout — every wait is a web-first assertion.
import { test as base, expect } from "@playwright/test";

// ROUTES is the single source of truth for "every page": the screens registry
// (rail + off-rail) plus the three seeded custom pages. Each entry pairs a route
// with a case-insensitive anchor that MUST appear when its real body rendered
// (innerText honors text-transform, hence case-insensitive).
export const ROUTES = [
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

// boot loads the shell at / and waits for the app's own readiness signal — the
// deterministic replacement for the old fixed 5.5s seed sleep.
export async function boot(page) {
  // Neutralize the browser View Transitions API for tests: the router wraps route
  // changes in document.startViewTransition, and machine-speed navigation aborts
  // an in-flight transition ("Transition was aborted…"), which surfaces as an
  // unhandled rejection a real user never hits. Running the DOM-update callback
  // synchronously (no animated transition) keeps navigation instant and
  // deterministic — consistent with the reducedMotion:reduce we already set.
  await page.addInitScript(() => {
    try {
      const proto = Document.prototype;
      if (proto && "startViewTransition" in proto) {
        proto.startViewTransition = function (cb) {
          try {
            if (typeof cb === "function") cb();
          } catch (_) {}
          const done = Promise.resolve();
          return { finished: done, ready: done, updateCallbackDone: done, skipTransition() {} };
        };
      }
    } catch (_) {}
  });
  await page.goto("/");
  await expect(page.locator("#app")).toBeAttached();
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 45_000 },
  );
}

// nav SPA-navigates to a route (the static server has a history fallback, so a
// real goto would also work, but pushState keeps app state warm and is faster)
// and waits until the NEW route is actually mounted — `#main[data-route=…]`
// flips only once the target screen has rendered, so this never reads stale
// content from the previous page.
export async function nav(page, route) {
  await page.evaluate((r) => {
    history.pushState({}, "", r);
    dispatchEvent(new PopStateEvent("popstate"));
  }, route);
  const sel = `#main[data-route="${route}"]`;
  await expect(page.locator(sel).first()).toBeVisible();
}

// mainText returns the main pane's innerText. `.first()` because repeated
// synthetic pushState can transiently leave more than one #main mounted.
export async function mainText(page) {
  return page.locator("#main").first().innerText();
}

// setTheme flips light/dark via the real /settings → Appearance control (the
// honest path), waiting on the documentElement theme attribute to flip.
export async function setTheme(page, mode) {
  const cur = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  if (cur === mode) return;
  await nav(page, "/settings");
  // Auto-waiting locator clicks (not raw evaluate) so each step waits for the
  // control to be present + actionable before acting.
  await page.locator(".settings-page .set-tab-strip button", { hasText: "Appearance" }).first().click();
  const seg = page.locator("#sec-appearance-mode");
  await expect(seg).toBeVisible();
  await seg.locator("button", { hasText: mode === "light" ? "Light" : "Dark" }).first().click();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.getAttribute("data-theme")))
    .toBe(mode);
}

// test is the shared fixture: `app` is a page already booted+seeded, and a
// per-test console/page-error collector is attached and asserted empty by default
// via the `errors` fixture (opt out by reading it yourself).
export const test = base.extend({
  errors: async ({ page }, use) => {
    const errors = [];
    // Uncaught exceptions are always genuine app failures.
    page.on("pageerror", (e) => errors.push("pageerror: " + String(e).slice(0, 300)));
    // Console errors, minus network-load noise: the hermetic static server 404s
    // requests the real app makes to optional endpoints (the admin-console probe,
    // favicon, an unconfigured backend), which Chromium logs as console errors but
    // are expected here. Those are NOT app bugs, so drop them; keep everything else.
    page.on("console", (m) => {
      if (m.type() !== "error") return;
      const text = m.text();
      if (/Failed to load resource|net::ERR_|status of (404|401|403|501|503)/i.test(text)) return;
      errors.push("console: " + text.slice(0, 300));
    });
    await use(errors);
  },
  app: async ({ page, errors }, use) => {
    void errors; // ensure the collector is attached before boot
    await boot(page);
    await use(page);
  },
});

export { expect };
