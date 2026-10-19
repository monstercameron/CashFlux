param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string] $WasmBinary,

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]] $WasmArgs = @()
)

$ErrorActionPreference = "Stop"

# Resolve the same Go-provided runner used by lib/wasm/go_js_wasm_exec, without
# sending the binary path or test flags through cmd.exe's batch-file parser.
$goRoot = (& go env GOROOT).Trim()
if ($LASTEXITCODE -ne 0 -or -not $goRoot) {
  throw "Could not resolve GOROOT for the wasm test runner."
}
$nodeScript = Join-Path $goRoot "lib/wasm/wasm_exec_node.js"

# This override is used by the argument round-trip test. Production callers
# leave it unset and always use Go's wasm_exec_node.js.
if ($env:CASHFLUX_WASM_EXEC_NODE_SCRIPT) {
  $nodeScript = $env:CASHFLUX_WASM_EXEC_NODE_SCRIPT
}

$node = (Get-Command node).Source
$childArgs = @("--stack-size=8192", $nodeScript, $WasmBinary) + $WasmArgs

# Windows PowerShell 5.1's .NET Framework lacks ProcessStartInfo.ArgumentList.
# Quote each argument using the Windows CRT rules; Process.Start launches node
# directly, so cmd metacharacters such as | and & have no special meaning.
function ConvertTo-WindowsArgument([string] $Value) {
  if ($Value.Length -gt 0 -and $Value -notmatch '[\s"]') {
    return $Value
  }

  $quoted = [System.Text.StringBuilder]::new()
  [void] $quoted.Append('"')
  $slashes = 0
  foreach ($character in $Value.ToCharArray()) {
    if ($character -eq '\') {
      $slashes++
      continue
    }
    if ($character -eq '"') {
      [void] $quoted.Append(('\' * (2 * $slashes + 1)))
      [void] $quoted.Append('"')
      $slashes = 0
      continue
    }
    if ($slashes -gt 0) {
      [void] $quoted.Append(('\' * $slashes))
      $slashes = 0
    }
    [void] $quoted.Append($character)
  }
  if ($slashes -gt 0) {
    [void] $quoted.Append(('\' * (2 * $slashes)))
  }
  [void] $quoted.Append('"')
  return $quoted.ToString()
}

$startInfo = [System.Diagnostics.ProcessStartInfo]::new()
$startInfo.FileName = $node
$startInfo.Arguments = (($childArgs | ForEach-Object { ConvertTo-WindowsArgument $_ }) -join ' ')
$startInfo.UseShellExecute = $false
$process = [System.Diagnostics.Process]::Start($startInfo)
$process.WaitForExit()
exit $process.ExitCode
