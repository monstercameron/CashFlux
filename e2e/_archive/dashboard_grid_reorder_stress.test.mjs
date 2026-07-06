// Stress test for the in-Go bento drag coordinator (internal/ui/bentoflip.go, the
// port of the former web/flip.js): three REAL back-to-back mouse drags of grip
// handles. Each drag must (1) actually move the tile, (2) leave NO stuck state —
// no lingering .w.drag dim and no lingering [data-bento-dragging] cursor lock — and
// (3) not bounce the scroll container while the grid reflows.
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

const stuckState = (page, src) =>
  page.evaluate((SRC) => {
    const s = document.querySelector(`.bento > .w[data-widget="${SRC}"]`);
    const bento = document.querySelector(".bento");
    return {
      dim: s ? s.classList.contains("drag") : false,
      dragging: document.documentElement.getAttribute("data-bento-dragging"),
      cursor: bento ? getComputedStyle(bento).cursor : null,
    };
  }, src);

try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-widget="kpi-assets"]', { timeout: 60000 });
  // Keep the bento pinned to the top so the upper tiles we drag are on-screen.
  await page.evaluate(() => { const m = document.querySelector("main.cf-scroll"); if (m) m.scrollTop = 0; });

  const boxOf = (sel) => page.locator(sel).first().boundingBox();
  const scrollOf = () => page.evaluate(() => { const m = document.querySelector("main.cf-scroll"); return m ? m.scrollTop : 0; });

  const realDrag = async (src, dst) => {
    const grip = await boxOf(`.bento > .w[data-widget="${src}"] .grip`);
    const sbox = await boxOf(`.bento > .w[data-widget="${src}"]`);
    const dbox = await boxOf(`.bento > .w[data-widget="${dst}"]`);
    if (!sbox || !dbox) throw new Error(`missing tile ${src}/${dst}`);
    const from = grip ? { x: grip.x + grip.width / 2, y: grip.y + grip.height / 2 } : { x: sbox.x + 20, y: sbox.y + 14 };
    const to = { x: dbox.x + dbox.width / 2, y: dbox.y + dbox.height / 2 };

    const scrollStart = await scrollOf();
    let scrollMax = scrollStart, scrollMin = scrollStart;
    await page.mouse.move(from.x, from.y);
    await page.mouse.down();
    for (let i = 1; i <= 24; i++) {
      await page.mouse.move(from.x + (to.x - from.x) * (i / 24), from.y + (to.y - from.y) * (i / 24));
      await page.waitForTimeout(12);
      const s = await scrollOf();
      scrollMax = Math.max(scrollMax, s);
      scrollMin = Math.min(scrollMin, s);
    }
    // Hold over the target a moment, then release.
    for (let i = 0; i < 6; i++) { await page.mouse.move(to.x, to.y); await page.waitForTimeout(20); }
    await page.mouse.up();
    // The drop reorders the layout, which re-renders the data-heavy dashboard once
    // (and runs the FLIP settle). Wait for that to fully settle before the next drag so
    // we read stable tile geometry — a real user doesn't start a new drag mid-reflow.
    await page.waitForTimeout(900);
    return { scrollDelta: scrollMax - scrollMin };
  };

  // Upper, on-screen tiles so no scrolling is needed mid-drag.
  const pairs = [
    ["kpi-safetospend", "kpi-assets"],
    ["recent", "kpi-assets"],
    ["trend", "kpi-safetospend"],
  ];

  // Per-drag we assert the STRESS properties (no stuck dim/cursor, no scroll bounce, no
  // errors). We do NOT assert a per-pair visual move, because the packer can mask a
  // sequence reorder when a wide tile shifts a row (a 2-wide tile moved before a 1-wide
  // tile wraps to the next row while the 1-wide fills the gap — the sequence changed but
  // the packed order didn't). The sibling reorder test verifies move correctness; here we
  // assert the overall grid changed across the three drags, proving they did real work.
  const initial = await order(page);
  for (const [src, dst] of pairs) {
    const { scrollDelta } = await realDrag(src, dst);
    const st = await stuckState(page, src);
    if (st.dim) fail(`tile ${src} STUCK dimmed after drag ${src}->${dst}`);
    if (st.dragging != null) fail(`[data-bento-dragging] STUCK on <html> after drag ${src}->${dst}`);
    if (st.cursor === "grabbing") fail(`grid cursor STUCK grabbing after drag ${src}->${dst}`);
    if (scrollDelta > 12) fail(`scroll container bounced ${scrollDelta}px during drag ${src}->${dst}`);
  }
  const finalOrder = await order(page);
  if (initial.join(",") === finalOrder.join(",")) fail(`3 drags changed nothing: ${finalOrder.join(",")}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: 3 real back-to-back grip drags reordered the grid and left no stuck dim/cursor or scroll bounce.");
} finally {
  await browser.close();
}
