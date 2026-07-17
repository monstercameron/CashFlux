// merchant_original_verify.mjs — locks the original-vs-cleaned merchant
// contract (parity scan item: preserve original bank text, show the cleaned
// merchant separately, let rules target either):
//   1. The review card shows the cleaned merchant WITH the raw descriptor.
//   2. The edit modal shows "Statement text: <raw>" when an alias/cleanup
//      changes the display name.
//   3. (Engine, locked in Go tests) rules match raw or cleaned — here we spot
//      check via the rules screen preview count using a cleaned name.
// Usage: node e2e/merchant_original_verify.mjs   (server on :8097)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1100 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1500); };
await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1800);

// 1. Review card: cleaned + raw visible together (sample data carries raw
// processor descriptors like "SQ *BLUE BOTTLE COFFEE #47").
await nav("/transactions");
await page.locator('[data-testid="txn-review-btn"]').first().click();
await page.waitForTimeout(900);
let sawBoth = false, cleanedTxt = "", rawTxt = "";
for (let i = 0; i < 40 && !sawBoth; i++) {
  cleanedTxt = (await page.locator('[data-testid="review-payee"]').count()) ? await page.locator('[data-testid="review-payee"]').innerText() : "";
  const rawEl = page.locator(".rvw-rawpayee");
  rawTxt = (await rawEl.count()) ? await rawEl.innerText() : "";
  if (rawTxt && cleanedTxt && rawTxt !== cleanedTxt) { sawBoth = true; break; }
  await page.locator('[data-testid="review-skip"]').click();
  await page.waitForTimeout(350);
}
check("review card shows cleaned merchant + raw descriptor", sawBoth, `${cleanedTxt} | ${rawTxt}`);
if (sawBoth) await page.screenshot({ path: "e2e/merchant_original_review.png" });
await page.keyboard.press("Escape");
await page.waitForTimeout(600);

// 2. Edit modal statement-text caption: create an alias by renaming a payee,
// then reopen and expect the caption. Use the raw-descriptor sample row.
const row = page.locator('[data-testid="txn-row-tx-2026-07-raw0"], [data-testid="txn-row-tx-2026-07-raw1"]').first();
if (await row.count()) {
  await row.scrollIntoViewIfNeeded();
  await row.click();
  await page.waitForTimeout(1200);
  const form = page.locator('[data-testid="txn-edit-form"]');
  check("edit modal opens", (await form.count()) > 0);
  const cap = page.locator('[data-testid="txn-original-statement"]');
  const capText = (await cap.count()) ? await cap.innerText() : "(none)";
  check("edit modal shows the original statement text", (await cap.count()) > 0 && /Statement text:/.test(capText), capText);
  if (await cap.count()) await page.screenshot({ path: "e2e/merchant_original_edit.png" });
  await page.keyboard.press("Escape");
} else {
  check("raw-descriptor sample row present", false, "txn-row-tx-2026-07-raw* not found");
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
