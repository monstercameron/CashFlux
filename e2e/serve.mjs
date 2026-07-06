// serve.mjs — a tiny Node static server for the e2e suite, mirroring serve.go:
// SPA history fallback (extensionless paths → index.html) and the correct
// application/wasm MIME. Node (not Go) so it runs unchanged inside the Playwright
// Docker container used for visual baselines — the container has Node but no Go.
//
//   node e2e/serve.mjs [root=web] [port=8099]
import http from "node:http";
import { createReadStream, statSync } from "node:fs";
import path from "node:path";

const root = process.argv[2] || "web";
const port = Number(process.argv[3] || 8099);

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".wasm": "application/wasm",
  ".webmanifest": "application/manifest+json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".ico": "image/x-icon",
  ".woff2": "font/woff2",
};

function sendFile(res, full) {
  const ext = path.extname(full).toLowerCase();
  res.setHeader("Content-Type", MIME[ext] || "application/octet-stream");
  createReadStream(full).pipe(res);
}

const server = http.createServer((req, res) => {
  const clean = path.posix.normalize(decodeURIComponent(req.url.split("?")[0]));
  const full = path.join(root, clean);
  try {
    const info = statSync(full);
    if (info.isFile()) return sendFile(res, full);
  } catch {
    /* fall through to SPA fallback */
  }
  // SPA history fallback: an extensionless route boots the shell.
  if (!path.basename(clean).includes(".")) return sendFile(res, path.join(root, "index.html"));
  res.statusCode = 404;
  res.end("not found");
});

server.listen(port, "0.0.0.0", () => {
  console.log(`e2e: serving ${root} at http://127.0.0.1:${port}`);
});
