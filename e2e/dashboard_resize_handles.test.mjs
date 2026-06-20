// Verifies the dashboard resize UX: hover/focus directional handles, direct
// grow/shrink actions, and persisted layout state.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { mkdirSync } from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
mkdirSync(path.join(__dirname, ".artifacts"), { recursive: true });

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const savedSpan = (page, id) =>
  page.evaluate((widgetID) => {
    const items = JSON.parse(localStorage.getItem("cashflux:layout") || "[]");
    const item = items.find((it) => (it.ID || it.id) === widgetID);
    return item ? { col: item.ColSpan ?? item.colSpan, row: item.RowSpan ?? item.rowSpan } : null;
  }, id);

const handleState = (locator) =>
  locator.evaluate((el) => {
    const style = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    return {
      disabled: el.disabled,
      opacity: Number.parseFloat(style.opacity),
      pointerEvents: style.pointerEvents,
      width: rect.width,
      height: rect.height,
    };
  });

const effectivelyHidden = (state) => state.opacity <= 0.02 && state.pointerEvents === "none";

try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.evaluate(async () => {
    if ("serviceWorker" in navigator) {
      const regs = await navigator.serviceWorker.getRegistrations();
      await Promise.all(regs.map((reg) => reg.unregister()));
    }
    if ("caches" in window) {
      const keys = await caches.keys();
      await Promise.all(keys.map((key) => caches.delete(key)));
    }
  });
  await page.evaluate(() => {
    localStorage.removeItem("cashflux:layout");
    localStorage.removeItem("cashflux:layout-mode");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-widget="kpi-income"] .rz[data-dir="r"]', { timeout: 60000 });

  const tile = page.locator('[data-widget="kpi-income"]');
  await tile.hover();
  const wider = tile.locator('.rz[data-dir="r"]');
  const narrower = tile.locator('.rz[data-dir="l"]');
  const taller = tile.locator('.rz[data-dir="b"]');
  const shorter = tile.locator('.rz[data-dir="t"]');

  const minLeft = await handleState(narrower);
  const minTop = await handleState(shorter);
  if (!minLeft.disabled || !effectivelyHidden(minLeft)) {
    fail(`min-width left handle should be disabled and hidden: ${JSON.stringify(minLeft)}`);
  }
  if (!minTop.disabled || !effectivelyHidden(minTop)) {
    fail(`min-height top handle should be disabled and hidden: ${JSON.stringify(minTop)}`);
  }

  await page.waitForFunction(
    (el) => Number.parseFloat(getComputedStyle(el).opacity) > 0.5,
    await wider.elementHandle(),
    { timeout: 1000 },
  );
  const visibleOpacity = await wider.evaluate((el) => Number.parseFloat(getComputedStyle(el).opacity));
  if (!(visibleOpacity > 0.5)) fail(`wider handle should reveal on hover; opacity=${visibleOpacity}`);
  const visual = await wider.evaluate((el) => {
    const box = el.getBoundingClientRect();
    const before = getComputedStyle(el, "::before");
    const after = getComputedStyle(el, "::after");
    return {
      width: box.width,
      height: box.height,
      beforeBackground: before.backgroundColor,
      beforeShadow: before.boxShadow,
      afterBorderLeft: after.borderLeftColor,
    };
  });
  if (visual.width < 14 || visual.height < 100) fail(`wider handle visual target too small: ${JSON.stringify(visual)}`);
  if (visual.beforeBackground === "rgba(0, 0, 0, 0)" || visual.beforeBackground === "transparent") {
    fail(`wider handle rail should be visibly painted: ${JSON.stringify(visual)}`);
  }
  await tile.screenshot({ path: "e2e/.artifacts/dashboard-resize-hover.png" });

  await wider.click();
  await page.waitForTimeout(250);
  let span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 2 || span.row !== 1) fail(`wider should persist 2x1, got ${JSON.stringify(span)}`);

  await tile.hover();
  await narrower.click();
  await page.waitForTimeout(250);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 1 || span.row !== 1) fail(`narrower should persist 1x1, got ${JSON.stringify(span)}`);

  await tile.hover();
  await taller.click();
  await page.waitForTimeout(250);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 1 || span.row !== 2) fail(`taller should persist 1x2, got ${JSON.stringify(span)}`);

  await tile.hover();
  await taller.click();
  await page.waitForTimeout(250);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 1 || span.row !== 3) fail(`second taller should persist 1x3, got ${JSON.stringify(span)}`);
  const maxBottom = await handleState(taller);
  if (!maxBottom.disabled || !effectivelyHidden(maxBottom)) {
    fail(`max-height bottom handle should be disabled and hidden: ${JSON.stringify(maxBottom)}`);
  }
  const tallRect = await tile.evaluate((el) => {
    const rect = el.getBoundingClientRect();
    return { width: rect.width, height: rect.height, gridRow: getComputedStyle(el).gridRow };
  });
  if (tallRect.height < 440 || !tallRect.gridRow.includes("span 3")) {
    fail(`3-row tile should render tall, got ${JSON.stringify(tallRect)}`);
  }
  await tile.screenshot({ path: "e2e/.artifacts/dashboard-resize-3row.png" });

  await tile.hover();
  await shorter.click();
  await page.waitForTimeout(250);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 1 || span.row !== 2) fail(`shorter should persist 1x2, got ${JSON.stringify(span)}`);

  await tile.hover();
  await shorter.click();
  await page.waitForTimeout(250);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 1 || span.row !== 1) fail(`second shorter should persist 1x1, got ${JSON.stringify(span)}`);

  for (let i = 0; i < 3; i++) {
    await tile.hover();
    await wider.click();
    await page.waitForTimeout(200);
  }
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 4 || span.row !== 1) fail(`wider should clamp at 4x1, got ${JSON.stringify(span)}`);
  const maxRight = await handleState(wider);
  if (!maxRight.disabled || !effectivelyHidden(maxRight)) {
    fail(`max-width right handle should be disabled and hidden: ${JSON.stringify(maxRight)}`);
  }

  await tile.focus();
  await page.keyboard.press("Shift+ArrowDown");
  await page.waitForTimeout(200);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 4 || span.row !== 2) fail(`Shift+ArrowDown should resize to 4x2, got ${JSON.stringify(span)}`);
  await page.keyboard.press("Shift+ArrowLeft");
  await page.waitForTimeout(200);
  span = await savedSpan(page, "kpi-income");
  if (!span || span.col !== 3 || span.row !== 2) fail(`Shift+ArrowLeft should resize to 3x2, got ${JSON.stringify(span)}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: dashboard hover resize handles grow and shrink directly.");
} finally {
  await browser.close();
}
