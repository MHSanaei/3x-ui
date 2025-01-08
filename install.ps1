# Color definitions for PowerShell
$colors = @{
    Red = [ConsoleColor]::Red
    Green = [ConsoleColor]::Green
    Blue = [ConsoleColor]::Blue
    Yellow = [ConsoleColor]::Yellow
}

$curDir = Get-Location

# Check admin privileges
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host -ForegroundColor $colors.Red "Fatal error: Please run this script as Administrator"
    exit 1
}

# Get system architecture
function Get-CPUArchitecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "x86" { return "386" }
        "ARM64" { return "arm64" }
        default {
            Write-Host -ForegroundColor $colors.Red "Unsupported CPU architecture!"
            exit 1
        }
    }
}

$arch = Get-CPUArchitecture
Write-Host "arch: $arch"

# Random string generator
function New-RandomString {
    param([int]$length)
    return -join ((65..90) + (97..122) + (48..57) | Get-Random -Count $length | ForEach-Object { [char]$_ })
}

function Install-BasePackages {
    # Check if winget is available
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Host "Installing required packages using winget..."
        winget install -e --id Git.Git
        winget install -e --id Microsoft.PowerShell
    } else {
        Write-Host -ForegroundColor $colors.Red "Please install winget package manager or install Git manually"
        exit 1
    }
}

function Configure-AfterInstall {
    $configPath = "C:\Program Files\x-ui\config.json"
    if (Test-Path $configPath) {
        $config = Get-Content $configPath | ConvertFrom-Json
        
        $existingUsername = $config.username
        $existingPassword = $config.password
        $existingWebBasePath = $config.webBasePath
        $existingPort = $config.port
        
        # Get public IP
        $serverIP = (Invoke-WebRequest -Uri "https://api.ipify.org" -UseBasicParsing).Content

        if ($existingWebBasePath.Length -lt 4) {
            if ($existingUsername -eq "admin" -and $existingPassword -eq "admin") {
                $configWebBasePath = New-RandomString -length 15
                $configUsername = New-RandomString -length 10
                $configPassword = New-RandomString -length 10
                
                $response = Read-Host "Would you like to customize the Panel Port settings? (If not, a random port will be applied) [y/n]"
                if ($response -eq "y" -or $response -eq "Y") {
                    $configPort = Read-Host "Please set up the panel port"
                } else {
                    $configPort = Get-Random -Minimum 1024 -Maximum 62000
                    Write-Host -ForegroundColor $colors.Yellow "Generated random port: $configPort"
                }

                # Update config file
                $config.username = $configUsername
                $config.password = $configPassword
                $config.port = $configPort
                $config.webBasePath = $configWebBasePath
                $config | ConvertTo-Json | Set-Content $configPath

                Write-Host "New configuration:"
                Write-Host "###############################################"
                Write-Host -ForegroundColor $colors.Green "Username: $configUsername"
                Write-Host -ForegroundColor $colors.Green "Password: $configPassword"
                Write-Host -ForegroundColor $colors.Green "Port: $configPort"
                Write-Host -ForegroundColor $colors.Green "WebBasePath: $configWebBasePath"
                Write-Host -ForegroundColor $colors.Green "Access URL: http://${serverIP}:${configPort}/${configWebBasePath}"
                Write-Host "###############################################"
            }
        }
    }
}

function Install-XUI {
    param([string]$version)

    # Create installation directory
    $installDir = "C:\Program Files\x-ui"
    New-Item -ItemType Directory -Force -Path $installDir

    if (-not $version) {
        # Get latest release
        $apiUrl = "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest"
        $release = Invoke-RestMethod -Uri $apiUrl
        $version = $release.tag_name
        
        Write-Host "Got x-ui latest version: $version, beginning the installation..."
    }

    $downloadUrl = "https://github.com/MHSanaei/3x-ui/releases/download/${version}/x-ui-windows-${arch}.zip"
    $zipFile = Join-Path $installDir "x-ui.zip"

    # Download and extract
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipFile
    Expand-Archive -Path $zipFile -DestinationPath $installDir -Force
    Remove-Item $zipFile

    # Create service
    $servicePath = Join-Path $installDir "x-ui-service.ps1"
    @"
# Service script content
`$process = Start-Process -FilePath "C:\Program Files\x-ui\x-ui.exe" -NoNewWindow -PassThru
while (-not `$process.HasExited) {
    Start-Sleep -Seconds 1
}
"@ | Set-Content $servicePath

    # Register service using nssm
    # Note: NSSM needs to be installed separately
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

function Register-XUICommand {
    # Create a directory for x-ui if it doesn't exist in Program Files
    $installDir = "C:\Program Files\x-ui"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force
    }

    # Copy x-ui.ps1 to the installation directory
    Copy-Item "$PSScriptRoot\x-ui.ps1" "$installDir\x-ui.ps1" -Force

    # Add the installation directory to PATH if not already present
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*$installDir*") {
        [Environment]::SetEnvironmentVariable(
            "Path",
            "$currentPath;$installDir",
            "Machine"
        )
    }

    # Create an alias script to run x-ui.ps1 with admin privileges
    $aliasScript = @"
@echo off
PowerShell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process PowerShell -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File \"%ProgramFiles%\x-ui\x-ui.ps1\" %*' -Verb RunAs"
"@
    
    Set-Content -Path "$installDir\x-ui.cmd" -Value $aliasScript -Force

    Write-Host "x-ui command has been registered. You can now use 'x-ui' from any terminal with admin privileges."
}

Write-Host -ForegroundColor $colors.Green "Running..."
Install-BasePackages
Install-XUI $args[0]

# After installing x-ui, register the command
Register-XUICommand