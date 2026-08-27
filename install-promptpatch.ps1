[CmdletBinding()]
param(
    [switch]$SkipCodexCheck,
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host "[PromptPatch] $Message" -ForegroundColor Cyan
}

function Write-Failure([string]$Message) {
    Write-Host "[PromptPatch] HATA: $Message" -ForegroundColor Red
}

function Refresh-UserPath {
    $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($machinePath -and $userPath) {
        $env:Path = "$machinePath;$userPath"
    }
}

function Find-CommandPath([string]$Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    return $null
}

function Ensure-Codex {
    if ($SkipCodexCheck) {
        return
    }

    $codex = Find-CommandPath "codex"
    if (-not $codex) {
        throw "Codex CLI bulunamadı. Önce Codex CLI'yi kurun, codex --version komutunu doğrulayın ve scripti yeniden çalıştırın."
    }

    & $codex --version
    if ($LASTEXITCODE -ne 0) {
        throw "Codex bulundu ancak çalıştırılamadı: $codex"
    }
}

function Install-PromptPatch {
    Write-Step "Prompter release paketi kuruluyor."
    $release = $Version
    if ($release -eq "latest") { $release = (Invoke-RestMethod "https://api.github.com/repos/EyupEfeDuvarbasi/promptpatch/releases/latest").tag_name }
    $name = "promptcheck_${release}_windows_amd64"
    $temp = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $temp | Out-Null
    try {
        $archive = Join-Path $temp "$name.zip"
        Invoke-WebRequest "https://github.com/EyupEfeDuvarbasi/promptpatch/releases/download/$release/$name.zip" -OutFile $archive
        $checksum = (Invoke-WebRequest "https://github.com/EyupEfeDuvarbasi/promptpatch/releases/download/$release/$name.zip.sha256").Content.Split(' ')[0].Trim()
        if ((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $checksum.ToLowerInvariant()) { throw "Release checksum doğrulanamadı." }
        Expand-Archive $archive -DestinationPath $temp
        $bin = Join-Path $env:LOCALAPPDATA "Prompter\bin"
        New-Item -ItemType Directory -Force -Path $bin | Out-Null
        $target = Join-Path $bin "prompter.exe"
        Copy-Item (Join-Path $temp "$name\prompter.exe") $target -Force
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if (($userPath -split ';') -notcontains $bin) { [Environment]::SetEnvironmentVariable("Path", (($userPath.TrimEnd(';') + ';' + $bin).Trim(';')), "User") }
        Refresh-UserPath
        return $target
    } finally { Remove-Item $temp -Recurse -Force -ErrorAction SilentlyContinue }
}

function Configure-Codex([string]$PromptcheckPath) {
    $newline = [Environment]::NewLine
    $setupInput = "1" + $newline
    Write-Step "Codex entegrasyonu yapılandırılıyor."

    $setupInput | & $PromptcheckPath setup-codex
    if ($LASTEXITCODE -ne 0) {
        throw "Codex entegrasyonu kurulamadı."
    }
}

try {
    Write-Step "Ön koşullar kontrol ediliyor."
    Ensure-Codex

    $promptcheck = Install-PromptPatch
    & $promptcheck --help | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "promptcheck --help çalışmadı."
    }

    Configure-Codex $promptcheck

    Write-Host ""
    Write-Host "Kurulum tamamlandı." -ForegroundColor Green
    Write-Host ""
    Write-Host "Yeni bir PowerShell penceresi açın, sonra:" -ForegroundColor Yellow
    Write-Host "  codex" -ForegroundColor White
    Write-Host ""
    Write-Host "Codex içinde prompt yazıp Ctrl-G tuşlarına basın." -ForegroundColor Yellow
}
catch {
    Write-Failure $_.Exception.Message
    exit 1
}
