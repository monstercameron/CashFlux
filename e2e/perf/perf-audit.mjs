// perf-audit.mjs — CashFlux per-page performance framework (Lighthouse-style).
//
// WHAT IT DOES
//   Boots the real wasm app in Chromium and, for every route, measures the
//   user-perceived cost of *arriving on that page*: how long until its body
//   mounts, how much the main thread is blocked, how much layout shifts, how
//   long until it's visually stable, and how heavy its DOM is. Each metric is
//   scored on Lighthouse's log-normal curve and combined into a 0–100 page
//   score + letter grade. A separate section scores the one-time cold boot
//   (wasm download/instantiate/seed) with FCP/LCP/TBT/transfer-weight.
//
//   This is intentionally "think Lighthouse, not code-level": it rates what a
//   user feels, measured from the browser's own Performance timeline — no Go
//   profiling, no source instrumentation.
//
// RUN (from the e2e/ directory, vendored Playwright):
//   node perf/perf-audit.mjs                 # full audit, 3 passes, all routes
//   node perf/perf-audit.mjs --passes 2      # fewer passes (faster, noisier)
//   node perf/perf-audit.mjs --routes /budgets,/debt   # subset
//   node perf/perf-audit.mjs --port 8097     # server port to spin up
//
// OUTPUT (versioned, committed):
//   perf/results/v<version>.json   machine-readable ratings + raw metrics
//   perf/results/v<version>.md     detailed human report (the analysis)
//   perf/results/index.json        version → {avgScore, loadScore} history
import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import { gzipSync } from "node:zlib";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ROUTES } from "../regression/fixtures.mjs";
import { PAGE_METRICS, LOAD_METRICS, scoreGroup, grade, ratingWord } from "./scoring.mjs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, "..", "..");
const RESULTS_DIR = path.join(HERE, "results");

// ---- args ----
const argv = process.argv.slice(2);
const argVal = (flag, def) => {
  const i = argv.indexOf(flag);
  return i >= 0 && argv[i + 1] ? argv[i + 1] : def;
};
const PORT = argVal("--port", "8097");
const PASSES = Number(argVal("--passes", "3"));
const routeFilter = argVal("--routes", "");
const ROUTE_LIST = routeFilter ? routeFilter.split(",").map((s) => s.trim()) : ROUTES.map((r) => r[0]);
const BASE = `http://127.0.0.1:${PORT}`;

function readVersion() {
  const src = readFileSync(path.join(REPO, "internal", "version", "version.go"), "utf8");
  const m = src.match(/Version\s*=\s*"([^"]+)"/);
  return m ? m[1] : "unknown";
}
const median = (xs) => {
  const v = xs.filter((n) => n !== null && n !== undefined && !Number.isNaN(n)).sort((a, b) => a - b);
  if (!v.length) return null;
  const mid = Math.floor(v.length / 2);
  return v.length % 2 ? v[mid] : (v[mid - 1] + v[mid]) / 2;
};
// best is the load-robust reducer for CPU-time metrics (mount / TBT / settle). OS
// scheduler contention (here: dozens of concurrent Chrome processes competing for
// cores) can only ADD time to a render, never subtract it — so the fastest pass
// across N approximates the page's intrinsic render cost, i.e. what a user on an
// unloaded machine sees. This denoises without touching the scoring curves: a page
// that is genuinely slow stays slow across every pass, so its best is still slow.
const best = (xs) => {
  const v = xs.filter((n) => n !== null && n !== undefined && !Number.isNaN(n));
  return v.length ? Math.min(...v) : null;
};
const r1 = (n) => (n === null || n === undefined ? null : Math.round(n * 10) / 10);
const r3 = (n) => (n === null || n === undefined ? null : Math.round(n * 1000) / 1000);

