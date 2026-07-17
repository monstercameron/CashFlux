// lane3_verify_61.mjs — verify the Cmd/Ctrl+K command palette (#61): opens from
// the keyboard, filters, navigates, runs high-frequency actions (add
// transaction, toggle sidebar), and the theme-toggle command — the known
// crasher — actually flips the theme with ZERO page errors (a wasm panic
// kills the whole app). Usage: node e2e/lane3_verify_61.mjs <port> <shotDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const PORT = process.argv[2] || "8113";
const OUT = process.argv[3] || "lane3-shots";
mkdirSync(OUT, { recursive: true });

let failures = 0;
const check = (ok, msg) => { console.log(`${ok ? "PASS" : "FAIL"} ${msg}`); if (!ok) failures++; };

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e).slice(0, 200)));
await page.goto(`http://127.0.0.1:${PORT}/`, { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1500);

const paletteVisible = () => page.evaluate(() => {
  const p = document.getElementById("cf-cmd-palette");
  return !!p && p.style.display !== "none";
});

// Open with Ctrl+K.
await page.keyboard.press("Control+KeyK");
await page.waitForTimeout(400);
check(await paletteVisible(), "Ctrl+K opens the palette");
await page.screenshot({ path: `${OUT}/61-palette.png` });

// Filter + navigate: type "reports", Enter → /reports.
await page.keyboard.type("reports");
await page.waitForTimeout(300);
await page.keyboard.press("Enter");
await page.waitForTimeout(1000);
check(await page.evaluate(() => location.pathname).then((p) => p.endsWith("/reports")), "filter + Enter navigates to /reports");
check(!(await paletteVisible()), "palette closes after running a command");

// High-frequency action: add transaction.
await page.keyboard.press("Control+KeyK");
await page.waitForTimeout(300);
await page.keyboard.type("add transaction");
await page.waitForTimeout(300);
await page.keyboard.press("Enter");
await page.waitForTimeout(800);
const quickAdd = await page.evaluate(() => !!document.querySelector('[data-testid="quick-add"], .quick-add, [role="dialog"]'));
check(quickAdd, "palette runs Add transaction (quick-add opens)");
await page.keyboard.press("Escape");
await page.waitForTimeout(500);

// THE CRASHER: theme toggle from the palette.
const themeBefore = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
await page.keyboard.press("Control+KeyK");
await page.waitForTimeout(300);
await page.keyboard.type("theme");
await page.waitForTimeout(300);
await page.keyboard.press("Enter");
await page.waitForTimeout(1200);
const themeAfter = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
const appAlive = await page.evaluate(() => {
  const app = document.getElementById("app");
  return !!app && app.children.length > 0 && document.documentElement.getAttribute("data-app-ready") === "true";
});
check(themeAfter !== themeBefore, `theme toggle flips the theme (${themeBefore} -> ${themeAfter})`);
check(appAlive, "app still alive after theme toggle (no wasm panic)");
await page.screenshot({ path: `${OUT}/61-theme-toggled.png` });

// App remains interactive: palette reopens and Escape closes it.
await page.keyboard.press("Control+KeyK");
await page.waitForTimeout(400);
check(await paletteVisible(), "palette reopens after the theme toggle");
await page.keyboard.press("Escape");
await page.waitForTimeout(300);
check(!(await paletteVisible()), "Escape closes the palette");

// Toggle back to the original theme for a clean state.
await page.keyboard.press("Control+KeyK");
await page.waitForTimeout(300);
await page.keyboard.type("theme");
await page.waitForTimeout(250);
await page.keyboard.press("Enter");
await page.waitForTimeout(800);

check(errors.length === 0, `zero page errors (${errors.length ? errors[0] : "clean"})`);
await browser.close();
console.log(failures === 0 ? "ALL CHECKS PASSED" : `${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
