// L29 gate — receipt attachments display + artifact↔txn linkage.
//
// pickFile (the real attach path) is an off-DOM native picker Playwright can't
// drive, so this gate exercises the DISPLAY + linkage half: it injects a small
// image artifact + an AttachmentRef on one transaction via a one-shot
// addInitScript (document-start, after the reloading page's pagehide→autosave),
// then asserts the paperclip marker, the preview image, and the Artifacts
// screen's "Referenced by 1 transaction" linkage.
//
// Injected shape: ds.artifacts += {id,name,kind:"image",mime:"image/png",
// bytes:<base64 PNG>,size,createdAt}; ds.transactions[0].attachments +=
// {artifactId,name,kind:"image",mime:"image/png"}.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const ART_NAME = "e2e-receipt.png";
// 1x1 transparent PNG.
const PNG_B64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getDS = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitDS(page, pred, timeoutMs = 10000) {
  let d = {};
  for (let w = 0; w < timeoutMs; w += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("tr.row[data-id]", { timeout: 60000 });
  await waitDS(page, (d) => (d.transactions || []).length > 0);

  // Arm the one-shot injection.
  await page.evaluate(([name, b64]) => {
    localStorage.setItem("e2e-attach", JSON.stringify({ name, b64 }));
  }, [ART_NAME, PNG_B64]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-attach");
    if (!raw) return;
    localStorage.removeItem("e2e-attach"); // one-shot
    try {
      const { name, b64 } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      const tx = (ds.transactions || [])[0];
      if (!tx) return;
      ds.artifacts = ds.artifacts || [];
      ds.artifacts.push({ id: "e2e-art-1", name, kind: "image", mime: "image/png", bytes: b64, size: 70, createdAt: "2026-06-22T00:00:00Z" });
      tx.attachments = tx.attachments || [];
      tx.attachments.push({ artifactId: "e2e-art-1", name, kind: "image", mime: "image/png" });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
      localStorage.setItem("e2e-attach-desc", tx.desc || "");
    } catch (e) { /* ignore */ }
  });

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("tr.row[data-id]", { timeout: 60000 });
  await waitDS(page, (d) => (d.artifacts || []).some((a) => a.id === "e2e-art-1"));

  // Filter the ledger to the attached transaction so its row is on the first page.
  const desc = await page.evaluate(() => localStorage.getItem("e2e-attach-desc") || "");
  if (desc) {
    const search = page.getByPlaceholder(/search/i).first();
    if (await search.count()) { await search.fill(desc); await page.waitForTimeout(500); }
  }

  // The paperclip marker shows on a row with an attachment.
  const marker = page.locator('[data-testid="txn-attach-marker"]').first();
  await marker.waitFor({ state: "visible", timeout: 10000 });

  // Clicking it opens an image preview.
  await marker.click();
  const previewImg = page.locator('[role="dialog"] img');
  const seen = await previewImg.first().isVisible().catch(() => false);
  if (!seen) fail("receipt preview image not shown after clicking the paperclip");

  // The Artifacts screen lists the transaction reference.
  await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".row", { timeout: 30000 });
  const refs = page.locator('[data-testid="artifact-refs"]', { hasText: /Referenced by/i });
  const refSeen = await refs.first().isVisible().catch(() => false);
  if (!refSeen) fail('Artifacts screen does not show a "Referenced by …" linkage');

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: paperclip marker + receipt preview render, and Artifacts shows the transaction linkage.");
} finally {
  await browser.close();
}
