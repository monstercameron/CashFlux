const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on('console', m => { if (m.type() === 'error') errors.push('console: ' + m.text()); });
  page.on('pageerror', e => errors.push('pageerror: ' + e.message));

  const t0 = Date.now();
  await page.goto('http://localhost:8082/login', { waitUntil: 'load', timeout: 120000 });
  console.log('load ms:', Date.now() - t0);

  // Wait for the WASM app to paint something.
  try { await page.waitForFunction(() => document.body.innerText.trim().length > 0, { timeout: 90000 }); }
  catch { console.log('!! body never got text'); }

  console.log('title:', await page.title());
  console.log('--- visible text ---');
  console.log((await page.innerText('body')).slice(0, 900));
  console.log('--- inputs ---');
  for (const el of await page.$$('input')) {
    console.log(JSON.stringify({
      type: await el.getAttribute('type'), id: await el.getAttribute('id'),
      value: await el.inputValue().catch(() => null),
    }));
  }
  console.log('--- buttons ---');
  for (const b of await page.$$('button')) {
    console.log(JSON.stringify({ text: (await b.innerText()).trim(), disabled: await b.isDisabled() }));
  }
  console.log('--- cookies:', JSON.stringify(await ctx.cookies()));
  if (errors.length) { console.log('--- JS errors ---'); errors.slice(0, 12).forEach(e => console.log(e)); }
  await browser.close();
})();