// The in-page measurement, stringified into the browser. Sets up longtask +
// layout-shift observers, triggers the SPA navigation, and times mount + settle.
async function measureInPage(route) {
  const sel = `#main[data-route="${route}"]`;
  let tbt = 0;
  let cls = 0;
  const lt = new PerformanceObserver((l) => {
    for (const e of l.getEntries()) if (e.duration > 50) tbt += e.duration - 50;
  });
  try { lt.observe({ type: "longtask", buffered: false }); } catch (_) {}
  const ls = new PerformanceObserver((l) => {
    for (const e of l.getEntries()) if (!e.hadRecentInput) cls += e.value;
  });
  try { ls.observe({ type: "layout-shift", buffered: false }); } catch (_) {}

  const heapBefore = performance.memory ? performance.memory.usedJSHeapSize : 0;
  const t0 = performance.now();
  history.pushState({}, "", route);
  dispatchEvent(new PopStateEvent("popstate"));

  const visible = (el) => el && el.children.length > 0 && el.getBoundingClientRect().height > 4;
  const deadline = t0 + 15000;
  let mountMs = null;
  // eslint-disable-next-line no-constant-condition
  while (performance.now() < deadline) {
    if (visible(document.querySelector(sel))) { mountMs = performance.now() - t0; break; }
    await new Promise((r) => requestAnimationFrame(r));
  }
  if (document.fonts && document.fonts.ready) { try { await document.fonts.ready; } catch (_) {} }
  const imgs = [...document.querySelectorAll("#main img")];
  await Promise.all(imgs.map((im) => (im.complete ? null : new Promise((r) => {
    im.addEventListener("load", r, { once: true });
    im.addEventListener("error", r, { once: true });
  }))));
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  const stableMs = performance.now() - t0;

  await new Promise((r) => setTimeout(r, 80)); // let trailing long tasks flush
  try { lt.takeRecords(); ls.takeRecords(); } catch (_) {}
  lt.disconnect();
  ls.disconnect();

  const heapAfter = performance.memory ? performance.memory.usedJSHeapSize : 0;
  return {
    mountMs,
    stableMs,
    tbtMs: tbt,
    cls,
    domNodes: document.querySelectorAll("#main *").length,
    totalNodes: document.getElementsByTagName("*").length,
    heapMB: heapAfter / 1048576,
    heapDeltaMB: (heapAfter - heapBefore) / 1048576,
  };
}

async function waitServer(url, ms = 60000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    try {
      const res = await fetch(url);
      if (res.ok || res.status === 404) return true;
    } catch (_) {}
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`server ${url} did not come up`);
}

