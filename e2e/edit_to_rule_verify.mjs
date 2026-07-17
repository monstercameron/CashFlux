// edit_to_rule_verify.mjs — locks the edit-to-rule loop (parity scan item 15):
// correcting a category offers "apply to similar" with a preview, and an
// "Always do this" path that lands in the prefilled rule flow with a live
// match count ("preview affected transactions") and Apply-to-existing.
// Usage: node e2e/edit_to_rule_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 1100 }, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(BASE + "/transactions", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2000);

// Find a repeated merchant: search Blue Bottle, open the first row.
await page.locator('input[placeholder*="Search"]').first().fill("Blue Bottle");
await page.waitForTimeout(1200);
const row = page.locator('[data-testid^="txn-row-"]').first();
check("found a repeated-merchant row", (await row.count()) > 0);
await row.click();
await page.waitForTimeout(1200);
const form = page.locator('[data-testid="txn-edit-form"]');
check("edit modal open", (await form.count()) > 0);

// Change the category and save.
const sel = form.locator("select").first();
const opts = await sel.locator("option").allInnerTexts();
const target = opts.find((o) => /dining/i.test(o)) || opts[2];
await sel.selectOption({ label: target });
await page.waitForTimeout(300);
await page.locator('.flip-panel button:has-text("Save"), dialog button:has-text("Save"), button[type="submit"]:has-text("Save")').last().click();
await page.waitForTimeout(1500);

// The similar-offer should replace the form body.
const body = await page.locator("body").innerText();
const offer = /similar|also (apply|use)|Always/i.test(body);
check("apply-to-similar offer appears with a preview", offer, body.slice(0, 160).replace(/\n/g, " "));
await page.screenshot({ path: "e2e/edit_to_rule_offer.png" });

// Take the "Always do this" path → prefilled rule flow on /rules.
const always = page.locator('button:has-text("Always")').first();
if (await always.count()) {
  await always.click();
  await page.waitForTimeout(1800);
  check("Always-do-this lands on /rules", page.url().endsWith("/rules"), page.url());
  const rbody = await page.locator("body").innerText();
  const matchVal = await page.locator('[data-testid="rule-add-form"] input[type="text"]').first().inputValue().catch(() => "");
  check("rule flow is prefilled with the merchant phrase", /blue bottle/i.test(matchVal), JSON.stringify(matchVal));
  check("live match-count preview shows affected transactions", /match(es)? \d+|\d+ transactions?/i.test(rbody), (rbody.match(/match(es)? \d+[^\n]*|\d+ transactions?[^\n]*/i) || [""])[0]);
  check("Apply-to-existing retroactive action exists", /Apply to existing/i.test(rbody));
  await page.screenshot({ path: "e2e/edit_to_rule_prefilled.png" });
} else {
  check("Always-do-this button present", false);
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
