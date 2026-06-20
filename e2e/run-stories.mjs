// Cross-platform E2E suite runner (CI-friendly, no PowerShell). Builds the wasm
// app, builds + starts the static server (e2e/serve.go) on :8099, runs every
// Playwright story (*.test.mjs) and feature check (*_check.mjs) in its own fresh
// browser, then stops the server. Prints a per-file result and a summary; exits
// non-zero if any file fails.
//
//   node e2e/run-stories.mjs
import { spawn, spawnSync } from "child_process";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import http from "http";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, "..");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const nativeEnv = { ...process.env };
delete nativeEnv.GOOS;
delete nativeEnv.GOARCH;

function run(cmd, args, env = nativeEnv) {
  return spawnSync(cmd, args, { stdio: "inherit", cwd: root, env }).status ?? 1;
}

function waitForServer(timeoutMs = 30000) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const tick = () => {
      http
        .get(BASE + "/", (res) => {
          res.resume();
          resolve();
        })
        .on("error", () => {
          if (Date.now() - start > timeoutMs) reject(new Error("server did not start in time"));
          else setTimeout(tick, 300);
        });
    };
    tick();
  });
}

console.log("Building wasm...");
if (run("go", ["build", "-o", "web/bin/main.wasm", "."], { ...process.env, GOOS: "js", GOARCH: "wasm" }) !== 0) {
  console.error("wasm build failed");
  process.exit(1);
}

const serveBin = path.join(__dirname, process.platform === "win32" ? "serve-bin.exe" : "serve-bin");
console.log("Building serve binary...");
if (run("go", ["build", "-o", serveBin, "e2e/serve.go"]) !== 0) {
  console.error("serve build failed");
  process.exit(1);
}

const server = spawn(serveBin, [], { cwd: root, stdio: "ignore" });
let exitCode = 0;
try {
  await waitForServer();

  const files = fs
    .readdirSync(__dirname)
    .filter((f) => f.endsWith(".test.mjs") || f.endsWith("_check.mjs"))
    .sort();
  const failed = [];
  for (const f of files) {
    console.log(`\n--- ${f} ---`);
    if (run("node", [path.join("e2e", f)]) !== 0) failed.push(f);
  }

  console.log("\n==========================================");
  console.log(`E2E suite: ${files.length - failed.length} passed, ${failed.length} failed`);
  if (failed.length) {
    console.log("FAILED: " + failed.join(", "));
    exitCode = 1;
  } else {
    console.log("ALL GREEN");
  }
} catch (e) {
  console.error(String(e));
  exitCode = 1;
} finally {
  server.kill();
  try {
    fs.unlinkSync(serveBin);
  } catch {}
}
process.exit(exitCode);