async function main() {
  const version = readVersion();
  mkdirSync(RESULTS_DIR, { recursive: true });

  console.log(`CashFlux perf audit — v${version} — ${ROUTE_LIST.length} routes × ${PASSES} passes`);

  // 1) Build fresh wasm so the audit reflects HEAD — same flags + gzip sibling as
  // the deploy pipeline, so cold-load transfer reflects what real users download.
  console.log("building wasm (stripped) + gzip sibling…");
  const wasmPath = path.join(REPO, "web", "bin", "main.wasm");
  // Quote the ldflags value so the win32 shell doesn't split "-s -w" into two args.
  await run("go", ["build", '-ldflags="-s -w"', "-trimpath", "-o", wasmPath, "."], {
    cwd: REPO,
    env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
  });
  writeFileSync(wasmPath + ".gz", gzipSync(readFileSync(wasmPath), { level: 9 }));
  console.log(`serving web/ on :${PORT}…`);
  const server = spawn("node", ["e2e/serve.mjs", "web", PORT], { cwd: REPO, stdio: "ignore" });
  const cleanup = () => { try { server.kill(); } catch (_) {} };
  process.on("exit", cleanup);
  await waitServer(BASE + "/");

  const browser = await chromium.launch({ args: ["--disable-gpu", "--enable-precise-memory-info"] });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: "reduce" });
  const page = await ctx.newPage();

  // Observers that must exist BEFORE navigation (LCP/boot-TBT/CLS + app-ready mark).
  await page.addInitScript(() => {
    window.__perf = { lcp: 0, bootTbt: 0, cls: 0, appReadyMs: null };
    try { new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__perf.lcp = e.startTime; }).observe({ type: "largest-contentful-paint", buffered: true }); } catch (_) {}
    try { new PerformanceObserver((l) => { for (const e of l.getEntries()) if (e.duration > 50) window.__perf.bootTbt += e.duration - 50; }).observe({ type: "longtask", buffered: true }); } catch (_) {}
    try { new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__perf.cls += e.value; }).observe({ type: "layout-shift", buffered: true }); } catch (_) {}
    const mo = new MutationObserver(() => {
      if (document.documentElement.getAttribute("data-app-ready") === "true" && window.__perf.appReadyMs == null) {
        window.__perf.appReadyMs = performance.now();
        mo.disconnect();
      }
    });
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-app-ready"] });
    performance.mark && performance.mark("perf-harness-init");
    // Neutralize View Transitions so machine-speed nav is instant + deterministic.
    try {
      const proto = Document.prototype;
      if (proto && "startViewTransition" in proto) {
        proto.startViewTransition = function (cb) {
          try { if (typeof cb === "function") cb(); } catch (_) {}
          const d = Promise.resolve();
          return { finished: d, ready: d, updateCallbackDone: d, skipTransition() {} };
        };
      }
    } catch (_) {}
  });

  // 2) COLD LOAD — the one-time wasm boot. (No clock pinning: it perturbs the
  // performance timeline / resource-timing reads. The seed just uses real "now",
  // which is fine for measuring render cost.)
  console.log("measuring cold load…");
  const bootT0 = Date.now();
  await page.goto(BASE + "/", { waitUntil: "commit" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60000 });
  const appReadyWallMs = Date.now() - bootT0;
  await page.waitForTimeout(400); // let LCP/boot long tasks settle
  const load = await page.evaluate(() => {
    const nav = performance.getEntriesByType("navigation")[0] || {};
    const paints = performance.getEntriesByType("paint");
    const fcp = (paints.find((p) => p.name === "first-contentful-paint") || {}).startTime || null;
    const res = performance.getEntriesByType("resource");
    let bytes = nav.transferSize || 0;
    let wasmBytes = 0;
    for (const r of res) {
      bytes += r.transferSize || 0;
      if (r.name.endsWith(".wasm") || r.name.endsWith(".wasm.gz")) {
        wasmBytes += r.transferSize || r.encodedBodySize || 0;
      }
    }
    return {
      appReadyMs: window.__perf.appReadyMs,
      fcpMs: fcp,
      lcpMs: window.__perf.lcp || null,
      tbtMs: window.__perf.bootTbt,
      cls: window.__perf.cls,
      ttfbMs: nav.responseStart || null,
      domContentLoadedMs: nav.domContentLoadedEventEnd || null,
      loadEventMs: nav.loadEventEnd || null,
      transferMB: bytes / 1048576,
      wasmMB: wasmBytes / 1048576,
    };
  });
  // Prefer the in-page app-ready mark; fall back to the robust Node wall-clock delta.
  if (load.appReadyMs == null) load.appReadyMs = appReadyWallMs;
  load.appReadyMs = Math.round(load.appReadyMs);
  const loadScored = scoreGroup(LOAD_METRICS, load);

  // 3) PER-ROUTE — warm SPA navigation, resetting to "/" before each measure.
  const raw = {}; // route -> [perPassMetrics]
  for (const route of ROUTE_LIST) raw[route] = [];
  for (let pass = 1; pass <= PASSES; pass++) {
    for (const route of ROUTE_LIST) {
      try {
        // Reset to a consistent baseline before measuring (unmeasured), so every
        // page's mount is timed as a real transition. Use the dashboard as the hub,
        // except when the dashboard IS the target — then come from /transactions so
        // the nav isn't a no-op that flatters the score.
        const resetRoute = route === "/" ? "/transactions" : "/";
        await page.evaluate((rr) => { history.pushState({}, "", rr); dispatchEvent(new PopStateEvent("popstate")); }, resetRoute);
        await page.locator(`#main[data-route="${resetRoute}"]`).first().waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
        await page.waitForTimeout(120);
        const m = await page.evaluate(measureInPage, route);
        raw[route].push(m);
      } catch (e) {
        raw[route].push({ error: String(e).slice(0, 140) });
      }
    }
    console.log(`  pass ${pass}/${PASSES} done`);
  }

  // 4) reduce + score each page
  const pages = [];
  const labelOf = Object.fromEntries(ROUTES.map((r) => [r[0], r[0]]));
  for (const route of ROUTE_LIST) {
    const runs = raw[route].filter((m) => m && !m.error && m.mountMs !== null);
    const errored = raw[route].length - runs.length;
    if (!runs.length) {
      pages.push({ route, label: labelOf[route] || route, score: null, grade: grade(null), error: "did not mount", metrics: {}, passes: raw[route].length, errored });
      continue;
    }
    const metrics = {
      // CPU-time metrics: best-of-N (intrinsic cost, robust to machine load).
      mountMs: r1(best(runs.map((m) => m.mountMs))),
      tbtMs: r1(best(runs.map((m) => m.tbtMs))),
      stableMs: r1(best(runs.map((m) => m.stableMs))),
      // Deterministic / layout metrics: median (not CPU-load sensitive).
      cls: r3(median(runs.map((m) => m.cls))),
      domNodes: Math.round(median(runs.map((m) => m.domNodes))),
      totalNodes: Math.round(median(runs.map((m) => m.totalNodes))),
      heapMB: r1(median(runs.map((m) => m.heapMB))),
    };
    const scored = scoreGroup(PAGE_METRICS, metrics);
    pages.push({ route, label: labelOf[route] || route, score: scored.score, grade: grade(scored.score), metrics, parts: scored.parts, passes: raw[route].length, errored });
  }

  const rated = pages.filter((p) => p.score !== null);
  const avgScore = rated.length ? Math.round(rated.reduce((a, p) => a + p.score, 0) / rated.length) : null;
  const sortedByScore = [...rated].sort((a, b) => a.score - b.score);

  const result = {
    version,
    capturedAt: new Date().toISOString(),
    generatedWith: { passes: PASSES, viewport: "1440x900", server: "e2e/serve.mjs (uncompressed static)", reducer: "best-of-N for CPU-time metrics (mount/TBT/settle) to filter OS-scheduler contention; median for CLS/DOM", note: "warm SPA navigation from '/'; cold-load section is the one-time wasm boot" },
    summary: {
      avgPageScore: avgScore,
      loadScore: loadScored.score,
      rated: rated.length,
      unrated: pages.length - rated.length,
      worst: sortedByScore.slice(0, 5).map((p) => ({ route: p.route, score: p.score })),
      best: [...rated].sort((a, b) => b.score - a.score).slice(0, 5).map((p) => ({ route: p.route, score: p.score })),
    },
    coldLoad: { metrics: load, score: loadScored.score, parts: loadScored.parts },
    pages,
  };

  const jsonPath = path.join(RESULTS_DIR, `v${version}.json`);
  writeFileSync(jsonPath, JSON.stringify(result, null, 2) + "\n");
  const mdPath = path.join(RESULTS_DIR, `v${version}.md`);
  writeFileSync(mdPath, renderReport(result));
  updateIndex(version, avgScore, loadScored.score);

  console.log(`\n  avg page score: ${avgScore}   cold-load score: ${loadScored.score}`);
  console.log(`  wrote ${path.relative(REPO, jsonPath)}`);
  console.log(`  wrote ${path.relative(REPO, mdPath)}`);

  await browser.close();
  cleanup();
}

