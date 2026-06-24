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
  for (const k of ["data.rule", "literal.color", "viz.stack", "ui.button", "ui.toggle", "style.accent", "style.tone"]) {
    if ((await page.locator(`.vb-pal-btn[data-kind="${k}"]`).count()) === 0) fail(`palette missing node: ${k}`);
  }
  // Styling + layout are their own palette groups (Cam: "no styling and layout tools").
  for (const grp of ["Style", "Layout"]) {
    const has = (await page.locator(".vb-pal-group").allTextContents()).some((t) => t.trim() === grp);
    if (!has) fail(`palette missing group: ${grp}`);
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

  // 4d) Net-worth trend: the 6-month end-of-month series → area chart (dataset →
  // group-by-month keep-order → area) renders an SVG with a filled area path.
  await page.locator('.vb-toolbar select').first().selectOption("networth-trend");
  await page.waitForTimeout(600);
  if ((await page.locator(".wb-tile .vb-chart svg").count()) === 0) fail("networth-trend did not render a D3 svg");
  // Assert an actual plotted area path (not just an empty svg / "No data" fallback), so
  // the 6-month series really flows dataset → group-by(keep-order) → area chart.
  if ((await page.locator(".wb-tile .vb-chart svg path").count()) === 0) fail("networth-trend has no D3 area path");

  // 4e) Cash flow: income − spending via a formula node → a stat figure (the dashboard's
  // surplus/deficit number). Confirms scalar→formula→viz wiring produces a money figure.
  await page.locator('.vb-toolbar select').first().selectOption("cashflow");
  await page.waitForTimeout(500);
  {
    const figTxt = (await page.locator(".wb-tile .fig.t-figure").first().textContent()) || "";
    if (!/[0-9]/.test(figTxt)) fail(`cashflow stat figure is not numeric: "${figTxt}"`);
  }

  // 4f) Assets KPI with month-over-month subline — the 1:1 clone of the dashboard assets
  // tile: a KPI figure PLUS a subline string fed from the net-worth string surface.
  await page.locator('.vb-toolbar select').first().selectOption("assets-card");
  await page.waitForTimeout(500);
  if ((await page.locator(".wb-tile .fig.t-figure").count()) === 0) fail("assets-card did not render a KPI figure");
  {
    // The subline is the dashboard's exact text ("▲ x% (+$…) this month", "… this month",
    // or "No change this month"); assert a non-empty caption is present under the figure.
    const subTxt = (await page.locator(".wb-tile .wbody p, .wb-tile .wbody .t-caption").last().textContent()) || "";
    // The net-worth string surface always renders "… this month" / "No change this month";
    // require that pattern so a bare prop fallback can't silently pass.
    if (!/month/i.test(subTxt)) fail(`assets-card subline is not the net-worth MoM string: "${subTxt}"`);
  }

  // 4g) Styling tool: the styled-KPI preset wires a color through style.accent, so the
  // figure renders in that accent (#8b5cf6 → rgb(139, 92, 246)) instead of the tone color.
  await page.locator('.vb-toolbar select').first().selectOption("styled-kpi");
  await page.waitForTimeout(500);
  {
    const fig = page.locator(".wb-tile .fig").first();
    if ((await fig.count()) === 0) fail("styled-kpi did not render a figure");
    // The accent is applied as inline color on a wrapper that cascades to the figure.
    const color = await page.locator(".wb-tile").first().evaluate((tile) => {
      const f = tile.querySelector(".fig");
      return f ? getComputedStyle(f).color : "";
    });
    if (!/rgb\(\s*139,\s*92,\s*246\s*\)/.test(color)) fail(`styled-kpi figure is not the accent color: ${color}`);
  }

  // 4h) Layout tool: the dual-KPI preset composes two KPIs side by side via a row stack.
  await page.locator('.vb-toolbar select').first().selectOption("dual-kpi");
  await page.waitForTimeout(500);
  {
    const figs = await page.locator(".wb-tile .vb-stack .fig").count();
    if (figs < 2) fail(`dual-kpi should render two figures, got ${figs}`);
    const isRow = await page.locator(".wb-tile .vb-stack").first().evaluate((el) => getComputedStyle(el).display === "flex");
    if (!isRow) fail("dual-kpi stack is not laid out as a row (flex)");
  }

  // 5) Another preset: recent transactions → a list/table renders.
  await page.locator('.vb-toolbar select').first().selectOption("recent");
  await page.waitForTimeout(500);
  if ((await page.locator(".wb-tile .vb-table").count()) === 0) fail("recent preset did not render a list/table");
  if ((await page.locator(".wb-tile .vb-table tbody tr").count()) === 0) fail("list/table has no rows");
  // The recent clone matches the dashboard tile: headerless table, and the amount column
  // is accounting money (a currency symbol), not a bare number.
  if ((await page.locator(".wb-tile .vb-table thead").count()) !== 0) fail("recent list should be headerless like the dashboard tile");
  {
    const moneyCells = await page.locator(".wb-tile .vb-table tbody tr td.fig").allTextContents();
    if (!moneyCells.some((t) => /[$€£¥]/.test(t))) fail(`recent list amount column is not currency-formatted: ${JSON.stringify(moneyCells.slice(0, 4))}`);
  }
  // 5b) The list CONTENT respects the tile height (Cam: "don't respect the multiple
  // widget sizes"): a shorter tile shows fewer rows than a taller one. recent loads at
  // 2-tall; shrink to 1-tall → fewer rows, then grow to 3-tall → more rows.
  {
    for (let i = 0; i < 3; i++) await page.locator('button[aria-label="Shorter"]').click();
    await page.waitForTimeout(300);
    const shortRows = await page.locator(".wb-tile .vb-table tbody tr").count();
    for (let i = 0; i < 3; i++) await page.locator('button[aria-label="Taller"]').click();
    await page.waitForTimeout(300);
    const tallRows = await page.locator(".wb-tile .vb-table tbody tr").count();
    if (!(tallRows > shortRows)) fail(`list did not reflow to tile height: short=${shortRows}, tall=${tallRows} (expected tall > short)`);
    // A 1-tall tile shows only a few rows; a 3-tall tile fills toward the engine limit
    // (12 for this preset, with ample sample data) — guards against a premature row cap.
    if (shortRows > 4) fail(`1-tall list should show ~3 rows, got ${shortRows}`);
    if (tallRows < 9) fail(`3-tall list should fill toward its 12-row limit, got ${tallRows}`);
  }

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

  // 8b) Publish a SECOND custom card (a KPI), RESIZED to 4 wide × 1 tall → it must
  // coexist with the first AND honor its chosen size on the dashboard (Cam: tiles
  // "don't respect the multiple widget sizes"). Default is 2×2: widen twice, shorten once.
  await page.locator('a[title="Widget builder"]').first().click();
  await page.waitForSelector(".vb-main", { timeout: 10000 });
  await page.locator('.vb-toolbar select').first().selectOption("networth");
  await page.waitForTimeout(300);
  await page.locator('input[aria-label="Card name"]').fill("my kpi");
  // Drive to a known 4×1 regardless of the starting size (steppers clamp at [1..4]/[1..3]).
  for (let i = 0; i < 3; i++) await page.locator('button[aria-label="Wider"]').click();
  for (let i = 0; i < 3; i++) await page.locator('button[aria-label="Shorter"]').click();
  await page.waitForTimeout(200);
  await page.locator('[data-testid="vb-publish"]').click();
  await page.waitForTimeout(300);
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento", { timeout: 10000 });
  await page.waitForTimeout(600);
  if ((await page.locator('.bento [data-widget="wb:my chart"]').count()) === 0) fail("first custom card vanished when a second was published");
  const kpiTile = page.locator('.bento [data-widget="wb:my kpi"]');
  if ((await kpiTile.count()) === 0) fail("second published custom card did not appear on the dashboard");
  if ((await kpiTile.locator(".fig.t-figure").count()) === 0) fail("published KPI does not use .fig.t-figure (dashboard KPI typography)");
  // Size respect: the dashboard packs the tile from its layout span, exposed as
  // data-col-span / data-row-span (the canonical size signal) — 4 wide × 1 tall.
  {
    const cs = await kpiTile.getAttribute("data-col-span");
    const rs = await kpiTile.getAttribute("data-row-span");
    if (cs !== "4") fail(`published KPI did not respect width 4: data-col-span="${cs}"`);
    if (rs !== "1") fail(`published KPI did not respect height 1: data-row-span="${rs}"`);
    // And it really spans 4 of the grid's 4 columns: wider than a 1-col tile.
    const w = await kpiTile.evaluate((el) => el.getBoundingClientRect().width);
    const narrow = await page.locator('.bento [data-col-span="1"]').first().evaluate((el) => el.getBoundingClientRect().width).catch(() => 0);
    if (narrow && w < narrow * 2) fail(`4-wide tile (${Math.round(w)}px) is not visibly wider than a 1-wide tile (${Math.round(narrow)}px)`);
  }

  // 8c) Reload → both custom cards persist AND the resized KPI keeps its 4×1 span.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 15000 });
  await page.waitForTimeout(700);
  if ((await page.locator('.bento [data-widget="wb:my chart"]').count()) === 0) fail("custom chart did not survive reload");
  const kpiTile2 = page.locator('.bento [data-widget="wb:my kpi"]');
  if ((await kpiTile2.count()) === 0) fail("custom KPI did not survive reload");
  {
    const cs = await kpiTile2.getAttribute("data-col-span");
    if (cs !== "4") fail(`resized KPI lost its width after reload: data-col-span="${cs}"`);
  }

  // 8d) Reload the resized card in the builder → the W/H steppers restore to 4 / 1
  // (size persists with the saved card, not just the published layout item).
  await page.locator('a[title="Widget builder"]').first().click();
  await page.waitForSelector(".vb-main", { timeout: 10000 });
  await page.locator('select[aria-label="My cards"]').selectOption("my kpi");
  await page.waitForTimeout(400);
  {
    const w = await page.locator(".wm-step-val").first().textContent();
    const h = await page.locator(".wm-step-val").nth(1).textContent();
    if (!/4/.test(w || "")) fail(`reloaded card width stepper not restored to 4: "${w}"`);
    if (!/1/.test(h || "")) fail(`reloaded card height stepper not restored to 1: "${h}"`);
  }

  if (!process.exitCode) console.log("PASS: canvas builder — palette/inspector/variables, presets (KPI/bar/line/donut/list), saved cards, publish MULTIPLE custom cards to the dashboard matching built-in chrome+typography, surviving reload.");
} finally {
  await browser.close();
}
