# Runs the wasm-target Go tests — the middle layer of the test pyramid: component
# tests via GoWebComponents' testkit (mock DOM, no browser) plus the js/wasm-tagged
# registry/browserstore tests. These never run under `go test ./...` (that target
# builds the js&&wasm files out), so they need this dedicated lane.
#
# Scoped to packages that BOTH compile for wasm and hold wasm tests — NOT ./...,
# because internal/server doesn't compile for js/wasm.
#
# Usage:  pwsh tools/wasm-test.ps1
param(
  [string[]] $Packages = @("./internal/screens/", "./internal/ui/", "./internal/browserstore/")
)
$ErrorActionPreference = "Stop"
$repo = Split-Path $PSScriptRoot -Parent
# Put this dir (with go_js_wasm_exec.bat) first so `go test` finds the wrapper.
$env:PATH = "$PSScriptRoot;$env:PATH"
$env:GOOS = "js"; $env:GOARCH = "wasm"
Push-Location $repo
try {
  go test @Packages
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  Write-Host "wasm tests passed"
} finally {
  Pop-Location
}
