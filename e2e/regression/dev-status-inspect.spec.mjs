import { test, expect, boot, nav } from "./fixtures.mjs";

test("inspect live sync status surfaces", async ({ page }) => {
  await boot(page);
  await expect(page.locator('[data-testid="sample-start-fresh"]')).toBeVisible();
  await nav(page, "/settings");
  await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();

  const toggle = page.locator('[role="switch"][aria-label="Sync with a backend server"]');
  if ((await toggle.getAttribute("aria-checked")) !== "true") {
    await toggle.click();
  }
  await expect(toggle).toHaveAttribute("aria-checked", "true");

  const remote = page.locator(".cloud-tab button", { hasText: "Remote" });
  if (await remote.count()) {
    await remote.click();
  }
  await page.locator('[data-testid="sync-server-url"]').fill("http://127.0.0.1:8198");
  await expect(page.locator(".cloud-tab")).toContainText("Connected.", { timeout: 15_000 });

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
