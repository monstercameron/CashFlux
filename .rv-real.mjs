import { chromium } from "playwright";
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 1440, height: 900 } });
await p.goto("http://127.0.0.1:8241/transactions", { waitUntil: "domcontentloaded", timeout: 60000 });
await p.waitForSelector("[data-app-ready]", { timeout: 120000 });
await p.waitForSelector('[data-testid="txn-review-btn"]', { timeout: 60000 });
await p.getByTestId("txn-review-btn").click();
await p.waitForSelector('[data-testid="review-inbox"]');
await p.waitForTimeout(800);

// Measure IN THE PAGE: click, then wait for the DOM to actually reflect it.
// No fixed sleeps inside the timed window.
const measure = async (label, script) => {
  const ms = await p.evaluate(script);
  console.log(`${label}: ${Math.round(ms)}ms`);
};

await measure("select a merchant", `(async () => {
  const t0 = performance.now();
  document.querySelector('[data-testid^="review-pick-"]').click();
  const sel = document.querySelector('[data-testid="review-selection"]');
  while (!/\d+ merchant/.test(sel.textContent)) await new Promise(r => requestAnimationFrame(r));
  return performance.now() - t0;
})()`);

await measure("clear selection", `(async () => {
  const t0 = performance.now();
  document.querySelector('[data-testid="review-clear"]').click();
  const sel = document.querySelector('[data-testid="review-selection"]');
  while (/\d+ merchant/.test(sel.textContent)) await new Promise(r => requestAnimationFrame(r));
  return performance.now() - t0;
})()`);

await measure("expand a merchant", `(async () => {
  const t0 = performance.now();
  document.querySelector('[data-testid^="review-expand-"]').click();
  while (!document.querySelector('.rvs-members .rvs-mem')) await new Promise(r => requestAnimationFrame(r));
  return performance.now() - t0;
})()`);

await measure("switch to single mode", `(async () => {
  const t0 = performance.now();
  document.querySelector('[data-testid="review-mode-single"]').click();
  while (!document.querySelector('[data-testid="review-payee"]')) await new Promise(r => requestAnimationFrame(r));
  return performance.now() - t0;
})()`);
await b.close();
