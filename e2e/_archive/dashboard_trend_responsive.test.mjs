// Verifies the net-worth trend widget responds across every dashboard cell size.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import { mkdirSync } from "fs";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
mkdirSync(path.join(__dirname, ".artifacts", "trend-matrix"), { recursive: true });

const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const defaultLayout = [
  { ID: "kpi-networth", ColSpan: 1, RowSpan: 1 },
  { ID: "kpi-income", ColSpan: 1, RowSpan: 1 },
  { ID: "kpi-spending", ColSpan: 1, RowSpan: 1 },
  { ID: "kpi-liabilities", ColSpan: 1, RowSpan: 1 },
  { ID: "recent", ColSpan: 2, RowSpan: 2 },
  { ID: "budgets", ColSpan: 1, RowSpan: 2 },
  { ID: "trend", ColSpan: 1, RowSpan: 2 },
  { ID: "goals", ColSpan: 1, RowSpan: 1 },
  { ID: "todo", ColSpan: 1, RowSpan: 1 },
  { ID: "accounts", ColSpan: 2, RowSpan: 1 },
  { ID: "cashflow", ColSpan: 2, RowSpan: 1 },
  { ID: "bills", ColSpan: 2, RowSpan: 1 },
  { ID: "savings", ColSpan: 2, RowSpan: 1 },
  { ID: "breakdown", ColSpan: 2, RowSpan: 1 },
  { ID: "freshness", ColSpan: 4, RowSpan: 1 },
];

async function setTrendSpan(page, col, row) {
  await page.evaluate(
    ({ defaultLayout, col, row }) => {
      const layout = defaultLayout.map((item) =>
        item.ID === "trend" ? { ...item, ColSpan: col, RowSpan: row } : item,
      );
      localStorage.setItem("cashflux:layout", JSON.stringify(layout));
      localStorage.setItem("cashflux:layout-mode", "custom");
    },
    { defaultLayout, col, row },
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    ([col, row]) => {
      const el = document.querySelector('[data-widget="trend"]');
      return el && el.getAttribute("data-col-span") === String(col) && el.getAttribute("data-row-span") === String(row);
    },
    [col, row],
    { timeout: 60000 },
  );
  await page.waitForSelector('[data-widget="trend"] .trend-chart svg', { timeout: 60000 });
  await page.waitForTimeout(200);
}

async function setTrendConfig(page, cfg) {
  await page.evaluate((cfg) => {
    localStorage.setItem("cashflux:widget-config", JSON.stringify({ trend: cfg }));
  }, cfg);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-widget="trend"] .trend-chart svg', { timeout: 60000 });
  await page.waitForTimeout(200);
}

