// global-setup — makes the regression suite self-contained: it (re)builds the
// wasm app and drops the matching wasm_exec.js into web/ before any test runs, so
// a fresh CI checkout (where web/bin and web/wasm_exec.js are git-ignored build
// artifacts) has everything the static server needs. Runs once, before webServer.
import { execFileSync } from "node:child_process";
import { existsSync, copyFileSync, mkdirSync, renameSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export default function globalSetup() {
  // 1. Copy wasm_exec.js from the active Go toolchain (Go 1.24+ moved it from
  //    misc/wasm to lib/wasm — try both so this survives a toolchain bump).
  const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
  const src = [
    path.join(goroot, "lib", "wasm", "wasm_exec.js"),
    path.join(goroot, "misc", "wasm", "wasm_exec.js"),
  ].find(existsSync);
  if (!src) throw new Error(`global-setup: wasm_exec.js not found under GOROOT ${goroot}`);
  copyFileSync(src, path.join(root, "web", "wasm_exec.js"));

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
