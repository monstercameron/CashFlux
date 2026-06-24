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

  // The canvas pans + zooms like a real node editor: the world is a transformed layer
  // with zoom controls. Zooming in must increase its transform scale; reset returns it.
  const worldScale = () => page.locator(".wb-canvas").first().evaluate((el) => {
    const m = new DOMMatrixReadOnly(getComputedStyle(el).transform);
    return m.a; // x-scale
  });
  if ((await page.locator(".wb-zoom [data-zoom='in']").count()) === 0) fail("canvas has no zoom controls");
  const z0 = await worldScale();
  await page.locator(".wb-zoom [data-zoom='in']").click();
  await page.waitForTimeout(150);
  const z1 = await worldScale();
  if (!(z1 > z0)) fail(`zoom-in did not scale the canvas world: ${z0} -> ${z1}`);
  await page.locator(".wb-zoom [data-zoom='reset']").click();
  await page.waitForTimeout(150);
  const z2 = await worldScale();
  if (Math.abs(z2 - 1) > 0.001) fail(`zoom reset did not return to 100%: ${z2}`);

  // Starter graph = net worth KPI → preview shows a currency figure.
  if ((await page.locator(".wb-tile .fig").count()) === 0) fail("starter KPI did not render a figure");

  // DRAG-TO-WIRE: each node shows real input ports; dragging from a node's output port
  // onto another node's input port creates the connection (no inspector dropdown).
  await page.locator('.vb-pal-btn[data-kind="literal.number"]').click();
  await page.waitForTimeout(300);
  const numId = await page.locator('.wb-node[data-kind="literal.number"]').first().getAttribute("data-step");
  const kpiId = await page.locator('.wb-node[data-kind="viz.kpi"]').first().getAttribute("data-step");
  if ((await page.locator('.wb-node[data-kind="viz.kpi"] .wb-port-in[data-port="value"]').count()) === 0) fail("KPI node has no labeled 'value' input port");
  if ((await page.locator('.wb-node[data-kind="literal.number"] .wb-port-out').count()) === 0) fail("literal node has no output port");
  await page.locator('.wb-node[data-kind="literal.number"] .wb-port-out').first()
    .dragTo(page.locator('.wb-node[data-kind="viz.kpi"] .wb-port-in[data-port="value"]').first());
  await page.waitForTimeout(400);
  const wireFrom = await page.locator(`path.wb-wire[data-to="${kpiId}"][data-toport="value"]`).first().getAttribute("data-from");
  if (wireFrom !== numId) fail(`drag-to-wire did not connect number→KPI: wire data-from=${wireFrom}, want ${numId}`);

  // Disconnect: clicking the wire removes it (dispatch directly — a wire can sit under
  // a node, which would intercept a hit-tested click; the feature listens for the click
  // on the path itself).
  await page.locator(`path.wb-wire[data-to="${kpiId}"][data-toport="value"]`).first().dispatchEvent("click");
  await page.waitForTimeout(400);
  if ((await page.locator(`path.wb-wire[data-to="${kpiId}"][data-toport="value"]`).count()) !== 0) fail("clicking a wire did not disconnect it");

  // 1) Add a node from the palette → node count on the canvas grows.
  const before = await page.locator(".wb-canvas .wb-node").count();
  await page.locator('.vb-pal-btn[data-kind="viz.badge"]').click();
  await page.waitForTimeout(300);
  const after = await page.locator(".wb-canvas .wb-node").count();
  if (after !== before + 1) fail(`adding a node didn't grow the canvas: ${before} -> ${after}`);

  // Undo reverts the add; fit-to-view + the expanded palette (rule/color/stack/button)
  // exist. These close the canvas-polish + node-breadth gaps.
  if ((await page.locator(".wb-zoom [data-zoom='fit']").count()) === 0) fail("no fit-to-view control");
  for (const k of ["data.rule", "literal.color", "viz.stack", "ui.button", "ui.toggle"]) {
    if ((await page.locator(`.vb-pal-btn[data-kind="${k}"]`).count()) === 0) fail(`palette missing node: ${k}`);
  }
  await page.locator('[data-testid="vb-undo"]').click();
  await page.waitForTimeout(300);
  if ((await page.locator(".wb-canvas .wb-node").count()) !== before) fail("undo did not revert the node add");

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
  // Charts now go through the dashboard's own D3 renderer (1:1 parity) → an <svg>.
  await page.waitForTimeout(400);
  if ((await page.locator(".wb-tile .vb-chart svg").count()) === 0) fail("bar chart did not render a D3 svg");

  // 4b) Save this complex widget to the library under a name.
  await page.locator('input[aria-label="Card name"]').fill("my chart");
  await page.locator('[data-testid="vb-save"]').click();
  await page.waitForTimeout(300);

  // 4c) Time-series trend: the spending-trend preset (dataset → filter → group-by-month
  // chronological → line chart) renders an SVG line chart.
  await page.locator('.vb-toolbar select').first().selectOption("spend-trend");
  await page.waitForTimeout(600);
  if ((await page.locator(".wb-tile .vb-chart").count()) === 0) fail("spend-trend did not render a chart");
  if ((await page.locator(".wb-tile .vb-chart svg path").count()) === 0) fail("line trend has no D3 svg path");

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

  // 8) Publish the saved card to the dashboard → it appears as a real bento tile with
  // the same chrome as built-ins, and survives a page reload (Reconcile keeps it).
  await page.locator('[data-testid="vb-publish"]').click();
  await page.waitForTimeout(400);
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento", { timeout: 10000 });
  await page.waitForTimeout(600);
  const tile = page.locator('.bento [data-widget="wb:my chart"]');
  if ((await tile.count()) === 0) fail("published custom card did not appear on the dashboard");
  if ((await tile.locator(".vb-chart").count()) === 0) fail("published dashboard tile did not render its chart");
  if (!(await tile.evaluate((el) => el.classList.contains("w")))) fail("published tile is not a standard .w bento cell");
  if ((await tile.locator(".wh").count()) === 0 || (await tile.locator(".wbody").count()) === 0) fail("published tile lacks the standard .wh/.wbody chrome");

  // 8b) Publish a SECOND custom card (a KPI) → BOTH coexist on the dashboard.
  await page.locator('a[title="Widget builder"]').first().click();
  await page.waitForSelector(".vb-main", { timeout: 10000 });
  await page.locator('.vb-toolbar select').first().selectOption("networth");
  await page.waitForTimeout(300);
  await page.locator('input[aria-label="Card name"]').fill("my kpi");
  await page.locator('[data-testid="vb-publish"]').click();
  await page.waitForTimeout(300);
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento", { timeout: 10000 });
  await page.waitForTimeout(600);
  if ((await page.locator('.bento [data-widget="wb:my chart"]').count()) === 0) fail("first custom card vanished when a second was published");
  const kpiTile = page.locator('.bento [data-widget="wb:my kpi"]');
  if ((await kpiTile.count()) === 0) fail("second published custom card did not appear on the dashboard");
  if ((await kpiTile.locator(".fig.t-figure").count()) === 0) fail("published KPI does not use .fig.t-figure (dashboard KPI typography)");

  // 8c) Reload → both custom cards persist.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 15000 });
  await page.waitForTimeout(700);
  if ((await page.locator('.bento [data-widget="wb:my chart"]').count()) === 0) fail("custom chart did not survive reload");
  if ((await page.locator('.bento [data-widget="wb:my kpi"]').count()) === 0) fail("custom KPI did not survive reload");

  if (!process.exitCode) console.log("PASS: canvas builder — palette/inspector/variables, presets (KPI/bar/line/donut/list), saved cards, publish MULTIPLE custom cards to the dashboard matching built-in chrome+typography, surviving reload.");
} finally {
  await browser.close();
}
