# Runs wasm-target Go tests in isolated Node processes. By default it preserves
# the original one-process-per-package behavior.
#
# Usage:
#   powershell -File tools/wasm-test.ps1
#   powershell -File tools/wasm-test.ps1 -Packages ./internal/app/ -Shards 6 -v
#   powershell -File tools/wasm-test.ps1 -Packages ./internal/app/ -Run '^TestSave' -count=1
#   powershell -File tools/wasm-test.ps1 -Packages ./internal/app/ -Rerun
#
# Sharded runs compile each package once, divide its top-level tests as evenly
# as possible, and run the shards concurrently. Every run is recorded under
# the system temporary directory; -Rerun uses the latest run's binary and
# failed.txt for each package.
param(
  [string[]] $Packages = @("./internal/screens/", "./internal/ui/", "./internal/browserstore/"),
  [ValidateRange(1, 256)]
  [int] $Shards = 1,
  [switch] $Rerun,
  [string] $Run,
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]] $ExtraArgs = @()
)
$ErrorActionPreference = "Stop"
# PowerShell treats -v as its common -Verbose switch before remaining arguments
# are bound. For this runner, -v is the familiar go test verbosity flag.
if ($PSBoundParameters.ContainsKey('Verbose')) { $ExtraArgs += '-v' }
$repo = Split-Path $PSScriptRoot -Parent

# Resolve everything that needs the caller's full environment before reducing
# it. Go's wasm runtime copies the environment and has an approximately 8 KB cap.
$goExe = (Get-Command go).Source
$powershell = (Get-Command powershell).Source
$nodeDir = Split-Path (Get-Command node).Source -Parent
$goDir = Split-Path $goExe -Parent
$sys32 = Join-Path $env:SystemRoot "System32"
$goCache = (& go env GOCACHE).Trim()
$goModCache = (& go env GOMODCACHE).Trim()
$goPath = (& go env GOPATH).Trim()

$keep = [ordered]@{
  PATH        = "$PSScriptRoot;$nodeDir;$goDir;$sys32;$env:SystemRoot"
  PATHEXT     = ".COM;.EXE;.BAT;.CMD"
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
Get-ChildItem env: | ForEach-Object { Remove-Item "env:$($_.Name)" -ErrorAction SilentlyContinue }
foreach ($key in $keep.Keys) { Set-Item "env:$key" $keep[$key] }

Set-Location -LiteralPath $repo
$wasmExec = Join-Path $PSScriptRoot "go_js_wasm_exec.ps1"
$tempRoot = Join-Path $env:TEMP "cashflux-wasm-tests"
if (-not (Test-Path -LiteralPath $tempRoot)) {
  New-Item -ItemType Directory -Path $tempRoot | Out-Null
}

function Get-PackageKey([string] $Package) {
  return ($Package -replace '[^A-Za-z0-9._-]', '_')
}

function Get-WasmOutput([string] $Binary, [string[]] $WasmArgs) {
  $runnerArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $wasmExec, $Binary) + $WasmArgs
  $output = & $powershell @runnerArgs 2>&1
  $code = $LASTEXITCODE
  return [pscustomobject]@{ ExitCode = $code; Output = @($output) }
}

function Convert-GoTestArgs([string[]] $Arguments) {
  # `go test` accepts these short command-line flags, while a compiled test
  # binary requires the corresponding -test.* spellings.
  $converted = @()
  for ($index = 0; $index -lt $Arguments.Count; $index++) {
    $argument = $Arguments[$index]
    if ($argument -eq '-v') { $converted += '-test.v'; continue }
    if ($argument -match '^-count=(.*)$') { $converted += "-test.count=$($Matches[1])"; continue }
    if ($argument -eq '-count' -or $argument -eq '-timeout' -or $argument -eq '-parallel' -or $argument -eq '-shuffle') {
      if (($index + 1) -ge $Arguments.Count) { throw "Missing value after $argument." }
      $index++
      $converted += "-test.$($argument.Substring(1))=$($Arguments[$index])"
      continue
    }
    if ($argument -match '^-(timeout|parallel|shuffle)=(.*)$') { $converted += "-test.$($Matches[1])=$($Matches[2])"; continue }
    if ($argument -eq '-short' -or $argument -eq '-failfast') { $converted += "-test.$($argument.Substring(1))"; continue }
    $converted += $argument
  }
  return ,$converted
}

function Compile-TestBinary([string] $Package, [string] $Binary) {
  Write-Host "Compiling $Package -> $Binary"
  & go test -c -o $Binary $Package
  if ($LASTEXITCODE -ne 0) {
    throw "Could not compile wasm tests for $Package (exit $LASTEXITCODE)."
  }
}

function Get-TopLevelTests([string] $Package, [string] $Binary, [string] $TestPattern) {
  $listed = Get-WasmOutput $Binary @("-test.list", $TestPattern)
  if ($listed.ExitCode -ne 0) {
    $listed.Output | ForEach-Object { Write-Host $_ }
    throw "Could not list wasm tests for $Package (exit $($listed.ExitCode))."
  }
  return @($listed.Output | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -match '^Test\S+$' })
}

