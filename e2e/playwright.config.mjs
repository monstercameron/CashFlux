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
const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
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
  // One local retry (two in CI): interaction tests drive a CPU-heavy wasm app and
  // can flake transiently under worker contention; a test that fails twice is a
  // real failure, not noise.
  retries: process.env.CI ? 2 : 1,
  // A clean GitHub Windows runner cannot reliably boot two ~80 MB Go/WASM app
  // instances at once: both starve and fail the shared 45-second readiness gate
  // before their test assertions run. Serialize CI for deterministic boots;
  // local machines retain two workers for faster feedback.
  // A clean GitHub Windows runner cannot reliably boot two ~80 MB Go/WASM app
  // instances at once: both starve and fail the shared 45-second readiness gate
  // before their assertions run. That constraint is why the full suite could not
  // simply be parallelised out of its timeout, and why the CI lanes shard across
  // separate runners instead.
  workers: process.env.CI ? 1 : 2,
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
  // Two lanes, because one lane was doing two incompatible jobs.
  //
  //   prod  — the DEPLOY GATE. Every test tagged @prod, and nothing else. It has
  //           to be fast enough to finish and stable enough to be believed,
  //           because the droplet's deploy hook fires on this workflow
  //           succeeding: a gate that times out does not fail loudly, it
  //           silently stops shipping (see deploy/production/README.md).
  //   chromium — the FULL suite, for development and the nightly run. Free to be
  //           slow, free to be exhaustive, free to include the specs that are
  //           currently red for known reasons.
  //
  // Membership is by tag rather than by file so a big spec can contribute its
  // load-bearing case to the gate without dragging its long tail along.
  projects: [
    {
      name: "prod",
      grep: /@prod/,
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  ],
  webServer: EXTERNAL
    ? undefined
    : [{
        command: `node e2e/serve.mjs web ${PORT}`,
        cwd: "..",
        url: BASE,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        stdout: "ignore",
        stderr: "pipe",
      }, {
        command: `node e2e/backend.mjs ${RPC_PORT} ${PORT}`,
        cwd: "..",
        url: `http://127.0.0.1:${RPC_PORT}/v1/version`,
        reuseExistingServer: false,
        timeout: 120_000,
        stdout: "ignore",
        stderr: "pipe",
      }],
});