function updateIndex(version, avgScore, loadScore) {
  const p = path.join(RESULTS_DIR, "index.json");
  let idx = { versions: [] };
  if (existsSync(p)) { try { idx = JSON.parse(readFileSync(p, "utf8")); } catch (_) {} }
  idx.versions = (idx.versions || []).filter((v) => v.version !== version);
  idx.versions.push({ version, avgPageScore: avgScore, loadScore, capturedAt: new Date().toISOString() });
  idx.versions.sort((a, b) => a.version.localeCompare(b.version, undefined, { numeric: true }));
  writeFileSync(p, JSON.stringify(idx, null, 2) + "\n");
}

// ---- report renderer (the "detailed analysis") ----
function bar(score) {
  if (score === null) return "—";
  const n = Math.round(score / 10);
  return "█".repeat(n) + "░".repeat(10 - n);
}
function fmt(v, unit) {
  if (v === null || v === undefined) return "n/a";
  if (unit === "ms") return `${v} ms`;
  if (unit === "MB") return `${v} MB`;
  if (unit === "nodes") return `${v}`;
  return `${v}`;
}
function analyzePage(p) {
  if (p.score === null) return `Did not render within budget (${p.error}). Excluded from the average.`;
  const parts = Object.entries(p.parts).map(([k, v]) => ({ k, ...v }));
  const worst = parts.filter((x) => x.sub !== null).sort((a, b) => a.sub * a.weight - b.sub * b.weight)[0];
  const good = parts.filter((x) => x.sub !== null && x.sub >= 0.9).map((x) => x.label);
  const bits = [];
  bits.push(`Overall **${p.score}/100 (${p.grade.letter})**.`);
  if (worst) {
    bits.push(`Weakest signal: **${worst.label}** at ${fmt(worst.value, worst.unit)} (${ratingWord(worst.sub)}) — ${worst.hint}`);
  }
  if (good.length) bits.push(`Strong: ${good.join(", ")}.`);
  return bits.join(" ");
}
function renderReport(r) {
  const L = [];
  L.push(`# CashFlux performance ratings — v${r.version}`);
  L.push("");
  L.push(`_Captured ${r.capturedAt} · ${r.generatedWith.passes} passes · ${r.generatedWith.viewport} · Lighthouse-style log-normal scoring._`);
  L.push("");
  L.push(`**Average page score: ${r.summary.avgPageScore}/100** · Cold-load score: ${r.summary.loadScore}/100 · ${r.summary.rated} pages rated${r.summary.unrated ? `, ${r.summary.unrated} unrated` : ""}.`);
  L.push("");
  L.push(`> Methodology: each page is measured as a **warm SPA navigation** from the dashboard — the cost of *arriving on that page* with the wasm runtime already booted. Metrics come from the browser's own Performance timeline (long-task, layout-shift, paint observers), not source instrumentation. The one-time wasm boot is scored separately under **Cold load**. CPU-time metrics (mount/TBT/settle) use **best-of-${r.generatedWith.passes}** — OS-scheduler contention can only add time, so the fastest pass reflects intrinsic render cost; CLS/DOM use the median.`);
  L.push("");
  // Cold load section
  L.push(`## Cold load (one-time wasm boot)`);
  L.push("");
  L.push(`Score: **${r.coldLoad.score}/100** ${bar(r.coldLoad.score)}`);
  L.push("");
  L.push(`| Metric | Value | Sub-score | Rating |`);
  L.push(`|---|---:|---:|---|`);
  for (const [k, v] of Object.entries(r.coldLoad.parts)) {
    L.push(`| ${v.label} | ${fmt(r1(v.value), v.unit)} | ${v.sub === null ? "—" : Math.round(v.sub * 100)} | ${ratingWord(v.sub)} |`);
  }
  L.push("");
  L.push(`Cold-load transfer is **${r1(r.coldLoad.metrics.transferMB)} MB** (wasm alone ${r1(r.coldLoad.metrics.wasmMB)} MB), served uncompressed by the local harness. On a real network this is the dominant cost; gzip/brotli on the host typically cuts the wasm 3–4×, but the binary size is the lever that matters.`);
  L.push("");
  // Ranked table
  L.push(`## Per-page ratings`);
  L.push("");
  L.push(`| Page | Score | Grade | Mount | Blocking | Settle | CLS | DOM |`);
  L.push(`|---|---:|:--:|---:|---:|---:|---:|---:|`);
  const rows = [...r.pages].sort((a, b) => (b.score ?? -1) - (a.score ?? -1));
  for (const p of rows) {
    const m = p.metrics || {};
    L.push(`| \`${p.route}\` | ${p.score ?? "—"} | ${p.grade.letter} | ${fmt(m.mountMs, "ms")} | ${fmt(m.tbtMs, "ms")} | ${fmt(m.stableMs, "ms")} | ${m.cls ?? "—"} | ${m.domNodes ?? "—"} |`);
  }
  L.push("");
  // Detailed per-page analysis
  L.push(`## Detailed analysis`);
  L.push("");
  for (const p of rows) {
    L.push(`### \`${p.route}\` — ${p.score ?? "—"}/100 ${p.grade.letter}  ${bar(p.score)}`);
    L.push("");
    L.push(analyzePage(p));
    L.push("");
  }
  return L.join("\n") + "\n";
}

function run(cmd, args, opts) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { ...opts, stdio: "inherit", shell: process.platform === "win32" });
    p.on("exit", (code) => (code === 0 ? resolve() : reject(new Error(`${cmd} exited ${code}`))));
    p.on("error", reject);
  });
}

main().catch((e) => { console.error(e); process.exit(1); });
