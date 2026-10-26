#Requires -Version 5.1
<#
.SYNOPSIS
    Installs Plot Launcher from GitHub Releases on Windows.

.DESCRIPTION
    Downloads the Windows archive for this machine from a Plot Launcher release,
    verifies its SHA-256 checksum against the release's checksums.txt, and
    installs plot-launcher.exe into the user's local application data.

.PARAMETER Version
    Release to install, for example v0.1.0. Defaults to the latest release.

.PARAMETER InstallDir
    Directory to install into. Defaults to %LOCALAPPDATA%\Plot\bin.

.PARAMETER Repository
    Release repository in owner/name form. Defaults to PlotAI/plot-launcher.

.EXAMPLE
    Invoke-RestMethod https://github.com/PlotAI/plot-launcher/releases/latest/download/install.ps1 | Invoke-Expression

.EXAMPLE
    .\install.ps1 -Version v0.1.0 -InstallDir C:\tools\plot
#>
[CmdletBinding()]
param(
    [string]$Version = $env:PLOT_LAUNCHER_VERSION,
    [string]$InstallDir = $env:PLOT_LAUNCHER_INSTALL_DIR,
    [string]$Repository = $env:PLOT_LAUNCHER_REPOSITORY
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not $Repository) { $Repository = 'PlotAI/plot-launcher' }
if (-not $Version) { $Version = 'latest' }
if (-not $InstallDir) {
    $root = if ($env:LOCALAPPDATA) { $env:LOCALAPPDATA } else { [Environment]::GetFolderPath('LocalApplicationData') }
    $InstallDir = Join-Path $root 'Plot\bin'
}

$token = if ($env:GITHUB_TOKEN) { $env:GITHUB_TOKEN } elseif ($env:GH_TOKEN) { $env:GH_TOKEN } else { $null }

function Write-Step([string]$Message) { Write-Host "-> $Message" -ForegroundColor DarkGray }
function Write-Ok([string]$Message) { Write-Host "OK $Message" -ForegroundColor Green }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    'x86' { throw 'Plot Launcher requires a 64-bit Windows installation.' }
    default { throw "Unsupported processor architecture: $env:PROCESSOR_ARCHITECTURE" }
}

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$headers = @{
    'Accept'     = 'application/vnd.github+json'
    'User-Agent' = 'plot-launcher-installer'
}
if ($token) { $headers['Authorization'] = "Bearer $token" }

$release = $null
if ($Version -eq 'latest') {
    Write-Step 'Resolving the latest Plot Launcher release'
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest" -Headers $headers
    }
    catch {
        throw "Could not query releases for $Repository. If the repository is private, set GITHUB_TOKEN. $($_.Exception.Message)"
    }
    $Version = $release.tag_name
}
if (-not $Version) { throw 'Could not determine which Plot Launcher version to install.' }

$tag = if ($Version.StartsWith('v')) { $Version } else { "v$Version" }
$versionNumber = $tag.TrimStart('v')
$asset = "plot-launcher_${versionNumber}_windows_${arch}.zip"
$baseUrl = "https://github.com/$Repository/releases/download/$tag"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("plot-launcher-install-" + [guid]::NewGuid().ToString('N'))

if ($token -and -not $release) {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/tags/$tag" -Headers $headers
    }
    catch {
        throw "Could not read release $tag of $Repository. $($_.Exception.Message)"
    }
}

