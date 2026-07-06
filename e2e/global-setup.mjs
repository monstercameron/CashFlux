// global-setup — makes the regression suite self-contained: it (re)builds the
// wasm app and drops the matching wasm_exec.js into web/ before any test runs, so
// a fresh CI checkout (where web/bin and web/wasm_exec.js are git-ignored build
// artifacts) has everything the static server needs. Runs once, before webServer.
import { execFileSync } from "node:child_process";
import { existsSync, copyFileSync, mkdirSync, renameSync, rmSync, chmodSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export default function globalSetup() {
  // E2E_SKIP_BUILD: the wasm + wasm_exec.js were already built by the caller
  // (e.g. a CI job with Go, before handing off to the Go-less Playwright Docker
  // container). Nothing to do — the static server serves the prebuilt web/.
  if (process.env.E2E_SKIP_BUILD) return;

  // 1. Copy wasm_exec.js from the active Go toolchain (Go 1.24+ moved it from
  //    misc/wasm to lib/wasm — try both so this survives a toolchain bump).
  const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
  const src = [
    path.join(goroot, "lib", "wasm", "wasm_exec.js"),
    path.join(goroot, "misc", "wasm", "wasm_exec.js"),
  ].find(existsSync);
  if (!src) throw new Error(`global-setup: wasm_exec.js not found under GOROOT ${goroot}`);
  // The toolchain source is read-only (Go's module cache is 0444), so a prior copy
  // can leave web/wasm_exec.js read-only and block the next overwrite (EPERM on
  // Windows). Remove any existing copy first, then copy and mark it writable.
  const wasmExecDst = path.join(root, "web", "wasm_exec.js");
  rmSync(wasmExecDst, { force: true });
  copyFileSync(src, wasmExecDst);
  chmodSync(wasmExecDst, 0o644);

  // 2. Build the wasm app to a temp file then atomic-rename into place, so a
  //    concurrent `gwc dev` rebuild can never observe a half-written main.wasm.
  const binDir = path.join(root, "web", "bin");
  mkdirSync(binDir, { recursive: true });
  const tmp = path.join(binDir, "main.wasm.e2e-tmp");
  execFileSync("go", ["build", "-o", tmp, "."], {
    cwd: root,
    stdio: "inherit",
    env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
  });
  renameSync(tmp, path.join(binDir, "main.wasm"));
}
