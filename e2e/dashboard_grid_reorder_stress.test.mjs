// Stress test for B2 dashboard bento reordering.
//
// Runs three visible back-to-back drags and checks two UX failure modes:
// 1) flicker/oscillation while the pointer is held over the final target
// 2) scroll-container / viewport bounce while the grid is reflowing
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

const orderFn = () =>
  [...document.querySelectorAll(".bento > .w[data-widget]")]
    .map((el) => {
      const cs = getComputedStyle(el);
      return {
        id: el.dataset.widget,
        col: Number.parseInt(cs.gridColumnStart, 10),
        row: Number.parseInt(cs.gridRowStart, 10),
      };
    })
    .sort((a, b) => a.row - b.row || a.col - b.col)
    .map((p) => p.id);

const key = (sample) => sample.order.slice(0, 8).join(">");
const transitions = (samples) => {
  const keys = samples.map(key);
  return keys.filter((v, i) => i === 0 || keys[i - 1] !== v);
};
const hasABA = (ts) => {
  for (let i = 0; i + 2 < ts.length; i++) {
    if (ts[i] === ts[i + 2] && ts[i] !== ts[i + 1]) return true;
  }
  return false;
};

try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.evaluate(() => {
    localStorage.removeItem("cashflux:layout");
    localStorage.removeItem("cashflux:layout-mode");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-widget="todo"]', { timeout: 60000 });
  await page.evaluate(() => {
    const main = document.querySelector("main.cf-scroll");
    if (main) main.scrollTop = 180;
  });
  await page.waitForTimeout(300);

  const sample = () =>
    page.evaluate((src) => {
      const order = new Function("return (" + src + ")")();
      const main = document.querySelector("main.cf-scroll");
      const b = document.querySelector(".bento").getBoundingClientRect();
      return { order: order(), mainTop: main ? main.scrollTop : null, bentoTop: Math.round(b.top) };
    }, orderFn.toString());

  const center = async (id) => {
    const b = await page.locator(`[data-widget="${id}"]`).boundingBox();
    if (!b) throw new Error("missing widget " + id);
    return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
  };

  const dragOnce = async (src, dst) => {
    const from = await center(src);
    const to = await center(dst);
    const travel = [await sample()];
    const hold = [];

    await page.mouse.move(from.x, from.y);
    await page.mouse.down();
    for (let i = 1; i <= 42; i++) {
      const t = i / 42;
      const ease = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
      await page.mouse.move(from.x + (to.x - from.x) * ease, from.y + (to.y - from.y) * ease);
      await page.waitForTimeout(18);
      travel.push(await sample());
    }
    for (let i = 0; i < 18; i++) {
      await page.mouse.move(to.x, to.y);
      await page.waitForTimeout(30);
      hold.push(await sample());
    }
    await page.mouse.up();
    await page.waitForTimeout(260);

    const all = travel.concat(hold);
    const mainTops = all.map((s) => s.mainTop ?? 0);
    const bentoTops = all.map((s) => s.bentoTop);
    const holdTransitions = transitions(hold);
    const saved = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:layout") || "[]").map((it) => it.ID || it.id));
    return {
      src,
      dst,
      travelTransitions: transitions(travel),
      holdTransitions,
      holdABA: hasABA(holdTransitions),
      mainScrollDelta: Math.max(...mainTops) - Math.min(...mainTops),
      bentoTopDelta: Math.max(...bentoTops) - Math.min(...bentoTops),
      saved,
      savedSrcIndex: saved.indexOf(src),
      savedDstIndex: saved.indexOf(dst),
    };
  };

  const pairs = [
    ["goals", "kpi-income"],
    ["trend", "accounts"],
    ["todo", "recent"],
  ];
  const results = [];
  for (const [src, dst] of pairs) results.push(await dragOnce(src, dst));

  const bad = results.filter(
    (r) =>
      r.holdABA ||
      r.holdTransitions.length > 1 ||
      r.mainScrollDelta > 8 ||
      r.bentoTopDelta > 8 ||
      r.savedSrcIndex < 0 ||
      r.savedDstIndex < 0 ||
      Math.abs(r.savedSrcIndex - r.savedDstIndex) > 1
  );
  if (bad.length) fail("drag reflow instability: " + JSON.stringify(bad, null, 2));
  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: 3 visible back-to-back dashboard reorders stayed stable.");
} finally {
  await browser.close();
}
