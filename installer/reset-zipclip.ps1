<#
.SYNOPSIS
    Wipes ZipClip's per-user data so the next launch is a fresh install.

.DESCRIPTION
    Deletes the folders ZipClip creates for the current Windows user:

      %APPDATA%\zipclip        settings (config.json), log, download
                               archive, catalog records, YouTube token
      %LOCALAPPDATA%\zipclip   the self-updated yt-dlp copy (bin\) and
                               the default output\ and work\ folders

    Output or work folders set to somewhere else in config.json are
    listed and left alone. The ZipClip program folder is not touched.

.PARAMETER Force
    Delete without asking for confirmation.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File reset-zipclip.ps1
#>
[CmdletBinding()]
param(
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

if (Get-Process -Name 'zipclip' -ErrorAction SilentlyContinue) {
    Write-Host 'ZipClip is running. Please close it (also from the tray icon) and run this again.'
    exit 1
}

$configDir = Join-Path $env:APPDATA 'zipclip'
$dataDir = Join-Path $env:LOCALAPPDATA 'zipclip'
$targets = @($configDir, $dataDir) | Where-Object { Test-Path $_ }

if ($targets.Count -eq 0) {
    Write-Host 'Nothing to reset: no ZipClip folders found for this user.'
    exit 0
}

# Custom folders are the user's own; report them instead of deleting.
$configFile = Join-Path $configDir 'config.json'
if (Test-Path $configFile) {
    try {
        $config = Get-Content -Raw -Path $configFile | ConvertFrom-Json
        foreach ($name in @('outputDir', 'workDir')) {
            $folder = $config.$name
            if ($folder -and -not $folder.StartsWith($dataDir, [System.StringComparison]::OrdinalIgnoreCase)) {
                Write-Host "Leaving alone the custom $name at $folder"
            }
        }
    } catch {
        Write-Host "Could not read $configFile ($($_.Exception.Message)); custom folders, if any, are left alone."
    }
}

Write-Host 'The following folders will be deleted:'
foreach ($target in $targets) {
    Write-Host "  $target"
}

if (-not $Force) {
    $answer = Read-Host 'Delete them? Type yes to continue'
    if ($answer -ne 'yes') {
        Write-Host 'Nothing deleted.'
        exit 0
    }
}

foreach ($target in $targets) {
    Remove-Item -Path $target -Recurse -Force
    Write-Host "Deleted $target"
}
Write-Host 'ZipClip will start as a fresh install next time.'
