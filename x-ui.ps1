# Color definitions
$colors = @{
    Red = "`e[31m"
    Green = "`e[32m"
    Yellow = "`e[33m"
    Blue = "`e[34m"
    Plain = "`e[0m"
}

# Basic logging functions
function Write-Log {
    param (
        [string]$Level,
        [string]$Message
    )
    
    $color = switch ($Level) {
        "DEG" { $colors.Yellow }
        "ERR" { $colors.Red }
        "INF" { $colors.Green }
        default { $colors.Plain }
    }
    
    Write-Host "$($color)[$Level] $Message$($colors.Plain)"
}

function LOGD { param([string]$Message) Write-Log "DEG" $Message }
function LOGE { param([string]$Message) Write-Log "ERR" $Message }
function LOGI { param([string]$Message) Write-Log "INF" $Message }

# Check admin privileges
function Test-Administrator {
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($user)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Administrator)) {
    LOGE "ERROR: You must run this script as Administrator!"
    exit 1
}

# Get Windows version information
$osInfo = Get-WmiObject Win32_OperatingSystem
$osVersion = [Version]$osInfo.Version

# Check minimum Windows version (Windows 10/Server 2016 or higher)
$minVersion = [Version]"10.0.0.0"
if ($osVersion -lt $minVersion) {
    LOGE "This script requires Windows 10/Server 2016 or higher"
    exit 1
}

# Variables
$scriptPath = $PSScriptRoot
$configPath = Join-Path $scriptPath "config"
$logFolder = $env:XUI_LOG_FOLDER
if (-not $logFolder) {
    $logFolder = "C:\ProgramData\3x-ui\logs"
}

$iplimitLogPath = Join-Path $logFolder "3xipl.log"
$iplimitBannedLogPath = Join-Path $logFolder "3xipl-banned.log"

# Function to confirm actions
function Confirm-Action {
    param (
        [string]$Prompt,
        [string]$DefaultChoice = "N"
    )
    
    if ($DefaultChoice) {
        $prompt = "$Prompt [Default $DefaultChoice]: "
    } else {
        $prompt = "$Prompt [Y/N]: "
    }
    
    $choice = Read-Host -Prompt $prompt
    if (-not $choice) { $choice = $DefaultChoice }
    
    return $choice -in @('Y','y')
}

# Function to restart the service
function Restart-XUI {
    Stop-Service x-ui -ErrorAction SilentlyContinue
    Start-Service x-ui -ErrorAction SilentlyContinue
    
    Start-Sleep -Seconds 2
    
    $service = Get-Service x-ui -ErrorAction SilentlyContinue
    if ($service.Status -eq 'Running') {
        LOGI "x-ui restarted successfully"
    } else {
        LOGE "Failed to restart x-ui"
    }
}

# Main menu function
function Show-Menu {
    Write-Host @"
    
╔────────────────────────────────────────────────╗
│   $($colors.Green)3X-UI Panel Management Script$($colors.Plain)                │
│   $($colors.Green)0.$($colors.Plain) Exit Script                               │
│────────────────────────────────────────────────│
│   $($colors.Green)1.$($colors.Plain) Install                                   │
│   $($colors.Green)2.$($colors.Plain) Update                                    │
│   $($colors.Green)3.$($colors.Plain) Legacy Version                            │
│   $($colors.Green)4.$($colors.Plain) Uninstall                                 │
│────────────────────────────────────────────────│
│   $($colors.Green)5.$($colors.Plain) Reset Username & Password                 │
│   $($colors.Green)6.$($colors.Plain) Reset Web Base Path                       │
│   $($colors.Green)7.$($colors.Plain) Reset Settings                            │
│   $($colors.Green)8.$($colors.Plain) Change Port                               │
│   $($colors.Green)9.$($colors.Plain) View Current Settings                     │
│────────────────────────────────────────────────│
│   $($colors.Green)10.$($colors.Plain) Start                                    │
│   $($colors.Green)11.$($colors.Plain) Stop                                     │
│   $($colors.Green)12.$($colors.Plain) Restart                                  │
│   $($colors.Green)13.$($colors.Plain) Check Status                             │
│   $($colors.Green)14.$($colors.Plain) View Logs                                │
│────────────────────────────────────────────────│
│   $($colors.Green)15.$($colors.Plain) Enable Autostart                         │
│   $($colors.Green)16.$($colors.Plain) Disable Autostart                        │
╚────────────────────────────────────────────────╝
"@

    Show-Status
    
    $choice = Read-Host "`nPlease enter your selection [0-16]"
    
    switch ($choice) {
        "0" { exit 0 }
        "1" { Install-XUI }
        "2" { Update-XUI }
        "3" { Install-LegacyVersion }
        "4" { Uninstall-XUI }
        "5" { Reset-XUICredentials }
        "6" { Reset-WebBasePath }
        "7" { Reset-Settings }
        "8" { Set-XUIPort }
        "9" { Show-Settings }
        "10" { Start-XUI }
        "11" { Stop-XUI }
        "12" { Restart-XUI }
        "13" { Show-Status }
        "14" { Show-Logs }
        "15" { Enable-XUIAutostart }
        "16" { Disable-XUIAutostart }
        default { LOGE "Please enter the correct number [0-16]" }
    }
    
    Show-Menu
}

