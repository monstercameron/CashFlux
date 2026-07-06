// Dashboard bento drag-reorder test — backed by the in-Go FLIP + drag-target
// coordinator (internal/ui/bentoflip.go, the port of the former web/flip.js).
//
// This drives a REAL mouse drag of a tile's grip handle (press → move in steps →
// release), which exercises the actual native HTML5 drag pipeline the user touches
// — not just synthetic DragEvents. It asserts the exact symptoms a broken coordinator
// would show:
//   - the tile actually MOVES (reorders) — not "dims but stays put"
//   - while dragging it dims (.w.drag) and the grid cursor is "grabbing"
//   - after release the dim AND the [data-bento-dragging] cursor lock are CLEARED
//     (no stuck dim, cursor returns to default)
//   - the reorder persists across a reload (layout lives in SQLite/IndexedDB now)
//
// A second, synthetic-event phase proves the stable insertion target holds when a
// FLIP-animated tile slides under the pointer (can't be reproduced with a real mouse).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Curated default layout: full-width "attention" band, then a KPI row of
// assets · liabilities · safe-to-spend. Drag the (2-wide) safe-to-spend tile onto
// assets so it reflows to the front of that row.
const SRC = "kpi-safetospend";
const INTENDED = "kpi-assets";
const CHURN = "kpi-liabilities";

const order = (page) =>
  page.evaluate(() =>
    [...document.querySelectorAll(".bento > .w[data-widget]")]
      .map((el) => {
        const cs = getComputedStyle(el);
        return { id: el.dataset.widget, x: Number.parseInt(cs.gridColumnStart, 10), y: Number.parseInt(cs.gridRowStart, 10) };
      })
      .sort((a, b) => a.y - b.y || a.x - b.x)
      .map((p) => p.id)
  );

const idx = (arr, id) => arr.indexOf(id);

