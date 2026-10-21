// Warm-route benchmark for the nine primary CashFlux pages.
//
// This is intentionally separate from perf-audit.mjs: it does not build wasm,
// start a server, score pages, or update versioned historical results. Point it at
// the already-built app that is being compared and save each run under a distinct
// before/after filename.
//
//   node e2e/perf/primary-pages-benchmark.mjs --base-url http://127.0.0.1:8099 \
//     --label before --runs 5 --out e2e/perf/results/primary-before.json
// For a paired source-artifact comparison on the same server, pass both
// --wasm-file and its verified --build-id. That path uses the app's gzip fallback
// so Playwright does not transfer an identity WASM body larger than its 100 MiB cap.
import { chromium } from "@playwright/test";
import { createHash } from "node:crypto";
import { cpus } from "node:os";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, "..", "..");
const ROUTES = [
  ["Dashboard", "/"],
  ["Transactions", "/transactions"],
  ["Accounts", "/accounts"],
  ["Budgets", "/budgets"],
  ["Goals", "/goals"],
  ["Todo", "/todo"],
  ["Notifications", "/notifications"],
  ["Assistant", "/assistant"],
  ["Reports", "/reports"],
];
const FIXED_NOW = "2026-07-01T12:00:00.000Z";
let fatalReport = null;

function options(argv) {
  const out = { baseUrl: process.env.E2E_BASE_URL || "", label: "run", buildId: "", wasmFile: "", runs: 5, warmups: 1, outFile: "", profileDir: "", profileRoute: "/", profileRouteSet: false };
  const keys = { "--base-url": "baseUrl", "--label": "label", "--build-id": "buildId", "--wasm-file": "wasmFile", "--runs": "runs", "--warmups": "warmups", "--out": "outFile", "--profile-dir": "profileDir", "--profile-route": "profileRoute" };
  for (let i = 0; i < argv.length; i++) {
    const [flag, inline] = argv[i].split(/=(.*)/s, 2);
    if (flag === "--help" || flag === "-h") {
      console.log("Usage: node e2e/perf/primary-pages-benchmark.mjs --base-url URL --label NAME [--wasm-file PATH --build-id SHA256] [--runs 5] [--warmups 1] [--out FILE] [--profile-dir DIR [--profile-route /]]");
      process.exit(0);
    }
    const key = keys[flag];
    if (!key) throw new Error(`unknown option: ${flag}`);
    const value = inline ?? argv[++i];
    if (!value) throw new Error(`missing value for ${flag}`);
    out[key] = value;
    if (key === "profileRoute") out.profileRouteSet = true;
  }
  out.runs = Number(out.runs);
  out.warmups = Number(out.warmups);
  if (!out.baseUrl) throw new Error("pass --base-url for the already-built app; the benchmark does not build or start a server");
  const parsed = new URL(out.baseUrl);
  if (parsed.protocol !== "http:" || !["localhost", "127.0.0.1", "[::1]"].includes(parsed.hostname)) {
    throw new Error("--base-url must be a loopback HTTP address so the run stays on the local sample workspace");
  }
  if (!Number.isInteger(out.runs) || out.runs < 1) throw new Error("--runs must be a positive integer");
  if (!Number.isInteger(out.warmups) || out.warmups < 0) throw new Error("--warmups must be a non-negative integer");
  if (Boolean(out.wasmFile) !== Boolean(out.buildId)) throw new Error("--wasm-file and --build-id must be supplied together");
  if (!out.profileDir && out.profileRouteSet) throw new Error("--profile-route requires --profile-dir");
  if (out.profileDir && !ROUTES.some(([, route]) => route === out.profileRoute)) throw new Error(`--profile-route must be one of: ${ROUTES.map(([, route]) => route).join(", ")}`);
  if (out.profileDir) out.profileDir = path.resolve(process.cwd(), out.profileDir);
  out.baseUrl = out.baseUrl.replace(/\/$/, "");
  fatalReport = { label: out.label, baseUrl: out.baseUrl, outFile: out.outFile, cpuProfiling: { enabled: Boolean(out.profileDir), profiler: "Chromium CDP Profiler", route: out.profileRoute, outputDirectory: out.profileDir || null, expectedProfiles: out.profileDir ? out.runs : 0, warmupsProfiled: 0, timingsExcludedFromPerformanceMedians: true, profiles: [] } };
  return out;
}