function New-RunRegex([string[]] $Names) {
  if ($Names.Count -eq 0) { return '^$' }
  $escaped = @($Names | ForEach-Object { [regex]::Escape($_) })
  return '^(' + ($escaped -join '|') + ')$'
}

function New-ExactTestRegex([string] $Name) {
  $parts = @($Name -split '/')
  $escapedParts = @($parts | ForEach-Object { '^' + [regex]::Escape($_) + '$' })
  return ($escapedParts -join '/')
}

function Start-WasmShard([string] $PackageDirectory, [string] $Binary, [string] $TestRegex, [string[]] $ForwardArgs, [string] $LogPath) {
  $forwardJson = ConvertTo-Json -InputObject @($ForwardArgs) -Compress
  $environmentJson = ConvertTo-Json -InputObject $keep -Compress
  return Start-Job -ScriptBlock {
    param($PackagePath, $PowerShellPath, $RunnerPath, $WasmBinary, $Regex, $JsonArgs, $Environment, $OutputPath)
    Set-Location -LiteralPath $PackagePath
    $minimalEnvironment = ConvertFrom-Json -InputObject $Environment
    Get-ChildItem env: | ForEach-Object { Remove-Item "env:$($_.Name)" -ErrorAction SilentlyContinue }
    foreach ($entry in $minimalEnvironment.PSObject.Properties) { Set-Item "env:$($entry.Name)" $entry.Value }
    $forwarded = @((ConvertFrom-Json -InputObject $JsonArgs))
    $invokeArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $RunnerPath, $WasmBinary, "-test.run=$Regex") + $forwarded
    & $PowerShellPath @invokeArgs *> $OutputPath
    $exitCode = $LASTEXITCODE
    [pscustomobject]@{ ExitCode = $exitCode }
  } -ArgumentList $PackageDirectory, $powershell, $wasmExec, $Binary, $TestRegex, $forwardJson, $environmentJson, $LogPath
}

function Get-FailedTests([string[]] $LogPaths) {
  $names = @()
  foreach ($path in $LogPaths) {
    if (Test-Path -LiteralPath $path) {
      $matches = Select-String -Path $path -Pattern '^\s*--- FAIL: ([^\s]+)'
      foreach ($match in $matches) { $names += $match.Matches[0].Groups[1].Value }
    }
  }
  return @($names | Sort-Object -Unique)
}

function Write-FailureFile([string] $Path, [string[]] $Names) {
  if ($Names.Count -eq 0) {
    Set-Content -LiteralPath $Path -Value @() -Encoding UTF8
  } else {
    Set-Content -LiteralPath $Path -Value $Names -Encoding UTF8
  }
}

