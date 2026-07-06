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

# The wasm test runner (lib/wasm/wasm_exec.js) copies the ENTIRE environment into
# the Go wasm program, but Go's wasm runtime allots only ~8 KB for argv+env. CI's
# environment (a multi-KB PATH plus dozens of GITHUB_*/RUNNER_* vars) blows past
# that ("total length of command line and environment variables exceeds limit").
# So resolve what the toolchain needs, then run go test with a MINIMAL env. This is
# a no-op-ish safety on a dev box (small env) and the fix on CI.
$goExe   = (Get-Command go).Source
$nodeDir = Split-Path (Get-Command node).Source -Parent
$goDir   = Split-Path $goExe -Parent
$sys32   = Join-Path $env:SystemRoot "System32"
# Capture cache locations (needs the full PATH, so read them BEFORE scrubbing).
$goCache    = (& go env GOCACHE).Trim()
$goModCache = (& go env GOMODCACHE).Trim()
$goPath     = (& go env GOPATH).Trim()

$keep = [ordered]@{
  PATH        = "$PSScriptRoot;$nodeDir;$goDir;$sys32;$env:SystemRoot"
  PATHEXT     = ".COM;.EXE;.BAT;.CMD"  # Windows needs this to resolve `go` -> go.exe
  SystemRoot  = $env:SystemRoot
  ComSpec     = $env:ComSpec
  USERPROFILE = $env:USERPROFILE
  TEMP        = $env:TEMP
  TMP         = $env:TMP
  GOCACHE     = $goCache
  GOMODCACHE  = $goModCache
  GOPATH      = $goPath
  GOFLAGS     = "-mod=mod"
  GOOS        = "js"
  GOARCH      = "wasm"
}
# Wipe the environment, then set only the keepers.
Get-ChildItem env: | ForEach-Object { Remove-Item "env:$($_.Name)" -ErrorAction SilentlyContinue }
foreach ($k in $keep.Keys) { Set-Item "env:$k" $keep[$k] }

Set-Location $repo
& go test @Packages
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "wasm tests passed"