const inspectTrend = (page) =>
  page.locator('[data-widget="trend"]').evaluate((el) => {
    const tile = el.getBoundingClientRect();
    const body = el.querySelector(".wbody").getBoundingClientRect();
    const chart = el.querySelector(".trend-chart");
    const chartRect = chart.getBoundingClientRect();
    const svg = chart.querySelector("svg");
    const standard = el.querySelector(".trend-standard");
    const expanded = el.querySelector(".trend-expanded");
    const figure = el.querySelector(".trend-figure");
    const visibleTicks = [...chart.querySelectorAll(".tick")].filter((tick) => getComputedStyle(tick).display !== "none");
    const xTickText = [...chart.querySelectorAll(".x-axis .tick text")].map((tick) => tick.textContent.trim()).filter(Boolean);
    const paths = [...chart.querySelectorAll("path")].map((path) => path.getBBox()).map((box) => ({
      width: box.width,
      height: box.height,
    }));
    const parts = [...el.querySelectorAll(".trend-head,.trend-expanded,.trend-chart")].map((node) => {
      const rect = node.getBoundingClientRect();
      return {
        className: String(node.className),
        display: getComputedStyle(node).display,
        top: rect.top,
        bottom: rect.bottom,
        height: rect.height,
      };
    });
    return {
      col: Number(el.getAttribute("data-col-span")),
      row: Number(el.getAttribute("data-row-span")),
      tile: { top: tile.top, bottom: tile.bottom, width: tile.width, height: tile.height },
      body: { top: body.top, bottom: body.bottom, width: body.width, height: body.height },
      chart: { top: chartRect.top, bottom: chartRect.bottom, width: chartRect.width, height: chartRect.height },
      svg: { width: Number(svg.getAttribute("width")), height: Number(svg.getAttribute("height")) },
      standardDisplay: getComputedStyle(standard).display,
      expandedDisplay: getComputedStyle(expanded).display,
      figureSize: Number.parseFloat(getComputedStyle(figure).fontSize),
      visibleTicks: visibleTicks.length,
      xTickText,
      paths,
      parts,
      text: el.innerText,
      tileOverflow: el.scrollHeight > el.clientHeight + 1,
      bodyOverflow: el.querySelector(".trend-body").scrollHeight > el.querySelector(".trend-body").clientHeight + 1,
    };
  });

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

  const seen = [];
  for (const col of [1, 2, 3, 4]) {
    for (const row of [1, 2, 3]) {
      await setTrendSpan(page, col, row);
      const tile = page.locator('[data-widget="trend"]');
      await tile.screenshot({ path: path.join(__dirname, ".artifacts", "trend-matrix", `${col}x${row}.png`) });
      const info = await inspectTrend(page);
      seen.push(`${col}x${row}`);

      if (info.tileOverflow || info.bodyOverflow) fail(`${col}x${row} should not overflow: ${JSON.stringify(info)}`);
      if (Math.abs(info.svg.width - info.chart.width) > 2 || Math.abs(info.svg.height - info.chart.height) > 2) {
        fail(`${col}x${row} svg should match chart container: ${JSON.stringify(info)}`);
      }
      if (info.chart.height < 48 || info.chart.width < 120) fail(`${col}x${row} chart should remain useful: ${JSON.stringify(info)}`);
      if (!info.paths.some((box) => box.width > 20 && box.height > 0)) fail(`${col}x${row} chart should draw nonblank paths: ${JSON.stringify(info)}`);
      for (const part of info.parts) {
        if (part.display !== "none" && (part.top < info.body.top - 1 || part.bottom > info.body.bottom + 1)) {
          fail(`${col}x${row} content part clips outside body: ${JSON.stringify({ part, info })}`);
        }
      }

      if (row === 1) {
        if (info.visibleTicks !== 0) fail(`${col}x${row} compact row should not show axes: ${JSON.stringify(info)}`);
        if (info.expandedDisplay !== "none") fail(`${col}x${row} compact row should hide expanded stats: ${JSON.stringify(info)}`);
      }
      if (col === 1 && row === 1 && info.standardDisplay !== "none") {
        fail("1x1 should be sparkline + figure only: " + JSON.stringify(info));
      }
      if (col === 1 && row === 2 && (info.standardDisplay === "none" || info.expandedDisplay !== "none")) {
        fail("1x2 should use standard mode without expanded stats: " + JSON.stringify(info));
      }
      if (col === 1 && row === 3) {
        if (info.expandedDisplay !== "none") fail("1x3 should prioritize chart over stat cards: " + JSON.stringify(info));
        if (info.chart.bottom < info.body.bottom - 16) fail("1x3 chart should sit toward the bottom: " + JSON.stringify(info));
      }
      if (col >= 2 && row >= 2 && info.expandedDisplay === "none") {
        fail(`${col}x${row} should show expanded stats: ${JSON.stringify(info)}`);
      }
    }
  }

  await setTrendSpan(page, 4, 3);
  let info = await inspectTrend(page);
  if (info.xTickText.some((text) => /^\d+$/.test(text))) {
    fail("default trend x-axis should use date labels, not point indexes: " + JSON.stringify(info));
  }
  if (!info.xTickText.some((text) => /[A-Za-z]{3}|20\d{2}/.test(text))) {
    fail("default trend x-axis should include month/year labels: " + JSON.stringify(info));
  }

  await setTrendConfig(page, { months: "120", showXAxis: "true" });
  await setTrendSpan(page, 4, 3);
  info = await inspectTrend(page);
  if (!info.text.includes("10 years")) fail("120-month trend should label the history as 10 years: " + JSON.stringify(info));
  if (!info.xTickText.some((text) => /^20\d{2}$/.test(text) || /'\d{2}/.test(text))) {
    fail("10-year trend should show year-aware x labels: " + JSON.stringify(info));
  }

  await setTrendConfig(page, { months: "120", showXAxis: "false" });
  await setTrendSpan(page, 4, 3);
  info = await inspectTrend(page);
  if (info.xTickText.length !== 0) fail("showXAxis=false should hide time labels: " + JSON.stringify(info));

  if (seen.length !== 12) fail("did not cover all 12 trend sizes: " + seen.join(", "));
  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: net-worth trend responds across 1x1 through 4x3.");
} finally {
  await browser.close();
}
