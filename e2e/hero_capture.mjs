import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const errs = [];
try {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 });
  const p = await ctx.newPage();
  p.on("pageerror", e => errs.push(String(e)));
  p.on("console", m => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector('[data-widget="hero"]', { timeout: 60000 });
  await p.waitForTimeout(3500);
  const hero = await p.$('[data-widget="hero"]');
  // Idle (move mouse far away first).
  await p.mouse.move(5, 5);
  await p.waitForTimeout(300);
  await hero.screenshot({ path: "e2e/screenshots/hero_idle.png" });
  const idle = await hero.evaluate(n => {
    const wh = n.querySelector('.wh');
    return { border: getComputedStyle(n).borderColor, bg: getComputedStyle(n).backgroundColor, whOpacity: wh ? getComputedStyle(wh).opacity : null };
  });
  // Hover.
  await hero.hover();
  await p.waitForTimeout(400);
  await hero.screenshot({ path: "e2e/screenshots/hero_hover.png" });
  const hov = await hero.evaluate(n => {
    const wh = n.querySelector('.wh');
    return { border: getComputedStyle(n).borderColor, bg: getComputedStyle(n).backgroundColor, whOpacity: wh ? getComputedStyle(wh).opacity : null };
  });
  console.log("idle :", JSON.stringify(idle));
  console.log("hover:", JSON.stringify(hov));
  console.log("console-errors:", errs.length, errs.slice(0, 4));
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
