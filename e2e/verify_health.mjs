// Quick verification for R27 Financial-health: dashboard widget + /health page.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let pass = 0, fail = 0;
const P = (l) => { console.log("PASS: " + l); pass++; };
const F = (l) => { console.error("FAIL: " + l); fail++; };
const jsErr = [];
try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErr.push(m); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1800);
  // 1. Dashboard widget present
  const hasWidget = await page.evaluate(() => [...document.querySelectorAll('.w')].some(w => /financial health/i.test(w.textContent || "")));
  hasWidget ? P("Dashboard 'Financial health' widget renders") : F("widget missing");
  // 2. Score ring SVG with an arc + a numeric figure
  const ring = await page.evaluate(() => {
    const w = [...document.querySelectorAll('.w')].find(w => /financial health/i.test(w.textContent || ""));
    if (!w) return null;
    const svg = w.querySelector('svg');
    const fig = w.querySelector('.fig');
    const band = (w.textContent || "").match(/Excellent|Good|Fair|Needs work|Critical|Not enough data/);
    return { svg: !!svg, circles: svg ? svg.querySelectorAll('circle').length : 0, fig: fig ? fig.textContent.trim() : null, band: band ? band[0] : null };
  });
  if (ring && ring.svg && ring.circles >= 2) P("Score ring SVG present (" + ring.circles + " circles)"); else F("ring missing: " + JSON.stringify(ring));
  if (ring && ring.fig && /^\d+$|—/.test(ring.fig)) P("Score figure = '" + ring.fig + "'"); else F("score figure bad: " + JSON.stringify(ring));
  if (ring && ring.band) P("Band label = '" + ring.band + "'"); else F("band missing");
  // 3. Navigate to /health page via "View steps"
  await page.evaluate(() => { const b = [...document.querySelectorAll('.w button')].find(b => /view steps/i.test(b.textContent || "")); if (b) b.click(); });
  await page.waitForTimeout(1200);
  const onHealth = await page.evaluate(() => location.pathname.endsWith("/health"));
  onHealth ? P("'View steps' navigates to /health") : F("did not navigate to /health (path=" + await page.evaluate(()=>location.pathname) + ")");
  // 4. /health page shows the factor breakdown
  const page2 = await page.evaluate(() => {
    const t = document.body.textContent || "";
    return {
      breakdown: /what goes into your score/i.test(t),
      factors: ["Savings rate","Emergency fund","Debt payments","Budget adherence","Credit utilization"].filter(f => t.includes(f)).length,
      privacy: /never uploaded or shared/i.test(t),
      contribution: /% of your score/i.test(t),
    };
  });
  page2.breakdown ? P("/health breakdown section present") : F("no breakdown section");
  page2.factors >= 3 ? P("/health shows " + page2.factors + " factor rows") : F("too few factors: " + page2.factors);
  page2.contribution ? P("/health shows per-factor contribution %") : F("no contribution %");
  page2.privacy ? P("/health privacy note present") : F("no privacy note");
  jsErr.length === 0 ? P("zero runtime JS errors") : F(jsErr.length + " JS errors: " + jsErr.slice(0,3).join(" | "));
} catch (e) { F("UNEXPECTED: " + e.message); console.error(e); }
finally { await browser.close(); }
console.log("\nRESULT: " + pass + " PASS · " + fail + " FAIL");
process.exit(fail > 0 ? 1 : 0);
