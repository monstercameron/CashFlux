// Hermetic token-auth backend for browser RPC worker tests.
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import os from "node:os";
import path from "node:path";

const port = Number(process.argv[2] || 8198);
const appPort = Number(process.argv[3] || 8099);
const dataDir = mkdtempSync(path.join(os.tmpdir(), "cashflux-e2e-backend-"));
const root = path.resolve(import.meta.dirname, "..");
const command = process.platform === "win32" ? "go.exe" : "go";

const child = spawn(command, ["run", "./cmd/cashflux-server"], {
  cwd: root,
  env: {
    ...process.env,
    CASHFLUX_SERVER_ADDR: `127.0.0.1:${port}`,
    CASHFLUX_SERVER_DATA_DIR: dataDir,
    CASHFLUX_SERVER_AUTH_MODE: "token",
    CASHFLUX_SERVER_TOKEN: "e2e-worker-token",
    CASHFLUX_SERVER_APP_ORIGIN: `http://127.0.0.1:${appPort}`,
    CASHFLUX_SERVER_LOG_LEVEL: "warn",
  },
  stdio: "inherit",
});

let stopping = false;
function stop(signal = "SIGTERM") {
  if (stopping) return;
  stopping = true;
  child.kill(signal);
}

process.on("SIGINT", () => stop("SIGINT"));
process.on("SIGTERM", () => stop("SIGTERM"));
child.on("error", (err) => {
  console.error("e2e backend failed to start:", err);
  process.exitCode = 1;
});
child.on("exit", (code, signal) => {
  try {
    rmSync(dataDir, { recursive: true, force: true });
  } catch (err) {
    console.warn("e2e backend temp cleanup failed:", err);
  }
  if (!stopping && code !== 0) {
    console.error(`e2e backend exited (${signal || code})`);
  }
  process.exit(code ?? (stopping ? 0 : 1));
});
