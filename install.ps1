<#
.SYNOPSIS
    Installs the latest release of ghostport.

.DESCRIPTION
    Detects your architecture, downloads the matching release from GitHub,
    verifies its checksum, and installs it to %LOCALAPPDATA%\ghostport\bin
    (no administrator rights required), adding that folder to your user PATH.

.EXAMPLE
    irm https://raw.githubusercontent.com/mubashirtaj/ghostport/main/install.ps1 | iex
#>
[CmdletBinding()]
param(
    [string] $Version,
    [string] $InstallDir = "$env:LOCALAPPDATA\ghostport\bin"
)

$ErrorActionPreference = 'Stop'
$ProgressPreference    = 'SilentlyContinue'  # keeps Invoke-WebRequest fast

$Repo    = 'mubashirtaj/ghostport'
$BinName = 'ghostport'

function Write-Log { param([string] $Message) Write-Host '==> ' -ForegroundColor Magenta -NoNewline; Write-Host $Message }
function Write-Err { param([string] $Message) Write-Host 'ERROR: ' -ForegroundColor Red -NoNewline; Write-Host $Message; exit 1 }

# TLS 1.2 for Windows PowerShell 5.1, whose default is often too old for GitHub.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# --- Detect architecture ---------------------------------------------------
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $Arch = 'amd64' }
    'ARM64' { $Arch = 'arm64' }
    'x86'   {
        # A 32-bit shell on a 64-bit host still reports x86; check the host.
        if ($env:PROCESSOR_ARCHITEW6432 -eq 'AMD64')      { $Arch = 'amd64' }
        elseif ($env:PROCESSOR_ARCHITEW6432 -eq 'ARM64')  { $Arch = 'arm64' }
        else { Write-Err '32-bit Windows is not supported. ghostport ships amd64 and arm64 builds only.' }
    }
    default { Write-Err "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

Write-Log "Detected platform: windows/$Arch"

# --- Resolve version -------------------------------------------------------
if (-not $Version) {
    Write-Log 'Fetching latest release information...'
    try {
        $Version = (Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ 'User-Agent' = 'ghostport-installer' }).tag_name
    } catch {
        Write-Err "Could not determine the latest ghostport version. $($_.Exception.Message)"
    }
}
if (-not $Version) { Write-Err 'Could not determine the latest ghostport version.' }
if (-not $Version.StartsWith('v')) { $Version = "v$Version" }

Write-Log "Version: $Version"

# --- Download --------------------------------------------------------------
$VersionNum = $Version.TrimStart('v')
$Archive    = "${BinName}_${VersionNum}_windows_${Arch}.zip"
$BaseUrl    = "https://github.com/$Repo/releases/download/$Version"

$TmpDir = Join-Path ([IO.Path]::GetTempPath()) "$BinName-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    $ZipPath = Join-Path $TmpDir $Archive

    Write-Log "Downloading $BaseUrl/$Archive"
    try {
        Invoke-WebRequest -Uri "$BaseUrl/$Archive" -OutFile $ZipPath -UseBasicParsing
    } catch {
        Write-Err "Failed to download $BaseUrl/$Archive - $($_.Exception.Message)"
    }

    # --- Verify checksum ---------------------------------------------------
    Write-Log 'Verifying checksum...'
    try {
        # Download to disk rather than reading .Content: Windows PowerShell 5.1
        # hands back a byte[] when the server labels the file octet-stream.
        $SumsPath = Join-Path $TmpDir 'checksums.txt'
        Invoke-WebRequest -Uri "$BaseUrl/checksums.txt" -OutFile $SumsPath -UseBasicParsing

        $Expected = (Get-Content -Path $SumsPath | Where-Object { $_ -match [regex]::Escape($Archive) } |
                     ForEach-Object { ($_.Trim() -split '\s+')[0] } | Select-Object -First 1)

        if ($Expected) {
            $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash
            if ($Actual -ne $Expected.ToUpper()) {
                Write-Err "Checksum mismatch for $Archive.`n  expected: $Expected`n  actual:   $Actual"
            }
            Write-Log 'Checksum OK'
        } else {
            Write-Warning "No checksum entry found for $Archive - skipping verification."
        }
    } catch {
        Write-Warning "Could not fetch checksums.txt - skipping verification. $($_.Exception.Message)"
    }

    # --- Extract -----------------------------------------------------------
    Write-Log 'Extracting archive...'
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force

    $Extracted = Join-Path $TmpDir "$BinName.exe"
    if (-not (Test-Path $Extracted)) { Write-Err 'Binary not found in downloaded archive.' }

    # --- Install -----------------------------------------------------------
    if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }
    $Target = Join-Path $InstallDir "$BinName.exe"

    Write-Log "Installing to $Target"
    try {
        Move-Item -Path $Extracted -Destination $Target -Force
    } catch {
        Write-Err "Could not write to $Target - it may be running. Close any open ghostport process and retry."
    }

    # --- Add to user PATH --------------------------------------------------
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $UserPath) { $UserPath = '' }

    $OnPath = ($UserPath -split ';' | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })
    if (-not $OnPath) {
        Write-Log "Adding $InstallDir to your user PATH"
        $NewPath = if ($UserPath.TrimEnd(';')) { "$($UserPath.TrimEnd(';'));$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')
        $PathChanged = $true
    }

    # Make it usable in the current session too.
    if ($env:Path -notlike "*$InstallDir*") { $env:Path = "$env:Path;$InstallDir" }

    Write-Log "ghostport $Version installed successfully!"
    & $Target --version

    if ($PathChanged) {
        Write-Host ''
        Write-Warning 'PATH was updated. Open a NEW terminal before running ghostport elsewhere.'
    }
}
finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
