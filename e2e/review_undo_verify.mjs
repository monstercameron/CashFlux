// review_undo_verify.mjs — verifies the review-inbox undo loop (commercial-parity
// report: "persist the decision, advance immediately, and expose undo"):
//   1. Categorize & next advances the queue AND posts an undoable toast.
//   2. Clicking the toast's Undo returns the item to the review queue.
//   3. The reason label ("why is this in review") is visible on the card.
// Usage: node e2e/review_undo_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8097";
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

const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1300); };

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1500);

await nav("/transactions");
await page.locator('[data-testid="txn-review-btn"]').first().click();
await page.waitForTimeout(900);

const progress = async () => (await page.locator('[data-testid="review-progress"]').count()) ? await page.locator('[data-testid="review-progress"]').innerText() : "(none)";

// Reason label present ("why is this item in review").
const reason = (await page.locator(".rvw-reason").count()) ? await page.locator(".rvw-reason").innerText() : "";
check("reason label shows why the item needs review", reason !== "", reason);

const p0 = await progress();

// Arm a category and confirm.
const sel = page.locator('[data-testid="review-category-select"]');
for (const o of await sel.locator("option").all()) {
  const v = await o.getAttribute("value");
  if (v) { await sel.selectOption(v); break; }
}
await page.waitForTimeout(300);
await page.locator('[data-testid="review-commit"]').click();
await page.waitForTimeout(700);

const p1 = await progress();
check("categorize advances the queue", p1 !== p0, `${p0} → ${p1}`);

// Undoable toast with an explicit Undo button.
const toastMsg = (await page.locator(".toast .toast-msg").count()) ? await page.locator(".toast .toast-msg").innerText() : "";
check("toast names the categorization", /categorized as/i.test(toastMsg), toastMsg);
const undoBtn = page.locator(".toast .toast-undo");
check("toast exposes an Undo button", (await undoBtn.count()) > 0);
await page.screenshot({ path: "e2e/review_undo_toast.png" });

// Click Undo: the item returns to the queue (left-count goes back up).
if (await undoBtn.count()) {
  await undoBtn.click();
  await page.waitForTimeout(1200);
  const p2 = await progress();
  check("undo returns the item to the review queue", p2 === p0, `${p1} → ${p2} (expected ${p0})`);
  await page.screenshot({ path: "e2e/review_undo_after.png" });
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
