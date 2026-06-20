# Runs the whole E2E suite as one command: builds the wasm app once, serves web/
# on :8099, then runs every Playwright story (*.test.mjs) and feature check
# (*_check.mjs) against it, each in its own fresh browser (so tests are isolated).
# Prints a per-file result and a summary; exits non-zero if any file fails.
#
# Usage:  .\e2e\run-stories.ps1
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$srv = $null
$failed = @()
$passed = @()
try {
  $env:GOOS = "js"; $env:GOARCH = "wasm"
  Write-Host "Building wasm..."
  go build -o web/bin/main.wasm .
  $env:GOOS = ""; $env:GOARCH = ""

  $srv = Start-Process -FilePath "go" -ArgumentList "run", "e2e/serve.go" -PassThru -WindowStyle Hidden
  Start-Sleep -Seconds 3

  # Every assertion-bearing test: the user-journey stories and the feature checks.
  # (theme_shot.mjs is a screenshot tool with no assertions, so it is excluded.)
  $tests = Get-ChildItem -Path (Join-Path $root "e2e") -Filter "*.mjs" |
    Where-Object { $_.Name -like "*.test.mjs" -or $_.Name -like "*_check.mjs" } |
    Sort-Object Name
  foreach ($t in $tests) {
    Write-Host ""
    Write-Host "--- $($t.Name) ---"
    node "e2e/$($t.Name)"
    if ($LASTEXITCODE -eq 0) { $passed += $t.Name } else { $failed += $t.Name }
  }
}
finally {
  if ($srv) { Stop-Process -Id $srv.Id -Force -ErrorAction SilentlyContinue }
  Pop-Location
}

Write-Host ""
Write-Host "=========================================="
Write-Host "E2E suite: $($passed.Count) passed, $($failed.Count) failed"
if ($failed.Count -gt 0) {
  Write-Host "FAILED: $($failed -join ', ')"
  exit 1
}
Write-Host "ALL GREEN"
exit 0
