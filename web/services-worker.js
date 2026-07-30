// Dedicated Worker bootstrap for CashFlux's GWC 5 services WASM.
importScripts("./wasm_exec.js");

const go = new Go();

function instantiateCompressed() {
  if (typeof DecompressionStream !== "function") {
    return Promise.reject(new Error("DecompressionStream unavailable"));
  }
  return fetch("./bin/services.wasm.gz").then((response) => {
    if (!response.ok || !response.body) {
      throw new Error("services.wasm.gz " + response.status);
    }
    const stream = response.body.pipeThrough(new DecompressionStream("gzip"));
    const wasmResponse = new Response(stream, {
      headers: { "Content-Type": "application/wasm" },
    });
    return WebAssembly.instantiateStreaming(wasmResponse, go.importObject);
  });
}

function instantiateRaw() {
  return WebAssembly.instantiateStreaming(
    fetch("./bin/services.wasm"),
    go.importObject,
  );
}

instantiateCompressed()
  .catch(() => instantiateRaw())
  .then((result) => {
    // services main blocks forever; do not await this promise.
    go.run(result.instance);
  })
  .catch((error) => {
    postMessage({
      version: 1,
      kind: "fatal",
      error: {
        code: "Unavailable",
        message: "services worker instantiate failed: " + error,
        mayHaveApplied: false,
      },
    });
  });
