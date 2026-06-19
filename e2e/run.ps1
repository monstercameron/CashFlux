# Builds the wasm app into web/bin, serves web/ on :8099, runs the Playwright
# navigation E2E, then stops the server. Exits with the test's status.
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$srv = $null
try {
  $env:GOOS = "js"; $env:GOARCH = "wasm"
  go build -o web/bin/main.wasm .
  $env:GOOS = ""; $env:GOARCH = ""
  $srv = Start-Process -FilePath "go" -ArgumentList "run", "e2e/serve.go" -PassThru -WindowStyle Hidden
  Start-Sleep -Seconds 2
  node e2e/navigation.test.mjs
  $code = $LASTEXITCODE
}
finally {
  if ($srv) { Stop-Process -Id $srv.Id -Force -ErrorAction SilentlyContinue }
  Pop-Location
}
exit $code
