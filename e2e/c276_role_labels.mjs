// C276 gate — member rows show real role labels (Owner/Admin/Viewer) and the
// default-member seed chip is visually distinct from the role badge.
// Exits non-zero on any failure. Never kills the user's real browser.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
let passed = 0;
let failed = 0;
const fail = (m) => { console.error("FAIL: " + m); failed++; process.exitCode = 1; };
const pass = (m) => { console.log("PASS: " + m); passed++; };

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  // Wait for at least one member row to appear (sample data seeds members).
  await page.waitForSelector("[data-testid^='member-role-badge-']", { timeout: 60000 });

  // 1. Every role badge must show a real role string (Owner, Admin, or Viewer).
  const roleBadges = page.locator("[data-testid^='member-role-badge-']");
  const roleCount = await roleBadges.count();
  if (roleCount === 0) {
    fail("no role badges found on members screen");
  } else {
    pass(`found ${roleCount} role badge(s)`);
    for (let i = 0; i < roleCount; i++) {
      const text = (await roleBadges.nth(i).innerText()).trim();
      if (!/^(owner|admin|viewer)$/i.test(text)) {
        fail(`role badge ${i} has unexpected text "${text}" (want Owner|Admin|Viewer)`);
      } else {
        pass(`role badge ${i}: "${text}"`);
      }
    }
  }

  // 2. The default member must have a separate "default" chip.
  const defaultChips = page.locator("[data-testid^='member-default-chip-']");
  const defaultCount = await defaultChips.count();
  if (defaultCount === 0) {
    fail("no default-member chip found (expected at least one IsDefault member in sample data)");
  } else {
    pass(`found ${defaultCount} default-member chip(s)`);
    // The chip must NOT read "Owner", "Admin", or "Viewer" — it should carry the
    // "defaultBadge" label (e.g. "Default" or similar) to keep concepts distinct.
    for (let i = 0; i < defaultCount; i++) {
      const text = (await defaultChips.nth(i).innerText()).trim();
      if (/^(owner|admin|viewer)$/i.test(text)) {
        fail(`default chip ${i} shows a role string "${text}" — must be a distinct label`);
      } else {
        pass(`default chip ${i}: "${text}" (distinct from role)`);
      }
    }
  }

  // 3. The default member's row must show BOTH a role badge AND a default chip.
  if (defaultCount > 0) {
    const firstChip = defaultChips.first();
    const chipTestId = await firstChip.getAttribute("data-testid");
    // Extract the member ID from "member-default-chip-<id>".
    const memberID = chipTestId.replace("member-default-chip-", "");
    const roleBadge = page.locator(`[data-testid="member-role-badge-${memberID}"]`);
    const roleText = (await roleBadge.innerText()).trim();
    if (!/^(owner|admin|viewer)$/i.test(roleText)) {
      fail(`default member (id=${memberID}) role badge shows "${roleText}" not a real role`);
    } else {
      pass(`default member (id=${memberID}) has role badge "${roleText}" AND a default chip`);
    }
  }

  // 4. Take a screenshot for visual review.
  const screenshotPath = path.join(__dirname, "c276_role_labels.png");
  await page.screenshot({ path: screenshotPath, fullPage: false });
  pass(`screenshot saved to ${screenshotPath}`);

  if (errors.length) fail("page JS errors: " + errors.join(" | "));
} finally {
  await browser.close();
  console.log(`\n${passed} PASS · ${failed} FAIL`);
}
