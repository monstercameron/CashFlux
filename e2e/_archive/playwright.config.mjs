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
// EXTERNAL mode: the app is already built and served elsewhere (e.g. a host
// serve.go reached from inside the Playwright Docker container via
// host.docker.internal). In that mode we skip globalSetup's wasm build and the
// webServer, and just point at E2E_BASE_URL. Used to generate/compare visual
// snapshots in the same Linux env CI uses.
const EXTERNAL = process.env.E2E_BASE_URL;
const BASE = EXTERNAL || `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: "./regression",
  testMatch: "**/*.spec.mjs",
  fullyParallel: true,
  forbidOnly: !!process.env.CI, // a stray test.only fails the CI build
  retries: process.env.CI ? 2 : 0,
  // Cap workers: each test boots a full wasm app (CPU-heavy), so too many in
  // parallel starve each other and skew render-timing-sensitive checks. Two keeps
  // the suite fast without contention.
  workers: 2,
  reporter: process.env.CI
    ? [["list"], ["html", { open: "never" }], ["github"]]
    : [["list"], ["html", { open: "never" }]],
  // Visual baselines live next to the specs, committed to the repo. The platform
  // suffix keeps Linux (CI + container) baselines separate from any generated
  // elsewhere — visual snapshots are only trustworthy when made in the same env.
  snapshotPathTemplate: "{testDir}/__screenshots__/{testFilePath}/{arg}-{platform}{ext}",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  globalSetup: EXTERNAL ? undefined : "./global-setup.mjs",
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
  webServer: EXTERNAL
    ? undefined
    : {
        command: `node e2e/serve.mjs web ${PORT}`,
        cwd: "..",
        url: BASE,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        stdout: "ignore",
        stderr: "pipe",
      },
});
