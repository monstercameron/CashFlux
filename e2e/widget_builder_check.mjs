// Widget Builder (free-form node-graph editor): the canvas is the primary surface —
// add nodes from the palette, configure them in the inspector, load presets that
// reproduce dashboard widgets as graphs, and a live preview renders the evaluated card
// (KPI / chart / list / …). This drives it end-to-end. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  // Start clean so the starter graph (net worth KPI) is what loads.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.evaluate(() => { localStorage.removeItem("cashflux:wb-graph"); localStorage.removeItem("cashflux:wb-canvas-pos"); });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.locator('a[title="Widget builder"]').first().click();
  await page.waitForSelector(".vb-main", { timeout: 15000 });
  await page.waitForTimeout(500);

  // The canvas is the primary interface: palette + canvas + inspector + preview.
  if ((await page.locator(".vb-palette .vb-pal-btn").count()) < 8) fail("palette is missing node buttons");
  if ((await page.locator(".wb-canvas").count()) === 0) fail("no node canvas");
  if ((await page.locator(".vb-previewpane .wb-tile").count()) === 0) fail("no live preview tile");

  // Starter graph = net worth KPI → preview shows a currency figure.
  if ((await page.locator(".wb-tile .fig").count()) === 0) fail("starter KPI did not render a figure");

  // 1) Add a node from the palette → node count on the canvas grows.
  const before = await page.locator(".wb-canvas .wb-node").count();
  await page.locator('.vb-pal-btn[data-kind="viz.badge"]').click();
  await page.waitForTimeout(300);
  const after = await page.locator(".wb-canvas .wb-node").count();
  if (after !== before + 1) fail(`adding a node didn't grow the canvas: ${before} -> ${after}`);

  // 2) Selecting a node opens the inspector with its parameter fields.
  await page.locator(".wb-canvas .wb-node").first().click();
  await page.waitForTimeout(300);
  if ((await page.locator(".vb-inspector .wb-field").count()) === 0) fail("inspector shows no fields for the selected node");

  // 3) Name a node (variable) → the node box reflects the name.
  const nameField = page.locator(".vb-inspector .wb-field", { has: page.locator('input[aria-label="Name (variable)"]') }).first();
  await nameField.locator("input").fill("myvar");
  await page.waitForTimeout(300);
  const firstNodeText = await page.locator(".wb-canvas .wb-node").first().innerText();
  if (!/myvar/.test(firstNodeText)) fail("named variable not shown on the node: " + firstNodeText);

  // 4) Complex widget via preset: spending-by-category (dataset → filter → groupby →
  // chart). The preview must render a bar chart with multiple bars.
  await page.locator('.vb-toolbar select').first().selectOption("spend-by-cat");
  await page.waitForTimeout(500);
  if ((await page.locator(".wb-canvas .wb-node").count()) !== 4) fail("spend-by-cat preset should have 4 nodes");
  if ((await page.locator(".wb-tile .vb-chart").count()) === 0) fail("chart preset did not render a chart");
  if ((await page.locator(".wb-tile .vb-bar-col").count()) < 2) fail("bar chart should have multiple bars");

  // 4b) Save this complex widget to the library under a name.
  await page.locator('input[aria-label="Card name"]').fill("my chart");
  await page.locator('[data-testid="vb-save"]').click();
  await page.waitForTimeout(300);

  // 5) Another preset: recent transactions → a list/table renders.
  await page.locator('.vb-toolbar select').first().selectOption("recent");
  await page.waitForTimeout(500);
  if ((await page.locator(".wb-tile .vb-table").count()) === 0) fail("recent preset did not render a list/table");
  if ((await page.locator(".wb-tile .vb-table tbody tr").count()) === 0) fail("list/table has no rows");

  // 6) Wiring via the inspector: on the recent-list graph, select the dataset node and
  // confirm an input-source dropdown drives connections (the list node has an "in"
  // input listing the dataset as a candidate).
  await page.locator('.wb-node[data-kind="viz.list"]').first().click();
  await page.waitForTimeout(300);
  const inWire = page.locator(".vb-inspector .wb-field", { has: page.locator('select[aria-label="in ←"]') });
  if ((await inWire.count()) === 0) fail("list node has no 'in' input-source dropdown in the inspector");

  // 7) Reload the saved widget from "My cards" → the 4-node chart graph comes back.
  await page.locator('select[aria-label="My cards"]').selectOption("my chart");
  await page.waitForTimeout(400);
  if ((await page.locator(".wb-canvas .wb-node").count()) !== 4) fail("reloading the saved card did not restore its 4 nodes");
  if ((await page.locator(".wb-tile .vb-chart").count()) === 0) fail("reloaded saved card did not render its chart");

  if (!process.exitCode) console.log("PASS: canvas-first builder — palette/inspector/variables, presets render KPI/chart/list, and saved cards round-trip.");
} finally {
  await browser.close();
}
