// assistant_share.spec.mjs — the pre-send data-sharing preview: "What's
// shared?" discloses exactly what the next assistant message carries, with a
// rough token estimate, before anything leaves the device.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("assistant: pre-send data-sharing preview", () => {
  test("the What's-shared chip expands the disclosure with a token estimate", async ({ app }) => {
    await nav(app, "/insights");
    const chip = app.getByTestId("assistant-share-chip");
    await chip.scrollIntoViewIfNeeded();
    await expect(chip).toHaveAttribute("aria-expanded", "false");
    await chip.click();
    const panel = app.getByTestId("assistant-share-panel");
    await expect(panel).toBeVisible();
    await expect(panel).toContainText(/Sent with your next message/);
    await expect(panel).toContainText(/Privacy: (full detail|aggregates only)/);
    await expect(panel).toContainText(/Headline aggregates:/);
    await expect(panel).toContainText(/category names/);
    await expect(app.getByTestId("assistant-share-tokens")).toContainText(/≈ \d+ tokens/);
    // Collapses again.
    await chip.click();
    await expect(app.getByTestId("assistant-share-panel")).toHaveCount(0);
  });
});
