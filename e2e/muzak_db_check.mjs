// Muzak DB persistence (checkpoint-only): the music state is mirrored into the
// dataset at checkpoints (so it travels with export/import + backups), and on a
// fresh device (no local music state) it's seeded back from the dataset so the
// player resumes. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const datasetMusic = (page) => page.evaluate(() => {
  try { return (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").settings || {}).music || null; }
  catch (e) { return null; }
});
// The dataset autosaves every 4s or on visibilitychange/pagehide — force a flush.
const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(350);
};

try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(700);

  // Seed a known live position, then toggle (a checkpoint) to mirror into the dataset.
  await page.evaluate(() => localStorage.setItem("cashflux:muzak-pos", JSON.stringify({ i: 4, t: 51 })));
  const btn = page.locator(".muzak-btn").first();
  await btn.click(); // off → checkpointMusic()
  await flush(page);
  let m = await datasetMusic(page);
  if (!m) fail("toggling did not checkpoint music into the dataset");
  else {
    if (m.enabled !== false) fail(`dataset music.enabled = ${m.enabled}, want false after toggling off`);
    if (m.index !== 4 || Math.round(m.position) !== 51) fail(`dataset music did not capture the position: ${JSON.stringify(m)}`);
  }
  await btn.click(); // back on, also checkpointed
  await flush(page);
  m = await datasetMusic(page);
  if (!m || m.enabled !== true) fail("re-enabling was not checkpointed to the dataset");

  // Capture the saved dataset, stamp a distinct resume point into its music state.
  const dataset = await page.evaluate(() => localStorage.getItem("cashflux:dataset"));
  if (!dataset) fail("no dataset blob to seed a fresh device from");
  const patched = JSON.stringify((() => {
    const d = JSON.parse(dataset);
    d.settings = d.settings || {};
    d.settings.music = { enabled: true, volume: 0.2, index: 6, position: 42 };
    return d;
  })());
  await ctx.close();

  // A genuinely fresh device: a new context with the dataset (and seeded flag) but
  // NO local music keys → on load the resume point is seeded from the dataset and
  // the player resumes that track.
  const fresh = await browser.newContext();
  const fp = await fresh.newPage();
  await fp.addInitScript((ds) => {
    localStorage.setItem("cashflux:dataset", ds);
    localStorage.setItem("cashflux:seeded", "1");
  }, patched);
  await fp.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await fp.waitForSelector(".bento", { timeout: 60000 });
  await fp.waitForTimeout(700);
  const seededPos = await fp.evaluate(() => localStorage.getItem("cashflux:muzak-pos"));
  if (!seededPos || !seededPos.includes('"i":6')) fail(`resume point was not seeded from the dataset: ${seededPos}`);
  const idx = await fp.evaluate(() => window.cashfluxMuzak.state().index);
  if (idx !== 6) fail(`player did not resume the dataset track on a fresh device: index ${idx}`);
  const seededVol = await fp.evaluate(() => localStorage.getItem("cashflux:muzak-volume"));
  if (!seededVol || Math.abs(parseFloat(seededVol) - 0.2) > 0.001) fail(`volume not seeded from the dataset: ${seededVol}`);
  await fresh.close();
  if (!process.exitCode) console.log("PASS: music state checkpoints into the dataset and reseeds/resumes on a fresh device.");
} finally {
  await browser.close();
}
