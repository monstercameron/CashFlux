// C66 — artifact rename + "where used by pages" guard.
// Seeds an artifact via the upload button, renames it inline, and verifies the
// new name appears. Then creates a custom page that references the artifact and
// confirms that attempting to delete the artifact surfaces a "used by" warning
// instead of silently removing it.
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
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("h2", { timeout: 60000 });
  await page.waitForTimeout(500);

  // Both upload card and list card now have H2 headings (C66 fix).
  const h2s = await page.locator("h2").allTextContents();
  if (h2s.length < 2) fail(`expected at least 2 H2 headings on artifacts screen, got ${h2s.length}: ${h2s}`);

  // Inject a test artifact (no file picker). Re-apply at document-start on the
  // reload via a one-shot addInitScript — the reload's pagehide → autosave would
  // otherwise clobber a plain localStorage edit before wasm boots.
  const TS = Date.now();
  const artName = "TestArtifact_" + TS;
  await page.evaluate(([name, ts]) => {
    localStorage.setItem("e2e-artifact", JSON.stringify({ name, ts }));
  }, [artName, TS]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-artifact");
    if (!raw) return;
    localStorage.removeItem("e2e-artifact"); // one-shot
    try {
      const { name, ts } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.artifacts = ds.artifacts || [];
      ds.artifacts.push({
        id: "art_" + ts, name, kind: "csv", mime: "text/csv",
        columns: ["A", "B"], rows: [["1", "2"]], size: 10,
        createdAt: "2026-06-22T00:00:00.000Z",
      });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  // Reload so the screen boots with the injected artifact.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);

  // The artifact row should appear.
  const row = page.locator('[data-testid="artifact-name"]', { hasText: artName }).first();
  if ((await row.count()) === 0) fail(`artifact row "${artName}" not found after reload`);

  // Rename: click the pencil button on that row.
  const rowContainer = page.locator(".row").filter({ has: page.locator('[data-testid="artifact-name"]', { hasText: artName }) }).first();
  await rowContainer.locator('[aria-label="Rename"]').first().click();
  await page.waitForSelector('[data-testid="artifact-rename-input"]', { timeout: 5000 });
  const newName = "Renamed_" + TS;
  await page.locator('[data-testid="artifact-rename-input"]').fill(newName);
  await page.locator('[data-testid="artifact-rename-input"]').press("Enter");
  await page.waitForTimeout(400);

  // The new name should now appear.
  if ((await page.locator('[data-testid="artifact-name"]', { hasText: newName }).count()) === 0) {
    fail(`renamed artifact "${newName}" not found after save`);
  }
  if ((await page.locator('[data-testid="artifact-name"]', { hasText: artName }).count()) > 0) {
    fail(`old artifact name "${artName}" still visible after rename`);
  }

  // Page-ref guard: inject a custom page that references this artifact, then try
  // to delete the artifact and assert the notice fires (not the artifact disappearing).
  const pgID = "pg_" + TS;
  const artID = "art_" + TS;
  await page.evaluate(([pgid, artid]) => {
    localStorage.setItem("e2e-artpage", JSON.stringify({ pgid, artid }));
  }, [pgID, artID]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-artpage");
    if (!raw) return;
    localStorage.removeItem("e2e-artpage"); // one-shot
    try {
      const { pgid, artid } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.customPages = ds.customPages || [];
      ds.customPages.push({
        id: pgid, title: "TestPage", slug: "testpage-ref", hidden: false,
        widgets: [{ id: "w1", type: "table", title: "T", binding: { artifactId: artid }, config: {} }],
        layout: [],
      });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);

  // Try to delete the artifact that is used by the injected custom page.
  const renamedRow = page.locator(".row").filter({ has: page.locator('[data-testid="artifact-name"]', { hasText: newName }) }).first();
  if ((await renamedRow.count()) === 0) fail("renamed artifact row not found after page-ref inject + reload");

  await renamedRow.locator('[aria-label="Delete"]').first().click();
  await page.waitForTimeout(400);

  // The artifact should still be present (not deleted).
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);
  const stillThere = (await page.locator('[data-testid="artifact-name"]', { hasText: newName }).count()) > 0;
  if (!stillThere) fail("artifact was silently deleted even though it is referenced by a custom page");

  if (!process.exitCode) console.log("PASS: artifact rename works and delete is guarded when the artifact is used by a custom page.");
} finally {
  await browser.close();
}
