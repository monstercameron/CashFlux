'use strict';
const { chromium } = require('C:\\Users\\mreca\\Desktop\\CashFlux\\.tools\\node_modules\\playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  // Bypass SW by using a fresh context
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error' && !/already exited/i.test(msg.text())) errors.push(msg.text());
  });

  // Load the app — don't clear LS, just see what's there
  await page.goto('http://127.0.0.1:8099/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(4000);

  const hasBanner = await page.$('[data-testid="sample-data-banner"]');
  const hasAddBtn = await page.$('[data-testid="add-transaction-btn"]');
  console.log('Sample banner:', !!hasBanner, 'Add btn:', !!hasAddBtn);

  // If no sample data, try to load it via the hero button
  const heroBtn = await page.$('text=Load sample data');
  if (heroBtn) {
    console.log('Clicking "Load sample data"');
    await heroBtn.click();
    await page.waitForTimeout(4000);
  }

  // Navigate to members
  await page.goto('http://127.0.0.1:8099/members', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  const bodyText = await page.evaluate(() => document.body.innerText);
  const hasSpend = /spending this period/i.test(bodyText);
  const hasNetWorth = /net worth by member/i.test(bodyText);
  const hasMarcus = /marcus/i.test(bodyText);
  const memberCountMatch = bodyText.match(/(\d+) members/i);

  console.log('Member count:', memberCountMatch ? memberCountMatch[0] : 'not found');
  console.log('"Spending this period":', hasSpend);
  console.log('"Net worth by member":', hasNetWorth);
  console.log('Marcus:', hasMarcus);
  if (!hasSpend) {
    // Print the relevant portion
    const idx = bodyText.indexOf('Household members');
    console.log('Members section text:', bodyText.substring(idx, idx + 800));
  }

  await page.screenshot({ path: 'e2e/screenshots/c280_v4.png', fullPage: true });
  await browser.close();

  if (!hasSpend) { console.error('FAIL: spending section missing'); process.exit(1); }
  if (errors.length) { console.error('FAIL: JS errors', errors); process.exit(1); }
  console.log('PASS C280');
})();
