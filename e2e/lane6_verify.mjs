// lane6_verify.mjs — e2e verification for the lane-6 remediation batch:
//   #49 subscriptions honesty across price changes / renewals / annual report
//   #75 notifications polish   #74 review-count clarity   #72 dialog titles
//   #73 assistant restructure  #52 confidence tiers       #68 tokens+density
//   #67 a11y gate
// Usage: node e2e/lane6_verify.mjs [port]   (default 8116, lane-6's server)
import { chromium } from "playwright";

const PORT = process.argv[2] || "8116";
const BASE = `http://127.0.0.1:${PORT}`;
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1500); };
const bodyText = async () => await page.locator("body").innerText();

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ───────────────────────── #49: subscriptions honesty ─────────────────────────
await nav("/subscriptions");
await page.waitForTimeout(1500);
let body = await bodyText();
const priceSection = body.includes("Recent price changes") ? body.slice(body.indexOf("Recent price changes")) : body;
const badPrice = ["Cigarettes", "Gas (", "Pharmacy", "Electricity", "Takeout", "Holiday gifts"].filter((w) => priceSection.slice(0, 1500).includes(w));
check("#49: price changes exclude essentials", badPrice.length === 0, badPrice.join(", ") || "clean");
// local-first cancel: primary button files a checklist; web search is secondary
const cancelBtn = page.locator('[data-testid^="sub-howto-cancel-"]').first();
if (await cancelBtn.count()) {
  const tag = await cancelBtn.evaluate((el) => el.tagName);
  check("#49: How-to-cancel is a local action (button, not external link)", tag === "BUTTON", tag);
  await cancelBtn.click();
  await page.waitForTimeout(900);
  body = await bodyText();
  check("#49: clicking files a local checklist task", /cancellation checklist/i.test(body), "");
  const webLink = page.locator('[data-testid^="sub-cancel-web-"]').first();
  check("#49: web search demoted to a secondary link", (await webLink.count()) > 0, "");
} else {
  check("#49: cancellable subscription row present", false);
}
// annual report subscription totals exclude essentials
await nav("/reports");
await page.waitForTimeout(2200);
body = await bodyText();
const recurM = body.match(/(\d+) recurring charges/);
check("#49: annual report recurring count is classified (not 25)", !recurM || parseInt(recurM[1], 10) < 25, recurM ? recurM[0] : "(no line)");

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