$overallFailed = $false
foreach ($package in $Packages) {
  Set-Location -LiteralPath $repo
  $packageKey = Get-PackageKey $package
  $packageFailed = $false
  $packageDirectory = (& go list -f '{{.Dir}}' $package).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $packageDirectory) { throw "Could not resolve source directory for $package." }
  $latestPath = Join-Path $tempRoot ("latest-{0}.txt" -f $packageKey)
  if ($Rerun) {
    if (-not (Test-Path -LiteralPath $latestPath)) {
      Write-Error "No previous wasm test run is recorded for $package. Run it once before -Rerun."
      $overallFailed = $true
      continue
    }
    $runDir = (Get-Content -LiteralPath $latestPath -Raw).Trim()
    $binary = Join-Path $runDir "$packageKey.test.wasm"
    $failurePath = Join-Path $runDir "failed.txt"
    if (-not (Test-Path -LiteralPath $binary) -or -not (Test-Path -LiteralPath $failurePath)) {
      Write-Error "The previous run for $package is incomplete: $runDir"
      $overallFailed = $true
      continue
    }

    # Rebuild before rerunning so source edits made after the recorded run are included.
    Compile-TestBinary $package $binary
    Set-Location -LiteralPath $packageDirectory
    $failedNames = @(Get-Content -LiteralPath $failurePath | Where-Object { $_.Trim() })
    if ($failedNames.Count -eq 0) {
      Write-Host "${package}: no failed tests to rerun (recorded at $runDir)"
      continue
    }
    if ($Run) {
      Write-Warning "-Run is ignored with -Rerun; rerunning the names recorded in failed.txt."
    }
    # A parent is reported as FAIL when one of its subtests fails. Keep both
    # names in failed.txt, but select only the deepest failed paths on rerun.
    $rerunNames = @($failedNames | Where-Object {
      $candidate = $_
      -not ($failedNames | Where-Object { $_ -like "$candidate/*" } | Select-Object -First 1)
    })
    $rerunLogs = @()
    $timer = [System.Diagnostics.Stopwatch]::StartNew()
    $forwardArgs = Convert-GoTestArgs $ExtraArgs
    $rerunExitCode = 0
    for ($index = 0; $index -lt $rerunNames.Count; $index++) {
      $logPath = Join-Path $runDir ("rerun-{0:D2}.log" -f ($index + 1))
      $rerunLogs += $logPath
      $regex = New-ExactTestRegex $rerunNames[$index]
      $result = Get-WasmOutput $binary (@("-test.run=$regex") + $forwardArgs)
      $result.Output | Set-Content -LiteralPath $logPath -Encoding UTF8
      if ($result.ExitCode -ne 0) { $rerunExitCode = $result.ExitCode }
    }
    $timer.Stop()
    $stillFailed = @(Get-FailedTests $rerunLogs)
    Write-FailureFile $failurePath $stillFailed
    if ($rerunExitCode -eq 0) {
      Write-Host ("{0}: rerun PASS ({1} failed selections, {2:N1}s)" -f $package, $rerunNames.Count, $timer.Elapsed.TotalSeconds)
    } else {
      Write-Host ("{0}: rerun FAIL ({1} failed selections, {2:N1}s)" -f $package, $rerunNames.Count, $timer.Elapsed.TotalSeconds)
      foreach ($path in $rerunLogs) { Get-Content -LiteralPath $path | ForEach-Object { Write-Host $_ } }
      $packageFailed = $true
      $overallFailed = $true
    }
    continue
  }

  $runName = "{0}-{1}" -f (Get-Date -Format "yyyyMMdd-HHmmss-fff"), ([guid]::NewGuid().ToString("N").Substring(0, 8))
  $runDir = Join-Path $tempRoot (Join-Path $packageKey $runName)
  New-Item -ItemType Directory -Path $runDir -Force | Out-Null
  Set-Content -LiteralPath $latestPath -Value $runDir -Encoding UTF8
  $binary = Join-Path $runDir "$packageKey.test.wasm"
  $failurePath = Join-Path $runDir "failed.txt"
  Compile-TestBinary $package $binary

  $tests = @(Get-TopLevelTests $package $binary '.*')
  $runSuffix = $null
  if ($Run) {
    $runParts = $Run -split '/', 2
    $topPattern = $runParts[0]
    try { $tests = @($tests | Where-Object { $_ -match $topPattern }) }
    catch { throw "Invalid -Run top-level expression '$topPattern': $($_.Exception.Message)" }
    if ($runParts.Count -gt 1) { $runSuffix = $runParts[1] }
  }
  $forwardArgs = Convert-GoTestArgs $ExtraArgs
  $shardCount = [Math]::Min($Shards, [Math]::Max(1, $tests.Count))
  $shardTests = @(); for ($i = 0; $i -lt $shardCount; $i++) { $shardTests += ,@() }
  for ($i = 0; $i -lt $tests.Count; $i++) {
    $shardIndex = $i % $shardCount
    $shardTests[$shardIndex] += $tests[$i]
  }

  $jobs = @()
  $logPaths = @()
  $timers = @()
  for ($i = 0; $i -lt $shardCount; $i++) {
    $logPath = Join-Path $runDir ("shard-{0:D2}.log" -f ($i + 1))
    $logPaths += $logPath
    if ($shardCount -eq 1) {
      # Keep the default lane's compact regex; enumerating hundreds of names
      # can exceed the wasm runtime's combined argv/environment limit.
      $regex = if ($Run) { $Run } else { '.*' }
    } else {
      $regex = New-RunRegex $shardTests[$i]
      if ($runSuffix) {
        $topRegex = $regex.Substring(0, $regex.Length - 1)
        $regex = $topRegex + '/' + $runSuffix
      }
    }
    $timers += [System.Diagnostics.Stopwatch]::StartNew()
    $jobs += Start-WasmShard $packageDirectory $binary $regex $forwardArgs $logPath
  }
  $jobResults = @()
  foreach ($job in $jobs) {
    $received = @(Receive-Job -Job $job -Wait -AutoRemoveJob)
    if ($received.Count -gt 0 -and $received[-1].PSObject.Properties.Name -contains 'ExitCode') {
      $jobResults += $received[-1]
    } else {
      $jobResults += [pscustomobject]@{ ExitCode = 1 }
    }
  }

  for ($i = 0; $i -lt $shardCount; $i++) {
    $timers[$i].Stop()
    $status = if ($jobResults[$i].ExitCode -eq 0) { 'PASS' } else { 'FAIL' }
    Write-Host ("{0}: shard {1}/{2} {3} ({4} tests, {5:N1}s) log={6}" -f $package, ($i + 1), $shardCount, $status, $shardTests[$i].Count, $timers[$i].Elapsed.TotalSeconds, $logPaths[$i])
    if ($jobResults[$i].ExitCode -ne 0) {
      $packageFailed = $true
      $overallFailed = $true
      Get-Content -LiteralPath $logPaths[$i] | ForEach-Object { Write-Host $_ }
    }
  }
  $failedNames = @(Get-FailedTests $logPaths)
  Write-FailureFile $failurePath $failedNames
  if ($packageFailed) {
    Write-Host ("{0}: FAIL ({1} failed test names; details in {2})" -f $package, $failedNames.Count, $failurePath)
  } else {
    Write-Host ("{0}: PASS ({1} tests, {2} shards; run directory {3})" -f $package, $tests.Count, $shardCount, $runDir)
  }
}

if ($overallFailed) { exit 1 }
Write-Host "wasm tests passed"
