'use strict';
const { chromium } = require('C:\\Users\\mreca\\Desktop\\CashFlux\\.tools\\node_modules\\playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' && !/already exited/i.test(msg.text())) errors.push(msg.text());
  });

  // Hard reload to bypass SW cache
  await page.goto('http://127.0.0.1:8099/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);

  // Click sample-start-fresh if present
  const sf = await page.$('[data-testid="sample-start-fresh"]');
  if (sf) {
    await sf.click();
    await page.waitForTimeout(3000); // wait for wasm re-init
  }

  // Now navigate to members using in-app navigation
  await page.evaluate(() => {
    window.history.pushState({}, '', '/members');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await page.waitForTimeout(2000);

  const bodyText = await page.evaluate(() => document.body.innerText);
  const hasSpend = /spending this period/i.test(bodyText);
  const hasNetWorth = /net worth by member/i.test(bodyText);
  const memberCount = (bodyText.match(/\d+ members/i) || ['unknown'])[0];

  console.log('Member count text:', memberCount);
  console.log('"Spending this period" present:', hasSpend);
  console.log('"Net worth by member" present:', hasNetWorth);
  console.log('JS errors:', errors.length);

  await page.screenshot({ path: 'e2e/screenshots/c280_v2.png', fullPage: true });
  await browser.close();

  if (!hasSpend) { console.error('FAIL: spending section missing'); process.exit(1); }
  if (!hasNetWorth) { console.error('FAIL: net worth regression'); process.exit(1); }
  if (errors.length) { console.error('FAIL: JS errors', errors); process.exit(1); }
  console.log('PASS C280');
})();
