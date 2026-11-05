<#
Builds a Windows installer (Salvanza-<version>-x64.msi, or a brand's) from the
programs in gui\build\bin: the app, the service and the command line. Build
those first: wails build (salvanza-gui.exe), go build -tags service -o
gui\build\bin\salvanza-svc.exe ./gui, and go build -o
gui\build\bin\salvanza-cli.exe ./cli.

  .\build-msi.ps1 [-Version 2.0.0] [-Brand Salvanza|NimbusBackup|EtitechBackup]
                  [-WixDir C:\Dev\tools\wix314] [-OutDir out]

The version defaults to gui\wails.json's productVersion; WiX 3 (candle.exe and
light.exe) is found in -WixDir or on PATH (https://github.com/wixtoolset/wix3/releases,
the wix314-binaries.zip is enough). A brand other than Salvanza gets copies of
the app and the service under its own names (<brand>-gui.exe, <brand>-svc.exe),
which is what makes the app take that brand.
#>
param(
    [string]$Version = '',
    [ValidateSet('Salvanza', 'NimbusBackup', 'EtitechBackup')]
    [string]$Brand = 'Salvanza',
    [string]$WixDir = 'C:\Dev\tools\wix314',
    [string]$OutDir = ''
)
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$repo = Resolve-Path (Join-Path $here '..\..')
$bin = Join-Path $repo 'gui\build\bin'
if (-not $OutDir) { $OutDir = Join-Path $here 'out' }

if (-not $Version) {
    $Version = (Get-Content (Join-Path $repo 'gui\wails.json') -Raw | ConvertFrom-Json).info.productVersion
}
# MSI versions are three numbers: "2.0.0-rc1" or "dev-..." are not allowed.
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "version '$Version' must be x.y.z" }

$candle = Join-Path $WixDir 'candle.exe'
$light = Join-Path $WixDir 'light.exe'
if (-not (Test-Path $candle)) { $candle = (Get-Command candle.exe -ErrorAction Stop).Source; $light = (Get-Command light.exe).Source }

foreach ($exe in 'salvanza-gui.exe', 'salvanza-svc.exe', 'salvanza-cli.exe') {
    if (-not (Test-Path (Join-Path $bin $exe))) { throw "gui\build\bin\$exe is missing: build it first" }
}

# The brand's WiX file and program names (installer\wix\<Brand>.wxs).
$wxs = 'Product.wxs'
if ($Brand -ne 'Salvanza') {
    $wxs = "$Brand.wxs"
    $base = $Brand.ToLowerInvariant()
    Copy-Item (Join-Path $bin 'salvanza-gui.exe') (Join-Path $bin "$base-gui.exe") -Force
    Copy-Item (Join-Path $bin 'salvanza-svc.exe') (Join-Path $bin "$base-svc.exe") -Force
}

New-Item -ItemType Directory -Force $OutDir | Out-Null
$obj = Join-Path $OutDir "$Brand.wixobj"
$msi = Join-Path $OutDir "$Brand-$Version-x64.msi"
Push-Location $here
try {
    & $candle -nologo -arch x64 "-dProductVersion=$Version" -ext WixUIExtension -ext WixUtilExtension -out $obj $wxs
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
