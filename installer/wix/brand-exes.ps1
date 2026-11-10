<#
Stamps a brand's name on its copies of the app and the service, so Windows shows
the brand where it reads the program's own details: the "Files in Use" box of
the installer, Task Manager, the Apps list, the file's Properties. All brands
ship the same two programs under different file names, and the version
information inside them says "Salvanza", so without this a Nimbus Backup
install lists "Salvanza for Proxmox Backup Server" there.

  .\brand-exes.ps1 -Bin gui\build\bin -Base nimbusbackup -Title "Nimbus Backup" `
                   -Version 2.0.0 -FileVersion 2.0.0-dev-abc1234

It edits <Base>-gui.exe and <Base>-svc.exe in -Bin in place: file description,
product name, company, internal and original file names, versions, and the
app's own icon (installer\icons\<Base>-gui.ico). The copyright line stays as it
is. Run it on the per-brand copies, never on the build's salvanza-*.exe.

The tool is rcedit (https://github.com/electron/rcedit), pinned to a release
and a SHA-256, downloaded once into -ToolDir (or pass -RcEdit to use your own).
#>
param(
    [Parameter(Mandatory)][string]$Bin,
    [Parameter(Mandatory)][string]$Base,
    [Parameter(Mandatory)][string]$Title,
    [string]$Company = '',
    [Parameter(Mandatory)][string]$Version,      # x.y.z
    [string]$FileVersion = '',                    # shown text, e.g. 2.0.0-dev-abc1234
    [string]$IconDir = '',
    [string]$RcEdit = '',
    [string]$ToolDir = ''
)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "version '$Version' must be x.y.z" }
if (-not $FileVersion) { $FileVersion = $Version }
if (-not $Company) { $Company = $Title }
if (-not $IconDir) { $IconDir = Join-Path $PSScriptRoot '..\icons' }
if (-not $ToolDir) { $ToolDir = Join-Path ([IO.Path]::GetTempPath()) 'rcedit' }

$rcVersion = 'v2.0.0'
$rcSha256 = '3e7801db1a5edbec91b49a24a094aad776cb4515488ea5a4ca2289c400eade2a'
if (-not $RcEdit) {
    $RcEdit = Join-Path $ToolDir 'rcedit-x64.exe'
    if (-not (Test-Path $RcEdit)) {
        New-Item -ItemType Directory -Force $ToolDir | Out-Null
        Invoke-WebRequest "https://github.com/electron/rcedit/releases/download/$rcVersion/rcedit-x64.exe" -OutFile $RcEdit -UseBasicParsing
    }
    $got = (Get-FileHash $RcEdit -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($got -ne $rcSha256) { Remove-Item $RcEdit -Force; throw "rcedit $rcVersion has SHA-256 $got, expected $rcSha256" }
}

$four = "$Version.0"
foreach ($kind in 'gui', 'svc') {
    $exe = Join-Path $Bin "$Base-$kind.exe"
    if (-not (Test-Path $exe)) { throw "missing $exe" }
    $desc = if ($kind -eq 'svc') { "$Title Service" } else { $Title }
    $rcArgs = @($exe,
        '--set-file-version', $four, '--set-product-version', $four,
        '--set-version-string', 'FileDescription', $desc,
        '--set-version-string', 'ProductName', $Title,
        '--set-version-string', 'CompanyName', $Company,
        '--set-version-string', 'InternalName', "$Base-$kind",
        '--set-version-string', 'OriginalFilename', "$Base-$kind.exe",
        '--set-version-string', 'FileVersion', $FileVersion,
        '--set-version-string', 'ProductVersion', $FileVersion)
    if ($kind -eq 'gui') {
        $ico = Join-Path $IconDir "$Base-gui.ico"
        if (-not (Test-Path $ico)) { throw "missing icon $ico" }
        $rcArgs += @('--set-icon', $ico)
    }
    & $RcEdit @rcArgs
    if ($LASTEXITCODE) { throw "rcedit failed on $exe (exit $LASTEXITCODE)" }
    # Read it back: a silent no-op would ship the wrong name again.
    $vi = [Diagnostics.FileVersionInfo]::GetVersionInfo((Resolve-Path $exe).Path)
    if ($vi.FileDescription -ne $desc -or $vi.ProductName -ne $Title) {
        throw "$exe still reports '$($vi.FileDescription)' / '$($vi.ProductName)' after stamping"
    }
    Write-Host "branded $Base-$kind.exe as '$desc'"
}
