// lane5_verify.mjs — e2e checks for lane 5 (goals/budgets/household refinements).
// Usage: node e2e/lane5_verify.mjs   (server on :8115 serving the lane5 webroot)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8115";
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

// ───────── #51: contribution slider keyboard + valuetext + numeric entry ─────────
await nav("/goals");
// open the first goal's Plan contribution disclosure
const planToggle = page.locator('button:has-text("Plan contribution") >> visible=true').first();
if (await planToggle.count()) { await planToggle.click(); await page.waitForTimeout(700); }
const slider = page.locator('[data-testid^="goal-plan-slider-"]').first();
if (await slider.count()) {
  const vt0 = await slider.getAttribute("aria-valuetext");
  check("#51: slider carries formatted aria-valuetext", !!vt0 && /\$[\d,]+\.\d{2}\/mo/.test(vt0), vt0 || "(none)");
  const amt0 = await page.locator('[data-testid^="goal-plan-amount-"]').first().inputValue();
  await slider.focus();
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(400);
  const amt1 = await page.locator('[data-testid^="goal-plan-amount-"]').first().inputValue();
  check("#51: ArrowRight steps the plan (numeric field follows)", amt1 !== amt0, `${amt0} → ${amt1}`);
  await page.keyboard.press("End");
  await page.waitForTimeout(400);
  const sMax = parseInt(await slider.getAttribute("max"), 10);
  const sStep = parseInt(await slider.getAttribute("step"), 10);
  const sVal = parseInt(await slider.inputValue(), 10);
  // the browser snaps the value to the step grid, so End lands within one step of max
  check("#51: End jumps to (within one step of) max", sMax - sVal < sStep, `${sVal}/${sMax} step ${sStep}`);
  // numeric entry drives the slider — type a mid-range amount so no clamp applies
  const sMin = parseInt(await slider.getAttribute("min"), 10);
  const midMinor = Math.round((sMin + sMax) / 2 / 100) * 100;
  const midStr = (midMinor / 100).toFixed(2);
  const numInput = page.locator('[data-testid^="goal-plan-amount-"]').first();
  await numInput.fill(midStr);
  await page.waitForTimeout(400);
  const vt1 = await slider.getAttribute("aria-valuetext");
  check("#51: typing an amount updates the slider valuetext", !!vt1 && vt1.includes(midStr.replace(/\B(?=(\d{3})+(?!\d))/g, ",")), `typed ${midStr} → ${vt1}`);
} else {
  check("#51: plan slider reachable", false);
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