try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(`[data-widget="${INTENDED}"]`, { timeout: 60000 });
  await page.waitForSelector(`[data-widget="${SRC}"]`, { timeout: 60000 });

  // The Go port owns this behavior now: no JS helper, no window globals.
  const legacy = await page.evaluate(() => ({
    globals: ["cashfluxBentoDragStart", "cashfluxBentoDragTarget", "cashfluxBentoDragEnd", "cashfluxFlipBento"].filter((g) => typeof window[g] !== "undefined"),
    scriptTag: !!document.querySelector('script[src*="flip.js"]'),
  }));
  if (legacy.globals.length) fail(`legacy JS drag globals still present: ${legacy.globals.join(",")}`);
  if (legacy.scriptTag) fail("flip.js <script> tag still in the page");

  // ---- Phase 1: a REAL mouse drag of the grip handle. ----
  const before = await order(page);

  const boxOf = (sel) => page.locator(sel).first().boundingBox();
  const gripBox = await boxOf(`.bento > .w[data-widget="${SRC}"] .grip`);
  const srcBox = await boxOf(`.bento > .w[data-widget="${SRC}"]`);
  const dstBox = await boxOf(`.bento > .w[data-widget="${INTENDED}"]`);
  if (!srcBox || !dstBox) fail("source/target tile not found");
  // Press on the grip when present, else the tile's top-left chrome.
  const from = gripBox ? { x: gripBox.x + gripBox.width / 2, y: gripBox.y + gripBox.height / 2 } : { x: srcBox.x + 20, y: srcBox.y + 14 };
  const to = { x: dstBox.x + dstBox.width / 2, y: dstBox.y + dstBox.height / 2 };

  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  // Intermediate moves are required for Chromium to initiate a native HTML5 drag.
  for (let i = 1; i <= 25; i++) {
    await page.mouse.move(from.x + (to.x - from.x) * (i / 25), from.y + (to.y - from.y) * (i / 25));
    await page.waitForTimeout(12);
  }
  await page.mouse.move(to.x, to.y);
  await page.waitForTimeout(60);

  // Mid-drag: the tile should be dimmed and the grid cursor should be "grabbing".
  const mid = await page.evaluate((SRC) => {
    const s = document.querySelector(`.bento > .w[data-widget="${SRC}"]`);
    const bento = document.querySelector(".bento");
    return {
      dim: s ? s.classList.contains("drag") : null,
      dragging: document.documentElement.getAttribute("data-bento-dragging"),
      cursor: bento ? getComputedStyle(bento).cursor : null,
    };
  }, SRC);
  if (!mid.dim) fail("source tile should dim (.w.drag) while dragging");
  if (mid.dragging == null) fail("[data-bento-dragging] should be set on <html> while dragging");
  if (mid.cursor !== "grabbing") fail(`grid cursor should be 'grabbing' while dragging; got '${mid.cursor}'`);

  await page.mouse.up();
  await page.waitForTimeout(320);

  // After release: dim AND the cursor lock MUST clear (this is the user's bug —
  // "stays dim on drag end and the cursor never goes back").
  const after = await page.evaluate((SRC) => {
    const s = document.querySelector(`.bento > .w[data-widget="${SRC}"]`);
    const bento = document.querySelector(".bento");
    return {
      dim: s ? s.classList.contains("drag") : null,
      dragging: document.documentElement.getAttribute("data-bento-dragging"),
      cursor: bento ? getComputedStyle(bento).cursor : null,
    };
  }, SRC);
  if (after.dim) fail("source tile is STUCK dimmed after drop (.w.drag not cleared)");
  if (after.dragging != null) fail("[data-bento-dragging] is STUCK on <html> after drop (cursor never reverts)");
  if (after.cursor === "grabbing") fail("grid cursor STUCK on 'grabbing' after drop");

  // The tile must have actually moved (in front of the intended target).
  const moved = await order(page);
  if (before.join(",") === moved.join(",")) fail(`real drag did not move anything; order unchanged: ${moved.join(",")}`);
  if (idx(moved, SRC) < 0 || idx(moved, SRC) >= idx(moved, INTENDED)) {
    fail(`real drag should place ${SRC} before ${INTENDED}; got ${moved.join(",")}`);
  }

  // ---- Phase 2: synthetic FLIP-churn — the stable insertion target must hold. ----
  const churn = await page.evaluate(async ({ INTENDED, CHURN }) => {
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    const el = (id) => document.querySelector(`.bento > .w[data-widget="${id}"]`);
    const c = (n) => { const r = n.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; };
    const dt = new DataTransfer();
    const d = (n, t, p) => n.dispatchEvent(new DragEvent(t, { bubbles: true, cancelable: true, clientX: p.x, clientY: p.y, dataTransfer: dt }));
    const vorder = () => [...document.querySelectorAll(".bento > .w[data-widget]")]
      .map((n) => { const cs = getComputedStyle(n); return { id: n.dataset.widget, x: +cs.gridColumnStart, y: +cs.gridRowStart }; })
      .sort((a, b) => a.y - b.y || a.x - b.x).map((p) => p.id);

    // Drag CHURN's neighbor toward INTENDED, then make a transformed tile slide under
    // the fixed pointer; the order must not oscillate.
    const src = el(CHURN);
    const intended = el(INTENDED);
    const pt = c(intended);
    d(src, "dragstart", pt);
    await sleep(30);
    d(intended, "dragover", pt);
    await sleep(60);
    const a = vorder();
    intended.style.transform = "translateX(240px)";
    el(CHURN).style.transform = "translateX(-240px)";
    d(el(CHURN), "dragover", pt);
    await sleep(60);
    const b = vorder();
    intended.style.transform = "";
    el(CHURN).style.transform = "";
    d(src, "dragend", pt);
    await sleep(120);
    return { a, b };
  }, { INTENDED, CHURN });
  if (churn.a.join(",") !== churn.b.join(",")) {
    fail(`stable target oscillated under FLIP churn: ${churn.a.join(",")} -> ${churn.b.join(",")}`);
  }

  // ---- Phase 3: persistence across reload (wait past the 4s autosave ticker). ----
  await page.waitForTimeout(4600);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(`[data-widget="${SRC}"]`, { timeout: 60000 });
  const reloaded = await order(page);
  if (idx(reloaded, SRC) < 0 || idx(reloaded, SRC) >= idx(reloaded, INTENDED)) {
    fail(`reorder should survive reload; ${SRC} no longer before ${INTENDED}: ${reloaded.join(",")}`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: real grip drag reorders the tile, dims only during the drag, clears the cursor on release, holds a stable target through FLIP churn, and persists across reload.");
} finally {
  await browser.close();
}
