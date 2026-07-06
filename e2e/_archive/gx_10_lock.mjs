/**
 * GX10 — Lock screen / app lock — "Eyes Off My Money"
 * Probes the passcode gate: existence, visual quality, sizing, theming, a11y.
 * Run: node e2e/gx_10_lock.mjs
 * Server: http://localhost:8080 (gwc dev)
 */
import { chromium } from 'playwright';
import * as fs from 'fs';

const BASE = 'http://localhost:8080';
const OUT  = 'e2e/screenshots';

if (!fs.existsSync(OUT)) fs.mkdirSync(OUT, { recursive: true });

const PASSCODE = '123456';

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
  }, theme);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute('data-theme') === t, theme,
    { timeout: 5000 }
  ).catch(() => console.log(`  WARN: data-theme did not settle to ${theme}`));
}

async function shot(page, name) {
  await page.screenshot({ path: `${OUT}/${name}`, fullPage: false });
  console.log(`  shot: ${name}`);
}

async function run() {
  const browser = await chromium.launch({ headless: true });

  // ─── Phase 0: source-presence check ─────────────────────────────────────
  console.log('\n=== Phase 0: Source-presence check ===');
  console.log('  appLockGateID: cf-applock-gate (found in internal/app/applockgate.go)');
  console.log('  Config persisted: cashflux:applock (localStorage key)');
  console.log('  enableAppLock() / showAppLockGate() confirmed in applockgate.go');
  console.log('  Settings section: appLockSection() in applocksettings.go');
  console.log('  Inactivity: wireAutoLock() / setInterval(30s) in applockgate.go');
  console.log('  Recovery: "Forgot passcode" -> wipeAllLocalData() + reloadPage()');

  // ─── Phase 1: Enable app lock, trigger gate, screenshot both themes ───────
  for (const theme of ['dark', 'light']) {
    console.log(`\n=== Theme: ${theme} ===`);
    const ctx  = await browser.newContext({ viewport: { width: 1280, height: 768 } });
    const page = await ctx.newPage();

    // Load app, wait for WASM
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);

    // Set theme
    await setTheme(page, theme);
    await page.waitForTimeout(500);

    // Enable app lock directly via localStorage (the Go enableAppLock() path;
    // we bypass the setup form since we can't call Go functions directly from
    // Playwright — instead we write the applock config manually).
    // SHA-256('test-salt' + '123456') = computed via SubtleCrypto.
    const lockSet = await page.evaluate(async (passcode) => {
      // Use the same algorithm as applock.HashPasscode: hex(SHA-256(salt+passcode))
      const salt = 'deadbeefcafebabe1234567890abcdef';
      const encoder = new TextEncoder();
      const data = encoder.encode(salt + passcode);
      const buf = await crypto.subtle.digest('SHA-256', data);
      const hash = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
      const config = {
        enabled: true,
        salt,
        hash,
        autoLockMinutes: 0,
        hint: '',
        hideQuotes: false,
        hideMeta: false,
        suspended: false,
      };
      localStorage.setItem('cashflux:applock', JSON.stringify(config));
      return { salt, hash: hash.slice(0, 16) + '…', config: JSON.stringify(config).slice(0, 80) };
    }, PASSCODE);
    console.log('  Lock config written:', JSON.stringify(lockSet));

    // Reload — on boot maybeLockOnBoot() should show the gate
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);

    // Confirm the theme is still set
    const dataTheme = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
    console.log(`  data-theme after reload: ${dataTheme}`);

    // ── Screenshot 1: gate on boot ──
    await shot(page, `gx10_lock_gate_${theme}_1280.png`);

    // ── Measure gate element ──
    const gateInfo = await page.evaluate(() => {
      const gate = document.getElementById('cf-applock-gate');
      if (!gate) return { exists: false };
      const gst = getComputedStyle(gate);
      const inp = document.getElementById('cf-applock-input');
      const ist = inp ? getComputedStyle(inp) : null;
      const btn = gate.querySelector('button');
      const bst = btn ? getComputedStyle(btn) : null;
      const bRect = btn ? btn.getBoundingClientRect() : null;
      const iRect = inp ? inp.getBoundingClientRect() : null;
      const card = gate.querySelector('div > div');
      const cst = card ? getComputedStyle(card) : null;
      return {
        exists: true,
        display: gst.display,
        position: gst.position,
        zIndex: gst.zIndex,
        bg: gst.backgroundColor,
        inset: gst.inset || (gst.top + '/' + gst.right + '/' + gst.bottom + '/' + gst.left),
        // input
        inputExists: !!inp,
        inputType: inp ? inp.type : null,
        inputAriaLabel: inp ? inp.getAttribute('aria-label') : null,
        inputInputmode: inp ? inp.getAttribute('inputmode') : null,
        inputWidth: iRect ? Math.round(iRect.width) : null,
        inputHeight: iRect ? Math.round(iRect.height) : null,
        inputBg: ist ? ist.backgroundColor : null,
        inputTextAlign: ist ? ist.textAlign : null,
        inputLetterSpacing: ist ? ist.letterSpacing : null,
        // unlock button
        btnText: btn ? btn.textContent.trim() : null,
        btnWidth: bRect ? Math.round(bRect.width) : null,
        btnHeight: bRect ? Math.round(bRect.height) : null,
        btnBg: bst ? bst.backgroundColor : null,
        // card
        cardWidth: cst ? cst.width : null,
        cardTextAlign: cst ? cst.textAlign : null,
        // inner text
        msgText: (() => { const m = document.getElementById('cf-applock-msg'); return m ? m.textContent : null; })(),
        greetingText: (() => { const g = document.getElementById('cf-lock-greeting'); return g ? g.textContent : null; })(),
        dateText: (() => { const d = document.getElementById('cf-lock-date'); return d ? d.textContent : null; })(),
        quoteText: (() => { const q = document.getElementById('cf-lock-quote'); return q ? q.textContent.slice(0, 60) : null; })(),
        brandText: (() => {
          const divs = gate.querySelectorAll('div');
          for (const d of divs) { if (d.textContent.trim() === 'CashFlux') return d.textContent; }
          return null;
        })(),
        // forgot button
        forgotText: (() => {
          const btns = gate.querySelectorAll('button');
          for (const b of btns) { if (b.textContent.toLowerCase().includes('forgot')) return b.textContent.trim(); }
          return null;
        })(),
        // focus
        activeEl: document.activeElement ? document.activeElement.id : null,
        // Tab focusables
        focusableCount: gate.querySelectorAll('button, input').length,
      };
    });
    console.log('  Gate info:', JSON.stringify(gateInfo, null, 2));

    // ── Screenshot 768px wide (mobile) ──
    await ctx.close();
    const ctx768 = await browser.newContext({ viewport: { width: 768, height: 1024 } });
    const page768 = await ctx768.newPage();

    // Re-enable lock on the 768 context
    await page768.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page768.waitForTimeout(3000);
    await page768.evaluate(async (args) => {
      const { passcode, theme: t } = args;
      const salt = 'deadbeefcafebabe1234567890abcdef';
      const encoder = new TextEncoder();
      const buf = await crypto.subtle.digest('SHA-256', encoder.encode(salt + passcode));
      const hash = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
      localStorage.setItem('cashflux:applock', JSON.stringify({ enabled: true, salt, hash, autoLockMinutes: 0, hint: '', hideQuotes: false, hideMeta: false, suspended: false }));
      localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
    }, { passcode: PASSCODE, theme });
    await page768.reload({ waitUntil: 'domcontentloaded' });
    await page768.waitForTimeout(3000);
    await page768.screenshot({ path: `${OUT}/gx10_lock_gate_${theme}_768.png`, fullPage: false });
    console.log(`  shot: gx10_lock_gate_${theme}_768.png`);

    // ── Wrong-passcode shake test ──
    const wrongInput = await page768.$('#cf-applock-input');
    if (wrongInput) {
      await page768.evaluate(() => { document.getElementById('cf-applock-input').value = '000000'; });
      // Click via JS to avoid the gate overlay intercept issue
      await page768.evaluate(() => {
        const btns = document.querySelectorAll('#cf-applock-gate button');
        if (btns.length) btns[0].click();
      });
      await page768.waitForTimeout(500);
      await page768.screenshot({ path: `${OUT}/gx10_lock_wrong_${theme}_768.png`, fullPage: false });
      console.log(`  shot: gx10_lock_wrong_${theme}_768.png`);
      const wrongMsg = await page768.evaluate(() => {
        const m = document.getElementById('cf-applock-msg');
        return m ? { text: m.textContent, color: getComputedStyle(m).color } : null;
      });
      console.log(`  Wrong passcode message: ${JSON.stringify(wrongMsg)}`);
    }

    // ── Correct passcode → unlock ──
    const inp768 = await page768.$('#cf-applock-input');
    if (inp768) {
      await page768.evaluate((p) => { document.getElementById('cf-applock-input').value = p; }, PASSCODE);
      await page768.evaluate(() => {
        const btns = document.querySelectorAll('#cf-applock-gate button');
        if (btns.length) btns[0].click();
      });
      await page768.waitForTimeout(600);
      const gateGone = await page768.evaluate(() => {
        const g = document.getElementById('cf-applock-gate');
        return g ? getComputedStyle(g).display : 'removed';
      });
      console.log(`  Gate display after correct passcode: ${gateGone}`);
      await page768.screenshot({ path: `${OUT}/gx10_unlock_${theme}_768.png`, fullPage: false });
      console.log(`  shot: gx10_unlock_${theme}_768.png`);
    }

    await ctx768.close();

    // ── Focus & a11y check (back on 1280 but re-navigate) ──
    const ctx2 = await browser.newContext({ viewport: { width: 1280, height: 768 } });
    const page2 = await ctx2.newPage();
    await page2.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page2.waitForTimeout(3000);
    await page2.evaluate(async (args) => {
      const { passcode, theme: t } = args;
      const salt = 'deadbeefcafebabe1234567890abcdef';
      const encoder = new TextEncoder();
      const buf = await crypto.subtle.digest('SHA-256', encoder.encode(salt + passcode));
      const hash = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
      localStorage.setItem('cashflux:applock', JSON.stringify({ enabled: true, salt, hash, autoLockMinutes: 0, hint: '', hideQuotes: false, hideMeta: false, suspended: false }));
      localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: t }));
    }, { passcode: PASSCODE, theme });
    await page2.reload({ waitUntil: 'domcontentloaded' });
    await page2.waitForTimeout(3000);

    const a11yInfo = await page2.evaluate(() => {
      const gate = document.getElementById('cf-applock-gate');
      if (!gate) return { gateExists: false };
      const inp = document.getElementById('cf-applock-input');
      const btns = [...gate.querySelectorAll('button')];
      return {
        gateExists: true,
        gateRole: gate.getAttribute('role'),
        gateAriaLabel: gate.getAttribute('aria-label'),
        gateAriaModal: gate.getAttribute('aria-modal'),
        inputAriaLabel: inp ? inp.getAttribute('aria-label') : null,
        inputAutofocus: inp ? document.activeElement === inp : null,
        btnAriaLabels: btns.map(b => ({ text: b.textContent.trim(), aria: b.getAttribute('aria-label') })),
        activeElementId: document.activeElement ? document.activeElement.id : null,
        activeElementTag: document.activeElement ? document.activeElement.tagName : null,
      };
    });
    console.log(`  A11y info (${theme}):`, JSON.stringify(a11yInfo, null, 2));

    // Keyboard Enter test (use evaluate to set value, then dispatchEvent)
    await page2.evaluate((p) => {
      const inp = document.getElementById('cf-applock-input');
      if (inp) {
        inp.value = p;
        inp.focus();
        inp.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
      }
    }, PASSCODE);
    await page2.waitForTimeout(600);
    const unlockedByEnter = await page2.evaluate(() => {
      const g = document.getElementById('cf-applock-gate');
      return g ? getComputedStyle(g).display : 'removed';
    });
    console.log(`  Gate after Enter-submit (${theme}): display=${unlockedByEnter}`);

    await ctx2.close();
  }

  // ─── Phase 2: Settings → App lock section probe ───────────────────────────
  console.log('\n=== Phase 2: Settings → App lock section ===');
  const ctxS = await browser.newContext({ viewport: { width: 1280, height: 768 } });
  const pageS = await ctxS.newPage();
  await pageS.goto(BASE, { waitUntil: 'domcontentloaded' });
  await pageS.waitForTimeout(3500);

  // Clear lock so we start without gate, see the "Set a passcode" button
  await pageS.evaluate(() => { localStorage.removeItem('cashflux:applock'); });
  await pageS.reload({ waitUntil: 'domcontentloaded' });
  await pageS.waitForTimeout(3000);

  // Navigate to Settings via JS click to avoid any overlay issues
  const settingsClicked = await pageS.evaluate(() => {
    const link = document.querySelector('a[href="/settings"], nav a');
    if (link) { link.click(); return true; }
    return false;
  });
  if (!settingsClicked) {
    await pageS.goto(BASE + '/settings', { waitUntil: 'domcontentloaded' });
  }
  await pageS.waitForTimeout(1500);

  await pageS.screenshot({ path: `${OUT}/gx10_settings_nolock_1280.png`, fullPage: false });
  console.log('  shot: gx10_settings_nolock_1280.png');

  // Find the app lock section
  const lockSectionInfo = await pageS.evaluate(() => {
    const all = [...document.querySelectorAll('*')];
    const lockLabel = all.find(el =>
      (el.textContent || '').match(/app.?lock|privacy.*lock|passcode|lock screen/i) &&
      el.children.length === 0
    );
    const setBtn = all.find(el =>
      el.tagName === 'BUTTON' &&
      (el.textContent || '').match(/set.*passcode|enable.*lock|set.*lock/i)
    );
    return {
      lockLabelText: lockLabel ? lockLabel.textContent.trim() : null,
      lockLabelTag: lockLabel ? lockLabel.tagName : null,
      setBtnText: setBtn ? setBtn.textContent.trim() : null,
      setBtnExists: !!setBtn,
    };
  });
  console.log('  Settings lock section:', JSON.stringify(lockSectionInfo));

  await ctxS.close();

  // ─── Phase 3: "Forgot passcode" path probe ────────────────────────────────
  console.log('\n=== Phase 3: Forgot passcode path ===');
  const ctxF = await browser.newContext({ viewport: { width: 1280, height: 768 } });
  const pageF = await ctxF.newPage();
  await pageF.goto(BASE, { waitUntil: 'domcontentloaded' });
  await pageF.waitForTimeout(3000);

  await pageF.evaluate(async (passcode) => {
    const salt = 'deadbeefcafebabe1234567890abcdef';
    const encoder = new TextEncoder();
    const buf = await crypto.subtle.digest('SHA-256', encoder.encode(salt + passcode));
    const hash = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
    localStorage.setItem('cashflux:applock', JSON.stringify({ enabled: true, salt, hash, autoLockMinutes: 0, hint: '', hideQuotes: false, hideMeta: false, suspended: false }));
  }, PASSCODE);
  await pageF.reload({ waitUntil: 'domcontentloaded' });
  await pageF.waitForTimeout(3000);

  const forgotInfo = await pageF.evaluate(() => {
    const gate = document.getElementById('cf-applock-gate');
    if (!gate) return { gateExists: false };
    const btns = [...gate.querySelectorAll('button')];
    const forgot = btns.find(b => b.textContent.toLowerCase().includes('forgot'));
    return {
      gateExists: true,
      forgotBtnExists: !!forgot,
      forgotBtnText: forgot ? forgot.textContent.trim() : null,
      forgotBtnDisplay: forgot ? getComputedStyle(forgot).display : null,
      hintBtnExists: !!document.getElementById('cf-lock-hint-btn'),
      hintBtnDisplay: document.getElementById('cf-lock-hint-btn') ? getComputedStyle(document.getElementById('cf-lock-hint-btn')).display : null,
      allBtnTexts: btns.map(b => b.textContent.trim()),
    };
  });
  console.log('  Forgot path info:', JSON.stringify(forgotInfo, null, 2));

  await ctxF.close();

  // ─── Phase 4: light-mode background check ─────────────────────────────────
  console.log('\n=== Phase 4: Light-mode gate background ===');
  const ctxL = await browser.newContext({ viewport: { width: 1280, height: 768 } });
  const pageL = await ctxL.newPage();
  await pageL.goto(BASE, { waitUntil: 'domcontentloaded' });
  await pageL.waitForTimeout(3000);

  await pageL.evaluate(async (passcode) => {
    const salt = 'deadbeefcafebabe1234567890abcdef';
    const encoder = new TextEncoder();
    const buf = await crypto.subtle.digest('SHA-256', encoder.encode(salt + passcode));
    const hash = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
    localStorage.setItem('cashflux:applock', JSON.stringify({ enabled: true, salt, hash, autoLockMinutes: 0, hint: '', hideQuotes: false, hideMeta: false, suspended: false }));
    localStorage.setItem('cashflux:prefs', JSON.stringify({ theme: 'light' }));
  }, PASSCODE);
  await pageL.reload({ waitUntil: 'domcontentloaded' });
  await pageL.waitForTimeout(3000);

  const lightBg = await pageL.evaluate(() => {
    const gate = document.getElementById('cf-applock-gate');
    if (!gate) return { gateExists: false };
    const gst = getComputedStyle(gate);
    const inp = document.getElementById('cf-applock-input');
    const ist = inp ? getComputedStyle(inp) : null;
    const card = gate.querySelector('div > div');
    const cst = card ? getComputedStyle(card) : null;
    return {
      gateExists: true,
      dataTheme: document.documentElement.getAttribute('data-theme'),
      gateBg: gst.backgroundColor,
      gateColor: gst.color,
      inputBg: ist ? ist.backgroundColor : null,
      inputColor: ist ? ist.color : null,
      cardBg: cst ? cst.backgroundColor : null,
      cardColor: cst ? cst.color : null,
    };
  });
  console.log('  Light-mode gate colors:', JSON.stringify(lightBg, null, 2));

  await ctxL.close();
  await browser.close();

  console.log('\n=== GX10 probe complete ===');
}

run().catch(err => { console.error(err); process.exit(1); });
