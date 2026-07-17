// recurring_match_verify.mjs — locks the recurring expected-vs-actual loop
// (parity scan item 18): an OVERDUE scheduled charge flips to PAID once a
// matching transaction posts (billmatch: amount tolerance + date window +
// payee identity), and the over/under delta is stated when amounts differ.
// Usage: node e2e/recurring_match_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };

const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1300 }, reducedMotion: "reduce" })).newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1600); };

await page.goto(BASE + "/recurring", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2200);

const bodyBefore = await page.locator("main").innerText();
check("an OVERDUE scheduled charge exists (expected, no actual)", /OVERDUE/.test(bodyBefore));
check("no PAID badge yet for it", (await page.locator(".rec-tag-paid").count()) === 0);

// Post a matching payment: Streaming & apps, $37.50 (slightly under the $38
// expected — inside tolerance, so it settles AND states the delta), dated Jul 3.
await nav("/transactions");
await page.locator('button:has-text("Add transaction")').first().click();
await page.waitForTimeout(1000);
await page.locator('select:has(option[value="Expense"])').first().selectOption("Expense").catch(() => {});
await page.locator('[data-testid="txn-add-amount"]').fill("37.50");
await page.locator('[data-testid="txn-add-payee"]').fill("Streaming & apps");
await page.locator('[data-testid="txn-add-desc"]').fill("Streaming & apps");
await page.locator('[data-testid="txn-add-date"]').fill("2026-07-03");
await page.locator('[data-testid="txn-add-save"], button:has-text("Save")').first().click();
await page.waitForTimeout(1500);

// Back on /recurring, the occurrence reads PAID with the under-run stated.
await nav("/recurring");
await page.waitForTimeout(1200);
const paid = await page.locator(".rec-tag-paid").count();
check("the matching payment flips the occurrence to PAID", paid > 0, `${paid} paid badges`);
const bodyAfter = await page.locator("main").innerText();
check("the over/under delta is stated", /\$[\d.,]+\s+(over|under)/i.test(bodyAfter), (bodyAfter.match(/.*\$[\d.,]+\s+(over|under).*/i) || [""])[0].trim());
await page.screenshot({ path: "e2e/recurring_match_paid.png" });

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
