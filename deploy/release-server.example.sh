#!/usr/bin/env sh
set -eu

# Example backend release helper. Run from the repository root.
# The output is deterministic for a fixed source tree, Go toolchain, GOOS/GOARCH,
# and dependency cache. Signing and upload remain operator-controlled.

version="${CASHFLUX_RELEASE_VERSION:-$(git rev-parse --short=12 HEAD)}"
out_dir="${CASHFLUX_RELEASE_OUT_DIR:-dist/server}"
goos="${GOOS:-linux}"
goarch="${GOARCH:-amd64}"

mkdir -p "$out_dir"

# Build the primary app/services worker plus the two console SPAs the server
# serves from web, web/admin (operator), and web/portal (customer self-service).
# Their compiled .wasm is git-ignored, so a fresh
# checkout has none — this step is what makes a release bundle actually contain them.
# Each browser surface needs the Go wasm runtime glue (wasm_exec.js) beside it.
if [ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then
  wasm_exec="$(go env GOROOT)/lib/wasm/wasm_exec.js"   # Go 1.24+
else
  wasm_exec="$(go env GOROOT)/misc/wasm/wasm_exec.js"  # older layout
fi
build_console() {
  # $1 = binary name, $2 = package path, $3 = destination dir
  GOOS=js GOARCH=wasm CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o "$3/$1.wasm" \
    "$2"
  cp "$wasm_exec" "$3/wasm_exec.js"
  echo "built $3/$1.wasm"
}
mkdir -p web/bin
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w -buildid=" \
  -o web/bin/main.wasm \
  .
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w -buildid=" \
  -o web/bin/services.wasm \
  ./cmd/cashflux-services
cp "$wasm_exec" web/wasm_exec.js
build_console admin ./cmd/cashflux-admin web/admin
build_console portal ./cmd/cashflux-portal web/portal

artifact="$out_dir/cashflux-server_${version}_${goos}_${goarch}"
sbom="$artifact.cdx.json"
checksums="$out_dir/SHA256SUMS"

GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
  -trimpath \
  -buildvcs=true \
  -ldflags="-s -w -buildid=" \
  -o "$artifact" \
  ./cmd/cashflux-server

sha256sum "$artifact" > "$checksums"

go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest app \
  -json \
  -output "$sbom" \
  -main ./cmd/cashflux-server

cosign sign-blob --yes --output-signature "$artifact.sig" "$artifact"
cosign sign-blob --yes --output-signature "$sbom.sig" "$sbom"

echo "wrote $artifact"
echo "wrote $sbom"
echo "wrote $checksums"