const round = (n) => n == null ? null : Math.round(n * 10) / 10;
const median = (xs) => {
  const values = xs.filter(Number.isFinite).sort((a, b) => a - b);
  if (!values.length) return null;
  const mid = Math.floor(values.length / 2);
  return round(values.length % 2 ? values[mid] : (values[mid - 1] + values[mid]) / 2);
};
const safeFilePart = (value) => String(value).replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 80) || "run";

async function main() {
  const opts = options(process.argv.slice(2));
  const cpuProfiling = fatalReport.cpuProfiling;
  const profileRunId = new Date().toISOString().replace(/[^0-9TZ]/g, "-");
  if (opts.profileDir) mkdirSync(opts.profileDir, { recursive: true });
  let wasmArtifact = null;
  const wasmDelivery = {
    mode: opts.wasmFile ? "injected-gzip-fallback" : "server-negotiated",
    artifactPath: null,
    artifactSha256: opts.wasmFile ? null : "not recorded",
    artifactByteLength: null,
    artifactGzipByteLength: null,
    rawInterceptionCount: 0,
    gzipFallbackInterceptionCount: 0,
    unexpectedFallbackAttempts: [],
    responseContentType: opts.wasmFile ? "application/gzip" : "server-negotiated",
    responseContentEncoding: opts.wasmFile ? "none; decompressed by page loader" : "server-negotiated",
  };
  fatalReport.wasmDelivery = wasmDelivery;
  if (opts.wasmFile) {
    const artifactPath = path.resolve(process.cwd(), opts.wasmFile);
    const bytes = readFileSync(artifactPath);
    if (bytes.length < 8 || !bytes.subarray(0, 4).equals(Buffer.from([0x00, 0x61, 0x73, 0x6d]))) {
      throw new Error("--wasm-file is not a WebAssembly artifact with a valid magic header");
    }
    if (!bytes.subarray(4, 8).equals(Buffer.from([0x01, 0x00, 0x00, 0x00]))) {
      throw new Error("--wasm-file has an unsupported WebAssembly version header");
    }
    const sha256 = createHash("sha256").update(bytes).digest("hex");
    if (!/^[0-9a-f]{64}$/i.test(opts.buildId) || sha256 !== opts.buildId.toLowerCase()) {
      throw new Error(`--wasm-file SHA-256 ${sha256} does not match --build-id ${opts.buildId}`);
    }
    const gzipBytes = gzipSync(bytes, { level: 9, mtime: 0 });
    wasmArtifact = { bytes, gzipBytes };
    opts.buildId = sha256;
    Object.assign(wasmDelivery, {
      artifactPath,
      artifactSha256: sha256,
      artifactByteLength: bytes.length,
      artifactGzipByteLength: gzipBytes.length,
    });
  }
  const browser = await chromium.launch({ args: ["--disable-gpu", "--enable-precise-memory-info"] });
  try {
    // A new browser context is the disposable sample workspace: its IndexedDB,
    // local storage, cookies, and service-worker state cannot touch a real profile.
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      deviceScaleFactor: 1,
      reducedMotion: "reduce",
      serviceWorkers: "block",
      locale: "en-US",
      timezoneId: "America/New_York",
    });
    const page = await context.newPage();
    // Keep all app traffic on loopback, including optional AI integrations. The
    // benchmark only reads the built-in sample pages; no paid/external request is
    // needed for route render measurements.
    const baseOrigin = new URL(opts.baseUrl).origin;
    await page.route("**/*", (route) => {
      const requestUrl = new URL(route.request().url());
      if (!["localhost", "127.0.0.1", "[::1]"].includes(requestUrl.hostname)) return route.abort();
      if (wasmArtifact && requestUrl.origin === baseOrigin && route.request().method() === "GET") {
        // Identity delivery exceeds the browser automation protocol's 100 MiB
        // response cap for these artifacts. Trigger the app's existing gzip
        // compatibility path and fulfill that request with the exact artifact.
        // The body is gzip bytes with no Content-Encoding: the page itself uses
        // DecompressionStream before instantiateStreaming.
        if (requestUrl.pathname === "/bin/main.wasm") {
          wasmDelivery.rawInterceptionCount++;
          return route.fulfill({ status: 404, headers: { "Cache-Control": "no-store" }, body: "" });
        }
        if (requestUrl.pathname === "/bin/main.wasm.gz") {
          wasmDelivery.gzipFallbackInterceptionCount++;
          return route.fulfill({
            status: 200,
            headers: { "Content-Type": "application/gzip", "Cache-Control": "no-store" },
            body: wasmArtifact.gzipBytes,
          });
        }
        if (requestUrl.pathname === "/bin/main.wasm.br") {
          wasmDelivery.unexpectedFallbackAttempts.push({ path: requestUrl.pathname, search: requestUrl.search });
          return route.abort();
        }
      }
      return route.continue();
    });
    await page.clock.setFixedTime(new Date(FIXED_NOW));
    await page.addInitScript(() => {
      // The isolated sample should expose the complete ledger on Transactions,
      // rather than the default current-month slice. This is persisted only in
      // this disposable browser context.
      try {
        localStorage.setItem("cashflux:tx-filter", JSON.stringify({ periodPreset: "all-dates" }));
      } catch (_) {}
      window.__primaryBenchBoot = { start: performance.now(), ready: null, longTaskMs: 0, longTaskCount: 0 };
      try {
        new PerformanceObserver((list) => {
          for (const entry of list.getEntries()) {
            if (entry.duration > 50) {
              window.__primaryBenchBoot.longTaskMs += entry.duration - 50;
              window.__primaryBenchBoot.longTaskCount++;
            }
          }
        }).observe({ type: "longtask", buffered: true });
      } catch (_) {}
      const ready = new MutationObserver(() => {
        const root = document.documentElement;
        if (root && root.getAttribute("data-app-ready") === "true") {
          window.__primaryBenchBoot.ready = performance.now();
          ready.disconnect();
        }
      });
      const root = document.documentElement;
      if (root?.getAttribute("data-app-ready") === "true") {
        window.__primaryBenchBoot.ready = performance.now();
      } else {
        // addInitScript can run before <html> has been parsed. Observing the
        // document catches the root and subsequent attribute mutation safely.
        ready.observe(document, { attributes: true, subtree: true, attributeFilter: ["data-app-ready"] });
      }
      // The production router may use View Transitions; removing animation keeps
      // this navigation benchmark about rendering work, not transition duration.
      try {
        const proto = Document.prototype;
        if (proto && "startViewTransition" in proto) {
          proto.startViewTransition = function (callback) {
            try { if (typeof callback === "function") callback(); } catch (_) {}
            const done = Promise.resolve();
            return { finished: done, ready: done, updateCallbackDone: done, skipTransition() {} };
          };
        }
      } catch (_) {}
    });

    const bootWallStart = Date.now();
    await page.goto(`${opts.baseUrl}/`, { waitUntil: "commit", timeout: 60_000 });
    await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    if (wasmArtifact && (wasmDelivery.rawInterceptionCount !== 1 || wasmDelivery.gzipFallbackInterceptionCount !== 1 || wasmDelivery.unexpectedFallbackAttempts.length > 0)) {
      throw new Error(`WASM override delivery was not exclusive: ${JSON.stringify(wasmDelivery)}`);
    }
    // Refuse to quietly benchmark a blank/new household. The app seeds its built-in
    // demo dataset in this fresh context; this marker confirms that path was taken.
    await page.locator('[data-testid="sample-data-banner"]').waitFor({ state: "visible", timeout: 15_000 });
    const boot = await page.evaluate(() => ({
      readyMs: window.__primaryBenchBoot.ready == null ? null : window.__primaryBenchBoot.ready - window.__primaryBenchBoot.start,
      longTaskMs: window.__primaryBenchBoot.longTaskMs,
      longTaskCount: window.__primaryBenchBoot.longTaskCount,
      wallMs: null,
      goPhases: window.__cashfluxBootTimings || null,
    }));
    boot.wallMs = Date.now() - bootWallStart;

    const samples = Object.fromEntries(ROUTES.map(([name]) => [name, []]));
    const failures = [];
    const sweep = async (record, measuredRunIndex = null) => {
      for (const [name, route] of ROUTES) {
        let cpuSession = null;
        let cpuProfilerStarted = false;
        try {
          if (record && opts.profileDir && route === opts.profileRoute) {
            cpuSession = await context.newCDPSession(page);
            await cpuSession.send("Profiler.enable");
            await cpuSession.send("Profiler.start");
            cpuProfilerStarted = true;
          }
          const m = await page.evaluate(async (target) => {
            const selector = `#main[data-route="${target}"]`;
            const started = performance.now();
            let mutationCount = 0;
            let firstMutationMs = null;
            const mutations = new MutationObserver((list) => {
              mutationCount += list.length;
              if (firstMutationMs == null) firstMutationMs = performance.now() - started;
            });
            mutations.observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ["data-route", "data-settled"] });
            const longTasks = [];
            const rememberLongTask = (e) => { if (e.duration > 50 && e.startTime >= started && !longTasks.includes(e)) longTasks.push(e); };
            let observer;
            try {
              observer = new PerformanceObserver((list) => {
                for (const e of list.getEntries()) rememberLongTask(e);
              });
              observer.observe({ type: "longtask", buffered: false });
            } catch (_) {}
            let shifts = 0;
            let shiftCount = 0;
            const rememberShift = (e) => { if (!e.hadRecentInput) { shifts += e.value; shiftCount++; } };
            let shiftObserver;
            try {
              shiftObserver = new PerformanceObserver((list) => {
                for (const e of list.getEntries()) rememberShift(e);
              });
              shiftObserver.observe({ type: "layout-shift", buffered: false });
            } catch (_) {}

            history.pushState({}, "", target);
            dispatchEvent(new PopStateEvent("popstate"));
            const deadline = started + 30_000;
            let mountedAt = null;
            while (performance.now() < deadline) {
              const el = document.querySelector(selector);
              if (el && el.children.length && el.getBoundingClientRect().height > 4) { mountedAt = performance.now(); break; }
              await new Promise(requestAnimationFrame);
            }
            if (mountedAt == null) throw new Error(`route did not mount: ${target}`);

            // Honor the app's explicit deferred-render completion flag where used.
            await new Promise((resolve, reject) => {
              const check = () => {
                const root = document.querySelector(selector);
                const deferred = root && root.querySelector("[data-settled]");
                if (!deferred || deferred.getAttribute("data-settled") === "true") return resolve();
                if (performance.now() >= deadline) return reject(new Error(`route did not settle: ${target}`));
                requestAnimationFrame(check);
              };
              check();
            });
            // The legacy Assistant artifact predates a deferred-render marker.
            // This fixed sample has flagged activity: its populated rail is a
            // semantic completion contract available in both compared builds.
            // Measuring only two frames after mount mixed initial and complete
            // Assistant content depending on whether its 300 ms timer had fired.
            if (target === "/assistant") {
              while (!document.querySelector(`${selector} [data-testid="assistant-note-flagged"]`)) {
                if (performance.now() >= deadline) throw new Error("sample Assistant rail did not finish preparing flagged activity");
                await new Promise(requestAnimationFrame);
              }
            }
            if (document.fonts?.ready) await document.fonts.ready;
            const imgs = [...document.querySelectorAll(`${selector} img`)];
            await Promise.all(imgs.map((img) => img.complete ? null : new Promise((resolve) => {
              img.addEventListener("load", resolve, { once: true });
              img.addEventListener("error", resolve, { once: true });
            })));
            await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
            const ended = performance.now();
            try {
              for (const e of observer?.takeRecords() || []) rememberLongTask(e);
              for (const e of shiftObserver?.takeRecords() || []) rememberShift(e);
              mutationCount += mutations.takeRecords().length;
            } catch (_) {}
            observer?.disconnect(); shiftObserver?.disconnect(); mutations.disconnect();
            const main = document.querySelector(selector);
            const textChars = main?.innerText?.length ?? 0;
            const visibleRows = main?.querySelectorAll("tr.row, [role=row], [data-testid='task-card'], [data-testid^='budget-card-'], [data-testid^='goal-row-'], [data-testid^='notif-group-']").length ?? 0;
            const routeEntitySelectors = {
              "/accounts": "[data-testid^='acct-row-']",
              "/budgets": "[data-testid^='budget-card-']",
              "/goals": "[data-testid^='goal-row-']",
              "/todo": "[data-testid='task-card']",
              "/notifications": "[data-testid^='notif-history-row-'], [data-testid^='notif-'][role='listitem']:not([data-testid^='notif-group-'])",
            };
            const entitySelector = target === "/transactions" ? '[data-testid="txn-count-transaction-total"]' : routeEntitySelectors[target] || null;
            const entityCountText = entitySelector ? main?.querySelector(entitySelector)?.textContent?.trim() ?? null : null;
            const entityCountMatch = entityCountText?.match(/[\d,]+/);
            return {
              route: target,
              mountMs: mountedAt - started,
              settledMs: ended - started,
              longTaskCount: longTasks.length,
              longTaskTotalMs: longTasks.reduce((sum, entry) => sum + Math.max(0, entry.duration - 50), 0),
              longTaskMaxMs: longTasks.length ? Math.max(...longTasks.map((entry) => entry.duration)) : 0,
              layoutShift: shifts,
              layoutShiftCount: shiftCount,
              mutationCount,
              firstMutationMs,
              mainElementCount: main?.querySelectorAll("*").length ?? 0,
              mainTextChars: textChars,
              mainText: (main?.innerText ?? "").replace(/\s+/g, " ").trim(),
              // Stable content identity lets paired builds prove that they rendered
              // the same sample route, rather than comparing timing over different data.
              contentSha256: await crypto.subtle.digest("SHA-256", new TextEncoder().encode((main?.innerText ?? "").replace(/\s+/g, " ").trim())).then((digest) =>
                [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("")),
              renderedRowLikeCount: visibleRows,
              entityCount: entitySelector
                ? target === "/transactions"
                  ? entityCountMatch ? Number(entityCountMatch[0].replaceAll(",", "")) : null
                  : main?.querySelectorAll(entitySelector).length ?? 0
                : null,
              entityCountText,
              entityCountSelector: entitySelector,
              heapUsedMB: performance.memory?.usedJSHeapSize ? performance.memory.usedJSHeapSize / 1048576 : null,
            };
          }, route);
          if (cpuProfilerStarted) {
            const { profile } = await cpuSession.send("Profiler.stop");
            cpuProfilerStarted = false;
            if (!profile || !Array.isArray(profile.nodes)) throw new Error("CDP Profiler.stop returned no CPU profile nodes");
            const fileName = `${safeFilePart(opts.label)}-${safeFilePart(name)}-${profileRunId}-run-${measuredRunIndex}.cpuprofile`;
            const filePath = path.join(opts.profileDir, fileName);
            writeFileSync(filePath, JSON.stringify(profile), { flag: "wx" });
            const profileRecord = {
              route,
              measuredRunIndex,
              file: filePath,
              nodeCount: profile.nodes.length,
              sampleCount: profile.samples?.length ?? 0,
              durationUs: profile.startTime != null && profile.endTime != null ? profile.endTime - profile.startTime : null,
            };
            cpuProfiling.profiles.push(profileRecord);
            Object.assign(m, { cpuProfiled: true, performanceComparable: false, timingMode: "diagnostic-only", cpuProfileFile: filePath, cpuProfileNodeCount: profileRecord.nodeCount, cpuProfileSampleCount: profileRecord.sampleCount });
          }
          m.performanceComparable ??= true;
          if (record) samples[name].push(m);
        } catch (error) {
          failures.push({ route, pass: record ? "measured" : "warmup", message: String(error).slice(0, 300) });
          if (cpuProfilerStarted) {
            try { await cpuSession.send("Profiler.stop"); cpuProfilerStarted = false; }
            catch (profileError) { failures.push({ route, pass: "measured", message: `could not stop CDP CPU profiler: ${String(profileError).slice(0, 200)}` }); }
          }
        } finally {
          if (cpuSession) await cpuSession.detach().catch(() => {});
        }
      }
    };

    // A warm-up sweep absorbs first-use route and JS/WASM effects. Each measured
    // sweep starts at Dashboard after the previous sweep ended at Reports.
    await page.evaluate(() => { history.pushState({}, "", "/reports"); dispatchEvent(new PopStateEvent("popstate")); });
    await page.waitForSelector('#main[data-route="/reports"]', { state: "visible", timeout: 30_000 });
    for (let i = 0; i < opts.warmups; i++) await sweep(false);
    for (let i = 0; i < opts.runs; i++) await sweep(true, i + 1);
    if (opts.profileDir && cpuProfiling.profiles.length !== opts.runs) {
      failures.push({ route: opts.profileRoute, pass: "measured", message: `expected ${opts.runs} CPU profiles, wrote ${cpuProfiling.profiles.length}` });
    }

    const pages = ROUTES.map(([name, route]) => {
      const rows = samples[name];
      const contentHashes = [...new Set(rows.map((row) => row.contentSha256).filter(Boolean))];
      if (rows.length !== opts.runs) failures.push({ route, pass: "measured", message: `expected ${opts.runs} measured route samples, recorded ${rows.length}` });
      if (contentHashes.length > 1) failures.push({ route, pass: "measured", message: `sample route content changed across measured sweeps: ${contentHashes.join(", ")}` });
      const comparableRows = rows.filter((row) => !row.cpuProfiled);
      const numeric = (key) => median(comparableRows.map((row) => row[key]));
      return {
        name,
        route,
        samples: rows.length,
        mountMsMedian: numeric("mountMs"),
        settledMsMedian: numeric("settledMs"),
        longTaskTotalMsMedian: numeric("longTaskTotalMs"),
        longTaskCountMedian: numeric("longTaskCount"),
        longTaskMaxMsMedian: numeric("longTaskMaxMs"),
        performanceComparableSamples: comparableRows.length,
        diagnosticOnlySamples: rows.length - comparableRows.length,
        layoutShiftMedian: comparableRows.length ? median(comparableRows.map((row) => row.layoutShift)) : null,
        mainElementCountMedian: numeric("mainElementCount"),
        renderedRowLikeCountMedian: numeric("renderedRowLikeCount"),
        entityCount: rows.at(-1)?.entityCount ?? null,
        entityCountText: rows.at(-1)?.entityCountText ?? null,
        entityCountSelector: rows.at(-1)?.entityCountSelector ?? null,
        contentSha256: contentHashes.length === 1 ? contentHashes[0] : null,
        raw: rows,
      };
    });
    const meta = await page.evaluate(() => ({
      userAgent: navigator.userAgent,
      platform: navigator.platform,
      hardwareConcurrency: navigator.hardwareConcurrency || null,
      deviceMemoryGB: navigator.deviceMemory || null,
      viewport: { width: innerWidth, height: innerHeight, deviceScaleFactor: devicePixelRatio },
      locale: navigator.language,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      appReadyAttribute: document.documentElement.getAttribute("data-app-ready"),
      sampleBannerVisible: !!document.querySelector('[data-testid="sample-data-banner"]'),
      navigationEntry: performance.getEntriesByType("navigation")[0]?.toJSON() || null,
    }));
    const result = {
      schemaVersion: 1,
      label: opts.label,
      capturedAt: new Date().toISOString(),
      baseUrl: opts.baseUrl,
      conditions: {
        wasmDelivery,
        cpuProfiling,
        node: process.version,
        playwright: JSON.parse(readFileSync(path.join(REPO, "e2e", "node_modules", "playwright-core", "package.json"), "utf8")).version,
        browser: browser.version(),
        os: `${process.platform} ${process.arch}`,
        logicalCpuCount: cpus().length,
        headless: true,
        viewport: "1440x900",
        deviceScaleFactor: 1,
        reducedMotion: "reduce",
        serviceWorkers: "blocked",
        fixedClock: FIXED_NOW,
        dataset: "built-in CashFlux sample; fresh isolated Playwright context; all-dates transaction filter; sample banner checked",
        dataSize: {
          transactions: pages.find((p) => p.route === "/transactions")?.entityCount ?? null,
          accounts: pages.find((p) => p.route === "/accounts")?.entityCount ?? null,
          budgets: pages.find((p) => p.route === "/budgets")?.entityCount ?? null,
          goals: pages.find((p) => p.route === "/goals")?.entityCount ?? null,
          tasks: pages.find((p) => p.route === "/todo")?.entityCount ?? null,
          notifications: pages.find((p) => p.route === "/notifications")?.entityCount ?? null,
          basis: "transactions is the all-dates filtered ledger count; accounts, budgets, goals, tasks, and notifications are visible route entities",
        },
        runs: opts.runs,
        warmups: opts.warmups,
        backgroundWork: process.env.PRIMARY_BENCH_BACKGROUND_WORK || "not recorded",
        buildId: opts.buildId || "not recorded; label alone does not identify a dirty build",
      },
      boot: { ...boot, wallMs: round(boot.wallMs) },
      environment: meta,
      pages,
      cpuProfiling,
      failures,
      note: opts.profileDir
        ? `Diagnostic CPU profiling is enabled for ${opts.profileRoute}. Each measured profile sample is saved as a standalone .cpuprofile; warmups are unprofiled. Profiler-overhead samples remain in raw data and are excluded from performance medians. ${wasmArtifact ? "WASM uses the per-context injected gzip compatibility fallback; startup is not comparable to normal negotiated startup." : ""} Compare unprofiled medians with matching environment and dataset.`
        : wasmArtifact
        ? "Descriptive route benchmark using a per-context injected WASM artifact through the app's gzip compatibility fallback. Startup timing includes the forced raw-404 and gzip fallback and is not comparable to normal negotiated startup. Compare route medians with matching environment and dataset; inspect raw entity counts and text sizes before interpreting differences."
        : "Descriptive benchmark only. Compare medians with matching environment and dataset; no latency threshold is a pass/fail gate.",
    };
    const json = JSON.stringify(result, null, 2) + "\n";
    if (failures.length) process.exitCode = 1;
    if (opts.outFile) {
      const target = path.resolve(process.cwd(), opts.outFile);
      mkdirSync(path.dirname(target), { recursive: true });
      writeFileSync(target, json);
      console.log(`wrote ${target}`);
    } else {
      process.stdout.write(json);
    }
    console.error(`boot ${round(boot.readyMs)}ms app-ready; ${pages.length} routes × ${opts.runs} measured sweeps; sample isolated context confirmed`);
    await context.close();
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  console.error(error);
  if (fatalReport) {
    const report = {
      schemaVersion: 1,
      label: fatalReport.label,
      capturedAt: new Date().toISOString(),
      baseUrl: fatalReport.baseUrl,
      failures: [{ phase: "startup", message: String(error).slice(0, 500) }],
      conditions: fatalReport.wasmDelivery ? { wasmDelivery: fatalReport.wasmDelivery } : undefined,
      cpuProfiling: fatalReport.cpuProfiling,
      pages: [],
      note: "Benchmark did not complete startup; see failures for the recorded error.",
    };
    const json = JSON.stringify(report, null, 2) + "\n";
    if (fatalReport.outFile) {
      const target = path.resolve(process.cwd(), fatalReport.outFile);
      mkdirSync(path.dirname(target), { recursive: true });
      writeFileSync(target, json);
      console.error(`wrote failure report ${target}`);
    } else {
      process.stdout.write(json);
    }
  }
  process.exitCode = 1;
});
