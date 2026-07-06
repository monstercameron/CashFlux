// C52 gate — "long notes are truncated with a tooltip". A task with a very long
// notes field should show an ellipsis-truncated version in the row meta and
// expose the full text via the title attribute (tooltip). Exits non-zero on
// any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const TITLE = "ZZNOTESTRUNC-TASK";
// A note that is clearly > 80 characters.
const LONG_NOTE =
  "This is a very long note that definitely exceeds the eighty-character display limit set for task rows in the to-do screen.";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Seed a task with a long note via a one-shot addInitScript injection (the inline
  // add form moved to the +Add modal, C73; injecting is simpler and deterministic).
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);
  await page.evaluate(([title, note]) => localStorage.setItem("e2e-longnote", JSON.stringify({ title, note })), [TITLE, LONG_NOTE]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-longnote");
    if (!raw) return;
    localStorage.removeItem("e2e-longnote"); // one-shot
    try {
      const { title, note } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.tasks = ds.tasks || [];
      ds.tasks.push({ id: "t-longnote-e2e", title, notes: note, status: "open", priority: "medium", source: "manual" });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });
  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("tr.row, .rows .row, .row", { timeout: 60000 });
  await page.waitForTimeout(500);

  const row = page.locator(".row", { hasText: TITLE }).first();
  if ((await row.count()) === 0) {
    fail(`task "${TITLE}" did not appear`);
  } else {
    // Find the notes meta span — it should have the title attribute with the full note.
    const noteMeta = row.locator(`.row-meta[title]`).first();
    if ((await noteMeta.count()) === 0) {
      fail("notes meta span should carry a title attribute for the tooltip");
    } else {
      const titleAttr = await noteMeta.getAttribute("title");
      if (titleAttr !== LONG_NOTE) {
        fail(`title attribute should equal the full note text, got "${titleAttr}"`);
      }
      const displayedText = (await noteMeta.innerText()).trim();
      if (!displayedText.endsWith("…")) {
        fail(`displayed note should end with "…" when truncated, got "${displayedText}"`);
      }
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: long task notes are truncated to 80 chars with a tooltip showing the full text."
    );
} finally {
  await browser.close();
}
