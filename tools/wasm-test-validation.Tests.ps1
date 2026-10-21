# Isolated checks for the compile artifact guard. This loads only the pure
# Assert-WasmBinary function from wasm-test.ps1; it does not run that script's
# environment setup, invoke Go, or create a test run directory.
$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'wasm-test.ps1'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw "Could not parse ${scriptPath}: $($parseErrors[0].Message)" }
$functionAst = $ast.Find({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Assert-WasmBinary' }, $true)
if ($null -eq $functionAst) { throw "Assert-WasmBinary was not found in $scriptPath." }
Invoke-Expression $functionAst.Extent.Text

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("cashflux-wasm-header-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
  function Assert-Rejected([string] $Name, [string] $Path, [string] $ExpectedText) {
    try {
      Assert-WasmBinary './internal/screens/' $Path
      throw "${Name}: expected artifact validation to fail."
    } catch {
      if ($_.Exception.Message -notlike "*$ExpectedText*") { throw "${Name}: unexpected error: $($_.Exception.Message)" }
      if ($_.Exception.Message -notlike '*./internal/screens/*' -or $_.Exception.Message -notlike "*$Path*") {
        throw "${Name}: error did not identify package and path: $($_.Exception.Message)"
      }
    }
  }

  $missing = Join-Path $tempDir 'missing.test.wasm'
  Assert-Rejected 'missing' $missing 'was not created'

  $empty = Join-Path $tempDir 'empty.test.wasm'
  [System.IO.File]::WriteAllBytes($empty, [byte[]]@())
  Assert-Rejected 'empty' $empty 'too short (0 bytes)'

  $truncated = Join-Path $tempDir 'truncated.test.wasm'
  [System.IO.File]::WriteAllBytes($truncated, [byte[]](0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00))
  Assert-Rejected 'truncated' $truncated 'too short (7 bytes)'

  $invalid = Join-Path $tempDir 'invalid.test.wasm'
  [System.IO.File]::WriteAllBytes($invalid, [byte[]](0x00, 0x61, 0x73, 0x6d, 0x02, 0x00, 0x00, 0x00))
  Assert-Rejected 'invalid-header' $invalid 'invalid WebAssembly header'

  $invalidMagic = Join-Path $tempDir 'invalid-magic.test.wasm'
  [System.IO.File]::WriteAllBytes($invalidMagic, [byte[]](0x01, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00))
  Assert-Rejected 'invalid-magic' $invalidMagic 'invalid WebAssembly header'

  $locked = Join-Path $tempDir 'locked.test.wasm'
  [System.IO.File]::WriteAllBytes($locked, [byte[]](0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00))
  $lock = [System.IO.File]::Open($locked, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
  try {
    Assert-Rejected 'unreadable' $locked 'could not be read'
  } finally {
    $lock.Dispose()
  }

  $valid = Join-Path $tempDir 'valid.test.wasm'
  [System.IO.File]::WriteAllBytes($valid, [byte[]](0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00))
  Assert-WasmBinary './internal/screens/' $valid
  Write-Output 'PASS: missing, empty, truncated, invalid, and valid WASM headers'
} finally {
  Remove-Item -LiteralPath $tempDir -Recurse -Force
}
