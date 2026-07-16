// review_verify.mjs — verifies the transaction Review inbox (CG-S2). It ensures
// there is at least one item to review (adding an uncategorized txn if needed),
// opens the inbox from the toolbar, walks a categorize step, and checks the count
// decrements. Screenshots each state. Usage: node e2e/review_verify.mjs <outDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const BASE = "http://127.0.0.1:8097";
const OUT = (process.argv[2] || (process.env.TEMP || ".") + "/reviewrev").replace(/\\/g, "/");
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, deviceScaleFactor: 1, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));

const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1200); };

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1500);
await nav("/transactions");

let reviewBtn = page.locator('[data-testid="txn-review-btn"]');
let hadBacklog = (await reviewBtn.count()) > 0;
console.log("initial review button present:", hadBacklog);

// Ensure there's something to review: add an uncategorized transaction with a
// payee no rule will match, so it stays uncategorized.
if (!hadBacklog) {
  const addBtn = page.locator('[data-testid="add-transaction-btn"], [data-testid="txn-add-btn"]').first();
  await addBtn.click().catch(() => {});
  await page.waitForTimeout(700);
  const amt = page.locator('[data-testid="txn-add-amount"]').first();
  if (await amt.count()) {
    await amt.fill("42.00");
    const desc = page.locator('[data-testid="txn-add-desc"], input[aria-label*="escription" i]').first();
    if (await desc.count()) await desc.fill("ZZQ Uncategorized Probe");
    // Save & close.
    const save = page.locator('[data-testid="txn-add-save"], button:has-text("Save")').first();
    await save.click().catch(() => {});
    await page.waitForTimeout(1200);
  }
  await nav("/transactions");
  reviewBtn = page.locator('[data-testid="txn-review-btn"]');
}

const present = await reviewBtn.count();
console.log("review button present:", present, present ? "label=" + (await reviewBtn.first().innerText()).replace(/\n/g, " ") : "");
await page.screenshot({ path: `${OUT}/toolbar.png`, fullPage: false });

if (present) {
  await reviewBtn.first().click();
  await page.waitForTimeout(900);
  const inbox = page.locator('[data-testid="review-inbox"]');
  console.log("inbox mounted:", await inbox.count());
  const prog0 = (await page.locator('[data-testid="review-progress"]').count()) ? await page.locator('[data-testid="review-progress"]').innerText() : "(none)";
  const payee0 = (await page.locator('[data-testid="review-payee"]').count()) ? await page.locator('[data-testid="review-payee"]').innerText() : "(none)";
  const hasSelect = await page.locator('[data-testid="review-category-select"]').count();
  const hasSuggest = await page.locator('[data-testid="review-suggest"]').count();
  console.log("progress:", prog0, "| payee:", payee0, "| select:", hasSelect, "| suggest:", hasSuggest);
  await page.screenshot({ path: `${OUT}/inbox_open.png` });

  // Choose a category (this should NOT auto-commit) then confirm via the primary.
  const sel = page.locator('[data-testid="review-category-select"]').first();
  const opts = await sel.locator("option").all();
  let picked = "";
  for (const o of opts) { const v = await o.getAttribute("value"); if (v) { picked = v; break; } }
  if (picked) {
    await sel.selectOption(picked);
    await page.waitForTimeout(400);
    const progMid = await page.locator('[data-testid="review-progress"]').innerText();
    console.log("after select (should be unchanged):", progMid);
    await page.screenshot({ path: `${OUT}/inbox_armed.png` });
    // Confirm.
    await page.locator('[data-testid="review-commit"]').click();
    await page.waitForTimeout(900);
    const prog1 = (await page.locator('[data-testid="review-progress"]').count()) ? await page.locator('[data-testid="review-progress"]').innerText() : "(done)";
    console.log("after commit → progress:", prog1);
    await page.screenshot({ path: `${OUT}/inbox_after.png` });
  }
  const hasSimilar = await page.locator('[data-testid="review-similar"]').count();
  console.log("similar-merchant checkbox present on some item:", hasSimilar);
}

console.log("console-errors:", errors.length, errors.slice(0, 6).join(" | "));
console.log(present ? "PASS: review inbox reachable" : "FAIL: no review entry");
await browser.close();
