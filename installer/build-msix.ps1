# Builds the Microsoft Store MSIX from desktop\zipclip.exe.
# Usage: .\installer\build-msix.ps1 -Version 0.1.0
#
# Packs directly with makeappx (no MSI, no capture): ZipClip is a single
# portable exe, so the package is just the exe, the license, and the
# logo assets generated from desktop\icon.png. makeappx comes from the
# Microsoft.Windows.SDK.BuildTools NuGet package, auto-downloaded into
# installer\.tools on first run.
#
# The output is unsigned. That is correct for the Store: Partner Center
# accepts unsigned packages and Microsoft signs them. To install a build
# locally for testing, see installer\README.md.
param(
    [Parameter(Mandatory = $true)][string]$Version
)

$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must be three-part, e.g. 0.1.0 (got '$Version')" }
$msixVersion = "$Version.0"   # Store requires 4-part with revision 0

$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root 'desktop\zipclip.exe'
if (-not (Test-Path $exe)) {
    throw "Missing $exe - build it first: cd desktop; go build -trimpath -ldflags -H=windowsgui -o zipclip.exe ."
}

# --- locate or fetch makeappx ---
$toolsDir = Join-Path $PSScriptRoot '.tools'
$makeappx = Get-ChildItem "$toolsDir\sdk\bin\*\x64\makeappx.exe" -ErrorAction SilentlyContinue |
    Select-Object -Last 1 -ExpandProperty FullName
if (-not $makeappx) {
    $makeappx = Get-ChildItem "C:\Program Files (x86)\Windows Kits\10\bin\10.*\x64\makeappx.exe" -ErrorAction SilentlyContinue |
        Select-Object -Last 1 -ExpandProperty FullName
}
if (-not $makeappx) {
    Write-Host 'makeappx not found - downloading Microsoft.Windows.SDK.BuildTools...'
    New-Item -ItemType Directory -Force $toolsDir | Out-Null
    $zip = Join-Path $env:TEMP 'sdkbuildtools.zip'
    Invoke-WebRequest -Uri 'https://www.nuget.org/api/v2/package/Microsoft.Windows.SDK.BuildTools' -OutFile $zip -UseBasicParsing
    Expand-Archive $zip -DestinationPath "$toolsDir\sdk" -Force
    $makeappx = Get-ChildItem "$toolsDir\sdk\bin\*\x64\makeappx.exe" | Select-Object -Last 1 -ExpandProperty FullName
}

# --- stage the package layout ---
$layout = Join-Path $root 'bin\msix-layout'
if (Test-Path $layout) { Remove-Item -Recurse -Force $layout }
New-Item -ItemType Directory -Force "$layout\Assets" | Out-Null

Copy-Item $exe $layout
Copy-Item (Join-Path $root 'desktop\LICENSE') $layout
Copy-Item (Join-Path $root 'desktop\README.md') $layout

(Get-Content (Join-Path $PSScriptRoot 'AppxManifest.xml') -Raw) -replace '\{\{VERSION\}\}', $msixVersion |
    Set-Content (Join-Path $layout 'AppxManifest.xml') -Encoding utf8

# --- generate logo assets from the 256x256 app icon ---
Add-Type -AssemblyName System.Drawing
$srcIcon = [System.Drawing.Image]::FromFile((Join-Path $root 'desktop\icon.png'))
foreach ($asset in @(
        @{ Name = 'StoreLogo.png'; Size = 50 },
        @{ Name = 'Square150x150Logo.png'; Size = 150 },
        @{ Name = 'Square44x44Logo.png'; Size = 44 })) {
    $bmp = New-Object System.Drawing.Bitmap($asset.Size, $asset.Size)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.DrawImage($srcIcon, 0, 0, $asset.Size, $asset.Size)
    $g.Dispose()
    $bmp.Save((Join-Path "$layout\Assets" $asset.Name), [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
}
$srcIcon.Dispose()

# --- pack ---
$msix = Join-Path $root "bin\ZipClip-$Version.msix"
& $makeappx pack /d $layout /p $msix /o
if ($LASTEXITCODE -ne 0) { throw 'makeappx failed' }

Write-Host "Built: $msix"
Write-Host 'Upload it in Partner Center > ZipClip > new submission (Microsoft signs it).'
