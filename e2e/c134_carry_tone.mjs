// C134: a carried-in deficit on a rollover budget renders in caution amber
// (TextWarn) — distinct from the danger-red used for a current overspend.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
let failed = 0;
const fail = (m) => { console.error("FAIL: " + m); failed++; process.exitCode = 1; };
const pass = (m) => console.log("PASS: " + m);
const errs = [];
try {
  const ctx = await browser.newContext();
  const p = await ctx.newPage();
  p.on("pageerror", (e) => errs.push(String(e)));
  p.on("console", (m) => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector("#app", { timeout: 60000 });
  await p.waitForTimeout(4500);
  await p.click('a[href="/budgets"]');
  await p.waitForTimeout(1800);

  const info = await p.evaluate(() => {
    const subs = [...document.querySelectorAll(".budget-sub")];
    const carry = subs.find((s) => /Carried from previous period/i.test(s.textContent || ""));
    if (!carry) return { found: false };
    const cs = getComputedStyle(carry);
    return { found: true, text: carry.textContent.trim(), color: cs.color };
  });
  if (!info.found) { fail("no rollover carry line found on /budgets (sample should have one)"); }
  else {
    console.log("  carry line: " + JSON.stringify(info.text) + "  color=" + info.color);
    // amber #cfa14e ≈ rgb(207,161,78); danger #d8716f ≈ rgb(216,113,111).
    const m = info.color.match(/(\d+),\s*(\d+),\s*(\d+)/);
    const [r,g,b] = m ? [ +m[1], +m[2], +m[3] ] : [0,0,0];
    const isAmber = Math.abs(r-207)<25 && Math.abs(g-161)<30 && Math.abs(b-78)<35;
    const isDanger = Math.abs(r-216)<20 && Math.abs(g-113)<25 && Math.abs(b-111)<25;
    if (isAmber) pass("carry line is caution amber (distinct from overspend red)");
    else if (isDanger) fail("carry line is STILL danger-red — C134 not fixed");
    else fail("carry color unexpected: " + info.color);
  }
  await p.screenshot({ path: "e2e/screenshots/c134_carry_tone.png" });
  console.log("errors: " + errs.length);
  if (errs.length) { errs.slice(0,5).forEach(e=>console.log("  ERR: "+e)); fail("console errors"); }
} catch (e) { fail("exception: " + e.message); } finally { await browser.close(); }
console.log(failed ? "RESULT: FAILED" : "RESULT: PASSED");
process.exit(failed ? 1 : 0);
