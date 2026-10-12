<#
Builds the Windows installer (ProxmoxBackupClient-<version>-x64.msi) from the
GUI and service programs in gui\build\bin. Build those first (wails build, and
go build -tags service -o gui\build\bin\ProxmoxBackupClientSVC.exe ./gui).

  .\build-msi.ps1 [-Version 0.9.0] [-WixDir C:\Dev\tools\wix314] [-OutDir out]

The version defaults to gui\wails.json's productVersion; WiX 3 (candle.exe and
light.exe) is found in -WixDir or on PATH (https://github.com/wixtoolset/wix3/releases,
the wix314-binaries.zip is enough).
#>
param(
    [string]$Version = '',
    [string]$WixDir = 'C:\Dev\tools\wix314',
    [string]$OutDir = ''
)
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$repo = Resolve-Path (Join-Path $here '..\..')
if (-not $OutDir) { $OutDir = Join-Path $here 'out' }

if (-not $Version) {
    $Version = (Get-Content (Join-Path $repo 'gui\wails.json') -Raw | ConvertFrom-Json).info.productVersion
}
# MSI versions are three numbers: "0.9.0-rc1" or "dev-..." are not allowed.
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "version '$Version' must be x.y.z" }

$candle = Join-Path $WixDir 'candle.exe'
$light = Join-Path $WixDir 'light.exe'
if (-not (Test-Path $candle)) { $candle = (Get-Command candle.exe -ErrorAction Stop).Source; $light = (Get-Command light.exe).Source }

foreach ($exe in 'ProxmoxBackupClient.exe', 'ProxmoxBackupClientSVC.exe') {
    if (-not (Test-Path (Join-Path $repo "gui\build\bin\$exe"))) { throw "gui\build\bin\$exe is missing: build it first" }
}

New-Item -ItemType Directory -Force $OutDir | Out-Null
$obj = Join-Path $OutDir 'Product.wixobj'
$msi = Join-Path $OutDir "ProxmoxBackupClient-$Version-x64.msi"
Push-Location $here
try {
    & $candle -nologo -arch x64 "-dProductVersion=$Version" -ext WixUIExtension -ext WixUtilExtension -out $obj Product.wxs
    if ($LASTEXITCODE) { throw 'candle failed' }
    # ICE61 warns about AllowSameVersionUpgrades, which is wanted (a rebuilt
    # x.y.z replaces an installed x.y.z).
    & $light -nologo -ext WixUIExtension -ext WixUtilExtension -sice:ICE61 -out $msi $obj
    if ($LASTEXITCODE) { throw 'light failed' }
} finally {
    Pop-Location
}
Remove-Item $obj, ([IO.Path]::ChangeExtension($msi, '.wixpdb')) -ErrorAction SilentlyContinue
Get-Item $msi | Select-Object Name, Length
(Get-FileHash $msi).Hash
