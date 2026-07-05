# install.ps1 - PowerShell installer for Goo CLI on Windows
$ErrorActionPreference = 'Stop'

$Repo = "kingjethro999/goo"
$BinaryName = "goo.exe"
$InstallDir = "$env:LOCALAPPDATA\goo\bin"

# Detect architecture
$Arch = $env:PROCESSOR_ARCHITECTURE
switch -Regex ($Arch) {
    'AMD64|x86_64' { $Arch = 'amd64' }
    'ARM64'        { $Arch = 'arm64' }
    Default        { 
        Write-Error "Unsupported architecture: $Arch"
        exit 1
    }
}

# Fetch latest release version from GitHub API
try {
    $ReleaseApi = "https://api.github.com/repos/$Repo/releases/latest"
    $Response = Invoke-RestMethod -Uri $ReleaseApi -UseBasicParsing
    $VersionTag = $Response.tag_name
    $Version = $VersionTag -replace '^v', ''
} catch {
    Write-Error "Failed to fetch latest release version from GitHub API: $_"
    exit 1
}

Write-Host "Installing Goo v$Version for windows/$Arch..." -ForegroundColor Cyan

# Construct release download URL
$ZipName = "goo_${Version}_windows_${Arch}.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$VersionTag/$ZipName"

# Create temporary workspace
$TempDir = Join-Path $env:TEMP ("goo-install-" + [System.Guid]::NewGuid().ToString())
$ZipPath = Join-Path $TempDir $ZipName
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

try {
    Write-Host "Downloading $DownloadUrl..." -ForegroundColor Gray
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

    Write-Host "Extracting archive..." -ForegroundColor Gray
    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

    # Ensure target installation directory exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    $SourceBinary = Join-Path $TempDir $BinaryName
    $TargetBinary = Join-Path $InstallDir $BinaryName

    if (-not (Test-Path $SourceBinary)) {
        Write-Error "Binary $BinaryName not found in extracted archive."
        exit 1
    }

    Copy-Item -Path $SourceBinary -Destination $TargetBinary -Force

    # Add to User PATH if not already present
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -split ';' -notcontains $InstallDir) {
        Write-Host "Adding $InstallDir to User PATH..." -ForegroundColor Yellow
        $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        $env:Path = "$env:Path;$InstallDir"
    }

    Write-Host "`n✓ Goo successfully installed to $TargetBinary`n" -ForegroundColor Green
    Write-Host "Get started:" -ForegroundColor Cyan
    Write-Host "  goo config set-key groq"
    Write-Host "  goo chat`n"
    Write-Host "Note: Restart your terminal window if 'goo' is not immediately recognized.`n" -ForegroundColor Yellow

} finally {
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