# Show service status
function Show-Status {
    $service = Get-Service x-ui -ErrorAction SilentlyContinue
    if ($service) {
        Write-Host "Panel Status: " -NoNewline
        if ($service.Status -eq 'Running') {
            Write-Host "$($colors.Green)Running$($colors.Plain)"
        } else {
            Write-Host "$($colors.Red)Stopped$($colors.Plain)"
        }
    } else {
        Write-Host "Panel Status: $($colors.Red)Not Installed$($colors.Plain)"
    }
}

# Main entry point
if ($args.Count -gt 0) {
    switch ($args[0]) {
        "start" { Start-XUI }
        "stop" { Stop-XUI }
        "restart" { Restart-XUI }
        "status" { Show-Status }
        "enable" { Enable-XUIAutostart }
        "disable" { Disable-XUIAutostart }
        "log" { Show-Logs -LogType "service" }
        "banlog" { Show-Logs -LogType "ban" }
        "update" { Update-XUI }
        "install" { Install-XUI }
        "uninstall" { Uninstall-XUI }
        "settings" { Show-Settings }
        "legacy" { Install-LegacyVersion }
        default { Show-Usage }
    }
} else {
    Show-Menu
}

# Add these functions after the existing logging functions

function Install-BasePackages {
    # Check if winget is available
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Host "Installing required packages using winget..."
        winget install -e --id Git.Git
        winget install -e --id Microsoft.PowerShell
        winget install -e --id Nssm.Nssm  # Add NSSM installation
    } else {
        Write-Host -ForegroundColor $colors.Red "Please install winget package manager or install Git and NSSM manually"
        exit 1
    }
}

function Install-XUI {
    param([string]$version = "")
    
    $installDir = "C:\Program Files\x-ui"
    
    # Create install directory if it doesn't exist
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force
    }
    
    # Get latest version if not specified
    if (-not $version) {
        $version = (Invoke-RestMethod "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest").tag_name
    }
    
    $arch = Get-CPUArchitecture
    $downloadUrl = "https://github.com/MHSanaei/3x-ui/releases/download/${version}/x-ui-windows-${arch}.zip"
    $zipFile = Join-Path $installDir "x-ui.zip"
    
    # Download and extract
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipFile
    Expand-Archive -Path $zipFile -DestinationPath $installDir -Force
    Remove-Item $zipFile
    
    # Create service script
    $servicePath = Join-Path $installDir "x-ui-service.ps1"
    @"
