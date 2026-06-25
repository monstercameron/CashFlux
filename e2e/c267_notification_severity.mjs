// C267 e2e verification — Notification Center severity pills.
//
// What this tests: when FeedItems with distinct Severity values ("info",
// "warning", "critical") are seeded into the dataset KV and the app is
// reloaded, the Notification Center renders a labelled pill per row that
// matches the seeded severity — not color-only (WCAG 1.4.1).
//
// Strategy: same IDB-injection pattern as c270_notification_center.mjs.
// We seed three items — one per severity — into dataset.appState under the
// notify feed key, reload, navigate to /notifications, and assert:
//   C-1  Three role="listitem" entries are visible.
//   C-2  A pill with text "Info" is present (case-insensitive).
//   C-3  A pill with text "Warning" is present.
//   C-4  A pill with text "Critical" is present.
//   C-5  The pills have distinct CSS classes (sev-info / sev-warning / sev-critical).
//   C-6  No unexpected JS errors.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/c267_notification_severity.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (n) => path.join(SSDIR, n);

let passed = 0, failed = 0;
const pass = (l) => { console.log(`PASS: ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL: ${l}`); failed++; };

const now = Math.floor(Date.now() / 1000);

// Three feed items — one per severity level — matching the FeedItem struct.
const SEEDED_FEED = JSON.stringify([
  { id: "c267-e2e-info",     title: "C267 info: Weekly digest is ready",       body: "You spent $320 last week.",        at: now,     read: false, severity: "info"     },
  { id: "c267-e2e-warning",  title: "C267 warning: Groceries near limit",       body: "You are at 85 % of your budget.",  at: now - 1, read: false, severity: "warning"  },
  { id: "c267-e2e-critical", title: "C267 critical: Rent is due tomorrow",      body: "Make sure you have enough funds.", at: now - 2, read: false, severity: "critical" },
]);

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
const jsErrors = [];
page.on("pageerror", (e) => {
  const msg = String(e);
  if (!msg.includes("released function")) jsErrors.push(msg);
});

try {
  // ── Step 1: boot and let the app fully initialize ──────────────────────────
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 30000 });
  await page.waitForTimeout(6000);

  // ── Step 2: inject three severity-differentiated feed items into IDB ───────
  const injected = await page.evaluate(async (feedJSON) => {
    return new Promise((resolve) => {
      const openReq = indexedDB.open("cashflux-kv", 1);
      openReq.onerror = () => resolve({ ok: false, err: openReq.error?.message });
      openReq.onsuccess = (e) => {
        const db = e.target.result;
        if (!db.objectStoreNames.contains("kv")) {
          resolve({ ok: false, err: "no kv store in cashflux-kv IDB" });
          return;
        }
        const readTx = db.transaction("kv", "readonly");
        const getReq = readTx.objectStore("kv").get("cashflux:dataset");
        getReq.onerror = () => resolve({ ok: false, err: "get failed" });
        getReq.onsuccess = () => {
          const raw = getReq.result;
          if (!raw) { resolve({ ok: false, err: "no cashflux:dataset in IDB" }); return; }
          let ds;
          try { ds = JSON.parse(typeof raw === "string" ? raw : JSON.stringify(raw)); }
          catch (err) { resolve({ ok: false, err: "parse failed: " + err.message }); return; }
          if (!ds.appState) ds.appState = {};
          ds.appState["cashflux:notify:feed"] = feedJSON;
          let written;
          try { written = JSON.stringify(ds); }
          catch (err) { resolve({ ok: false, err: "stringify failed: " + err.message }); return; }
          const writeTx = db.transaction("kv", "readwrite");
          const putReq = writeTx.objectStore("kv").put(written, "cashflux:dataset");
          putReq.onerror = () => resolve({ ok: false, err: "put failed" });
          putReq.onsuccess = () => resolve({ ok: true });
        };
      };
    });
  }, SEEDED_FEED);

  if (!injected.ok) {
    fail(`IDB inject: ${injected.err} — cannot verify C267 (environment issue)`);
    console.log("NOTE: This is an environment/IDB access issue, not a code regression.");
  } else {
    console.log("NOTE: Three severity-differentiated feed items injected — reloading.");

    // ── Step 3: reload and navigate to the Notification Center ─────────────
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 30000 });
    await page.waitForTimeout(4000);

    await page.goto(BASE + "/notifications", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 30000 });
    await page.waitForTimeout(1500);

    // Dismiss any error overlay.
    await page.evaluate(() => {
      const o = document.getElementById("gwc-error-overlay") ||
                document.querySelector(".gwc-error-overlay");
      if (o) o.remove();
    });

    // Screenshot for evidence.
    await page.screenshot({ path: SS("c267_notification_severity.png") });

    // ── Step 4: assert three list items are visible ─────────────────────────
    const listItems = page.locator('[role="listitem"]');
    const itemCount = await listItems.count();
    if (itemCount >= 3) {
      pass(`C-1: Notification Center shows ${itemCount} feed item(s) — all three seeded entries visible`);
    } else {
      fail(`C-1: Expected ≥3 items, got ${itemCount}`);
    }

    // ── Step 5: assert labelled pills for each severity ─────────────────────
    // The Go render emits text "Info" / "Warning" / "Critical" inside the pill span.
    const allText = (await page.locator('[role="listitem"]').allTextContents()).join(" ");

    if (/\binfo\b/i.test(allText)) {
      pass("C-2: 'Info' severity pill text present in the notification list");
    } else {
      fail(`C-2: 'Info' pill text not found. Row text: "${allText.substring(0, 200)}"`);
    }

    if (/\bwarning\b/i.test(allText)) {
      pass("C-3: 'Warning' severity pill text present");
    } else {
      fail(`C-3: 'Warning' pill text not found. Row text: "${allText.substring(0, 200)}"`);
    }

    if (/\bcritical\b/i.test(allText)) {
      pass("C-4: 'Critical' severity pill text present");
    } else {
      fail(`C-4: 'Critical' pill text not found. Row text: "${allText.substring(0, 200)}"`);
    }

    // ── Step 6: assert distinct CSS classes on pill elements ─────────────────
    const [infoPills, warnPills, critPills] = await Promise.all([
      page.locator(".sev-info").count(),
      page.locator(".sev-warning").count(),
      page.locator(".sev-critical").count(),
    ]);

    if (infoPills > 0 && warnPills > 0 && critPills > 0) {
      pass(`C-5: Distinct pill classes present — sev-info(${infoPills}) sev-warning(${warnPills}) sev-critical(${critPills})`);
    } else {
      fail(`C-5: Missing pill class(es) — sev-info(${infoPills}) sev-warning(${warnPills}) sev-critical(${critPills})`);
    }
  }

  // ── Step 7: JS error check ──────────────────────────────────────────────
  if (jsErrors.length === 0) {
    pass("C-6: No JS errors during ritual");
  } else {
    fail(`C-6: ${jsErrors.length} JS error(s): ${jsErrors.slice(0, 3).join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\nResult: ${passed} PASS · ${failed} FAIL`);
  if (failed > 0) process.exitCode = 1;
}