function Get-ReleaseAsset([string]$Name) {
    $target = Join-Path $tempDir $Name
    if ($release) {
        $assetInfo = $release.assets | Where-Object { $_.name -eq $Name } | Select-Object -First 1
        if (-not $assetInfo) { throw "release $tag does not contain $Name" }
        $assetHeaders = @{
            'Accept'        = 'application/octet-stream'
            'User-Agent'    = 'plot-launcher-installer'
            'Authorization' = "Bearer $token"
        }
        Invoke-WebRequest -Uri $assetInfo.url -Headers $assetHeaders -OutFile $target -UseBasicParsing
        return $target
    }
    try {
        Invoke-WebRequest -Uri "$baseUrl/$Name" -OutFile $target -UseBasicParsing
        return $target
    }
    catch {
        $gh = Get-Command gh -ErrorAction SilentlyContinue
        if ($gh) {
            Write-Step "Downloading $Name with the GitHub CLI"
            & $gh.Source release download $tag --repo $Repository --pattern $Name --dir $tempDir --clobber
            if (Test-Path $target) { return $target }
        }
        throw "Could not download $Name from $baseUrl. If the repository is private, set GITHUB_TOKEN. $($_.Exception.Message)"
    }
}

try {
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    Write-Step "Downloading Plot Launcher $tag for windows/$arch"
    $archive = Get-ReleaseAsset $asset
    $checksums = Get-ReleaseAsset 'checksums.txt'

    $line = Get-Content -Path $checksums |
        Where-Object { ($_ -split '\s+')[1] -eq $asset } |
        Select-Object -First 1
    if (-not $line) { throw "checksums.txt does not list $asset" }

    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Path $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -ne $actual) { throw "Checksum verification failed for $asset" }
    Write-Step 'Verified SHA-256 checksum'

    Expand-Archive -Path $archive -DestinationPath $tempDir -Force
    $binary = Join-Path $tempDir 'plot-launcher.exe'
    if (-not (Test-Path $binary)) { throw "$asset did not contain plot-launcher.exe" }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir 'plot-launcher.exe'
    $staged = Join-Path $InstallDir ('.plot-launcher.' + [guid]::NewGuid().ToString('N') + '.tmp')
    Copy-Item -Path $binary -Destination $staged -Force
    try {
        Move-Item -Path $staged -Destination $target -Force
    }
    catch {
        Remove-Item -Path $staged -Force -ErrorAction SilentlyContinue
        throw "Could not replace $target. Close any running plot-launcher process and try again. $($_.Exception.Message)"
    }
}
finally {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

$installedVersion = & $target version

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$entries = @()
if ($userPath) { $entries = @($userPath -split ';' | Where-Object { $_ }) }
if ($entries -notcontains $InstallDir) {
    $newPath = (@($entries) + $InstallDir) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    $pathAdded = $true
}
else {
    $pathAdded = $false
}
$env:Path = "$InstallDir;$env:Path"

Write-Host ''
Write-Ok "Plot Launcher $tag installed"
Write-Host "  binary    $target"
Write-Host "  version   $installedVersion"
Write-Host "  platform  windows/$arch"

# A browser link reaches the launcher through the operating system, so the web
# page needs no knowledge of the loopback address or port.
Write-Step 'Registering the plot:// scheme'
try {
    $schemeKey = 'HKCU:\Software\Classes\plot'
    New-Item -Path $schemeKey -Force | Out-Null
    Set-ItemProperty -Path $schemeKey -Name 'URL Protocol' -Value ''
    New-Item -Path "$schemeKey\shell\open\command" -Force | Out-Null
    Set-Item -Path "$schemeKey\shell\open\command" -Value ('"' + $target + '" "%1"')
}
catch {
    Write-Host "! Could not register plot:// links. $($_.Exception.Message)" -ForegroundColor Yellow
}
if ($pathAdded) {
    Write-Host ''
    Write-Host "! Added $InstallDir to your user PATH. Restart your terminal to pick it up." -ForegroundColor Yellow
}

Write-Host ''
Write-Host 'Next steps' -ForegroundColor White
Write-Host '  plot-launcher settings set-base-directory "$HOME\plot-repositories"'
Write-Host '  plot-launcher settings set-editor "C:\absolute\path\to\editor.exe"'
Write-Host '  plot-launcher'
Write-Host ''
Write-Host "Upgrade later by running this installer again; uninstall by deleting $target and the HKCU:\Software\Classes\plot key."
