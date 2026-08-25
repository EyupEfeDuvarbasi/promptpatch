[CmdletBinding()]
param(
    [switch]$SkipCodexCheck
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

function Ensure-Go {
    $go = Find-CommandPath "go"
    if ($go) {
        return $go
    }

    Write-Step "Go bulunamadı; winget ile Go kuruluyor."
    $winget = Find-CommandPath "winget"
    if (-not $winget) {
        throw "Go ve winget bulunamadı. Go'yu https://go.dev/dl/ adresinden kurup scripti yeniden çalıştırın."
    }

    & $winget install --id GoLang.Go --exact --source winget --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) {
        throw "Go kurulumu başarısız oldu. winget çıkış kodu: $LASTEXITCODE"
    }

    Refresh-UserPath
    $go = Find-CommandPath "go"
    if (-not $go) {
        throw "Go kuruldu ancak yeni PATH mevcut PowerShell'e yüklenmedi. Yeni PowerShell açıp scripti yeniden çalıştırın."
    }
    return $go
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

function Install-PromptPatch([string]$GoPath) {
    Write-Step "PromptPatch kuruluyor."
    & $GoPath install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
    if ($LASTEXITCODE -ne 0) {
        throw "PromptPatch kurulumu başarısız oldu."
    }

    Refresh-UserPath
    $gopath = (& $GoPath env GOPATH).Trim()
    $candidate = Join-Path $gopath "bin\promptcheck.exe"
    if (Test-Path -LiteralPath $candidate) {
        return $candidate
    }

    $promptcheck = Find-CommandPath "promptcheck"
    if ($promptcheck) {
        return $promptcheck
    }

    throw "PromptPatch kuruldu ancak promptcheck.exe bulunamadı: $candidate"
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
    $go = Ensure-Go
    Ensure-Codex

    $promptcheck = Install-PromptPatch $go
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
