'use strict';
const { chromium } = require('C:\\Users\\mreca\\Desktop\\CashFlux\\.tools\\node_modules\\playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ storageState: undefined });
  const page = await ctx.newPage();
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' && !/already exited/i.test(msg.text())) errors.push(msg.text());
  });

  // Clear localStorage so we start fresh (no stale state)
  await page.goto('http://127.0.0.1:8099/', { waitUntil: 'networkidle' });
  await page.evaluate(() => localStorage.clear());
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForTimeout(3000);

  // Click sample-start-fresh
  const sf = await page.$('[data-testid="sample-start-fresh"]');
  console.log('sample-start-fresh present:', !!sf);
  if (sf) {
    await sf.click();
    // Wait for wasm to re-init with sample data
    await page.waitForTimeout(4000);
  }

  // Check how many members are in state
  const bodyAtHome = await page.evaluate(() => document.body.innerText);
  console.log('Home text snippet:', bodyAtHome.substring(0, 300));

  // Navigate to /members via in-app router
  await page.click('text=Members', { timeout: 5000 }).catch(() => {});
  await page.waitForTimeout(2000);

  const bodyText = await page.evaluate(() => document.body.innerText);
  const hasSpend = /spending this period/i.test(bodyText);
  const hasNetWorth = /net worth by member/i.test(bodyText);
  const hasMarcus = /marcus/i.test(bodyText);

  console.log('"Spending this period" present:', hasSpend);
  console.log('"Net worth by member" present:', hasNetWorth);
  console.log('Marcus in page:', hasMarcus);
  console.log('Members page snippet:', bodyText.substring(bodyText.indexOf('Household'), bodyText.indexOf('Household') + 500));
  console.log('JS errors:', errors.length, errors.slice(0, 3));

  await page.screenshot({ path: 'e2e/screenshots/c280_v3.png', fullPage: true });
  await browser.close();

  if (!hasSpend) { console.error('FAIL: spending section missing'); process.exit(1); }
  console.log('PASS C280');
})();
