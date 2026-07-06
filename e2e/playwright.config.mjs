// Playwright Test configuration for the CashFlux regression suite.
//
// Enterprise properties this encodes:
//   - Hermetic: globalSetup builds the wasm + wasm_exec.js; webServer owns the
//     static server (e2e/serve.go) on a fixed port, torn down after the run. No
//     dependency on a hand-started `gwc dev`.
//   - Deterministic: no test may sleep on the wall clock — web-first assertions
//     only (enforced by convention + the shared fixtures). Retries only in CI.
//   - Diagnosable: trace + screenshot + video retained on failure, HTML report.
//
// The trusted, CI-gated tests live in ./regression/*.spec.mjs. Everything under
// e2e/_archive/ is legacy scratch and is NOT run here.
import { defineConfig, devices } from "@playwright/test";

const PORT = process.env.E2E_PORT || "8099";
const BASE = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: "./regression",
  testMatch: "**/*.spec.mjs",
  fullyParallel: true,
  forbidOnly: !!process.env.CI, // a stray test.only fails the CI build
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI
    ? [["list"], ["html", { open: "never" }], ["github"]]
    : [["list"], ["html", { open: "never" }]],
  // Visual baselines live next to the specs, committed to the repo.
  snapshotPathTemplate: "{testDir}/__screenshots__/{testFilePath}/{arg}{ext}",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  globalSetup: "./global-setup.mjs",
  use: {
    baseURL: BASE,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    // Blocking service workers keeps a stale cached wasm from masking a change.
    serviceWorkers: "block",
    reducedMotion: "reduce",
    launchOptions: { args: ["--disable-gpu"] },
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  ],
  webServer: {
    command: `go run e2e/serve.go web ${PORT}`,
    cwd: "..",
    url: BASE,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    stdout: "ignore",
    stderr: "pipe",
  },
});
