// Regression test for B2 dashboard bento drag preview oscillation.
//
// Native dragover can retarget when FLIP-animated tiles move under the pointer.
// The app should keep the preview target based on the stable pre-drag geometry,
// not bounce to whichever tile the browser reports after a reflow.
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

try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.evaluate(() => {
    localStorage.removeItem("cashflux:layout");
    localStorage.removeItem("cashflux:layout-mode");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-widget="kpi-income"]', { timeout: 60000 });

  const helperStable = await page.evaluate(() => {
    const src = document.querySelector('[data-widget="kpi-income"]');
    const intended = document.querySelector('[data-widget="kpi-networth"]');
    const churn = document.querySelector('[data-widget="kpi-spending"]');
    if (
      !src ||
      !intended ||
      !churn ||
      typeof window.cashfluxBentoDragStart !== "function" ||
      typeof window.cashfluxBentoDragTarget !== "function" ||
      typeof window.cashfluxBentoDragEnd !== "function"
    ) {
      return { ok: false, reason: "drag coordinator helpers missing" };
    }
    const r = intended.getBoundingClientRect();
    const pt = { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    window.cashfluxBentoDragStart("kpi-income");
    const first = window.cashfluxBentoDragTarget(pt.x, pt.y);
    intended.style.transform = "translateX(260px)";
    churn.style.transform = "translateX(-260px)";
    const second = window.cashfluxBentoDragTarget(pt.x, pt.y);
    intended.style.transform = "";
    churn.style.transform = "";
    window.cashfluxBentoDragEnd();
    return { ok: first === "kpi-networth" && second === "kpi-networth", first, second };
  });
  if (!helperStable.ok) {
    fail(`stable drag coordinator failed: ${JSON.stringify(helperStable)}`);
  }

  const result = await page.evaluate(async () => {
    const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
    const el = (id) => document.querySelector(`[data-widget="${id}"]`);
    const center = (node) => {
      const r = node.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    };
    const dt = new DataTransfer();
    const dispatchDrag = (node, type, pt) => {
      const ev = new DragEvent(type, {
        bubbles: true,
        cancelable: true,
        clientX: pt.x,
        clientY: pt.y,
        dataTransfer: dt,
      });
      node.dispatchEvent(ev);
    };
    const visualOrder = () =>
      [...document.querySelectorAll(".bento > .w[data-widget]")]
        .map((node) => {
          const cs = getComputedStyle(node);
          return { id: node.dataset.widget, x: Number.parseInt(cs.gridColumnStart, 10), y: Number.parseInt(cs.gridRowStart, 10) };
        })
        .sort((a, b) => a.y - b.y || a.x - b.x)
        .map((p) => p.id);

    const src = el("kpi-income");
    const intended = el("kpi-networth");
    const pt = center(intended);

    dispatchDrag(src, "dragstart", pt);
    await sleep(50);
    dispatchDrag(intended, "dragover", pt);
    await sleep(80);
    const afterIntended = visualOrder();

    // Simulate browser hit-test churn: a different tile reports dragover while
    // the pointer coordinates are still inside the original intended target.
    dispatchDrag(el("kpi-spending"), "dragover", pt);
    await sleep(80);
    const afterChurn = visualOrder();
    dispatchDrag(src, "dragend", pt);
    await sleep(260);

    return { afterIntended, afterChurn };
  });

  const intended = result.afterIntended.join(",");
  const churn = result.afterChurn.join(",");
  if (!result.afterIntended[0] || result.afterIntended[0] !== "kpi-income" || result.afterIntended[1] !== "kpi-networth") {
    fail(`first preview should insert income before net worth; got ${intended}`);
  }
  if (churn !== intended) {
    fail(`preview oscillated after retargeted dragover: ${intended} -> ${churn}`);
  }

  const finalOrder = await order(page);
  if (finalOrder[0] !== "kpi-income" || finalOrder[1] !== "kpi-networth") {
    fail(`release should persist the reflowed layout; got ${finalOrder.join(",")}`);
  }
  const saved = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:layout") || "[]").map((it) => it.ID || it.id));
  if (saved[0] !== "kpi-income" || saved[1] !== "kpi-networth") {
    fail(`saved layout should match release order; got ${saved.join(",")}`);
  }
  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: dashboard drag preview keeps a stable insertion target during animated reflow.");
} finally {
  await browser.close();
}
