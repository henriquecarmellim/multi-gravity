# Instala o mgy (multi-gravity) no Windows.
#
#   .\install.ps1                  instala em %LOCALAPPDATA%\Programs\mgy e põe no PATH do usuário
#   .\install.ps1 -Uninstall       remove o binário (as contas guardadas ficam)
#
# Dentro de um clone, compila com Go. Fora dele, baixa a última release do GitHub.
param(
    [string]$Prefix = (Join-Path $env:LOCALAPPDATA "Programs\mgy"),
    [switch]$Uninstall
)
$ErrorActionPreference = "Stop"
$repo = "henriquecarmellim/multi-gravity"

function Say($m)  { Write-Host "> $m" -ForegroundColor Red }
function Warn($m) { Write-Host "! $m" -ForegroundColor Yellow }

if ($Uninstall) {
    Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $Prefix "mgy.exe")
    Say "mgy removido de $Prefix (as contas em %LOCALAPPDATA%\mgy continuam)"
    exit 0
}

New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("mgy-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$out = Join-Path $tmp "mgy.exe"

try {
    $here = $PSScriptRoot
    $goMod = if ($here) { Join-Path $here "go.mod" } else { $null }
    if ($goMod -and (Test-Path $goMod) -and (Select-String -Path $goMod -Pattern "multi-gravity" -Quiet)) {
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "preciso do Go para compilar (https://go.dev/dl)" }
        Say "Compilando o mgy"
        Push-Location $here
        try { go build -trimpath -ldflags "-s -w" -o $out . } finally { Pop-Location }
        if ($LASTEXITCODE -ne 0) { throw "a compilação falhou" }
    } else {
        $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
        $url = "https://github.com/$repo/releases/latest/download/mgy_windows_$arch.zip"
        Say "Baixando $url"
        Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp "mgy.zip")
        Expand-Archive -Path (Join-Path $tmp "mgy.zip") -DestinationPath $tmp -Force
    }
    Copy-Item -Force $out (Join-Path $Prefix "mgy.exe")
    Say "Instalado em $Prefix\mgy.exe"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $Prefix) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$Prefix", "User")
        Say "Adicionado ao PATH do usuário (abra um novo terminal)"
    }
    if (-not (Get-Command agy -ErrorAction SilentlyContinue)) {
        Warn "o agy (Antigravity CLI) não foi encontrado no PATH — instale-o ou defina MGY_AGY"
    }
    Say "Pronto! Abra um novo terminal e rode:  mgy"
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tmp
}
