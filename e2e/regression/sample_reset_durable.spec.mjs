import { test, expect } from "./fixtures.mjs";

test("starting fresh waits for the sample save and stays empty after reload", async ({ app }) => {
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([
    app.waitForEvent("load", { timeout: 30_000 }),
    app.getByTestId("sample-start-fresh").click(),
  ]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await expect(app.getByTestId("sample-data-banner")).toHaveCount(0);
  await app.reload();
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await expect(app.getByTestId("sample-data-banner")).toHaveCount(0);
});
