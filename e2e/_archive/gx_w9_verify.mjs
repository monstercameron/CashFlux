/**
 * W-9 Page-enter transition verification (wonder-page-enter)
 *
 * Checks:
 *  1. DEFAULT  – nav triggers wonder-page-enter animation + .page-enter class toggles
 *  2. OFF      – data-wonder="off" gates the animation (animationName → "none")
 *  3. REDUCED  – prefers-reduced-motion:reduce gates the animation (animationName → "none")
 *  4. BOOT     – cold-reload has no double-animate of page content
 *  5. CONSOLE  – no errors during navigation
 *
 * Run from the CashFlux root:
 *   node e2e/gx_w9_verify.mjs
 */

import { createRequire } from 'module';
import { existsSync, mkdirSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const _require = createRequire(join(__dirname, '..', '.tools', 'package.json'));
const { chromium } = _require('playwright');

const SCREENSHOTS = join(__dirname, 'screenshots');
if (!existsSync(SCREENSHOTS)) mkdirSync(SCREENSHOTS, { recursive: true });

// e2e/serve.go serves web/ on 8099 (same as all other e2e tests)
const BASE = process.env.E2E_URL || 'http://127.0.0.1:8099';
const BOOT_SETTLE_MS = 4000; // wait for wasm boot to finish

const consoleErrors = [];

function log(label, pass, detail) {
  const tag = pass ? '  PASS' : '  FAIL';
  console.log(`${tag}  [${label}]  ${detail}`);
}

async function getPageViewAnim(page) {
  return page.evaluate(() => {
    const el = document.getElementById('cf-page-view');
    if (!el) return { animationName: 'ELEMENT_MISSING', hasClass: false, opacity: null };
    const cs = getComputedStyle(el);
    return {
      animationName: cs.animationName,
      hasClass: el.classList.contains('page-enter'),
      opacity: cs.opacity,
    };
  });
}

async function navigateAndSample(page, path, label) {
  // Trigger navigation. The page.evaluate IPC call can take 200-1500ms because the
  // wasm is busy re-rendering during that window — so we start the deadline AFTER it
  // returns, not before.
  await page.evaluate((p) => {
    const links = [...document.querySelectorAll('a[href]')];
    const match = links.find(a => a.getAttribute('href') === p || a.getAttribute('href') === '/CashFlux' + p || a.pathname === p);
    if (match) { match.click(); return; }
    history.pushState({}, '', p);
    window.dispatchEvent(new PopStateEvent('popstate', { state: {} }));
  }, path);

  // Poll for up to 1500ms after the navigate IPC returns.
  // The double-rAF fires ~32ms after triggerPageEnter() is called by the Go effect.
  // We need to catch the class at any point during that window.
  let result = { animationName: 'none', hasClass: false, opacity: '1' };
  let classEverPresent = false;
  let animNameSeen = 'none';

  const deadline = Date.now() + 1500;
  while (Date.now() < deadline) {
    const snap = await getPageViewAnim(page);
    if (snap.hasClass) classEverPresent = true;
    if (snap.animationName && snap.animationName !== 'none' && snap.animationName !== '') {
      animNameSeen = snap.animationName;
    }
    result = snap;
    if (classEverPresent && animNameSeen !== 'none') break;
    await page.waitForTimeout(30);
  }

  return { ...result, classEverPresent, animNameSeen };
}

async function main() {
  console.log('\n══════ W-9 WONDER PAGE-ENTER VERIFICATION ══════\n');

  const browser = await chromium.launch({ headless: true });
  let allPass = true;

  // ── CHECK 5 setup: collect console errors ──────────────────────────────────
  const page = await browser.newPage();
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });
  page.on('pageerror', err => consoleErrors.push(err.message));

  // ── Boot the app ───────────────────────────────────────────────────────────
  await page.goto(BASE, { waitUntil: 'networkidle' });
  await page.waitForTimeout(BOOT_SETTLE_MS);

  // ── CHECK 1: DEFAULT navigation ────────────────────────────────────────────
  console.log('CHECK 1 — DEFAULT: animation plays on nav (Dashboard→Transactions→Budgets)');

  // Brief stabilisation pause after boot settle to ensure the Go UseEffect
  // firstRender guard has committed before we trigger the first navigation.
  await page.waitForTimeout(500);

  const nav1 = await navigateAndSample(page, '/transactions', 'Transactions');
  const pass1a = nav1.classEverPresent && nav1.animNameSeen === 'wonder-page-enter';
  log('Dashboard→Transactions', pass1a,
    `animationName="${nav1.animNameSeen}" classEverPresent=${nav1.classEverPresent} opacity=${nav1.opacity}`);

  await page.waitForTimeout(400); // let previous anim finish

  const nav2 = await navigateAndSample(page, '/budgets', 'Budgets');
  const pass1b = nav2.classEverPresent && nav2.animNameSeen === 'wonder-page-enter';
  log('Transactions→Budgets', pass1b,
    `animationName="${nav2.animNameSeen}" classEverPresent=${nav2.classEverPresent} opacity=${nav2.opacity}`);

  await page.waitForTimeout(400);

  const nav3 = await navigateAndSample(page, '/', 'Dashboard');
  const pass1c = nav3.classEverPresent && nav3.animNameSeen === 'wonder-page-enter';
  log('Budgets→Dashboard', pass1c,
    `animationName="${nav3.animNameSeen}" classEverPresent=${nav3.classEverPresent} opacity=${nav3.opacity}`);

  // Screenshot: navigate to /transactions and grab mid-transition
  await page.evaluate((p) => {
    const links = [...document.querySelectorAll('a[href]')];
    const match = links.find(a => a.getAttribute('href') === p || a.pathname === p);
    if (match) { match.click(); return; }
    history.pushState({}, '', p);
    window.dispatchEvent(new PopStateEvent('popstate', { state: {} }));
  }, '/transactions');
  // Grab screenshot quickly after nav to catch animation in flight
  await page.waitForTimeout(30);
  await page.screenshot({ path: join(SCREENSHOTS, 'wonder_w9_default.png') });
  console.log('  Screenshot: e2e/screenshots/wonder_w9_default.png');

  const check1Pass = pass1a && pass1b && pass1c;
  allPass = allPass && check1Pass;
  console.log(`  → CHECK 1 ${check1Pass ? 'PASS' : 'FAIL'}\n`);

  // ── CHECK 2: OFF (data-wonder="off") ──────────────────────────────────────
  console.log('CHECK 2 — OFF: data-wonder="off" gates animation');
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    document.documentElement.setAttribute('data-wonder', 'off');
  });

  const navOff = await navigateAndSample(page, '/accounts', 'Accounts');
  // When off, the class may still be added, but animation should be "none"
  const pass2 = navOff.animNameSeen === 'none' || navOff.animNameSeen === '';
  log('data-wonder=off nav to /accounts', pass2,
    `animationName="${navOff.animNameSeen}" classEverPresent=${navOff.classEverPresent}`);
  allPass = allPass && pass2;
  console.log(`  → CHECK 2 ${pass2 ? 'PASS' : 'FAIL'}\n`);

  // ── CHECK 3: REDUCED MOTION ────────────────────────────────────────────────
  console.log('CHECK 3 — REDUCED-MOTION: prefers-reduced-motion:reduce gates animation');

  // Clear data-wonder override first
  await page.evaluate(() => {
    document.documentElement.removeAttribute('data-wonder');
  });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.waitForTimeout(200);

  const navReduced = await navigateAndSample(page, '/goals', 'Goals');
  const pass3 = navReduced.animNameSeen === 'none' || navReduced.animNameSeen === '';
  log('reduced-motion nav to /goals', pass3,
    `animationName="${navReduced.animNameSeen}" classEverPresent=${navReduced.classEverPresent}`);
  allPass = allPass && pass3;
  console.log(`  → CHECK 3 ${pass3 ? 'PASS' : 'FAIL'}\n`);

  // ── CHECK 4: BOOT double-animate ───────────────────────────────────────────
  console.log('CHECK 4 — BOOT: hard reload, check for double-animation of page content');

  // Reset media emulation and data-wonder
  await page.emulateMedia({ reducedMotion: 'no-preference' });

  // Hard reload
  const bootPage = await browser.newPage();
  bootPage.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push('[boot] ' + msg.text());
  });

  // Collect animationName snapshots during the first second after load
  const bootSnaps = [];
  const bootPromise = bootPage.goto(BASE, { waitUntil: 'domcontentloaded' });

  // Poll during boot to detect any early page-enter animation on #cf-page-view
  let bootPollDone = false;
  const bootPoller = (async () => {
    await bootPromise;
    const deadline = Date.now() + 1200;
    while (Date.now() < deadline) {
      const snap = await bootPage.evaluate(() => {
        const el = document.getElementById('cf-page-view');
        if (!el) return null;
        const cs = getComputedStyle(el);
        return {
          animationName: cs.animationName,
          hasClass: el.classList.contains('page-enter'),
          t: Date.now(),
        };
      }).catch(() => null);
      if (snap) bootSnaps.push(snap);
      await bootPage.waitForTimeout(50);
    }
    bootPollDone = true;
  })();

  await bootPoller;
  await bootPage.waitForTimeout(500); // settle
  await bootPage.screenshot({ path: join(SCREENSHOTS, 'wonder_w9_boot.png') });
  console.log('  Screenshot: e2e/screenshots/wonder_w9_boot.png');

  // Analyse: during first 1.2s after domcontentloaded, the page-enter class must NOT
  // be present (first render guard). If it appears once that's the boot settle; twice = double.
  const classOnSnaps = bootSnaps.filter(s => s.hasClass);
  const animOnSnaps = bootSnaps.filter(s => s.animationName === 'wonder-page-enter');

  // Count distinct "runs" of the class being present (rising edges)
  let rises = 0;
  let prev = false;
  for (const s of bootSnaps) {
    if (s.hasClass && !prev) rises++;
    prev = s.hasClass;
  }

  // The cold-boot guard (firstRender ref) should prevent ANY page-enter class during boot.
  // Acceptable: 0 rises (class never added on boot). NOT acceptable: >= 1 (double-animate).
  const pass4 = rises === 0;
  log('cold-boot page-enter class rises', pass4,
    `rises=${rises} (class-on-snapshots=${classOnSnaps.length}/${bootSnaps.length}) ${pass4 ? '— no double-animate' : '— DOUBLE-ANIMATE DETECTED'}`);
  if (bootSnaps.length > 0) {
    const sample = bootSnaps.slice(0, 3).map(s => `{cls:${s.hasClass},anim:${s.animationName}}`).join(' ');
    console.log(`  Boot snapshots sample: ${sample}`);
  }
  allPass = allPass && pass4;
  console.log(`  → CHECK 4 ${pass4 ? 'PASS' : 'FAIL'}\n`);

  // ── CHECK 5: Console errors ────────────────────────────────────────────────
  console.log('CHECK 5 — CONSOLE: no errors during navigation');
  const pass5 = consoleErrors.length === 0;
  if (pass5) {
    log('no console errors', true, '0 errors');
  } else {
    log('console errors', false, `${consoleErrors.length} error(s):`);
    consoleErrors.slice(0, 5).forEach(e => console.log(`    • ${e}`));
  }
  allPass = allPass && pass5;
  console.log(`  → CHECK 5 ${pass5 ? 'PASS' : 'FAIL'}\n`);

  // ── Summary ────────────────────────────────────────────────────────────────
  console.log('══════════════════════════════════════════════');
  console.log(`OVERALL: ${allPass ? 'ALL CHECKS PASS' : 'SOME CHECKS FAILED'}`);
  console.log('══════════════════════════════════════════════\n');

  await browser.close();
  process.exit(allPass ? 0 : 1);
}

main().catch(err => {
  console.error('Fatal:', err);
  process.exit(1);
});
