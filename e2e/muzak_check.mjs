// Background music ("muzak"): the top-bar ♪ toggle is on by default, drives the
// JS audio controller, and persists on/off across reloads. (Audio playback itself
// needs a real gesture + files, so we assert the control + state wiring.) Exits
// non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(600);

  const btn = page.locator(".muzak-btn").first();
  if ((await btn.count()) === 0) fail("the muzak toggle is missing from the top bar");
  if ((await btn.getAttribute("aria-pressed")) !== "true") fail("music should be ON by default");
  if (!(await page.evaluate(() => !!window.cashfluxMuzak))) fail("the muzak JS controller did not load");
  if ((await page.evaluate(() => window.cashfluxMuzak.isEnabled())) !== true) fail("controller should start enabled");

  // The internal playlist data structure is populated (default 3 tracks) at low volume.
  const st = await page.evaluate(() => window.cashfluxMuzak.state());
  if (st.size !== 3) fail(`playlist should hold the 3 default tracks, got size ${st.size}`);
  if (!(st.volume > 0 && st.volume <= 0.2)) fail(`expected a low default volume, got ${st.volume}`);
  if (!(st.crossfadeMs > 0)) fail("crossfade duration should be configured for track transitions");

  // Toggle off → button, controller, and storage all reflect it.
  await btn.click();
  await page.waitForTimeout(250);
  if ((await btn.getAttribute("aria-pressed")) !== "false") fail("button did not flip to off");
  if ((await page.evaluate(() => window.cashfluxMuzak.isEnabled())) !== false) fail("controller did not disable");
  if ((await page.evaluate(() => localStorage.getItem("cashflux:muzak"))) !== "0") fail("off state did not persist");

  // Off survives a reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(600);
  const btn2 = page.locator(".muzak-btn").first();
  if ((await btn2.getAttribute("aria-pressed")) !== "false") fail("music should stay OFF after reload");
  if ((await page.evaluate(() => window.cashfluxMuzak.isEnabled())) !== false) fail("controller should stay disabled after reload");

  // Playlist cursor advances on next() (checked while disabled, so no error cascade).
  const before = await page.evaluate(() => window.cashfluxMuzak.state().index);
  await page.evaluate(() => window.cashfluxMuzak.next());
  await page.waitForTimeout(150);
  const after = await page.evaluate(() => window.cashfluxMuzak.state().index);
  if (after !== (before + 1) % 3) fail(`next() did not advance the playlist cursor: ${before} -> ${after}`);

  // Toggle back on for a clean default.
  await btn2.click();
  await page.waitForTimeout(200);
  if ((await page.evaluate(() => localStorage.getItem("cashflux:muzak"))) !== "1") fail("could not re-enable music");

  if (!process.exitCode) console.log("PASS: muzak is on by default, toggles, drives the audio controller, and persists across reloads.");
} finally {
  await browser.close();
}
