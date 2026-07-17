// lane3_verify_78.mjs — verify (a) the wasm.gz freshness guard in serve.mjs and
// serve.go: a missing or stale bin/main.wasm.gz is synthesized live from the raw
// wasm, killing both the 404 and the stale-gz silent-old-code landmine; and (b)
// the page-shaped boot skeleton: the boot layer previews rail/topbar/tiles and
// dismisses after mount. Usage: node e2e/lane3_verify_78.mjs <appPort> <shotDir>
import { chromium } from "playwright";
import { mkdirSync, writeFileSync, utimesSync, rmSync } from "node:fs";
import { execFile, spawn } from "node:child_process";
import { gunzipSync } from "node:zlib";
import path from "node:path";

const APP_PORT = process.argv[2] || "8113";
const OUT = process.argv[3] || "lane3-shots";
mkdirSync(OUT, { recursive: true });

let failures = 0;
const check = (ok, msg) => { console.log(`${ok ? "PASS" : "FAIL"} ${msg}`); if (!ok) failures++; };

// ── Part A: gz freshness guard against a tiny fixture root ──────────────────
const FIX = path.join("e2e", ".lane3-gz-fixture");
rmSync(FIX, { recursive: true, force: true });
mkdirSync(path.join(FIX, "bin"), { recursive: true });
writeFileSync(path.join(FIX, "index.html"), "<html>fixture</html>");
writeFileSync(path.join(FIX, "bin", "main.wasm"), "FRESH-WASM-BYTES");
// Case 2 prep: a STALE gz whose mtime predates the raw wasm.
writeFileSync(path.join(FIX, "bin", "main.wasm.gz"), "OLD-GZ-GARBAGE");
const old = new Date(Date.now() - 3600_000);
utimesSync(path.join(FIX, "bin", "main.wasm.gz"), old, old);

async function probe(port) {
  const r = await fetch(`http://127.0.0.1:${port}/bin/main.wasm.gz`);
  if (!r.ok) return { status: r.status };
  const buf = Buffer.from(await r.arrayBuffer());
  let body = null;
  try { body = gunzipSync(buf).toString(); } catch { body = `<not gzip: ${buf.toString().slice(0, 20)}>`; }
  return { status: r.status, body };
}

async function testServer(name, start, stop, port) {
  await start();
  await new Promise((r) => setTimeout(r, 1200));
  // Stale gz on disk → served bytes must gunzip to the FRESH raw wasm.
  let res = await probe(port);
  check(res.status === 200 && res.body === "FRESH-WASM-BYTES", `${name}: stale gz replaced by fresh raw (got ${res.status} ${res.body})`);
  // Missing gz → synthesized, not 404.
  rmSync(path.join(FIX, "bin", "main.wasm.gz"));
  res = await probe(port);
  check(res.status === 200 && res.body === "FRESH-WASM-BYTES", `${name}: missing gz synthesized from raw (got ${res.status} ${res.body})`);
  // Fresh gz on disk → served as-is.
  writeFileSync(path.join(FIX, "bin", "main.wasm.gz"), Buffer.from([0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0]));
  res = await fetch(`http://127.0.0.1:${port}/bin/main.wasm.gz`);
  const raw = Buffer.from(await res.arrayBuffer());
  check(res.status === 200 && raw[0] === 0x1f && raw.length === 10, `${name}: fresh gz on disk served verbatim`);
  rmSync(path.join(FIX, "bin", "main.wasm.gz"), { force: true });
  writeFileSync(path.join(FIX, "bin", "main.wasm.gz"), "OLD-GZ-GARBAGE");
  utimesSync(path.join(FIX, "bin", "main.wasm.gz"), old, old);
  await stop();
}

let child = null;
await testServer("serve.mjs",
  () => { child = spawn(process.execPath, ["e2e/serve.mjs", FIX, "8177"], { stdio: "ignore" }); },
  () => { child.kill(); return new Promise((r) => setTimeout(r, 300)); },
  "8177");
await testServer("serve.go",
  () => { child = execFile("e2e/serve.exe", [FIX, "8178"]); },
  () => { child.kill(); return new Promise((r) => setTimeout(r, 300)); },
  "8178");
rmSync(FIX, { recursive: true, force: true });

// ── Part B: page-shaped skeleton on the real app ────────────────────────────
const browser = await chromium.launch();
for (const [vp, w, h] of [["desktop", 1440, 900], ["mobile", 390, 844]]) {
  const ctx = await browser.newContext({ viewport: { width: w, height: h } });
  const page = await ctx.newPage();
  let gzStatus = null;
  page.on("response", (r) => { if (r.url().endsWith("main.wasm.gz")) gzStatus = r.status(); });
  await page.goto(`http://127.0.0.1:${APP_PORT}/budgets`, { waitUntil: "commit" });
  // Catch the skeleton BEFORE the app mounts.
  const sk = await page.evaluate(() => ({
    rail: !!document.querySelector("#boot .sk-rail"),
    topbar: !!document.querySelector("#boot .sk-topbar"),
    tiles: document.querySelectorAll("#boot .sk-tile").length,
    tabbar: !!document.querySelector("#boot .sk-tabbar"),
    railVisible: (() => { const r = document.querySelector("#boot .sk-rail"); return r ? getComputedStyle(r).display !== "none" : false; })(),
    tabbarVisible: (() => { const t = document.querySelector("#boot .sk-tabbar"); return t ? getComputedStyle(t).display !== "none" : false; })(),
  })).catch(() => null);
  if (sk) {
    check(sk.rail && sk.topbar && sk.tiles >= 6, `${vp}: skeleton previews rail+topbar+tiles (tiles=${sk.tiles})`);
    check(vp === "desktop" ? sk.railVisible && !sk.tabbarVisible : !sk.railVisible && sk.tabbarVisible,
      `${vp}: skeleton matches viewport shell (rail=${sk.railVisible}, tabbar=${sk.tabbarVisible})`);
    await page.screenshot({ path: `${OUT}/78-skeleton-${vp}.png` });
  } else {
    check(false, `${vp}: skeleton not observable before mount`);
  }
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(1200);
  const bootGone = await page.evaluate(() => {
    const b = document.getElementById("boot");
    return !b || b.classList.contains("hidden") || getComputedStyle(b).display === "none";
  });
  check(bootGone, `${vp}: skeleton dismissed after mount`);
  check(gzStatus === 200, `${vp}: main.wasm.gz served 200 on a direct sub-route load (got ${gzStatus})`);
  await page.screenshot({ path: `${OUT}/78-booted-${vp}.png` });
  await ctx.close();
}
await browser.close();
console.log(failures === 0 ? "ALL CHECKS PASSED" : `${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
