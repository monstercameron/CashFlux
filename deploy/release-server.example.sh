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
