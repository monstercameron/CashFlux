# Shared source stamp discovery for scripts/dev.ps1 and its background job.
# Walk directories incrementally so generated/tool trees can be pruned before
# their contents are enumerated, and track the newest timestamp in one pass.

function Get-CashFluxDevSourceSnapshot {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root
    )

    $rootPath = [System.IO.Path]::GetFullPath($Root)
    if (-not [System.IO.Directory]::Exists($rootPath)) {
        throw "CashFlux dev watch root does not exist: $rootPath"
    }

    $skipDirectoryNames = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($name in @(".git", ".tools", "node_modules", "web")) {
        [void]$skipDirectoryNames.Add($name)
    }

    $pending = [System.Collections.Generic.Stack[string]]::new()
    $pending.Push($rootPath)
    $newest = [DateTime]::MinValue
    $goFileCount = 0

    while ($pending.Count -gt 0) {
        $directory = $pending.Pop()

        try {
            foreach ($path in [System.IO.Directory]::EnumerateFiles($directory, "*.go", [System.IO.SearchOption]::TopDirectoryOnly)) {
                try {
                    $writeTime = [System.IO.File]::GetLastWriteTimeUtc($path)
                    $goFileCount++
                    if ($writeTime -gt $newest) {
                        $newest = $writeTime
                    }
                }
                catch {
                    # A file can disappear while the watcher is walking.
                }
            }

            foreach ($moduleFile in @("go.mod", "go.sum")) {
                $path = Join-Path $directory $moduleFile
                if ([System.IO.File]::Exists($path)) {
                    try {
                        $writeTime = [System.IO.File]::GetLastWriteTimeUtc($path)
                        if ($writeTime -gt $newest) {
                            $newest = $writeTime
                        }
                    }
                    catch {
                        # A module file can disappear while the watcher is walking.
                    }
                }
            }

            foreach ($child in [System.IO.Directory]::EnumerateDirectories($directory, "*", [System.IO.SearchOption]::TopDirectoryOnly)) {
                try {
                    $attributes = [System.IO.File]::GetAttributes($child)
                    if (($attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                        continue
                    }

                    $trimmedChild = $child.TrimEnd([char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar))
                    $childName = [System.IO.Path]::GetFileName($trimmedChild)
                    if (-not $skipDirectoryNames.Contains($childName)) {
                        $pending.Push($child)
                    }
                }
                catch {
                    # Skip unreadable or concurrently removed directories.
                }
            }
        }
        catch {
            # Continue with sibling directories if one subtree is unreadable.
        }
    }

    [pscustomobject]@{
        LastWriteTimeUtc = $newest
        GoFileCount      = $goFileCount
    }
}
