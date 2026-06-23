// Widget Builder (visual programming system): the stage renders a LIVE card built
// from a node graph (source → optional formula → KPI) evaluated against the real
// app figures. This drives the builder end-to-end: it changes the data source, the
// transform formula, and the visualization format, and asserts the previewed figure
// updates accordingly; it also checks the pipeline nodes mirror the graph and that
// the size stepper resizes the preview tile. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const figNum = (s) => Number(String(s).replace(/[^0-9.-]/g, ""));

try {
  const page = await (await browser.newContext()).newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  // Boot on the dashboard, then open the Widget builder via the rail (deep-link
  // refreshes 404 on the dev server, so navigate in-app like the manager check).
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.locator('a[title="Widget builder"]').first().click();

  await page.waitForSelector(".wb-stage", { timeout: 15000 });
  await page.waitForSelector(".wb-canvas", { timeout: 10000 });
  await page.waitForTimeout(500);

  const stageText = () => page.locator(".wb-tile").first().innerText();
  const figText = () => page.locator(".wb-tile .fig").first().innerText();

  // 0) The stage shows a live figure, not the old placeholder or an error.
  if ((await page.locator(".wb-tile .fig").count()) === 0) fail("stage has no live figure (.fig)");
  if ((await stageText()).includes("$12,480")) fail("stage still shows the hardcoded placeholder");
  if ((await stageText()).toLowerCase().includes("isn't finished")) fail("stage stuck on the unfinished state");

  // 0b) It's a real 2D node canvas: three absolutely-positioned node boxes joined by
  // SVG bezier wires (not a flat strip of cards).
  const nodeCount = await page.locator(".wb-canvas .wb-node").count();
  if (nodeCount !== 3) fail(`expected 3 canvas nodes, got ${nodeCount}`);
  const wireCount = await page.locator(".wb-canvas svg.wb-wires path.wb-wire").count();
  if (wireCount < 2) fail(`expected >=2 bezier wires between nodes, got ${wireCount}`);
  const firstWireD = await page.locator("path.wb-wire").first().getAttribute("d");
  if (!firstWireD || !firstWireD.includes("C")) fail("wires are not bezier curves (no 'C' in path d): " + firstWireD);
  const srcPos = await page.locator(".wb-node").filter({ hasText: "Data source" }).first().evaluate((el) => ({
    position: getComputedStyle(el).position, left: el.style.left,
  }));
  if (srcPos.position !== "absolute") fail("nodes are not absolutely positioned on a canvas: " + srcPos.position);

  // 1) Source: select the "transactions" figure (a clean integer count). Read it.
  await page.locator(".wb-node").filter({ hasText: "Data source" }).first().click();
  await page.waitForSelector(".wb-config select", { timeout: 5000 });
  await page.locator(".wb-config select").first().selectOption("transactions");
  await page.waitForTimeout(400);
  const n0 = figNum(await figText());
  if (!Number.isFinite(n0)) fail("source figure did not render a number: " + (await figText()));

  // The source pipeline node now summarizes the chosen figure.
  const srcNodeVal = await page.locator(".wb-node").filter({ hasText: "Data source" }).first().locator(".wb-node-val").innerText();
  if (!/transactions/i.test(srcNodeVal)) fail("source node doesn't reflect the chosen figure, got: " + srcNodeVal);

  // 2) Transform: apply "a / 2" and confirm the previewed figure halves. This proves
  // the formula engine runs inside the builder over the live source value.
  await page.locator(".wb-node").filter({ hasText: "Transform" }).first().click();
  await page.waitForSelector(".wb-config input", { timeout: 5000 });
  await page.locator(".wb-config input").first().fill("a / 2");
  await page.waitForTimeout(400);
  const n1 = figNum(await figText());
  if (Math.abs(n1 - n0 / 2) > 0.01) fail(`transform a/2 did not halve the figure: ${n0} -> ${n1} (want ${n0 / 2})`);

  // The transform node reflects the expression.
  const xfNodeVal = await page.locator(".wb-node").filter({ hasText: "Transform" }).first().locator(".wb-node-val").innerText();
  if (!/a\s*\/\s*2/.test(xfNodeVal)) fail("transform node doesn't reflect the formula, got: " + xfNodeVal);

  // 3) Visualize: switch to currency format → the figure gains a currency symbol.
  await page.locator(".wb-node").filter({ hasText: "Visualize" }).first().click();
  await page.waitForSelector(".wb-config select", { timeout: 5000 });
  await page.locator(".wb-config select").first().selectOption("currency");
  await page.waitForTimeout(400);
  const cur = await figText();
  if (!/[$€£¥]/.test(cur)) fail("currency format did not render a money symbol, got: " + cur);

  // 4) Size: the W stepper grows the preview tile's rendered width.
  const tileW = () => page.evaluate(() => Math.round(document.querySelector(".wb-tile").getBoundingClientRect().width));
  const w0 = await tileW();
  await page.locator('.wb-size button[aria-label="Wider"]').click();
  await page.waitForTimeout(300);
  const w1 = await tileW();
  if (!(w1 > w0)) fail(`size stepper did not widen the tile: ${w0}px -> ${w1}px`);

  // 5) Drag a node across the canvas → its position (style.left) changes. Proves the
  // boxes are freely draggable like an n8n canvas, not fixed in a strip.
  const srcNode = page.locator(".wb-node").filter({ hasText: "Data source" }).first();
  const leftBefore = await srcNode.evaluate((el) => parseFloat(el.style.left) || 0);
  await srcNode.dragTo(page.locator(".wb-canvas"), { targetPosition: { x: 250, y: 280 } });
  await page.waitForTimeout(400);
  const leftAfter = await srcNode.evaluate((el) => parseFloat(el.style.left) || 0);
  if (Math.abs(leftAfter - leftBefore) < 10) fail(`dragging the source node did not move it: left ${leftBefore} -> ${leftAfter}`);

  if (!process.exitCode) console.log("PASS: n8n canvas (3 draggable nodes + bezier wires); live graph-driven card; source/transform/format/size and node drag all work.");
} finally {
  await browser.close();
}
