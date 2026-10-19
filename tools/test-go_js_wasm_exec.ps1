$ErrorActionPreference = "Stop"
$wrapper = Join-Path $PSScriptRoot "go_js_wasm_exec.ps1"
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("cashflux-wasm-exec-test-" + [guid]::NewGuid().ToString("N"))
$capture = Join-Path $temp "argv.json"
$fakeNode = Join-Path $temp "capture-argv.js"

try {
  New-Item -ItemType Directory -Path $temp | Out-Null
  @'
const fs = require("fs");
fs.writeFileSync(process.env.CASHFLUX_WASM_EXEC_CAPTURE, JSON.stringify(process.argv.slice(2)));
'@ | Set-Content -LiteralPath $fakeNode -Encoding utf8

  $wasmBinary = Join-Path $temp "wasm binary with spaces.wasm"
  $expected = @(
    $wasmBinary,
    "-test.run=A|B",
    '^Test(A|B)$',
    "argument with spaces",
    "argument&with&ampersand"
  )
  $env:CASHFLUX_WASM_EXEC_NODE_SCRIPT = $fakeNode
  $env:CASHFLUX_WASM_EXEC_CAPTURE = $capture
  & $wrapper @expected
  if ($LASTEXITCODE -ne 0) {
    throw "Wrapper exited with code $LASTEXITCODE."
  }

  $actualJSON = (Get-Content -LiteralPath $capture -Raw).Trim()
  Add-Type -AssemblyName System.Web.Extensions
  $serializer = [System.Web.Script.Serialization.JavaScriptSerializer]::new()
  $actual = $serializer.DeserializeObject($actualJSON)
  if ($actual.Length -ne $expected.Length) {
    throw "Wrapper changed the argument count. Expected $($expected.Length); actual $($actual.Length)."
  }
  for ($index = 0; $index -lt $expected.Length; $index++) {
    if ($expected[$index] -cne $actual[$index]) {
      throw "Wrapper changed argument $index. Expected '$($expected[$index])'; actual '$($actual[$index])'."
    }
  }
  Write-Host "PASS: wasm exec wrapper preserves regex, spaces, ampersands, and binary paths"
}
finally {
  Remove-Item Env:CASHFLUX_WASM_EXEC_NODE_SCRIPT -ErrorAction SilentlyContinue
  Remove-Item Env:CASHFLUX_WASM_EXEC_CAPTURE -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $temp) {
    Remove-Item -LiteralPath $temp -Recurse -Force
  }
}
