// txc_verify.mjs — verifies the transaction-level comp-parity features:
// TXC-3 quick-filter presets, TXC-2 memo + TXC-1 exclude (edit modal + row
// affordances), and TXC-4 non-lossy merge (duplicates panel). Screenshots each.
// Usage: node e2e/txc_verify.mjs <outDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const BASE = "http://127.0.0.1:8097";
const OUT = (process.argv[2] || (process.env.TEMP || ".") + "/txcrev").replace(/\\/g, "/");
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, deviceScaleFactor: 1, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1100); };

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1500);
await nav("/transactions");

// ---- TXC-3: quick-filter presets ----
const presets = page.locator('[data-testid="txn-presets"]');
console.log("presets row present:", await presets.count());
await page.screenshot({ path: `${OUT}/1_presets.png`, clip: { x: 250, y: 150, width: 1100, height: 220 } });
const rowsBefore = await page.locator('[data-testid^="txn-row-"]').count();
await page.locator('[data-testid="txn-preset-uncat"]').click();
await page.waitForTimeout(900);
const uncatOn = await page.locator('[data-testid="txn-preset-uncat"]').getAttribute("aria-pressed");
const rowsUncat = await page.locator('[data-testid^="txn-row-"]').count();
console.log(`Uncategorized preset: pressed=${uncatOn}, rows ${rowsBefore} → ${rowsUncat}`);
await page.screenshot({ path: `${OUT}/2_uncat_on.png`, fullPage: false });
// Toggle off.
await page.locator('[data-testid="txn-preset-uncat"]').click();
await page.waitForTimeout(700);

// ---- TXC-1 + TXC-2: edit modal Note + Exclude ----
await page.locator('[data-testid^="txn-row-"]').first().click();
await page.waitForTimeout(900);
const hasNote = await page.locator('[data-testid="txn-edit-note"]').count();
const hasExclude = await page.locator('[data-testid="txn-edit-exclude"]').count();
console.log("edit modal — note field:", hasNote, "| exclude checkbox:", hasExclude);
if (hasNote) await page.locator('[data-testid="txn-edit-note"]').fill("split with Priya — she owes half");
if (hasExclude) await page.locator('[data-testid="txn-edit-exclude"]').check();
await page.screenshot({ path: `${OUT}/3_editmodal.png` });
// Save (FlipPanel footer).
const save = page.locator('[data-testid="txn-edit-save"], button:has-text("Save")').first();
await save.click().catch(() => {});
await page.waitForTimeout(1000);
// Row affordances.
const noteGlyphs = await page.locator('[data-testid="txn-row-note"]').count();
const exclBadges = await page.locator('[data-testid="txn-excluded-badge"]').count();
console.log("row affordances — note glyphs:", noteGlyphs, "| excluded badges:", exclBadges);
await page.screenshot({ path: `${OUT}/4_row_affordances.png`, fullPage: false });

// ---- TXC-4: merge in the duplicates panel ----
const dupBtn = page.locator('[data-testid="txn-dupes-btn"]');
if (await dupBtn.count()) {
  await dupBtn.click();
  await page.waitForTimeout(900);
  const mergeBtns = await page.locator('button:has-text("Merge"), [data-testid*="merge"]').count();
  console.log("duplicates panel — merge buttons:", mergeBtns);
  await page.screenshot({ path: `${OUT}/5_duplicates_merge.png` });
}

console.log("console-errors:", errors.length, errors.slice(0, 6).join(" | "));
await browser.close();