# Service script content
`$process = Start-Process -FilePath "C:\Program Files\x-ui\x-ui.exe" -NoNewWindow -PassThru
while (-not `$process.HasExited) {
    Start-Sleep -Seconds 1
}
"@ | Set-Content $servicePath
    
    # Register service using nssm
    if (Get-Command nssm -ErrorAction SilentlyContinue) {
        nssm install x-ui powershell
        nssm set x-ui Application "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
        nssm set x-ui AppParameters "-ExecutionPolicy Bypass -NoProfile -File `"$servicePath`""
        nssm start x-ui
    } else {
        Write-Host -ForegroundColor $colors.Red "NSSM is not installed. Please install it to run x-ui as a service."
    }
    
    Configure-AfterInstall
    Write-Host -ForegroundColor $colors.Green "x-ui ${version} installation finished"
}

function Update-XUI {
    if (-not (Confirm-Action "This will reinstall the latest version. Continue?" "Y")) {
        LOGE "Cancelled"
        return
    }
    
    Stop-Service x-ui
    Install-XUI
    Start-Service x-ui
    LOGI "Update complete, service restarted"
}

function Uninstall-XUI {
    if (-not (Confirm-Action "Are you sure you want to uninstall x-ui?" "N")) {
        return
    }
    
    Stop-Service x-ui
    nssm remove x-ui confirm
    Remove-Item "C:\Program Files\x-ui" -Recurse -Force
    LOGI "x-ui uninstalled successfully"
}

function Reset-XUICredentials {
    if (-not (Confirm-Action "Reset username and password?" "N")) {
        return
    }
    
    $config = Get-Content "C:\Program Files\x-ui\config.json" | ConvertFrom-Json
    
    $newUsername = Read-Host "Enter new username (leave blank for random)"
    if (-not $newUsername) {
        $newUsername = -join ((65..90) + (97..122) | Get-Random -Count 8 | ForEach-Object {[char]$_})
    }
    
    $newPassword = Read-Host "Enter new password (leave blank for random)"
    if (-not $newPassword) {
        $newPassword = -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 12 | ForEach-Object {[char]$_})
    }
    
    $config.username = $newUsername
    $config.password = $newPassword
    
    $config | ConvertTo-Json | Set-Content "C:\Program Files\x-ui\config.json"
    
    LOGI "Credentials updated successfully"
    Write-Host "New username: $newUsername"
    Write-Host "New password: $newPassword"
    
    Restart-XUI
}

function Show-Usage {
    Write-Host @"
Usage: x-ui [command]
Commands:
    start       - Start x-ui panel
    stop        - Stop x-ui panel
    restart     - Restart x-ui panel
    status      - Check x-ui status
    enable      - Enable x-ui on system startup
    disable     - Disable x-ui on system startup
    log         - Show x-ui logs
    banlog      - Show ban logs
    update      - Update x-ui panel
    install     - Install x-ui panel
    uninstall   - Uninstall x-ui panel
    settings    - Show current settings
    legacy      - Install specific version
"@
}

function Get-CPUArchitecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    if ($arch -eq "AMD64") { return "amd64" }
    elseif ($arch -eq "X86") { return "386" }
    elseif ($arch -eq "ARM64") { return "arm64" }
    else { return "amd64" } # Default to amd64
}

function Start-XUI {
    $service = Get-Service x-ui -ErrorAction SilentlyContinue
    if ($service.Status -eq 'Running') {
        LOGI "Panel is already running"
        return
    }
    Start-Service x-ui
    Start-Sleep -Seconds 2
    if ((Get-Service x-ui).Status -eq 'Running') {
        LOGI "x-ui Started Successfully"
    } else {
        LOGE "Failed to start x-ui"
    }
}

function Stop-XUI {
    Stop-Service x-ui -ErrorAction SilentlyContinue
    if ((Get-Service x-ui).Status -eq 'Stopped') {
        LOGI "x-ui Stopped Successfully"
    } else {
        LOGE "Failed to stop x-ui"
    }
}

function Enable-XUIAutostart {
    Set-Service x-ui -StartupType Automatic
    LOGI "x-ui has been enabled to start on system boot"
}

function Disable-XUIAutostart {
    Set-Service x-ui -StartupType Manual
    LOGI "x-ui has been disabled from starting on system boot"
}

function Show-Logs {
    param (
        [string]$LogType = "service"
    )
    
    switch ($LogType) {
        "service" {
            $logPath = Join-Path $logFolder "x-ui.log"
            if (Test-Path $logPath) {
                Get-Content $logPath -Tail 50
            } else {
                LOGE "Log file not found"
            }
        }
        "ban" {
            if (Test-Path $iplimitBannedLogPath) {
                Get-Content $iplimitBannedLogPath -Tail 50
            } else {
                LOGE "Ban log file not found"
            }
        }
    }
}

function Show-Settings {
    $configFile = Join-Path $installDir "config.json"
    if (Test-Path $configFile) {
        $config = Get-Content $configFile | ConvertFrom-Json
        $serverIP = (Invoke-WebRequest -Uri "https://api.ipify.org" -UseBasicParsing).Content
        
        Write-Host "Current Settings:"
        Write-Host "----------------------------------------"
        Write-Host "Username: $($config.username)"
        Write-Host "Port: $($config.port)"
        Write-Host "Base Path: $($config.webBasePath)"
        Write-Host "Panel URL: http://${serverIP}:$($config.port)$($config.webBasePath)"
        Write-Host "----------------------------------------"
    } else {
        LOGE "Configuration file not found"
    }
}

function Set-XUIPort {
    $port = Read-Host "Enter new port number [1-65535]"
    if ($port -match '^\d+$' -and [int]$port -ge 1 -and [int]$port -le 65535) {
        $configFile = Join-Path $installDir "config.json"
        $config = Get-Content $configFile | ConvertFrom-Json
        $config.port = [int]$port
        $config | ConvertTo-Json | Set-Content $configFile
        LOGI "Port updated successfully. Restarting service..."
        Restart-XUI
    } else {
        LOGE "Invalid port number"
    }
}

function Install-LegacyVersion {
    $version = Read-Host "Enter the panel version (like 2.4.0)"
    if (-not $version) {
        LOGE "Version cannot be empty"
        return
    }
    Install-XUI -version "v$version"
}
