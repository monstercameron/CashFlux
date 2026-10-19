import { test, expect, boot, nav } from "./fixtures.mjs";

test("inspect live sync status surfaces", async ({ page }) => {
  await boot(page);
  await expect(page.locator('[data-testid="sample-start-fresh"]')).toBeVisible();
  await nav(page, "/settings");
  await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();

  await expect(page.getByTestId("cloud-setup-start")).toBeVisible();
  await page.getByTestId("cloud-setup-start").click();
  await expect(page.getByTestId("cloud-setup-where")).toBeVisible();

  const result = await page.evaluate(() => {
    const describe = (selector) => {
      const el = document.querySelector(selector);
      if (!el) return { present: false };
      const style = getComputedStyle(el);
      const rect = el.getBoundingClientRect();
      return {
        present: true,
        text: el.textContent?.trim() || "",
        state: el.getAttribute("data-sync-state"),
        className: el.className,
        display: style.display,
        visibility: style.visibility,
        opacity: style.opacity,
        width: rect.width,
        height: rect.height,
        title: el.getAttribute("title"),
      };
    };
    return {
      pulse: describe('[data-testid="sync-pulse"]'),
      pulseSlot: describe(".sync-pulse-slot"),
      railChip: describe('[data-testid="sync-chip"]'),
      cloudStatus: describe('[data-testid="sync-status-card"]'),
      toolbarText: document.querySelector(".tb-actions")?.textContent?.trim() || "",
    };
  });
  console.log("SYNC_STATUS_INSPECTION=" + JSON.stringify(result));
  await page.screenshot({ path: "test-results/dev-status-inspect.png", fullPage: true });
});
