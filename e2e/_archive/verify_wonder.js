/**
 * W-4 + W-19 WONDER verification script.
 * Tests: list-row hover nudge, table-row exclusion, wonder-off zeroing, skeleton shimmer.
 * Run: node e2e/verify_wonder.js
 *
 * Note: Playwright headless hover doesn't trigger CSS :hover pseudo-class; we use CDP
 * CSS.forcePseudoState to reliably force :hover and verify computed transforms.
 */
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE = 'http://127.0.0.1:8099';
const SS_DIR = path.join(__dirname, 'screenshots');

if (!fs.existsSync(SS_DIR)) fs.mkdirSync(SS_DIR, { recursive: true });

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();

  const errors = [];
  page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text()); });
  page.on('pageerror', err => errors.push(String(err)));

  let pass = true;
  const results = {};

  function check(label, actual, expectFn, expected) {
    const ok = expectFn(actual);
    const mark = ok ? '✓' : '✗';
    console.log(`${mark} ${label}`);
    console.log(`    measured: ${JSON.stringify(actual)}`);
    console.log(`    expected: ${expected}`);
    if (!ok) { pass = false; }
    results[label] = { actual, ok };
  }

  // Setup CDP session for forcePseudoState
  const client = await ctx.newCDPSession(page);

  // ── Navigate to /accounts (has list .row elements) ────────────────────────
  await page.goto(BASE + '/accounts', { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(2000); // wait for stagger animations

  await client.send('DOM.enable');
  await client.send('CSS.enable');

  // ── W-4: List-row hover nudge ─────────────────────────────────────────────
  console.log('\n── W-4 List-row hover nudge ──');

  const { root } = await client.send('DOM.getDocument');
  const listRowResult = await client.send('DOM.querySelector', {
    nodeId: root.nodeId,
    selector: '.rows .row'
  });
  const listNodeId = listRowResult.nodeId;

  if (listNodeId) {
    // Force :hover via CDP
    await client.send('CSS.forcePseudoState', { nodeId: listNodeId, forcedPseudoClasses: ['hover'] });
    await page.waitForTimeout(300);

    const t = await page.evaluate(() => {
      const row = document.querySelector('.rows .row');
      return {
        transform: getComputedStyle(row).transform,
        animName: getComputedStyle(row).animationName
      };
    });

    console.log(`  (CDP :hover forced on .rows .row)`);
    check('W-4 List-row nudge: transform is non-identity (translateX 2px)', t.transform,
      v => {
        // matrix(1, 0, 0, 1, 2, 0) means translateX(2px)
        const m = v.match(/matrix\(([^)]+)\)/);
        if (!m) return false;
        const vals = m[1].split(',').map(s => parseFloat(s.trim()));
        return Math.abs(vals[4] - 2) < 0.5; // e-value (x translation) ≈ 2
      },
      'matrix(1,0,0,1,2,0) — translateX(2px)');
    check('W-4 List-row animation cleared on hover', t.animName,
      v => v === 'none', '"none"');

    // Reset hover state
    await client.send('CSS.forcePseudoState', { nodeId: listNodeId, forcedPseudoClasses: [] });
    await page.waitForTimeout(200);
  } else {
    console.log('  SKIP: no .rows .row found on /accounts');
  }

  // ── W-4: Table-row exclusion ──────────────────────────────────────────────
  console.log('\n── W-4 Table-row exclusion ──');
  await page.goto(BASE + '/transactions', { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(1500);

  const { root: root2 } = await client.send('DOM.getDocument');
  const txnRowResult = await client.send('DOM.querySelector', {
    nodeId: root2.nodeId,
    selector: '.txn-table .row'
  });
  const txnNodeId = txnRowResult.nodeId;

  if (txnNodeId) {
    await client.send('CSS.forcePseudoState', { nodeId: txnNodeId, forcedPseudoClasses: ['hover'] });
    await page.waitForTimeout(300);

    const txnT = await page.evaluate(() => {
      const row = document.querySelector('.txn-table .row');
      return getComputedStyle(row).transform;
    });

    check('W-4 Table-row hover: transform is identity (no nudge)', txnT,
      v => v === 'none' || v === 'matrix(1, 0, 0, 1, 0, 0)',
      'none or matrix(1,0,0,1,0,0)');

    await client.send('CSS.forcePseudoState', { nodeId: txnNodeId, forcedPseudoClasses: [] });
  } else {
    console.log('  No .txn-table .row found — checking CSS rule');
    const ruleExists = await page.evaluate(() => {
      for (const sheet of document.styleSheets) {
        try {
          for (const rule of sheet.cssRules) {
            if (rule.selectorText && rule.selectorText.includes('txn-table') && rule.selectorText.includes('hover')) return true;
          }
        } catch (e) {}
      }
      return false;
    });
    check('W-4 Table-row exclusion CSS rule present', ruleExists, v => v, 'true');
  }

  // ── W-4: wonder=off zeroes nudge ─────────────────────────────────────────
  console.log('\n── W-4 wonder=off zeroes nudge ──');
  await page.goto(BASE + '/accounts', { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(2000);
  await page.evaluate(() => document.documentElement.setAttribute('data-wonder', 'off'));
  await page.waitForTimeout(100);

  const { root: root3 } = await client.send('DOM.getDocument');
  const offRowResult = await client.send('DOM.querySelector', { nodeId: root3.nodeId, selector: '.rows .row' });
  const offNodeId = offRowResult.nodeId;

  if (offNodeId) {
    await client.send('CSS.forcePseudoState', { nodeId: offNodeId, forcedPseudoClasses: ['hover'] });
    await page.waitForTimeout(300);

    const offT = await page.evaluate(() => {
      const row = document.querySelector('.rows .row');
      return getComputedStyle(row).transform;
    });

    check('W-4 Nudge zeroed when wonder=off', offT,
      v => v === 'none' || v === 'matrix(1, 0, 0, 1, 0, 0)',
      'none or matrix(1,0,0,1,0,0)');

    await client.send('CSS.forcePseudoState', { nodeId: offNodeId, forcedPseudoClasses: [] });
  }

  await page.evaluate(() => document.documentElement.removeAttribute('data-wonder'));

  // Screenshot W-4
  await page.screenshot({ path: path.join(SS_DIR, 'w4_row_hover.png') });
  console.log('\n  Screenshot → e2e/screenshots/w4_row_hover.png');

  // ── W-19: .wonder-skeleton shimmer ───────────────────────────────────────
  console.log('\n── W-19 .wonder-skeleton shimmer ──');

  // Inject skeleton
  await page.evaluate(() => {
    const div = document.createElement('div');
    div.className = 'wonder-skeleton';
    div.id = 'test-skeleton';
    div.style.cssText = 'width:200px;height:20px;position:fixed;top:20px;left:20px;z-index:9999';
    document.body.appendChild(div);
  });
  await page.waitForTimeout(200);

  const skelAnim = await page.evaluate(() => getComputedStyle(document.getElementById('test-skeleton')).animationName);
  check('.wonder-skeleton animation-name (default wonder=full)', skelAnim,
    v => v && v.includes('wonder-shimmer'), '"wonder-shimmer"');

  // Screenshot with skeleton
  await page.screenshot({ path: path.join(SS_DIR, 'w19_skeleton.png') });
  console.log('  Screenshot → e2e/screenshots/w19_skeleton.png');

  // wonder=off
  await page.evaluate(() => document.documentElement.setAttribute('data-wonder', 'off'));
  await page.waitForTimeout(100);
  const skelAnimOff = await page.evaluate(() => getComputedStyle(document.getElementById('test-skeleton')).animationName);
  check('.wonder-skeleton animation-name (wonder=off)', skelAnimOff, v => v === 'none', '"none"');

  await page.evaluate(() => document.documentElement.removeAttribute('data-wonder'));

  // ── Console errors ────────────────────────────────────────────────────────
  console.log('\n── Console errors ──');
  if (errors.length === 0) {
    console.log('  None');
  } else {
    errors.forEach(e => console.log('  ERROR:', e));
  }

  await browser.close();

  // Summary
  console.log('\n── Summary ──');
  for (const [label, { actual, ok }] of Object.entries(results)) {
    console.log(`  ${ok ? '✓' : '✗'} ${label}: ${JSON.stringify(actual)}`);
  }
  console.log(`\n── Result: ${pass ? 'PASS' : 'FAIL'} ──`);
  process.exit(pass ? 0 : 1);
})();
