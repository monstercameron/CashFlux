import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1100 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/notifications", { waitUntil: "domcontentloaded" });
await p.waitForSelector(".notif", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1000);
check("T1 summary tile", await p.locator('.notif-summary').count() === 1);
check("T2 filter strip", await p.locator('.filter-strip [data-testid="notif-filter"]').count() === 1);
check("T3 feed cards render", await p.locator('.notif').count() >= 3, `${await p.locator('.notif').count()}`);
check("T4 severity chips", await p.locator('.notif-sev-chip').count() >= 1);
check("T5 severity medallion + rail", await p.locator('.notif.sev-critical, .notif.sev-warning, .notif.sev-info').count() >= 1);
check("T6 NO ⋯ menu (inline actions only)", await p.locator('[data-testid^="notif-menu-btn-"]').count() === 0);
check("T7 inline dismiss + snooze buttons present", await p.locator('[data-testid^="notif-dismiss-"]').count() >= 3 && await p.locator('[data-testid^="notif-snooze-"]').count() >= 3);
// T8: inline dismiss removes that specific card (one click, no menu).
const firstId = await p.locator('.notif').first().getAttribute('data-testid');
await p.locator('.notif').first().locator('[data-testid^="notif-dismiss-"]').click({ force: true });
await p.waitForTimeout(700);
check("T8 inline dismiss removes that card", (await p.locator('[data-testid="'+firstId+'"]').count()) === 0, `${firstId}`);
// T9: severity filter.
await p.locator('[data-testid="notif-filter"]').selectOption("warning"); await p.waitForTimeout(500);
const warnOnly = await p.evaluate(() => { const c = [...document.querySelectorAll('.notif')]; return c.length > 0 && c.every(x => x.classList.contains('sev-warning')); });
check("T9 severity filter shows only that tier", warnOnly, `${await p.locator('.notif').count()} cards`);
await p.locator('[data-testid="notif-filter"]').selectOption(""); await p.waitForTimeout(400);
// T10: Clear all empties the feed.
await p.locator('[data-testid="notif-clear-all"]').click({ force: true }); await p.waitForTimeout(700);
check("T10 clear all empties the feed", await p.locator('.notif').count() === 0);
check("T11 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
