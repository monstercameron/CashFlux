// /assistant Smart tab (and /smart) comprehensive e2e: the flattened bento
// surface — an agent-voiced hero whose counts mirror the smart_* engine
// variables, the insight feed with its header at the TOP (component-sibling
// ordering regression guard), the digest, and the full catalog on one scroll
// (no nested tabs). Positive AND negative cases: toggling a feature updates
// the hero count live, the findings chip matches the capped feed, disable-all
// swaps the voice to onboarding, and the formula picker exposes the Smart
// features group. No page errors.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 2000 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));

// boot + sample
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await p.goto(URL + "/assistant", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(2000);
await p.locator('button, [role="radio"]', { hasText: "Smart" }).first().click({ force: true });
await p.waitForSelector(".smt-deck", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1500);

// --- flattened surface + hero ---
check("S1 the Smart tab is one flattened bespoke surface (no nested tabs)", await p.locator(".smt-deck").count() === 1 && (await p.locator('[data-testid="smart-tab-insights"]').count()) === 0);
const heroFindings = parseInt((await p.locator('[data-testid="smt-hero-count"]').innerText().catch(() => "-1")).trim(), 10);
check("H1 hero leads with the FINDINGS count (not the admin watcher tally)", heroFindings >= 0 && /findings worth a look/i.test(await p.locator("#sec-smart-hero").innerText()), `${heroFindings}`);
check("H2 the agent voice line reads as a sentence", /I've found|All quiet|Nothing is switched on/.test((await p.locator('[data-testid="smt-hero-voice"]').innerText().catch(() => "")) || ""));
check("H3 posture chips: watching, AI/billed, density", /Watching/i.test(await p.locator("#sec-smart-hero").innerText()) && /Density/i.test(await p.locator("#sec-smart-hero").innerText()));
// The hero findings figure equals what the feed pager exposes (capped count).
const pagerTxt = (await p.locator('[data-testid="smart-insights-pager"]').innerText().catch(() => "")) || "";
const pages = pagerTxt.match(/of (\d+)/);
if (pages) {
  const pg = parseInt(pages[1], 10);
  check("H4 the hero findings figure matches the capped feed (fits the page count)", heroFindings > (pg - 1) * 10 && heroFindings <= pg * 10, `findings=${heroFindings} pages=${pg}`);
} else {
  check("H4 the hero findings figure matches the capped feed (fits the page count)", true, "no pager (few findings)");
}

// --- feed header order regression guard ---
const firstChild = await p.evaluate(() => {
  const c = document.querySelector('[data-testid="smart-insights"]');
  return c && c.firstElementChild ? c.firstElementChild.className : "";
});
check("F1 the feed card's header renders at the TOP", /card-head/.test(firstChild), firstChild);

// --- everything on one scroll: digest + catalog visible without tab clicks ---
check("F2 digest + manage catalog are on the same surface", await p.locator('[data-testid="smart-digest-section"]').count() === 1 && await p.locator('[data-testid="smart-manage"]').count() === 1);

// --- toggling a feature updates the Watching chip live (the catalog groups
// fold behind accordions now — open the first group to reach a toggle) ---
const watching = async () => parseInt(((await p.locator("#sec-smart-hero").innerText()).match(/Watching\s*(\d+)/i) || [])[1] || "-1", 10);
const before = await watching();
await p.locator('[data-testid^="smart-group-"]').first().click({ force: true });
await p.waitForTimeout(400);
const firstToggle = p.locator('[data-testid="smart-manage"] input[type="checkbox"], [data-testid="smart-manage"] [role="switch"]').first();
if (await firstToggle.count()) {
  await firstToggle.click({ force: true });
  await p.waitForTimeout(900);
  const after = await watching();
  check("T1 toggling a feature moves the Watching chip live", after === before - 1 || after === before + 1, `${before} -> ${after}`);
  check("T2 the accordion group stays open across the toggle re-render", (await p.locator('[data-testid="smart-manage"] input[type="checkbox"], [data-testid="smart-manage"] [role="switch"]').count()) > 0);
  await firstToggle.click({ force: true }); // restore
  await p.waitForTimeout(600);
} else {
  check("T1 toggling a feature moves the Watching chip live", false, "no toggle found after opening a group");
  check("T2 the accordion group stays open across the toggle re-render", false, "skipped");
}

// --- (neg) disable-all → onboarding voice ---
await p.locator('[data-testid="smart-disable-all"]').click({ force: true });
await p.waitForTimeout(900);
check("N1 (neg) with everything off the voice switches to onboarding", /Nothing is switched on/.test((await p.locator('[data-testid="smt-hero-voice"]').innerText().catch(() => "")) || "") && (await p.locator('[data-testid="smt-hero-count"]').innerText()) === "0");
// Restore free features so the household isn't left dark.
await p.locator('button', { hasText: "Enable free" }).first().click({ force: true }).catch(() => {});
await p.waitForTimeout(900);
check("N2 enable-free restores the watchers", parseInt(await p.locator('[data-testid="smt-hero-count"]').innerText(), 10) > 0);

// --- the smart_* variables are in the formula picker ---
await p.goto(URL + "/studio", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(1500);
await p.locator('button, [role="radio"]', { hasText: "Formulas" }).first().click({ force: true });
await p.waitForSelector(".bento-studio", { timeout: 15000 }).catch(() => {});
await p.waitForTimeout(1200);
await p.locator('[data-testid="fb-search"]').fill("smart features");
await p.waitForTimeout(500);
check("V1 the picker exposes the smart_* posture variables", /Smart features on/.test(await p.locator(".fb-palette").innerText()));

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
